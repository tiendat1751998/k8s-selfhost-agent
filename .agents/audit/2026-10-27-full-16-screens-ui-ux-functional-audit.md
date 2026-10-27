# Full-Platform 16-Screen UI/UX, Dual-Theme & Functional Audit (2026-10-27)

- **Branch**: `feat/declone-sre-ux`
- **Auditors**: `research` (`188218f9`), `ux-designer` (`e3c97314`), `qa-test-engineer` (`ec710bb7`)
- **Scope**: All 16 primary views + `TOTPSetupView` + Global Shell (`App.vue`, `AppHeader`, `AppSidebar`, `PwaInstallPrompt`, `ZeroTrustDrawer`, `tokens.css`, `buttons.css`) + Go Backend (`internal/**`, `cmd/standalone/**`).

---

## 1. UI/UX, Dual-Theme & Responsive Defects (`ux-designer` Findings)

1. **`UI-01` (`buttons.css:39` Missing `--bg-card-hover` Token)**: `background: var(--bg-card-hover, #1e293b)` falls back to dark slate `#1e293b` in Light Mode because `--bg-card-hover` is undefined in `tokens.css`, causing unreadable black-on-dark button hover across all 16 screens.
2. **`UI-02` (29 Hardcoded Dark Surfaces & 8 Raw Neon Hexes)**: `rgba(15, 23, 42, ...)`, `#0f172a`, `#1e293b`, `#111827`, `#00e5ff`, `#00f0ff`, `#06b6d4` across `logstream-tree.css`, `host-remediation.css`, `infra-hosts-controls.css`, `pod-log-viewer.css`, `pod-log-stream.css`, `pod-terminal.css`, `overview-mesh.css`, `floating-alert-toast.css`, `events-timeline.css`, `explorer/table.css`, `login.css`, `storage.css`, `CanvasTimeSeries.vue`, `ClusterDensityMatrix.vue`, `WorkloadLogToolbar.vue`, `ClickHouseEngineBadge.vue`.
3. **`UI-03` (`PwaInstallPrompt` & Chaotic Z-Index Collisions)**: Bottom-right PWA banner stays dark in Light Mode, collides with modal footers (`Save Changes`) and table `[ ⋯ ]` menus, and `floating-alert-toast.css` / `alert-center.css` inflate `z-index` up to `100000`.
4. **`UI-04` (`/alerts` `[640px, 767.98px]` Blank-Screen Dead-Zone & Stacked Buttons)**: `AlertRulesMobileCards.vue:67` hides `.mobile-only` at `>=640px` while `alerts.css:12` hides `.desktop-only` at `<=767.98px`, leaving a blank screen at `640-767px`; desktop `Management` column stacks `[Edit]` and `[Delete]` unevenly.
5. **`UI-05` (Forbidden Mobile Chart Hiding in `overview.css:180,299`)**: `.topology-section` and `.desktop-overview-sections` use `display: none !important` on mobile, violating `AGENTS.md`.
6. **`UI-06` (Action Density `> 2` Inline Buttons & 27 Raw Emojis)**: `RunbooksTable.vue`, `AutomationRulesTable.vue`, and `AIProvidersTable.vue` render 3–4 inline buttons instead of max 2 + `[ ⋯ ]`; 27 raw emojis remain in log/event/picker components.
7. **`UI-07` (Cookie-Cutter CRUD Screens & Modal Escape/Form Polish)**: `/incidents`, `/fleet`, `/alerts`, `/tenancy`, `/settings`, `/explorer` lack domain SRE visual hierarchy; 8 modals lack `@keydown.esc` and 3 modals place submit buttons outside `<form>` without `form="..."`.
8. **`UI-08` (Misleading Static UI Copy)**: Pre-auth `LoginBrandingHero.vue` & `LoginView.vue` claim `"Control Plane SYNCHRONIZED"` / `"TLS v1.3"`; `IncidentDetailPane.vue` hardcodes `'Claude 3.5 Sonnet / Multi-Agent'`; `HelmCatalogView.vue:219` single-row repo `"Update"` triggers `handleUpdateAllRepos`; `TOTPSetupView.vue` says `"10 Codes"` while `RecoveryCodesCard.vue` renders 8.

---

## 2. Full-Stack Functional, Stub & Contract Defects (`research` Findings)

1. **`FUNC-01` (Helm Repo Name Validation Trap)**: `internal/infrastructure/helm/release_operations.go:103,186` runs `validateRepoURL(req.Repo)` on repo names like `"bitnami"`, failing with `unsupported URL scheme ""` before reaching Helm/K8s.
2. **`FUNC-02` (Explorer Events Prop Mismatch, Route Bypass & Apply YAML `?ns=`)**: `ExplorerDetailDrawer.vue:107-108` passes `:resource-name`/`:resource-kind` instead of `:name`/`:kind` to `EventsTimeline.vue`; `k8s.ts:279` hits `/resources/events` instead of `/events`; `k8s.ts:359` omits `?ns=` on `/apply`.
3. **`FUNC-03` (Alert Handler Stubs & Contradictory Seed Rule)**: `alert_handler.go:374-436` stubs `CreateSilence` (no persistence) and `TestChannel` (no webhook/SMTP probe), while `ResolveAlert` calls `AcknowledgeAlert`; seeded `"High Memory Usage"` rule uses CPU metric `container_cpu_usage_seconds_total`.
4. **`FUNC-04` (Incidents Orphaned `RcaTimelineModal`, Duplicate Modal, Missing Ack/Postmortem & Stale `"worker1"` Seed)**: `RcaTimelineModal.vue` and `CreateIncidentModal.vue` are orphaned; `/incidents/{id}/acknowledge` and `/postmortem` are missing; stale `"worker1"` mock rows remain in DB.
5. **`FUNC-05` (SLO Edit Creates Duplicates & Fleet Node Probe Allowlist)**: `SloCreateModal.vue` ignores edit prop and always `POST`s duplicates; `SloCatalogTable.vue` lacks `Edit`; `observability_handler.go` blocks custom probes to registered fleet nodes.
6. **`FUNC-06` (Orphaned Backup & DR Subsystem, Missing Notification Test & KMS Link in `/settings`)**: `useSettings.ts:24-31` omits `backup` while `/backup` redirects to `/`, making `backupApi` unreachable; `SettingsNotificationsTab.vue` lacks `Test Notification`; `SettingsSecurityTab.vue` lacks KMS drawer trigger.
7. **`FUNC-07` (Deployments Daemon Inspector Fake K8s Canary/DNS/YAML)**: `WorkloadInspectorDrawer.vue` & `workloadInspectorYaml.ts` render K8s Canary controls, `.svc.cluster.local`, and `kind: Deployment` YAML for Linux `systemd` daemons.
8. **`FUNC-08` (Fleet Kubeconfig Preflight, Host `./deploy-agent.sh` Snippet, Automation Saved Hours & RBAC Scoping)**: `FleetImportModal.vue` lacks dry-run validation; `HostAddEditModal.vue` lacks `./deploy-agent.sh` instructions; `useAutomationEngine.ts` hardcodes `'0.0'` hours; `platform_routes.go` blocks `tenant_admin` on own-tenant `/tenancy` and `operator`/`tenant_admin` on `GET /audit/findings|logs`.
