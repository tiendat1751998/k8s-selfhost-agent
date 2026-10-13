#!/usr/bin/env bash
# ==============================================================================
# SRE CHAOS SUITE — SCENARIO 1: NODE DRAIN EVICTION API & PDB ENFORCEMENT
# ==============================================================================
# PHYSICAL FAILURE MODE & SRE ENGINEERING NOTES:
# 1. etcd Split-Brain & Asymmetric Network Latency:
#    In multi-master K8s clusters, asymmetric cross-rack packet latency can
#    delay etcd Raft heartbeats beyond ElectionTimeout (e.g. >1000ms). During
#    partitioned states, concurrent node drains can lead to race conditions in
#    the PodDisruptionBudget (PDB) controller status subresource if lockless.
# 2. PDB Starvation Invariant:
#    With Deployment replicas=2 and PDB minAvailable=1, evicting pods via
#    CoreV1().Pods(ns).EvictV1 must reject any concurrent eviction with HTTP 429
#    (Too Many Requests) when only 1 ready replica remains. The drain process
#    MUST obey 429 backoff and terminate ONLY when all evictable pods on the
#    cordoned node have gracefully terminated without violating minAvailable.
# 3. Kernel OOM & Cascading Pod Termination:
#    Eviction must respect terminationGracePeriodSeconds to prevent premature
#    SIGKILL from destroying in-flight connections or corrupting state.
# ==============================================================================

set -euo pipefail

# Visual formatting
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}======================================================================${NC}"
echo -e "${BLUE}▶ CHAOS SCENARIO 1: Node Drain Eviction with PDB (minAvailable=1)${NC}"
echo -e "${BLUE}======================================================================${NC}"

# ------------------------------------------------------------------------------
# STEP 0: Pre-flight Verification (Environment & Tooling)
# ------------------------------------------------------------------------------
if ! command -v kubectl &>/dev/null; then
    echo -e "${YELLOW}[NOT RUN] kubectl command not found in PATH.${NC}"
    echo "Reason: Scenario S1 requires Kubernetes CLI and access to kind/k3d or live cluster."
    echo "Requirement: Install kubectl and ensure kind/k3d cluster context is active."
    exit 0
fi

if ! kubectl cluster-info &>/dev/null; then
    echo -e "${YELLOW}[NOT RUN] Kubernetes cluster not reachable via kubectl context.${NC}"
    echo "Reason: No responsive Kubernetes control plane at current kubeconfig target."
    echo "Requirement: Start a local cluster (e.g. 'kind create cluster --config deployments/chaos/kind-chaos.yaml') and retry."
    exit 0
fi

# Ensure at least 2 nodes exist for realistic drain scheduling
NODE_COUNT=$(kubectl get nodes --no-headers | wc -l)
if [ "$NODE_COUNT" -lt 2 ]; then
    echo -e "${YELLOW}[NOT RUN] Cluster has only $NODE_COUNT node(s). Minimum 2 nodes required for drain eviction.${NC}"
    echo "Reason: Multi-node topology needed to migrate pods without unschedulable deadlocks."
    exit 0
fi

# Target worker node for drain
TARGET_NODE=$(kubectl get nodes --no-headers -l '!node-role.kubernetes.io/control-plane' | head -n 1 | awk '{print $1}')
if [ -z "$TARGET_NODE" ]; then
    TARGET_NODE=$(kubectl get nodes --no-headers | tail -n 1 | awk '{print $1}')
fi

NAMESPACE="chaos-pdb-drain-$RANDOM"
WATCH_LOG="/tmp/s1_pdb_watch_${RANDOM}.log"
WATCHER_PID=""

cleanup() {
    echo -e "\n${YELLOW}▶ Cleaning up chaos resources in namespace ${NAMESPACE}...${NC}"
    if [ -n "$WATCHER_PID" ] && kill -0 "$WATCHER_PID" 2>/dev/null; then
        kill "$WATCHER_PID" 2>/dev/null || true
    fi
    rm -f "$WATCH_LOG"
    kubectl uncordon "$TARGET_NODE" 2>/dev/null || true
    kubectl delete namespace "$NAMESPACE" --ignore-not-found=true --timeout=30s 2>/dev/null || true
    echo -e "${GREEN}✔ Cleanup completed.${NC}"
}
trap cleanup EXIT

echo -e "${GREEN}✔ Kubernetes cluster responsive. Target worker node: ${TARGET_NODE}${NC}"
kubectl create namespace "$NAMESPACE"

# ------------------------------------------------------------------------------
# STEP 1: Deploy Workload with PodDisruptionBudget (minAvailable=1)
# ------------------------------------------------------------------------------
echo -e "${BLUE}▶ Deploying 2-replica workload with PDB minAvailable=1...${NC}"

cat <<EOF | kubectl apply -n "$NAMESPACE" -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: drain-victim
  namespace: ${NAMESPACE}
  labels:
    app: drain-victim
spec:
  replicas: 2
  selector:
    matchLabels:
      app: drain-victim
  template:
    metadata:
      labels:
        app: drain-victim
    spec:
      terminationGracePeriodSeconds: 10
      affinity:
        podAntiAffinity:
          preferredDuringSchedulingIgnoredDuringExecution:
            - weight: 100
              podAffinityTerm:
                labelSelector:
                  matchExpressions:
                    - key: app
                      operator: In
                      values:
                        - drain-victim
                topologyKey: kubernetes.io/hostname
      containers:
        - name: app
          image: mirror.gcr.io/library/nginx:alpine
          ports:
            - containerPort: 80
          readinessProbe:
            httpGet:
              path: /
              port: 80
            initialDelaySeconds: 1
            periodSeconds: 2
          resources:
            requests:
              cpu: 10m
              memory: 16Mi
            limits:
              cpu: 50m
              memory: 32Mi
