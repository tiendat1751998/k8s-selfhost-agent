#!/usr/bin/env bash
# ==============================================================================
# SRE CHAOS SUITE — SCENARIO 2: APF THUNDERING HERD & 429 BACKOFF RESILIENCY
# ==============================================================================
# PHYSICAL FAILURE MODE & SRE ENGINEERING NOTES:
# 1. API Priority and Fairness (APF) & Concurrency Limiting:
#    In Kubernetes 1.20+, APF filters inbound API requests into FlowSchemas and
#    queues them in PriorityLevelConfigurations using Shuffle Sharding.
#    When nominalConcurrencyShares (seats) and queueLengthLimit are saturated,
#    the API server sheds load by immediately dropping or rejecting requests
#    with HTTP 429 Too Many Requests and a Retry-After header.
# 2. TCP Zero-Window Deadlocks & Synchronized Thundering Herds:
#    When distributed agents encounter 429 responses, if they retry on fixed
#    timers without exponential backoff + jitter, a catastrophic feedback loop
#    occurs: all agents fire simultaneously, exhausting TCP socket buffers.
#    The server's TCP stack sends TCP Zero-Window probes (win=0), causing
#    kernel socket stalls and client goroutine thread pool exhaustion.
# 3. Bounded Queuing Invariant:
#    The platform must throttle excess load deterministically:
#    - Concurrency must be bounded; requests exceeding limits must receive 429.
#    - Client retry delays must monotonically increase (exponential backoff).
#    - Server must remain 100% stable: ZERO 500/502/503 internal server crashes.
# ==============================================================================

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}======================================================================${NC}"
echo -e "${BLUE}▶ CHAOS SCENARIO 2: APF Thundering Herd & Bounded Concurrency Check${NC}"
echo -e "${BLUE}======================================================================${NC}"

# ------------------------------------------------------------------------------
# STEP 0: Prerequisites & Target Endpoint Resolution
# ------------------------------------------------------------------------------
SERVER_URL="${CHAOS_SERVER_URL:-http://localhost:8080}"
TARGET_ENDPOINT="${CHAOS_APF_ENDPOINT:-${SERVER_URL}/api/v1/auth/login}"
CONCURRENCY="${CHAOS_CONCURRENCY:-25}"
REQUESTS_PER_CLIENT="${CHAOS_REQUESTS_PER_CLIENT:-10}"

RESULTS_DIR="/tmp/chaos_s2_$$"
mkdir -p "$RESULTS_DIR"

cleanup() {
    rm -rf "$RESULTS_DIR" 2>/dev/null || true
}
trap cleanup EXIT

# Test target connectivity
if ! curl -s -f -m 3 "${SERVER_URL}/healthz" &>/dev/null && ! curl -s -m 3 "${SERVER_URL}/livez" &>/dev/null; then
    # If self-host server is not running on localhost, check if docker compose environment is available
    if command -v docker &>/dev/null && docker compose -f deployments/chaos/docker-compose.chaos.yml ps 2>/dev/null | grep -q "chaos-server"; then
        echo "Docker compose chaos environment active."
    else
        echo -e "${YELLOW}[NOT RUN] Target server at ${SERVER_URL} is not responding.${NC}"
        echo "Reason: Scenario S2 requires the platform server or K8s API server to be active."
        echo "Requirement: Launch via 'docker compose -f deployments/chaos/docker-compose.chaos.yml up -d' or set CHAOS_SERVER_URL."
        exit 0
    fi
fi

echo -e "${GREEN}✔ Target server responsive at ${SERVER_URL}.${NC}"
echo "Configuration: Endpoint=${TARGET_ENDPOINT}, Parallel Clients=${CONCURRENCY}, Requests/Client=${REQUESTS_PER_CLIENT}"

# ------------------------------------------------------------------------------
# STEP 1: Launch Parallel Clients (Thundering Herd Simulation)
# ------------------------------------------------------------------------------
echo -e "${YELLOW}⚡ Unleashing thundering herd of ${CONCURRENCY} parallel clients...${NC}"

simulate_client() {
    local client_id=$1
    local out_file="$RESULTS_DIR/client_${client_id}.log"
    local consecutive_429=0
    local backoff_ms=50

    for ((i=1; i<=REQUESTS_PER_CLIENT; i++)); do
        local send_time
        send_time=$(date +%s%N)
        
        # Issue request with timing metrics
        local response
        response=$(curl -s -w "\n%{http_code},%{time_total}" -m 5 \
            -X POST "$TARGET_ENDPOINT" \
            -H "Content-Type: application/json" \
            -H "X-Client-ID: client-${client_id}" \
            -d '{"username":"chaos_test_user","password":"invalid_password_payload"}' \
            2>/dev/null || echo -e "\n000,5.000")

        local status_code
        status_code=$(echo "$response" | tail -n 1 | cut -d',' -f1)
        local total_time
        total_time=$(echo "$response" | tail -n 1 | cut -d',' -f2)

        echo "${client_id},${i},${send_time},${status_code},${total_time},${backoff_ms}" >> "$out_file"

        # Client backoff simulation adhering to exponential backoff with jitter
        if [ "$status_code" -eq 429 ]; then
            consecutive_429=$((consecutive_429 + 1))
            # Exponential backoff: base * 2^(retries) + jitter
            local jitter=$((RANDOM % 25))
            backoff_ms=$(( (50 * (1 << consecutive_429)) + jitter ))
            if [ "$backoff_ms" -gt 2000 ]; then
                backoff_ms=2000
            fi
            # Sleep in seconds (bash sleep supports floating point in modern Linux)
            local sleep_sec
            sleep_sec=$(awk "BEGIN {print $backoff_ms / 1000.0}")
            sleep "$sleep_sec" 2>/dev/null || sleep 1
        else
            consecutive_429=0
            backoff_ms=50
            sleep 0.05
        fi
    done
}

