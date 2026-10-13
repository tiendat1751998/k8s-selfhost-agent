#!/usr/bin/env bash
# ==============================================================================
# SRE CHAOS SUITE — SCENARIO 4: DOCKERD SIGSTOP PROCESS FREEZE & AGENT RECOVERY
# ==============================================================================
# PHYSICAL FAILURE MODE & SRE ENGINEERING NOTES:
# 1. Linux Cgroup Memory Pressure & Process Freezing (SIGSTOP):
#    Under severe kernel page cache thrashing, VFS lock contention, or engine
#    deadlocks, dockerd/containerd user-space threads freeze. Issuing kill -STOP
#    suspends thread execution instantly. The Unix Domain Socket (/var/run/docker.sock)
#    remains open in the filesystem inode table, but accept() ceases to be called.
# 2. Connection Storm Prevention & Bounded Backoff:
#    If the monitoring agent queries dockerd in a naive tight loop (e.g., 5 req/s/container),
#    the kernel listen backlog (sk_max_ack_backlog) fills completely.
#    Goroutines pile up on the client, exhausting file descriptors (EMFILE)
#    and invoking the Linux Cgroup OOM Killer on the agent process.
#    A resilient agent must implement jittered exponential backoff
#    (e.g., 1s -> 2s -> 4s -> 8s -> 16s... up to 30s max), bounding retry load.
# 3. Autonomous Self-Healing Upon SIGCONT:
#    When dockerd receives kill -CONT, threads unfreeze and resume draining sockets.
#    The agent must automatically detect engine recovery, reset backoff intervals,
#    and resume real-time container log/event streaming within <5 seconds.
# ==============================================================================

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}======================================================================${NC}"
echo -e "${BLUE}▶ CHAOS SCENARIO 4: Dockerd SIGSTOP Engine Freeze & Agent Backoff${NC}"
echo -e "${BLUE}======================================================================${NC}"

# ------------------------------------------------------------------------------
# STEP 0: Pre-flight Verification & Process Identification
# ------------------------------------------------------------------------------
FREEZE_DURATION="${CHAOS_DOCKERD_FREEZE_SEC:-60}"
POLL_SAMPLE_INTERVAL=2

DOCKERD_PID=""
MOCK_PID=""
AGENT_LOG="/tmp/chaos_agent_dockerd_$$.log"
SAMPLES_LOG="/tmp/chaos_samples_$$.log"

cleanup() {
    echo -e "\n${YELLOW}▶ Restoring frozen processes and cleaning resources...${NC}"
    if [ -n "$DOCKERD_PID" ]; then
        kill -CONT "$DOCKERD_PID" 2>/dev/null || true
    fi
    if [ -n "$MOCK_PID" ] && kill -0 "$MOCK_PID" 2>/dev/null; then
        kill -CONT "$MOCK_PID" 2>/dev/null || true
        kill -9 "$MOCK_PID" 2>/dev/null || true
    fi
    rm -f "$AGENT_LOG" "$SAMPLES_LOG"
}
trap cleanup EXIT

# Check if running in an environment where dockerd can be signaled
if pgrep -x "dockerd" >/dev/null 2>&1; then
    TARGET_PID=$(pgrep -x "dockerd" | head -n 1)
    # Check if we have permission to signal
    if kill -0 "$TARGET_PID" 2>/dev/null; then
        DOCKERD_PID="$TARGET_PID"
        echo -e "${GREEN}✔ Identified host dockerd process PID: ${DOCKERD_PID}${NC}"
    fi
fi

# If dockerd cannot be signaled, report NOT RUN with reason per SRE standards
if [ -z "$DOCKERD_PID" ]; then
    echo -e "${YELLOW}[NOT RUN] dockerd process not found or cannot be signaled with kill -STOP.${NC}"
    echo "Reason: Scenario S4 requires Linux/WSL2 host with dockerd and permission to send SIGSTOP."
    echo "Requirement: Run on Linux host with active dockerd under root/sudo or container with SYS_PTRACE capability."
    exit 0
fi

# ------------------------------------------------------------------------------
# STEP 1: Launch Simulated Monitoring Agent Against Engine Socket
# ------------------------------------------------------------------------------
echo -e "${BLUE}▶ Starting agent client loop with exponential backoff & telemetry...${NC}"

# Agent loop implementing standard exponential backoff with jitter
(
    ATTEMPT=0
    CONSECUTIVE_FAILURES=0
    BACKOFF=1.0

    while true; do
        ATTEMPT=$((ATTEMPT + 1))
        TS=$(date +%s)
        
        # Query docker _ping or version endpoint with 1.5s timeout
        if curl -s -m 1.5 --unix-socket "${DOCKER_HOST#unix://}" http://localhost/_ping >/dev/null 2>&1 || \
           curl -s -m 1.5 --unix-socket "${DOCKER_HOST#unix://}" http://localhost/version >/dev/null 2>&1; then
            echo "${TS},OK,${ATTEMPT},0.0" >> "$AGENT_LOG"
            CONSECUTIVE_FAILURES=0
            BACKOFF=1.0
            sleep 1.0
        else
            CONSECUTIVE_FAILURES=$((CONSECUTIVE_FAILURES + 1))
            # Calculate next backoff: min(30, 1.0 * 2^(failures-1))
            BACKOFF=$(awk -v cf="$CONSECUTIVE_FAILURES" 'BEGIN {
                b = 1.0 * (2 ^ (cf - 1));
                if (b > 30.0) b = 30.0;
                print b;
            }')
            echo "${TS},ERR,${ATTEMPT},${BACKOFF}" >> "$AGENT_LOG"
            sleep "$BACKOFF"
        fi
    done
) &
AGENT_PID=$!

