# Project State — Checkpoint 2026-10-27 (Task 070: Full-Platform Security & All Screens Verified)

## Branch: `feat/declone-sre-ux`
## Completed Tasks:
- [`.agents/tasks/070-platform-security-and-functional-remediation.md`](../tasks/070-platform-security-and-functional-remediation.md) (`COMPLETED`)
- [`.agents/tasks/069-backend-real-telemetry-purge-fake-data.md`](../tasks/069-backend-real-telemetry-purge-fake-data.md) (`COMPLETED`)
- [`.agents/tasks/068-ux-decloning-3-waves.md`](../tasks/068-ux-decloning-3-waves.md) (`COMPLETED`)

---

## 0. Runtime & Deployment Reality
- **No Automated CI/CD**: Remote Linux nodes (`10.10.10.60`, `10.10.10.133`, `10.10.10.80`) require explicit rollout via [`./deploy-agent.sh`](../../deploy-agent.sh) if `cmd/agent` changes (e.g., optional `AGENT_TLS_CERT`/`KEY` in `cmd/agent/tls.go`).
- **Active Local Ports**: Central Server (`bin/standalone.exe`) running on `:8080`, Frontend Vite Dev running on `:3000`, Remote PostgreSQL on `10.10.10.133:5432` (Migration `063` applied live), Remote Node Agents on `:9100`.

---

## 1. Security & Architecture Status (13/13 Remediated — `security-engineer` & `reviewer` APPROVED)
- **SSRF / RCE / BOLA**: `netguard` (`WithAllowPrivate(false)`) enforced on custom SLO probes (`collector_probes.go` & `observability_handler.go`); `/api/v1/iac/*` restricted to `platform_admin` with mandatory `DefaultSandboxDir()` & Ansible flag sanitization; unnamespaced Docker mutations require `platform_admin`; Explorer enforces tenant namespace ownership.
- **Auth / Session / Rate-Limit / API Keys**: `TrustedProxyRealIP` validates `TRUSTED_PROXIES` CIDRs; access JWT lives strictly in memory (`authStore.ts` + `useWebSocket.ts`) with silent `HttpOnly` refresh (`/api/v1/auth/refresh`); bounded `TokenDenylist` revokes tokens on logout/password-change; `k8s_live_` API keys persisted as SHA-256 hashes and authenticated by `JWTAuthMiddleware`; `/agents/register` gated by constant-time `AGENT_AUTH_TOKEN` or `platform_admin` + `SanitizeComputeHosts` masking across all host endpoints.
- **Database Schema Unification (`063`)**: All 22 tables from Migration `057` unified to `tenant_id VARCHAR(255) NOT NULL DEFAULT 'default-tenant'`; background workers (`HealthChecker`, `FleetComplianceScanner`) run with `tenancy.WithPlatformAdminContext(ctx)` (`LES-009`).

---

## 2. Live Telemetry & Functional Status Across All Screens (8/8 Chrome E2E QA PASS)
- **`/overview`**: Real node IPs (`10.10.10.80`, `10.10.10.60`, `10.10.10.133`), `7/7 RUNNING` workloads, synthetic `traefikRx/1200` RPS removed, sidebar logo spacing fixed (`48.67px` gap).
- **`/clusters` & Global Top-Bar**: `GET /api/v1/docker/swarm` returns `200 { active: false }`; `k8snode` honestly displays amber **`DEGRADED`** (`Control Plane :6443 unreachable · Host agents reporting`) in both `/clusters` and the global Top-Bar context selector.
- **`/helm`**: 120 real Bitnami/Helm v3 SDK charts normalized & rendered with real names, versions, descriptions, single toolbar, and working `Install Chart` drawer.
- **`/explorer`**: Explicit `CONTROL PLANE (:6443) UNREACHABLE` banner with deep links to `/deployments` and `/slo`; Swarm-as-Pod masquerade deleted.
- **`/runbooks`**: Safe `RunbookExecutor` executes real `:6443`, `postgres:5432`, and `:9100` diagnostics (`POST .../steps/{idx}/execute` & `/execute`) with verbatim output & duration.
- **`/audit`**: Live compliance scan persists `35` active violations + `5` runtime findings (`Container CVEs (5)`) + `POST .../violations/{id}/remediate` opens `RemediationActionDrawer` with copyable Linux commands.
- **`/incidents`, `/tenancy`, `/settings`**: `0` 404 polling errors on `/incidents` (`Simulate` hidden unless `?debug=1`); Tenancy RBAC/quotas/members & KMS rotation metadata persisted in PostgreSQL; `ZeroTrustDrawer` honestly reports `Local AES-256-GCM Envelope Encryption (Software KMS)`.
