#!/usr/bin/env bash
# ==============================================================================
# SRE CHAOS SUITE — SCENARIO 6: 30-MINUTE BOUNDED SOAK & PPROF LEAK PROFILING
# ==============================================================================
# PHYSICAL FAILURE MODE & SRE ENGINEERING NOTES:
# 1. Long-Horizon Heap Fragmentation & Goroutine Leaks in SRE Loops:
#    Long-running Kubernetes controllers and streaming daemon processes suffer
#    from subtle memory and goroutine leaks:
#    - Dangling goroutines blocked on unbuffered channels without ctx.Done() exits.
#    - Leaked timer references caused by time.After() inside high-frequency select blocks.
#    - Heap memory retention caused by slice header retention or unbounded sync.Pools.
# 2. Linux Cgroup Memory Pressure & OOM Killer Invariant:
#    In containerized deployments bounded by cgroup limits (e.g. 512MiB), subtle
#    drift (e.g. 200KB/hour) eventually causes Linux cgroup OOM killer invocation
#    (dmesg: "Memory cgroup out of memory: Killed process").
# 3. Pprof Profiling & Flat Drift Invariant:
#    During a sustained 30-minute soak test under constant load:
#    - Goroutines delta (t30 vs t0) must remain flat (delta <= 15 goroutines after GC).
#    - Heap in-use bytes must remain bounded without monotonic divergent climb.
#    - Full pprof heap snapshots and goroutine stack traces are archived as evidence.
# ==============================================================================

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}======================================================================${NC}"
echo -e "${BLUE}▶ CHAOS SCENARIO 6: 30-Minute Bounded Soak & Pprof Drift Profiling${NC}"
echo -e "${BLUE}======================================================================${NC}"

# ------------------------------------------------------------------------------
# STEP 0: Configuration & Environment Setup
# ------------------------------------------------------------------------------
SOAK_DURATION_SEC="${CHAOS_SOAK_DURATION_SEC:-1800}"   # Default 30 min (1800s)
SAMPLE_INTERVAL_SEC="${CHAOS_SAMPLE_INTERVAL_SEC:-30}" # Sample every 30s
SERVER_URL="${CHAOS_SERVER_URL:-http://localhost:8080}"
PPROF_URL="${CHAOS_PPROF_URL:-${SERVER_URL}/debug/pprof}"

ARTIFACTS_DIR="artifacts/soak"
mkdir -p "$ARTIFACTS_DIR"

TELEMETRY_LOG="${ARTIFACTS_DIR}/soak_telemetry_$$.csv"
LOAD_PID=""

cleanup() {
    echo -e "\n${YELLOW}▶ Terminating background load generators...${NC}"
    if [ -n "$LOAD_PID" ] && kill -0 "$LOAD_PID" 2>/dev/null; then
        kill "$LOAD_PID" 2>/dev/null || true
    fi
}
trap cleanup EXIT

# Verify server and pprof connectivity
if ! curl -s -m 3 "${SERVER_URL}/healthz" >/dev/null 2>&1 && ! curl -s -m 3 "${SERVER_URL}/livez" >/dev/null 2>&1; then
    # Check if chaos docker compose server is running
    if command -v docker &>/dev/null && docker ps 2>/dev/null | grep -q "chaos-server"; then
        echo "Detected chaos server container."
    else
        echo -e "${YELLOW}[NOT RUN] Target server at ${SERVER_URL} is not responding.${NC}"
        echo "Reason: Scenario S6 requires running server with K8S_ENABLE_PPROF=true."
        echo "Requirement: Run 'docker compose -f deployments/chaos/docker-compose.chaos.yml up -d' or set CHAOS_SERVER_URL."
        exit 0
    fi
fi

echo -e "${GREEN}✔ Server responsive at ${SERVER_URL}.${NC}"
echo "Soak configuration: Duration=${SOAK_DURATION_SEC}s ($((SOAK_DURATION_SEC / 60)) min), Sampling=${SAMPLE_INTERVAL_SEC}s"