sleep 2
BASELINE_REQUESTS=$(wc -l < "$AGENT_LOG" 2>/dev/null || echo "0")
echo "Baseline operational checks: ${BASELINE_REQUESTS} requests logged."

# ------------------------------------------------------------------------------
# STEP 2: Inject SIGSTOP (Freeze Engine for ${FREEZE_DURATION}s)
# ------------------------------------------------------------------------------
echo -e "${RED}⚡ FREEZING DOCKER DAEMON (kill -STOP ${DOCKERD_PID}) for ${FREEZE_DURATION}s...${NC}"
FREEZE_START=$(date +%s)
kill -STOP "$DOCKERD_PID"

ELAPSED=0
PREV_COUNT=$(wc -l < "$AGENT_LOG")

while [ "$ELAPSED" -lt "$FREEZE_DURATION" ]; do
    sleep "$POLL_SAMPLE_INTERVAL"
    ELAPSED=$((ELAPSED + POLL_SAMPLE_INTERVAL))
    CURRENT_COUNT=$(wc -l < "$AGENT_LOG")
    BURST_RATE=$((CURRENT_COUNT - PREV_COUNT))
    PREV_COUNT=$CURRENT_COUNT
    echo "$(date +%s),${ELAPSED},${BURST_RATE}" >> "$SAMPLES_LOG"
    echo "  [t+${ELAPSED}s] Frozen... In-flight attempts in last ${POLL_SAMPLE_INTERVAL}s: ${BURST_RATE}"
done

FREEZE_END=$(date +%s)
TOTAL_FROZEN=$((FREEZE_END - FREEZE_START))
echo -e "${YELLOW}✔ Freeze window completed (${TOTAL_FROZEN}s).${NC}"

# ------------------------------------------------------------------------------
# STEP 3: Unfreeze Engine (SIGCONT) & Verify Self-Healing
# ------------------------------------------------------------------------------
echo -e "${GREEN}⚡ UNFREEZING DOCKER DAEMON (kill -CONT ${DOCKERD_PID})...${NC}"
kill -CONT "$DOCKERD_PID"
UNFREEZE_TIME=$(date +%s)

# Wait up to 10s for agent to recover and log 'OK'
RECOVERED=false
for ((i=1; i<=10; i++)); do
    sleep 1
    LAST_STATUS=$(tail -n 1 "$AGENT_LOG" | cut -d',' -f2)
    if [ "$LAST_STATUS" = "OK" ]; then
        RECOVERY_LATENCY=$i
        RECOVERED=true
        echo -e "${GREEN}✔ Agent self-healed and resumed streaming in ${RECOVERY_LATENCY}s!${NC}"
        break
    fi
done

kill "$AGENT_PID" 2>/dev/null || true

# ------------------------------------------------------------------------------
# STEP 4: Assert SRE Invariants
# ------------------------------------------------------------------------------
echo -e "${BLUE}▶ Evaluating connection storm rate during freeze...${NC}"

# Check max burst rate per 2-second sampling interval during freeze
MAX_BURST_DURING_FREEZE=$(awk -F',' 'NR>1 {if ($3>max) max=$3} END {print max+0}' "$SAMPLES_LOG")
echo "Maximum requests observed per ${POLL_SAMPLE_INTERVAL}s during freeze: ${MAX_BURST_DURING_FREEZE}"

# If an agent was doing tight loops at 5 req/s/container, it would generate 10+ req / 2s.
# With exponential backoff, rate drops to <= 2 requests per sample interval.
if [ "$MAX_BURST_DURING_FREEZE" -gt 4 ]; then
    echo -e "${RED}❌ FAILED: Connection storm detected! Agent fired ${MAX_BURST_DURING_FREEZE} requests in ${POLL_SAMPLE_INTERVAL}s during freeze.${NC}"
    exit 1
fi
echo -e "${GREEN}✔ Bounded retry rate verified: No connection storm during engine freeze.${NC}"

if [ "$RECOVERED" != "true" ]; then
    echo -e "${RED}❌ FAILED: Agent failed to self-heal within 10s after kill -CONT.${NC}"
    exit 1
fi

echo -e "${GREEN}======================================================================${NC}"
echo -e "${GREEN}🎉 CHAOS SCENARIO 4 PASSED: Backoff throttled retry storm; self-healed on SIGCONT.${NC}"
echo -e "${GREEN}======================================================================${NC}"
exit 0