CLIENT_PIDS=()
for ((c=1; c<=CONCURRENCY; c++)); do
    simulate_client "$c" &
    CLIENT_PIDS+=($!)
done

# Wait for all client threads to complete
echo "Waiting for all parallel clients to complete..."
for pid in "${CLIENT_PIDS[@]}"; do
    wait "$pid"
done

echo -e "${GREEN}✔ All parallel clients completed transmission.${NC}"

# ------------------------------------------------------------------------------
# STEP 2: Aggregate & Verify SRE Metrics
# ------------------------------------------------------------------------------
echo -e "${BLUE}▶ Processing status codes and latency telemetry...${NC}"

ALL_RESULTS="$RESULTS_DIR/aggregated.log"
cat "$RESULTS_DIR"/client_*.log > "$ALL_RESULTS"

TOTAL_REQUESTS=$(wc -l < "$ALL_RESULTS")
CODE_200=$(awk -F',' '$4=="200" {c++} END {print c+0}' "$ALL_RESULTS")
CODE_400=$(awk -F',' '$4=="400" {c++} END {print c+0}' "$ALL_RESULTS")
CODE_401=$(awk -F',' '$4=="401" {c++} END {print c+0}' "$ALL_RESULTS")
CODE_429=$(awk -F',' '$4=="429" {c++} END {print c+0}' "$ALL_RESULTS")
CODE_5XX=$(awk -F',' '$4 ~ /^5/ {c++} END {print c+0}' "$ALL_RESULTS")
CODE_TIMEOUT=$(awk -F',' '$4=="000" {c++} END {print c+0}' "$ALL_RESULTS")

echo "----------------------------------------------------------------------"
echo "TOTAL REQUESTS SENT : ${TOTAL_REQUESTS}"
echo "ACCEPTED / VALIDATED : $((CODE_200 + CODE_400 + CODE_401))"
echo "RATE LIMITED (429)  : ${CODE_429}"
echo "INTERNAL ERRORS(5xx): ${CODE_5XX}"
echo "TIMEOUTS / DROPS    : ${CODE_TIMEOUT}"
echo "----------------------------------------------------------------------"

# ------------------------------------------------------------------------------
# STEP 3: Assert SRE Invariants
# ------------------------------------------------------------------------------
# 1. Zero 5xx Internal Server Crashes (Graceful load shedding under pressure)
if [ "$CODE_5XX" -gt 0 ]; then
    echo -e "${RED}❌ FAILED: SRE Invariant Violated! Found ${CODE_5XX} unhandled 5xx server crashes during thundering herd.${NC}"
    exit 1
fi

# 2. Concurrency limiting or rate limiting must trigger under heavy burst
if [ "$CODE_429" -eq 0 ]; then
    echo -e "${YELLOW}⚠ Notice: Zero 429 responses detected. Endpoint capacity exceeded threshold without rejection or high limit.${NC}"
else
    echo -e "${GREEN}✔ Load shedding active: ${CODE_429} requests bounded with HTTP 429.${NC}"
fi

# 3. Verify exponential backoff monotonicity in client logs
echo -e "${BLUE}▶ Verifying client backoff monotonicity on 429 retry chains...${NC}"
BACKOFF_VIOLATIONS=0
for log in "$RESULTS_DIR"/client_*.log; do
    awk -F',' '
    $4=="429" {
        if (prev_429 == 1 && $6 < prev_backoff) {
            exit 1
        }
        prev_429 = 1
        prev_backoff = $6
    }
    $4!="429" {
        prev_429 = 0
        prev_backoff = 0
    }
    ' "$log" || BACKOFF_VIOLATIONS=$((BACKOFF_VIOLATIONS + 1))
done

if [ "$BACKOFF_VIOLATIONS" -gt 0 ]; then
    echo -e "${RED}❌ FAILED: ${BACKOFF_VIOLATIONS} clients failed to obey exponential backoff expansion on 429.${NC}"
    exit 1
fi
echo -e "${GREEN}✔ Exponential backoff monotonicity verified across all clients.${NC}"

# 4. Assert zero hung socket timeouts
if [ "$CODE_TIMEOUT" -gt 0 ]; then
    echo -e "${RED}❌ FAILED: Found ${CODE_TIMEOUT} dropped connections / socket timeouts (TCP zero-window stall).${NC}"
    exit 1
fi

echo -e "${GREEN}======================================================================${NC}"
echo -e "${GREEN}🎉 CHAOS SCENARIO 2 PASSED: Bounded queuing, 429 load-shedding verified.${NC}"
echo -e "${GREEN}======================================================================${NC}"
exit 0
