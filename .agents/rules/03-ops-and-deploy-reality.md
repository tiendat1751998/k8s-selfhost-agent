# 03 — Operational & Deployment Reality

> Physical system topology, runtime ports, and canonical deployment commands.

## 1. Physical Deployment Reality (CRITICAL)
- **NO AUTO CI/CD**: Editing local Go code in `cmd/agent` or `internal/` does NOT automatically deploy or update remote Linux nodes.
- **Canonical Agent Rollout (`LES-006`)**: ALWAYS use the root deployment script [`./deploy-agent.sh`](../../deploy-agent.sh):
  - Remote Linux nodes:
    ```bash
    ./deploy-agent.sh <user>@<ip-1> <user>@<ip-2>
    ```
    (Cross-compiles `./k8s-agent` static `linux/amd64`, transfers via `/tmp`, cleans legacy part floods, preserves remote `~/.k8s-agent/agent.env`, and restarts `systemctl --user restart k8s-agent`).
  - Directly on Linux host: `./deploy-agent.sh` (local mode).
  - **Rule**: NEVER invent manual multi-line `scp`/`ssh` snippets when `./deploy-agent.sh` exists.

## 2. Runtime Ports & Services
- **Central Backend Server**: `cmd/standalone` or `cmd/server` running on `:8080`.
- **Frontend Dev UI**: `frontend-vue` (Vite) running on `:3000` (proxies `/api` & `/ws` to `:8080`).
- **Edge Node Agent**: `k8s-agent` (`cmd/agent`) running on `:9100` (`systemctl --user status k8s-agent`).
  - Embedded ClickHouse `MergeTree` columnar `logengine` active at `~/.k8s-agent/logengine` (hot in-memory buffer snapshot, compactor, tokenbf_v1 + 4-gram ngrambf_v1 skip index, K8s LowCardinality columns).

## 3. QA Evidence Hygiene
- All QA screenshots and visual audit artifacts MUST be stored under:
  `docs/qa-evidence/<YYYY-MM-DD>/<task-name>/`
- Never drop `.png` files into the project root, `frontend-vue/`, or `.agents/`.