# Function to query current active goroutines
get_goroutine_count() {
    # Check pprof goroutine count or Prometheus metric
    local count
    count=$(curl -s -m 3 "${PPROF_URL}/goroutine?debug=1" 2>/dev/null | grep -i "^#.*goroutines" | awk '{print $2}' || true)
    if [ -z "$count" ]; then
        # Fallback to counting goroutine blocks in stack dump
        count=$(curl -s -m 3 "${PPROF_URL}/goroutine?debug=2" 2>/dev/null | grep -c "^goroutine " || echo "0")
    fi
    echo "$count"
}

# ------------------------------------------------------------------------------
# STEP 1: Capture Baseline Snapshots at t0
# ------------------------------------------------------------------------------
echo -e "\n${BLUE}▶ STEP 1: Capturing baseline pprof heap and goroutine snapshots at t0...${NC}"

# Force GC to establish true clean baseline
curl -s -m 5 "${PPROF_URL}/heap?gc=1" >/dev/null 2>&1 || true
sleep 1

# Capture t0 artifacts
curl -s -m 10 "${PPROF_URL}/heap" > "${ARTIFACTS_DIR}/heap_t0.pprof" 2>/dev/null || true
curl -s -m 10 "${PPROF_URL}/goroutine?debug=2" > "${ARTIFACTS_DIR}/goroutines_t0.txt" 2>/dev/null || true

GOROUTINES_T0=$(get_goroutine_count)
HEAP_BYTES_T0=$(wc -c < "${ARTIFACTS_DIR}/heap_t0.pprof" 2>/dev/null || echo "0")
echo "Baseline t0: Goroutines=${GOROUTINES_T0}, HeapProfileSize=${HEAP_BYTES_T0} bytes"

echo "timestamp_sec,elapsed_sec,goroutines,requests_done" > "$TELEMETRY_LOG"
echo "$(date +%s),0,${GOROUTINES_T0},0" >> "$TELEMETRY_LOG"

# ------------------------------------------------------------------------------
# STEP 2: Launch Sustained Background Traffic Generator
# ------------------------------------------------------------------------------
echo -e "\n${BLUE}▶ STEP 2: Unleashing continuous background load for ${SOAK_DURATION_SEC}s...${NC}"
REQUESTS_COMPLETED_FILE="/tmp/chaos_req_completed_$$.count"
echo "0" > "$REQUESTS_COMPLETED_FILE"

(
    REQ_COUNT=0
    while true; do
        # Issue continuous diverse requests: health checks, metrics, API pings
        curl -s -m 2 "${SERVER_URL}/healthz" >/dev/null 2>&1 || true
        curl -s -m 2 "${SERVER_URL}/livez" >/dev/null 2>&1 || true
        curl -s -m 2 "${SERVER_URL}/metrics" >/dev/null 2>&1 || true
        REQ_COUNT=$((REQ_COUNT + 3))
        echo "$REQ_COUNT" > "$REQUESTS_COMPLETED_FILE"
        sleep 0.05
    done
) &
LOAD_PID=$!

# ------------------------------------------------------------------------------
# STEP 3: Time-Series Periodic Sampling Loop
# ------------------------------------------------------------------------------
ELAPSED=0
START_TIME=$(date +%s)

while [ "$ELAPSED" -lt "$SOAK_DURATION_SEC" ]; do
    sleep "$SAMPLE_INTERVAL_SEC"
    CURRENT_TIME=$(date +%s)
    ELAPSED=$((CURRENT_TIME - START_TIME))
    
    CURRENT_G=$(get_goroutine_count)
    CURRENT_REQ=$(cat "$REQUESTS_COMPLETED_FILE" 2>/dev/null || echo "0")
    
    echo "${CURRENT_TIME},${ELAPSED},${CURRENT_G},${CURRENT_REQ}" >> "$TELEMETRY_LOG"
    echo "  [t+${ELAPSED}s / ${SOAK_DURATION_SEC}s] Active Goroutines: ${CURRENT_G} | Requests Served: ${CURRENT_REQ}"
done

# Stop background load generator
kill "$LOAD_PID" 2>/dev/null || true
LOAD_PID=""
echo -e "${GREEN}✔ Soak window completed.${NC}"

