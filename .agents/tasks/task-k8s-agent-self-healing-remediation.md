# Task Specification: `k8s-agent` Local-First K8s Self-Healing & Active Node Remediation

> **Created**: 2026-10-16
> **Status**: ✅ COMPLETED (`ab9fa22c`, `2428e6ae`, `43201876`)
> **Branch**: `fix/comprehensive-audit`
> **Target Footprint**: Strictly $\le 50\text{ MB}$ RAM per `k8s-agent` daemon (zero external binaries/sidecars required; native Go + OS syscalls + guarded whitelist execution).

---

## 1. Architectural Problem Statement

`k8s-agent` (`cmd/agent/`) runs locally on every Linux/Kubernetes node (`:9100`), whereas the Control Plane (`cmd/standalone` / `cmd/server`) runs remotely. Currently:
1. `k8s-agent` is strictly **Read-Only** (`/health`, `/metrics`, `/logs*`), with zero capability to diagnose or remediate local Kubernetes node faults (`kubelet`/`containerd` hang, `DiskPressure`, `InodePressure`, `PLEG` deadlock, `D-state`/`Zombie` processes, stale CNI IPAM locks).
2. On the Control Plane:
   - [`internal/usecase/sre/node_service_rescue.go:268-271`](file:///d:/project/k8sseflhost/internal/usecase/sre/node_service_rescue.go#L268-L271) (`defaultServiceExecutor.RestartServices`) is a hardcoded stub returning `fmt.Errorf("direct node agent communication unavailable for services: ...")`.
   - [`internal/usecase/runtimeguard/resilience_manager.go:61-84`](file:///d:/project/k8sseflhost/internal/usecase/runtimeguard/resilience_manager.go#L61-L84) (`nopStorageProbe`, `nopRuntimeOperator`) inspects the Control Plane's own filesystem instead of querying the remote `k8s-agent` nodes.
   - [`internal/usecase/k8sresilience/resilience_manager.go:60-65`](file:///d:/project/k8sseflhost/internal/usecase/k8sresilience/resilience_manager.go#L60-L65) (`NodeCRIClient`) lacks a real `k8s-agent` RPC client implementation for checking Kubelet PLEG health, probing `containerd.sock`, and restarting stalled `containerd`/`kubelet` on the target node.

---

## 2. Phase-by-Phase Execution Plan

### Phase 1: Local K8s Health & Runtime Diagnostics in `k8s-agent` (`cmd/agent/k8s_health.go`)
Equip `k8s-agent` to inspect the local node's Kubernetes runtime health directly on the host OS:
1. **Kubelet & CRI Socket Probing**:
   - Probe local Kubelet healthz (`http://127.0.0.1:10248/healthz` or TCP `127.0.0.1:10250`) and parse Kubelet PLEG relist latency if available.
   - Probe local container runtime sockets (`/run/containerd/containerd.sock`, `/var/run/docker.sock`, `/var/run/crio/crio.sock`) via `net.DialTimeout("unix", sockPath, 1*time.Second)` to detect deadlocked CRI daemons (`Soft Death`) even when the Linux kernel is alive.
2. **D-State (Uninterruptible Sleep) & Zombie Process Scanner**:
   - Scan `/proc/[0-9]*/stat` locally in `k8s-agent` (reusing zero-allocation buffer reading) to count `Z` (zombie) and `D` (uninterruptible disk sleep) processes and identify stuck container shims (`containerd-shim-runc-v2`).
3. **Node Health Diagnostic Endpoint (`GET /k8s/health`)**:
   - Expose authenticated `GET /k8s/health` on `k8s-agent` (`:9100`) returning structured `NodeK8sHealthResponse`:
     - `kubelet_healthy`, `kubelet_latency_ms`, `pleg_healthy`
     - `containerd_healthy`, `docker_healthy`, `cri_socket_path`
     - `disk_pressure`, `inode_pressure`, `memory_pressure`, `pid_pressure`
     - `d_state_processes`, `zombie_count`, `conntrack_usage_pct`
     - `detected_anomalies` (e.g., `["KUBELET_UNRESPONSIVE", "DISK_PRESSURE_HIGH"]`)

---

### Phase 2: Safe, Allowlisted Local Remediation Engine in `k8s-agent` (`cmd/agent/remediator.go`)
Add an allowlisted, zero-shell-injection remediation executor (`NodeRemediator`) inside `k8s-agent` (`cmd/agent/remediator.go`) with strict command whitelisting (no arbitrary shell execution!), rate-limiting/cooldown, and audit logging:
1. **Action 1 — `restart_service` (Soft-Death Service Rescue)**:
   - Strictly allowlisted service names ONLY: `kubelet`, `containerd`, `docker`, `crio`, `k8s-agent`.
   - Executes `systemctl restart <service>` (or `sudo -n systemctl restart <service>` / user service fallback) with a 15s timeout, then re-probes the service/socket health and returns before/after status.
2. **Action 2 — `disk_pressure_cleanup` (Disk & Inode Pressure Relief)**:
   - Truncates rotated/compressed old log archives in `/var/log` (`*.gz`, `*.1`, `journalctl --vacuum-size=100M`).
   - Triggers `logengine` immediate retention/quota prune (`engineSrc.Writer()`).
   - Prunes exited containers and dangling images via Docker API (`cli.ContainersPrune`, `cli.ImagesPrune`) or `crictl rmi --prune` / `crictl rm -a` (only exited containers), returning exact `bytes_freed` and `inodes_freed`.
3. **Action 3 — `cni_ipam_cleanup` (Stale CNI Lock & Network Recovery)**:
   - Scans `/var/lib/cni/networks/` for stale IP allocation files whose container netns/sandbox no longer exists, freeing leaked Pod IPs when Pods get stuck in `ContainerCreating` (`FailedCreatePodSandBox`).
4. **Action 4 — `reap_zombies` (Stuck Terminating Pod / Orphan Shim Cleanup)**:
   - Identifies orphaned `containerd-shim` processes in `D`/`Z` state whose parent/container is dead and sends `SIGKILL` to release stuck mounts/ports.
5. **Remediation Endpoint (`POST /remediate`)**:
   - Expose authenticated `POST /remediate` on `k8s-agent` (`:9100`) accepting:
     ```json
     {
       "action": "restart_service | disk_pressure_cleanup | cni_ipam_cleanup | reap_zombies",
       "target": "kubelet | containerd | docker",
       "reason": "Triggered by Control Plane SRE / Autonomous Watchdog"
     }
     ```
   - Returns structured `RemediationActionResult` (`success`, `action`, `target`, `bytes_freed`, `duration_ms`, `details`, `timestamp`).

---

### Phase 3: Autonomous Local Self-Healing Watchdog Loop in `k8s-agent` (`cmd/agent/watchdog.go`)
Because a node can lose network connectivity to the Control Plane or K8s API Server during an incident, `k8s-agent` must also be able to **self-heal locally** (configurable via `--auto-heal=true`, enabled by default with safe cooldowns):
1. **Autonomous Disk/Inode Relief**:
   - Every 60s, if local root/var disk usage $\ge 88\%$ or inode usage $\ge 85\%$, automatically run `disk_pressure_cleanup` (with a 15-minute cooldown) and record a structured `[SELF-HEAL]` log entry into `logengine`.
2. **Autonomous Runtime Watchdog (`containerd` / `kubelet`)**:
   - If `k8s-agent` detects this node is a Kubernetes node (`/etc/kubernetes/kubelet.conf` or `kubelet.service` exists) and `containerd.sock` or `kubelet` fails 3 consecutive health checks (over 45s), automatically trigger `restart_service` (max 3 attempts per 30-minute window — circuit breaker to prevent boot loops) and log the event into `logengine`.
3. **In-Stream Log Anomaly Detector**:
   - Hook into `cmd/agent` log ingestion to detect critical K8s kernel/runtime signatures in real time:
     - `Out of memory: Killed process`
     - `PLEG is not healthy`
     - `nf_conntrack: table full, dropping packet`
     - `FailedCreatePodSandBox`

---

### Phase 4: Control Plane <-> `k8s-agent` Integration (`internal/infrastructure/agent/remediation_client.go` & SRE Wiring)
Connect the Control Plane's existing SRE modules directly to `k8s-agent`'s new `/k8s/health` and `/remediate` endpoints:
1. **`AgentRemediationClient` (`internal/infrastructure/agent/remediation_client.go`)**:
   - Implements:
     - `GetNodeK8sHealth(ctx, hostEndpoint, authToken) (*NodeK8sHealthResponse, error)`
     - `ExecuteRemediation(ctx, hostEndpoint, authToken string, req RemediationRequest) (*RemediationActionResult, error)`
   - Resolves node name / IP to the registered `compute_hosts` endpoint (`http://10.10.10.60:9100`, etc.) and authenticates with Bearer `AGENT_AUTH_TOKEN` / `host.Labels["auth_token"]`.
2. **Replace Stub in `internal/usecase/sre/node_service_rescue.go:265-271`**:
   - Wire `AgentServiceExecutor` & `AgentHostProber` into `NodeServiceRescueManager` (`internal/usecase/sre/node_service_rescue.go`) so `ExecuteServiceRescue` calls `POST http://<node>:9100/remediate` (`action: "restart_service"`) and `CheckHostConnectivity`/`CheckKubeletHealth` calls `GET http://<node>:9100/k8s/health`!
3. **Wire `K8sResilience` (`RestartContainerd`, `GetPLEGHealth`) & `RuntimeGuard` (`PruneDanglingImages`, `GetNodeStorageHealth`)**:
   - Connect `POST /api/v1/k8s/nodes/{node}/restart-containerd` and `GET /api/v1/k8s/nodes/{node}/pleg-health` in `cmd/standalone/standalone_wiring.go` to `AgentRemediationClient`!
   - Expose `POST /api/v1/hosts/{id}/remediate` (and `/api/v1/k8s/nodes/{node}/remediate`) for 1-click remediation from the UI (`Hosts` / `Incidents` / `SRE Runbooks`).

---

### Phase 5: Verification & Live Deployment Readiness
1. **Unit & Integration Tests**:
   - `go test -v ./cmd/agent/... ./internal/infrastructure/agent/... ./internal/usecase/sre/... ./internal/adapter/http/...`
   - Verify allowlist security (rejects any non-whitelisted service name or shell metacharacters with `400 Bad Request`).
   - Verify memory footprint remains strictly $\le 50\text{ MB}$ RAM.
2. **Cross-Compile & Live Verification**:
   - Rebuild `standalone.exe` and `k8s-agent` (`GOOS=linux GOARCH=amd64`).
   - Verify via `chrome-devtools-mcp` on the UI.

---

## 3. Phase 8: Multi-App Container & Systemd Remediation, Active Watchdog Triggers & UI Controls (COMPLETED)

### 8.1 `k8s-agent` Multi-App & Container Self-Healing (`cmd/agent/`) — COMPLETED
1. **Expanded Systemd Application Allowlist + Configurable Services (`cmd/agent/remediator.go`, `cmd/agent/remediator_containers.go`)**:
   - Default allowlist: `kubelet`, `containerd`, `docker`, `crio`, `postgresql`, `postgres`, `nginx`, `redis`, `redis-server`, `haproxy`, `keepalived`, `etcd`, `patroni`, `traefik`, `coredns`, `nats`, `minio`, `clickhouse-server`.
   - Configurable custom services via `--allowed-services` / `AGENT_ALLOWED_SERVICES` (strictly validated against `^[a-zA-Z0-9._@-]+$`, blocking OS-critical services like `sshd`, `networking`, `systemd-networkd`, `dbus` with case-insensitive normalization).
2. **Docker Container Health Diagnostics & `restart_container` Action (`cmd/agent/k8s_health.go`, `cmd/agent/remediator_containers.go`)**:
   - Inspect local containers via Docker CLI to report `unhealthy_containers` (`id`, `name`, `image`, `state`, `status`, `restart_count`) in `GET /k8s/health`.
   - Implement `restart_container` action (`target: "<container_name_or_id>"`) with 3-per-30m circuit breaker and audit trail.
3. **Active Watchdog Anomaly & Container/Zombie/CNI Auto-Healing (`cmd/agent/watchdog.go`, `cmd/agent/main.go`)**:
   - Auto-trigger `reap_zombies` when `ZombieCount > 0` (10m cooldown).
   - Auto-trigger `cni_ipam_cleanup` when `CNI_SANDBOX_FAILURE` log anomaly is detected (10m cooldown).
   - Auto-trigger `restart_container` for unhealthy/crashed containers in `UnhealthyContainers` (circuit-breaker guarded).
   - Wire `DetectLogAnomaly` into live Pod log ingestion via `watchdog.HookPodTailer(podTailer)` -> `RecordLogLine`.
4. **`deploy-agent.sh` Token Sync & CRLF Hardening**:
   - Strip `\r` when reading `.env` and preserve existing remote `~/.k8s-agent/agent.env` token when local `.env` has no explicit token.

### 8.2 Control Plane History & Container Remediation Proxy (`internal/`) — COMPLETED
- Added `restart_container` action support and `GET /api/v1/hosts/{id}/remediate/history` (plus `GET /api/v1/k8s/nodes/{node}/remediate/history`) in `internal/infrastructure/agent/remediation_client.go` and `internal/adapter/http/host_remediation_handler.go`.

### 8.3 Frontend UI Diagnostics & 1-Click Remediation (`frontend-vue/`) — COMPLETED
- Added `getK8sHealth`, `remediate`, and `getRemediationHistory` to `frontend-vue/src/api/hosts.ts`.
- Created `HostRemediationPanel.vue` (271 lines) inside `HostDetailDrawer.vue` (186 lines) with live K8s/App health badges, unhealthy container restart buttons, 1-click remediation actions (`disk_pressure_cleanup`, `cni_ipam_cleanup`, `reap_zombies`, `restart_service`), and self-healing history log.