---
apiVersion: policy/v1
kind: PodDisruptionBudget
metadata:
  name: drain-victim-pdb
  namespace: ${NAMESPACE}
spec:
  minAvailable: 1
  selector:
    matchLabels:
      app: drain-victim
EOF

# Wait for 2/2 replicas to be ready
echo "Waiting for drain-victim deployment rollout (2/2 ready)..."
kubectl rollout status deployment/drain-victim -n "$NAMESPACE" --timeout=90s

READY_INITIAL=$(kubectl get deployment drain-victim -n "$NAMESPACE" -o jsonpath='{.status.readyReplicas}')
if [ "$READY_INITIAL" -ne 2 ]; then
    echo -e "${RED}❌ Failed: Expected 2 ready replicas, got ${READY_INITIAL}${NC}"
    exit 1
fi
echo -e "${GREEN}✔ Workload ready: 2/2 replicas running across cluster.${NC}"

# ------------------------------------------------------------------------------
# STEP 2: Background High-Frequency Invariant Watcher (Sampling every 100ms)
# ------------------------------------------------------------------------------
echo -e "${BLUE}▶ Launching high-frequency ready replica invariant probe...${NC}"
(
    while true; do
        READY=$(kubectl get deployment drain-victim -n "$NAMESPACE" -o jsonpath='{.status.readyReplicas}' 2>/dev/null || echo "0")
        TIMESTAMP=$(date +%s%N)
        echo "${TIMESTAMP},${READY}" >> "$WATCH_LOG"
        sleep 0.1
    done
) &
WATCHER_PID=$!

# ------------------------------------------------------------------------------
# STEP 3: Execute Node Drain via Eviction Subresource
# ------------------------------------------------------------------------------
echo -e "${YELLOW}⚡ Initiating node drain on ${TARGET_NODE} with PDB constraints...${NC}"
DRAIN_START=$(date +%s)

# Execute drain with eviction enforcement
# --ignore-daemonsets: skip DaemonSets
# --delete-emptydir-data: proceed with pods utilizing emptyDir volumes
# --force: proceed with standalone pods
kubectl drain "$TARGET_NODE" \
    --ignore-daemonsets \
    --delete-emptydir-data \
    --force \
    --timeout=120s

DRAIN_END=$(date +%s)
DRAIN_DURATION=$((DRAIN_END - DRAIN_START))
echo -e "${GREEN}✔ Node ${TARGET_NODE} drained in ${DRAIN_DURATION}s.${NC}"

# Stop watcher
kill "$WATCHER_PID" 2>/dev/null || true
WATCHER_PID=""

# ------------------------------------------------------------------------------
# STEP 4: Assert SRE Invariants
# ------------------------------------------------------------------------------
echo -e "${BLUE}▶ Analyzing chaos telemetry for replica starvation (<1 ready)...${NC}"

MIN_READY=999
STARVATION_EVENTS=0

while IFS=',' read -r TS READY; do
    if [ -n "$READY" ] && [[ "$READY" =~ ^[0-9]+$ ]]; then
        if [ "$READY" -lt "$MIN_READY" ]; then
            MIN_READY=$READY
        fi
        if [ "$READY" -lt 1 ]; then
            STARVATION_EVENTS=$((STARVATION_EVENTS + 1))
            echo -e "${RED}🚨 VIOLATION at ${TS}: Ready replicas fell to ${READY} (minAvailable=1 violated)!${NC}"
        fi
    fi
done < "$WATCH_LOG"

echo "Invariant summary: Min ready observed = ${MIN_READY}, Starvation events = ${STARVATION_EVENTS}"

if [ "$STARVATION_EVENTS" -gt 0 ]; then
    echo -e "${RED}❌ FAILED: Replica starvation detected during drain eviction.${NC}"
    exit 1
fi

# Assert target node has 0 evictable workload pods remaining
REMAINING_PODS=$(kubectl get pods -n "$NAMESPACE" --field-selector spec.nodeName="$TARGET_NODE" --no-headers | wc -l)
if [ "$REMAINING_PODS" -ne 0 ]; then
    echo -e "${RED}❌ FAILED: Node ${TARGET_NODE} still contains ${REMAINING_PODS} workload pods after drain.${NC}"
    exit 1
fi

# Assert final deployment state restored to 2/2 ready
FINAL_READY=$(kubectl get deployment drain-victim -n "$NAMESPACE" -o jsonpath='{.status.readyReplicas}')
if [ "$FINAL_READY" -ne 2 ]; then
    echo -e "${RED}❌ FAILED: Deployment did not stabilize at 2 ready replicas (current: ${FINAL_READY}).${NC}"
    exit 1
fi

echo -e "${GREEN}======================================================================${NC}"
echo -e "${GREEN}🎉 CHAOS SCENARIO 1 PASSED: Zero replica starvation, PDB strictly honored.${NC}"
echo -e "${GREEN}======================================================================${NC}"
exit 0
