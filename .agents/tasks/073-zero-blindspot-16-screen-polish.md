# Task 073 — Zero-Blindspot 16-Screen Live Visual, Data & Behavioral Remediation

> **Status**: `IN_PROGRESS`
> **Branch**: `feat/declone-sre-ux`
> **Origin**: Main-Thread Unblurred 16/16 Live Screenshot Audit (`docs/qa-evidence/2026-10-28/full-16-screens/`) + `LES-012` 5-Gate Zero-Theater DoD

## 1. Objective
Eliminate every visual, dual-theme, layout, data-accuracy, and static-test-theater defect uncovered across the unblurred 16/16 live browser audit (`01_overview` through `16_totp_setup`) against live Go `:8080` and PostgreSQL `10.10.10.133:5432`.

## 2. Disjoint Parallel Slices (`<= 3` Builders, `Workspace: "branch"`, `Model: "pro"`)

### Slice 73A (`backend-coder`) — Helm Catalog Deduplication, Stale Seed Purge & Live DB Sync
- `internal/infrastructure/helm/release_values.go` (+ `release_values_test.go`): Deduplicate Helm repo index charts by `(repo, name)` keeping the latest non-deprecated version so `/api/v1/helm/charts` returns diverse charts instead of 120 historical versions of `airflow`.
- `internal/infrastructure/worker/node_history_worker.go` (+ test): Ensure anomaly/offline incident generation only targets active registered `compute_hosts`.
- `migrations/065_purge_stale_seed_artifacts.up.sql` & `.down.sql` (+ live apply on `10.10.10.133:5432` via `.env`): Purge residual fake-host incidents (`masterdb`, `k8sworker3`, `worker2`, `test-qa-incident`), purge placeholder `hooks.slack.com/services/T00000000` seed channel from `alert_channels`, and normalize `compliance_violations.framework` (`NSA-CISA`, `SOC 2`, `CIS`).

### Slice 73B (`frontend-coder`) — Global Shell, Hosts Light Mode, Fleet, Deployments, Explorer & Helm UI
- Global Shell & PWA (`App.vue`, `AppSidebar.vue`, `app-shell.css`, `PwaInstallBanner.vue`, `pwa-install-banner.css`): Fix left header breadcrumb clipping behind `.sidebar-toggle-btn`; only show `PwaInstallBanner` when real `beforeinstallprompt` fires and reserve `padding-bottom: 104px` when visible.
- `/hosts` (`HostBladeRackGrid.vue`, `infra-hosts-blade.css`): Scope `.blade-status-pill .status-dot` to stop global `.status-dot` pollution; fix Light Mode white-on-white blade cards and `.meter-track` contrast using `tokens.css` variables.
- `/fleet` (`FleetToolbar.vue`, `fleet-toolbar.css`): Add `gap: 6px` on filter pills (`Degraded`, `Offline`).
- `/deployments` (`DeploymentsHeaderBar.vue` trimmed `< 300` lines, `WorkloadNamespaceTopology.vue`, `useDeployments.ts`, `deployments.css`): Widen `.toolbar-select` (`All Status (8)` unclipped); replace K8s pod costume (`RollingUpdate`, `:latest`, `Pods Ready`) with truthful `Systemd Service` / `Active (running)` labels for `daemon` workloads.
- `/explorer` (`ExplorerView.vue`, `explorer.css`): Remove `max-width: 1040px !important` table cap; replace unicode emojis in `.sre-summary-strip` with `BaseIcon` SVGs + spacing.
- `/helm` (`HelmCatalogView.vue`, `useHelmCatalog.ts`, `helmCatalogConstants.ts`, `HelmChartsGrid.vue`): Client-side `(repo, name)` deduplication + replace unicode category emojis with `BaseIcon` SVGs.
- Behavioral tests (`tests/task072SliceB.test.ts`, `tests/task072SliceD.test.ts`, `tests/task073SliceB.test.ts`): Zero `fs.readFileSync`; real `@vue/test-utils` & composable assertions.

### Slice 73C (`frontend-coder`) — Semantic Status Dots, Incidents Math, Audit, Tenancy, Alerts, Settings, 2FA & Action Limits
- `StatusBadge.vue`: Add `'medium'` to `AMBER_STATUSES`, `'high'` to `ROSE_STATUSES`, and `'isolated'` to `EMERALD_STATUSES`.
- `/incidents` (`useIncidents.ts`, `IncidentsView.vue`): Exclude `resolved` incidents from active `criticalCount` (`P1 Active`); polish zero-incident empty state.
- `/audit` (`useCompliance.ts`): Match `NSA-CISA` before `\bCIS\b` so `NSA-CISA` findings never misrender as `CIS LEVEL 2`.
- `/tenancy` (`TenantListTable.vue`, `tenancy.css`): Widen `ORGANIZATION` `<col>` (`32%`) to prevent premature truncation; use semantic emerald status on `ISOLATED`.
- `/alerts` (`AlertsView.vue`, `AlertChannelsGrid.vue`, `useAlerts.ts`, `alerts.css`): Restore `.alerts-page` padding (`20px 24px`); widen search input (`min-width: 280px`); wire `DELETE /api/v1/alerts/channels/{id}` button on channel cards.
- `/settings` & `/settings/2fa-setup` (`SettingsSecurityTab.vue`, `TOTPSetupView.vue`, `QRCodeDisplay.vue`): Replace corrupted `Enable 2FA Wizard ?` with clean label + `BaseIcon`; fallback `QRCodeDisplay` account label to `authStore.user?.email || authStore.user?.username || 'platform_admin'`.
- `/automation` & `/runbooks` (`AutomationRulesTable.vue`, `RunbooksTable.vue`): Enforce `<= 2` row triggers (`1` primary button + `ActionDropdown`).
- Behavioral tests (`tests/task072SliceC.test.ts`, `tests/task073SliceC.test.ts`): Zero `fs.readFileSync`; real `@vue/test-utils` & composable assertions.
