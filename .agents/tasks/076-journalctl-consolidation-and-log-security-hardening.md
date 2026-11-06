# Task 076 — Consolidate Host `systemd` into Single `journalctl` Scope & Harden Multi-Tenant Log Security

> **Status**: `COMPLETED` (`73c899ec`)
> **Branch**: `feat/declone-sre-ux`
> **Trigger**: User security & UX inquiry + `security-engineer` CVSS 9.1 audit (`fd82ce66`)
> **User Approved Design**: Consolidate all permitted host `systemd` logs into **1 single `[journalctl (System Journal)]` option** inside the dropdown (**visible & accessible strictly to `platform_admin`**), keep `[All Apps ▾]` 100% pure Application/Container/Pod workloads, block sensitive OS auth/security logs (`auth`, `sshd`, `sudo`, `kern`, `polkit`, `shadow`), redact `entry.Raw` + CLI flags, and eliminate cross-tenant host log laundering.

## 1. Vulnerabilities & UX Flaws Remediated
1. **SEC-LOG-01 (`CRITICAL` — Read-Only RBAC Bypass & Tenant Host Laundering)**:
   - `platform_routes.go:214` used `RequireRolesForMutations`, letting any authenticated token call `GET /api/v1/logs/*`, while `distributed_log_repo.go:74,167`, `log_stream_handler.go:266`, and `log_error_radar_handler.go:53` fell back to `hostRepo.ListAll(ctx)` and re-stamped host OS logs with the caller's `TenantID`.
2. **SEC-LOG-02 (`HIGH` — Sensitive OS Auth/Kernel Log Ingestion)**:
   - `cmd/agent/journal_tailer.go` and `cmd/agent/log_reader.go` ingested `auth` (`/var/log/auth.log`), `kern` (`/var/log/kern.log`), `sshd`, `sudo`, `polkit`, `systemd-logind`, and exposed 40+ OS daemons in `/api/v1/logs/services`.
3. **SEC-LOG-03 (`HIGH` — Unredacted `entry.Raw` & CLI Credential Flags)**:
   - `internal/pkg/logengine/writer.go` masked `entry.Message` only, leaving `entry.Raw` (raw `journalctl -o json` payload with `_CMDLINE`) unredacted, and `redact.go` lacked CLI flag patterns (`-p`, `--password=`, `--token=`, `curl -u`).
4. **UX-LOG-04 (`[All Apps ▾]` Polluted by 35+ Individual `systemd` Units)**:
   - Instead of showing only real Apps/Containers/Pods plus 1 consolidated `journalctl` option for `platform_admin`, the dropdown dumped every `.service` unit and syslog file onto the user.

## 2. Disjoint Parallel Slices (`2` Builders, `Workspace: "branch"`, `Model: "pro"`)

### Slice 76A (`backend-coder`) — Agent Denylist, `entry.Raw` Redaction, Multi-Tenant RBAC & Consolidated `journalctl` Scope
- `internal/domain/logging/source_policy.go` (+ `_test.go`): Authoritative `IsSensitiveOSLogSource` (denylists `auth`, `auth.log`, `authpriv`, `secure`, `sshd`, `ssh`, `sudo`, `su`, `polkit`, `shadow`, `pam`, `systemd-logind`, `kern`, `kern.log`, `btmp`, `wtmp`, `lastlog`, `faillog`, syslog facility `4`/`10`) and `IsHostSystemLog` (`namespace == "__host__"`, `service == "journalctl"`, `*.service`/`*.socket`/`*.timer`/`*.scope`/`*.slice`, and known OS daemons like `vmware-vmsvc-root`, `vmware-vmtoolsd-root`, `multipath`, `ModemManager`, `cron`, `crontab`, `systemd*`, `dbus*`, `packagekit*`, `rsyslog*`, `chrony*`, `apt-*`, `unattended-upgrades`, `kubelet`, `containerd`, `dockerd`).
- `internal/pkg/logengine/redact.go` & `writer.go` (+ `_test.go`): Mask BOTH `entry.Message` and `entry.Raw`; add CLI secret flag redaction (`--password`, `--passwd`, `--token`, `--secret`, `--api-key`, `--client-secret`, `-p <val>`, `-u user:pass`).
- `cmd/agent/journal_tailer.go`, `log_reader.go`, `log_search.go`, `log_server.go`: Drop `IsSensitiveOSLogSource` at ingest & read; tag permitted journald/host entries with `Namespace: "__host__"`, sanitize `Raw` (never dump raw `journalctl -o json` env/cmdline), redact `FileLogSource`, support `service=journalctl` query on `:9100`, and consolidate host units in `:9100/services`.
- `internal/adapter/http/platform_routes.go`, `log_handler.go`, `log_stream_handler.go`, `log_error_radar_handler.go`, `internal/infrastructure/agent/distributed_log_repo.go`, `distributed_error_radar.go` (+ `log_rbac_security_test.go`):
  - Enforce explicit read RBAC on `/api/v1/logs/*`.
  - Non-`platform_admin`: `403 Forbidden` on `service=journalctl`, `namespace=__host__`, or any `IsHostSystemLog` service; NEVER fall back to `hostRepo.ListAll(ctx)` when tenant owns 0 hosts; NEVER re-stamp host logs as tenant logs; omit `"journalctl"` from `/api/v1/logs/services`.
  - `platform_admin`: `/api/v1/logs/services` returns ONLY real App/Container/Pod names PLUS the single `"journalctl"` entry per online node (zero individual `.service` / OS daemon clutter!). `service=""` (`All Apps`) returns ONLY App/Container/Pod logs; `service="journalctl"` returns all permitted host `journalctl` logs (with originating unit name preserved on entries, and `IsSensitiveOSLogSource` strictly stripped); `service="<specific.service>"` remains supported for `platform_admin` `/deployments` `Daemon Logs` inspector!

### Slice 76B (`frontend-coder`) — `[All Apps ▾]` Pure App List + Single `platform_admin`-Only `[journalctl (System Journal)]` Option
- `frontend-vue/src/components/logs/LogServiceCombobox.vue`, `LogStreamToolbar.vue`, `LogTerminalEmptyState.vue`, `frontend-vue/src/composables/useLogTargets.ts`, `frontend-vue/src/views/LogStreamView.vue`, `frontend-vue/src/assets/styles/views/logstream-selectors.css`, `frontend-vue/tests/logToolbarSelectors.test.ts`:
  - Filter out any `.service` / OS daemon names from the App list in `useLogTargets.ts` & `LogServiceCombobox.vue` so `[All Apps ▾]` lists **100% pure App/Container/Pod workloads**.
  - Render a dedicated, visually separated **`journalctl` (`System Journal`)** item inside `LogServiceCombobox.vue` **ONLY when `authStore.user?.role === 'platform_admin'`** (hidden completely for non-`platform_admin` roles).
  - When `journalctl` is selected, trigger pill shows `[🖥 journalctl × ▾]` (`×` resets to `All Apps` in 1 click).
  - When `All Apps` has 0 container logs on a host (e.g. `k8sworker`) and user is `platform_admin`, `LogTerminalEmptyState.vue` offers a clean 1-click `[Switch to journalctl (System Journal)]` button alongside offline handling.
