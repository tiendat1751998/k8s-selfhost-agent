# Project State — Checkpoint 2026-11-06 (Task 076: Multi-Tenant Log Security Hardening & Admin-Only `journalctl` Consolidation Completed)

## Branch: `feat/declone-sre-ux`
## Completed Tasks:
- [`.agents/tasks/076-journalctl-consolidation-and-log-security-hardening.md`](../tasks/076-journalctl-consolidation-and-log-security-hardening.md) (`COMPLETED` — `73c899ec`)
- [`.agents/tasks/075-logs-compact-toolbar-selectors.md`](../tasks/075-logs-compact-toolbar-selectors.md) (`COMPLETED` — `86c7ef65`)
- [`.agents/tasks/074-logs-pipeline-and-ui-remediation.md`](../tasks/074-logs-pipeline-and-ui-remediation.md) (`COMPLETED` — `a1a2b726`)
- [`.agents/tasks/073-zero-blindspot-16-screen-polish.md`](../tasks/073-zero-blindspot-16-screen-polish.md) (`COMPLETED` — `6b1c3e85`)
- [`.agents/tasks/072-full-platform-ui-ux-and-functional-overhaul.md`](../tasks/072-full-platform-ui-ux-and-functional-overhaul.md) (`COMPLETED` — `3732ac37`)
- [`.agents/tasks/071-deep-interactive-defects-remediation.md`](../tasks/071-deep-interactive-defects-remediation.md) (`COMPLETED` — `6d2a8602`)
- [`.agents/tasks/070-platform-security-and-functional-remediation.md`](../tasks/070-platform-security-and-functional-remediation.md) (`COMPLETED`)
- [`.agents/tasks/069-backend-real-telemetry-purge-fake-data.md`](../tasks/069-backend-real-telemetry-purge-fake-data.md) (`COMPLETED`)
- [`.agents/tasks/068-ux-decloning-3-waves.md`](../tasks/068-ux-decloning-3-waves.md) (`COMPLETED`)

---

## 0. Runtime & Deployment Reality
- **Physical Fleet State (`2026-11-06`)**: `10.10.10.133` (`k8smater133`, 8 pure app containers + `journalctl`) and `10.10.10.80` (`k8sworker`, bare-metal K8s worker with `journalctl`) are online (`:9100` LogEngine + PostgreSQL `10.10.10.133:5432` healthy); `10.10.10.60` (`k8smasterdeb`) VM is physically powered off and gracefully fast-failed via `HostBackoffManager`. Static Linux `./k8s-agent` binary is freshly built (`9.96 MB`) for `./deploy-agent.sh <user>@<ip>` when operator SSH keys are loaded; control-plane read/stream filters enforce 100% security & compatibility even against pre-rollout `:9100` agents.
- **Active Local Ports**: Central Server (`bin/standalone.exe` serving freshly built `frontend-vue/dist`) running on `:8080`, Frontend Vite Dev running on `:3000`.
- **Binding 5-Gate Zero-Theater DoD (`LES-012`..`LES-014`)**: `492/492` genuine behavioral frontend tests pass across `165` suites (`0` `fs.readFileSync` regex theater) + Go `domain/logging`, `pkg/logengine`, `infrastructure/logging`, `infrastructure/agent`, `adapter/http`, `cmd/agent` suites `100%` PASS.

---

## 1. Log Security, Multi-Tenant RBAC & `journalctl` Consolidation (`73c899ec` — `security-engineer` & `reviewer` APPROVED)
- **Zero OS Auth/Kernel Log Exposure**: `internal/domain/logging/source_policy.go` (`IsSensitiveOSLogSource`) unconditionally blocks `auth`, `auth.log`, `authpriv`, `secure`, `sshd`, `sudo`, `su`, `polkit`, `shadow`, `pam`, `systemd-logind`, `kern`, `kern.log`, `dmesg`, `auditd`, `btmp`, `wtmp`, `lastlog`, `faillog` (`403 Forbidden` for ALL roles including `platform_admin`, dropped at both edge ingest and control-plane read/WebSocket layers).
- **Strict `platform_admin` Gating & Zero Tenant Laundering**: `internal/adapter/http/log_rbac.go` + `distributed_log_filter.go` restrict `journalctl`, `__host__`, and all host `systemd`/OS daemons (`*.service`, `kubelet`, `dockerd-rootless.sh`, `fwupd`, etc.) strictly to `platform_admin` (`403 Forbidden` for `viewer`/`developer`/`operator`/`tenant_admin`); eliminated `hostRepo.ListAll(ctx)` fallback and `TenantID` re-stamping for non-admin tenants.
- **Dual `Message` + `Raw` + CLI Credential Redaction**: `internal/pkg/logengine/redact.go` & `writer.go` + `processRemoteStreamLine` redact secrets across `Message`, `Raw`, and live WebSocket frames (including `-p`, `--password=`, `--token=`, `--secret=`, `--api-key=`, `curl -u user:pass`).
- **Pure `[All Apps ▾]` + Single Admin-Only `[journalctl · System Journal]`**: Consolidated 35+ host systemd units into 1 separated `journalctl` option in `LogServiceCombobox.vue` (shown only to `platform_admin`), preserved `[unit.service]` badges on `journalctl` rows, added bounded pure-app backfill (`backfillPureAppLogs` + `seedRecentLogs`) so noisy daemons never starve quiet containers (`db`, `my-nginx`, `nats`, `tiki_redis`), and preserved `/deployments` `Daemon Logs` for `platform_admin`.
