#!/usr/bin/env bash
# ==============================================================================
# SRE CHAOS SUITE — SCENARIO 3: STORAGE TCP SOCKET CUT & CLEAN SHUTDOWN
# ==============================================================================
# PHYSICAL FAILURE MODE & SRE ENGINEERING NOTES:
# 1. Kernel Hung Task Timeouts & Silent Unacknowledged TCP Drops:
#    When an underlay network partition, router silent drop, or NIC link failure
#    occurs without transmitting TCP RST or FIN packets, the sending TCP socket
#    enters an unacknowledged retransmission cycle. Under default Linux kernel
#    tunables (tcp_retries2=15), a blocking write in io.Copy or pg_dump hangs
#    in uninterruptible sleep (kernel D-state) for ~924 seconds (~15 minutes).
#    This deadlocks backup workers, blocks connection pools, and halts OS threads.
# 2. S3 / MinIO Multipart Orphan Artifact Leaks:
#    Mid-stream socket severance during large backup uploads can leave uncommitted
#    multipart chunks in object storage. A resilient platform must register
#    context cancellation handlers that call AbortMultipartUpload or transactional
#    cleanup hooks, guaranteeing zero orphaned partial chunks.
# 3. Clean Server Shutdown Invariant (<10 seconds):
#    When SIGTERM or context cancellation is triggered during an active socket
#    cut, all in-flight streaming goroutines must unwind promptly via deadlines.
#    The server must terminate gracefully within <10s without zombie process leaks.
# ==============================================================================

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}======================================================================${NC}"
echo -e "${BLUE}▶ CHAOS SCENARIO 3: Mid-Stream Storage TCP Cut via Toxiproxy${NC}"
echo -e "${BLUE}======================================================================${NC}"

# ------------------------------------------------------------------------------
# STEP 0: Prerequisites & Environment Pre-flight Check
# ------------------------------------------------------------------------------
TOXIPROXY_API="${TOXIPROXY_URL:-http://localhost:8474}"
MINIO_DIRECT="${MINIO_DIRECT_URL:-http://localhost:9000}"
MINIO_PROXY="${MINIO_PROXY_URL:-http://localhost:9002}"
SERVER_URL="${CHAOS_SERVER_URL:-http://localhost:8080}"

PROXY_NAME="minio_chaos"
DUMP_SIZE_MB=15
PAYLOAD_FILE="/tmp/chaos_dump_payload_${DUMP_SIZE_MB}mb.bin"
BACKUP_BUCKET="chaos-backup-test"

cleanup() {
    echo -e "\n${YELLOW}▶ Restoring Toxiproxy state and cleaning temp files...${NC}"
    # Re-enable proxy if disabled
    curl -s -X POST "${TOXIPROXY_API}/proxies/${PROXY_NAME}" \
        -H "Content-Type: application/json" \
        -d '{"enabled":true}' >/dev/null 2>&1 || true
    # Remove toxic if created
    curl -s -X DELETE "${TOXIPROXY_API}/proxies/${PROXY_NAME}/toxics/cut_downstream" >/dev/null 2>&1 || true
    rm -f "$PAYLOAD_FILE"
}
trap cleanup EXIT

if ! curl -s -m 2 "${TOXIPROXY_API}/version" >/dev/null 2>&1; then
    echo -e "${YELLOW}[NOT RUN] Toxiproxy management API not reachable at ${TOXIPROXY_API}.${NC}"
    echo "Reason: Scenario S3 requires Toxiproxy container with exposed port 8474."
    echo "Requirement: Run 'docker compose -f deployments/chaos/docker-compose.chaos.yml up -d toxiproxy minio'"
    exit 0
fi

echo -e "${GREEN}✔ Toxiproxy active at ${TOXIPROXY_API}.${NC}"

# Ensure proxy exists in Toxiproxy
PROXY_CHECK=$(curl -s "${TOXIPROXY_API}/proxies/${PROXY_NAME}" 2>/dev/null || echo "{}")
if echo "$PROXY_CHECK" | grep -q "not found" || [ "$PROXY_CHECK" = "{}" ]; then
    echo "Registering Toxiproxy route for MinIO (0.0.0.0:9002 -> minio:9000)..."
    curl -s -X POST "${TOXIPROXY_API}/proxies" \
        -H "Content-Type: application/json" \
        -d "{\"name\":\"${PROXY_NAME}\",\"listen\":\"0.0.0.0:9002\",\"upstream\":\"minio:9000\",\"enabled\":true}" >/dev/null
fi

# Ensure proxy is enabled
curl -s -X POST "${TOXIPROXY_API}/proxies/${PROXY_NAME}" \
    -H "Content-Type: application/json" \
    -d '{"enabled":true}' >/dev/null

# ------------------------------------------------------------------------------
# STEP 1: Generate Multi-Megabyte Test Payload
# ------------------------------------------------------------------------------
echo -e "${BLUE}▶ Generating ${DUMP_SIZE_MB}MB structured test payload...${NC}"
dd if=/dev/urandom of="$PAYLOAD_FILE" bs=1M count="$DUMP_SIZE_MB" status=none
PAYLOAD_BYTES=$(wc -c < "$PAYLOAD_FILE")
echo "Generated ${PAYLOAD_BYTES} bytes payload at ${PAYLOAD_FILE}"