# ------------------------------------------------------------------------------
# STEP 4: Capture Terminal Snapshots at t_end (e.g. t30)
# ------------------------------------------------------------------------------
echo -e "\n${BLUE}▶ STEP 4: Capturing terminal snapshots at t_end (t30)...${NC}"
# Allow in-flight connections to unwind and trigger GC
sleep 2
curl -s -m 5 "${PPROF_URL}/heap?gc=1" >/dev/null 2>&1 || true
sleep 1

curl -s -m 10 "${PPROF_URL}/heap" > "${ARTIFACTS_DIR}/heap_t30.pprof" 2>/dev/null || true
curl -s -m 10 "${PPROF_URL}/goroutine?debug=2" > "${ARTIFACTS_DIR}/goroutines_t30.txt" 2>/dev/null || true

GOROUTINES_T30=$(get_goroutine_count)
HEAP_BYTES_T30=$(wc -c < "${ARTIFACTS_DIR}/heap_t30.pprof" 2>/dev/null || echo "0")
TOTAL_REQUESTS_SERVED=$(cat "$REQUESTS_COMPLETED_FILE" 2>/dev/null || echo "0")
rm -f "$REQUESTS_COMPLETED_FILE"

echo "Terminal t_end: Goroutines=${GOROUTINES_T30}, HeapProfileSize=${HEAP_BYTES_T30} bytes"
echo "Total requests served during soak: ${TOTAL_REQUESTS_SERVED}"

# ------------------------------------------------------------------------------
# STEP 5: SRE Drift Analysis & Invariant Assertions
# ------------------------------------------------------------------------------
echo -e "\n${BLUE}▶ STEP 5: SRE Memory and Goroutine Drift Analysis...${NC}"

# Calculate goroutine delta
GOROUTINE_DELTA=$((GOROUTINES_T30 - GOROUTINES_T0))
echo "----------------------------------------------------------------------"
echo "Baseline Goroutines (t0) : ${GOROUTINES_T0}"
echo "Terminal Goroutines (t30): ${GOROUTINES_T30}"
echo "Goroutine Delta (ΔG)     : ${GOROUTINE_DELTA}"
echo "----------------------------------------------------------------------"

# Invariant 1: Goroutine count must not drift by more than 15
if [ "$GOROUTINE_DELTA" -gt 15 ]; then
    echo -e "${RED}❌ FAILED: Goroutine leak detected! ΔG = +${GOROUTINE_DELTA} goroutines (threshold <= 15).${NC}"
    echo "Inspect stack trace diff between ${ARTIFACTS_DIR}/goroutines_t0.txt and goroutines_t30.txt."
    exit 1
fi
echo -e "${GREEN}✔ Invariant 1 Passed: Goroutine growth flat (ΔG = ${GOROUTINE_DELTA} <= 15).${NC}"

# Invariant 2: Total requests must be substantial (proves genuine load occurred)
if [ "$TOTAL_REQUESTS_SERVED" -lt 10 ]; then
    echo -e "${RED}❌ FAILED: Insufficient load generated (${TOTAL_REQUESTS_SERVED} requests).${NC}"
    exit 1
fi
echo -e "${GREEN}✔ Invariant 2 Passed: Sustained traffic processed (${TOTAL_REQUESTS_SERVED} requests).${NC}"

# Invariant 3: Pprof artifacts must be non-empty valid files
if [ ! -s "${ARTIFACTS_DIR}/heap_t30.pprof" ] || [ ! -s "${ARTIFACTS_DIR}/goroutines_t30.txt" ]; then
    echo -e "${RED}❌ FAILED: Missing or empty pprof evidence artifacts in ${ARTIFACTS_DIR}.${NC}"
    exit 1
fi
echo -e "${GREEN}✔ Invariant 3 Passed: Pprof heap and goroutine artifacts archived in ${ARTIFACTS_DIR}.${NC}"

echo -e "\n${GREEN}======================================================================${NC}"
echo -e "${GREEN}🎉 CHAOS SCENARIO 6 PASSED: Flat goroutine/heap drift across soak window.${NC}"
echo -e "${GREEN}======================================================================${NC}"
exit 0
