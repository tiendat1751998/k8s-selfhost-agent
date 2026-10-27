# Task 072 — Full-Platform 16-Screen UI/UX, Dual-Theme & Functional Overhaul

- **Status**: `COMPLETED` (`3732ac37` — `reviewer` APPROVED x2, `qa-test-engineer` Live Chrome Verification 10/10 + 5/5 Delta PASS)
- **Branch**: `feat/declone-sre-ux`
- **Source Audit**: [`.agents/audit/2026-10-27-full-16-screens-ui-ux-functional-audit.md`](../audit/2026-10-27-full-16-screens-ui-ux-functional-audit.md)
- **Tier**: Tier 2 Full-Stack Overhaul (3 Parallel File-Disjoint Coders + Live QA Delta Polish → `reviewer` → `qa-test-engineer` Live Chrome Verification)

---

## Parallel Work Breakdown Structure (100% Disjoint File Scopes)

### Subtask 72A — `backend-coder` (`*.go` ONLY)
- **Deliverables**:
  - `FUNC-01`: Guard `validateRepoURL` in `internal/infrastructure/helm/release_operations.go:103,186` when `req.Repo` is a repo name (`"bitnami"`).
  - `FUNC-03`: Replace fake `CreateSilence` and `TestChannel` stubs in `alert_handler.go` with real persistence & SSRF-guarded webhook/SMTP probes; genuine `ResolveAlert` status update; fix contradictory `"High Memory Usage"` CPU metric seed.
  - `FUNC-04`: Purge stale `"worker1"` mock incident rows; add `POST /api/v1/incidents/{id}/acknowledge` & `GET /api/v1/incidents/{id}/postmortem`.
  - `FUNC-05`: Ensure `PUT /api/v1/slo/definitions/{id}` updates SLOs cleanly and allow registered fleet node hosts in `validateCustomSLOQueryURL` while preserving `netguard.WithAllowPrivate(false)` for arbitrary private IPs.
  - `FUNC-08`: Add `POST /api/v1/fleet/validate-kubeconfig` dry-run endpoint; allow `GET /api/v1/audit/findings|logs` for `operator`/`tenant_admin` and tenant-scoped `tenant_admin` access on `/api/v1/tenancy`.

### Subtask 72B — `frontend-coder` (Global Tokens/Z-Index/Dual-Theme CSS + Auth, Overview, Logs, Fleet, Hosts, Deployments, Explorer, Helm)
- **Exclusive Files**: `tokens.css`, `buttons.css`, `host-remediation.css`, `pwa-install-banner.css`, `floating-alert-toast.css`, `alert-center.css`, `app-shell.css`, `pod-log-*.css`, `pod-terminal.css`, `overview-mesh.css`, `events-timeline.css`, `explorer/**`, `login.css`, `overview.css`, `logstream-tree.css`, `infra-hosts-controls.css`, `storage.css`, `PwaInstallPrompt*`, `BaseModal.vue`, `ModalDrawer.vue`, `LoginView.vue`, `LoginBrandingHero.vue`, `OverviewView.vue`, `components/overview/**`, `components/telemetry/**`, `LogStreamView.vue`, `useLogStream.ts`, `components/logs/**`, `FleetView.vue`, `components/fleet/**`, `api/cluster.ts`, `InfraHostsView.vue`, `components/hosts/**`, `DeploymentsView.vue`, `components/deployments/**`, `ExplorerView.vue`, `api/k8s.ts`, `HelmCatalogView.vue`, `components/helm/**`, `useHelmCatalog.ts`.
- **Deliverables**: `UI-01`, `UI-02`, `UI-03`, `UI-05`, `UI-08` (Auth/Helm), `FUNC-02` (Explorer), `FUNC-07` (Daemon Inspector), `FUNC-08` (Fleet preflight + Host `./deploy-agent.sh` snippet + Logs `since` resume + SVG icon cleanup).

### Subtask 72C — `frontend-coder` (SRE UX & Functional Overhaul of Incidents, SLO, Alerts, Tenancy, Settings, 2FA, Automation, Runbooks)
- **Exclusive Files**: `IncidentsView.vue`, `useIncidents.ts`, `components/incidents/**`, `api/incidents.ts`, `incidents*.css`, `SLOView.vue`, `useSLO.ts`, `components/slo/**`, `AlertsView.vue`, `useAlertManager.ts`, `components/alerts/**`, `alerts.css`, `TenancyRbacView.vue`, `useTenancyRbac.ts`, `components/tenancy/**`, `SettingsView.vue`, `TOTPSetupView.vue`, `RecoveryCodesCard.vue`, `useSettings.ts`, `components/settings/**`, `AutomationView.vue`, `useAutomationEngine.ts`, `components/automation/**`, `RunbooksView.vue`, `useRunbooks.ts`, `components/runbooks/**`, `components/ai/AIProvidersTable.vue`.
- **Deliverables**: `UI-04` (`/alerts` 640-768px blank screen + button alignment + SRE triage header), `UI-06` (max 2 inline actions + `[ ⋯ ]`), `UI-07` (SRE UX hierarchy + modal `@keydown.esc` & `form="..."`), `UI-08` (`IncidentDetailPane` provider & TOTP 10/8 codes), `FUNC-04` (wire `RcaTimelineModal`, `CreateIncidentModal`, `Acknowledge`, `Postmortem`), `FUNC-05` (`SloCreateModal` edit `PUT`), `FUNC-06` (restore `Backup & DR` tab in `/settings` + `Test Notification` + `ZeroTrust KMS` card), `FUNC-08` (`savedEngineeringHours`).