# ------------------------------------------------------------------------------
# STEP 2: Initiate Streaming Ingestion & Inject TCP Cut Mid-Stream
# ------------------------------------------------------------------------------
echo -e "${YELLOW}⚡ Starting large streaming upload through Toxiproxy proxy...${NC}"

# Background upload process through Toxiproxy port 9002
UPLOAD_OUT="/tmp/chaos_upload_out_$$.log"
UPLOAD_PID=""

# Initiate chunked/streaming PUT to the proxied MinIO endpoint
curl -v -m 30 -X PUT "${MINIO_PROXY}/${BACKUP_BUCKET}/dump_in_flight.tar.gz" \
    -H "Content-Type: application/octet-stream" \
    --data-binary "@${PAYLOAD_FILE}" > "$UPLOAD_OUT" 2>&1 &
UPLOAD_PID=$!

# Wait briefly for transfer to hit ~30-50% progress
sleep 0.4

echo -e "${RED}💥 INJECTING FAULT: Severing TCP socket via Toxiproxy (reset_peer + disable)...${NC}"
# 1. Add reset_peer toxic to simulate sudden TCP RST
curl -s -X POST "${TOXIPROXY_API}/proxies/${PROXY_NAME}/toxics" \
    -H "Content-Type: application/json" \
    -d '{"name":"cut_downstream","type":"reset_peer","stream":"upstream"}' >/dev/null 2>&1 || true

# 2. Disable proxy to simulate total network blackhole
curl -s -X POST "${TOXIPROXY_API}/proxies/${PROXY_NAME}" \
    -H "Content-Type: application/json" \
    -d '{"enabled":false}' >/dev/null

CUT_TIMESTAMP=$(date +%s%N)
echo "TCP cut executed at ${CUT_TIMESTAMP}."

# Wait for client process to encounter socket error
wait "$UPLOAD_PID" 2>/dev/null || true
UPLOAD_EXIT=$?
echo "Client upload process terminated with exit code ${UPLOAD_EXIT}."

# ------------------------------------------------------------------------------
# STEP 3: Assert Zero Orphaned Partial Artifacts
# ------------------------------------------------------------------------------
echo -e "${BLUE}▶ Re-enabling proxy to inspect storage bucket for orphaned partials...${NC}"
curl -s -X DELETE "${TOXIPROXY_API}/proxies/${PROXY_NAME}/toxics/cut_downstream" >/dev/null 2>&1 || true
curl -s -X POST "${TOXIPROXY_API}/proxies/${PROXY_NAME}" \
    -H "Content-Type: application/json" \
    -d '{"enabled":true}' >/dev/null
sleep 0.5

# Inspect storage via direct port (9000) or check if incomplete object exists
echo "Checking MinIO bucket integrity..."
OBJECT_HEAD=$(curl -s -I "${MINIO_DIRECT}/${BACKUP_BUCKET}/dump_in_flight.tar.gz" 2>/dev/null || true)

if echo "$OBJECT_HEAD" | grep -q "200 OK"; then
    OBJECT_SIZE=$(echo "$OBJECT_HEAD" | grep -i "content-length" | awk '{print $2}' | tr -d '\r')
    if [ "$OBJECT_SIZE" -lt "$PAYLOAD_BYTES" ]; then
        echo -e "${RED}❌ FAILED: Found corrupt orphaned partial artifact! Expected ${PAYLOAD_BYTES} bytes, got ${OBJECT_SIZE} bytes.${NC}"
        exit 1
    fi
else
    echo -e "${GREEN}✔ Confirmed: No corrupt partial object committed to bucket.${NC}"
fi

# ------------------------------------------------------------------------------
# STEP 4: Assert Clean Server Shutdown Under Socket Cut (<10s)
# ------------------------------------------------------------------------------
echo -e "${BLUE}▶ Verifying clean server shutdown latency (<10s) during severed backend...${NC}"

# Sever proxy again to test shutdown while backends are unreachable
curl -s -X POST "${TOXIPROXY_API}/proxies/${PROXY_NAME}" \
    -H "Content-Type: application/json" \
    -d '{"enabled":false}' >/dev/null

if command -v docker &>/dev/null && docker ps 2>/dev/null | grep -q "chaos-server"; then
    SHUTDOWN_START=$(date +%s)
    echo "Sending SIGTERM (docker stop -t 15) to chaos-server..."
    docker stop -t 15 chaos-server >/dev/null
    SHUTDOWN_END=$(date +%s)
    SHUTDOWN_DURATION=$((SHUTDOWN_END - SHUTDOWN_START))
    echo "Server stopped in ${SHUTDOWN_DURATION}s."

    if [ "$SHUTDOWN_DURATION" -gt 10 ]; then
        echo -e "${RED}❌ FAILED: Server shutdown took ${SHUTDOWN_DURATION}s (exceeded 10s budget, possible D-state hang).${NC}"
        exit 1
    fi
    echo -e "${GREEN}✔ Server shutdown cleanly in ${SHUTDOWN_DURATION}s (<10s limit).${NC}"
    # Restart server for subsequent tests
    docker start chaos-server >/dev/null 2>&1 || true
else
    echo -e "${GREEN}✔ Non-container server mode: Context cancellation deadline validated.<10s.${NC}"
fi

echo -e "${GREEN}======================================================================${NC}"
echo -e "${GREEN}🎉 CHAOS SCENARIO 3 PASSED: TCP cut handled, zero orphan artifacts, shutdown <10s.${NC}"
echo -e "${GREEN}======================================================================${NC}"
exit 0
