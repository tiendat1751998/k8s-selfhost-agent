# Screen & API Inventory

> Generated: 2026-09-09T15:49:00+07:00 | Branch: fix/comprehensive-audit

## Frontend Routes (34 routes)
| # | Path | Component | Name |
|---|------|-----------|------|
| 1 | /login | LoginView.vue | login |
| 2 | / | OverviewView.vue | overview |
| 3 | /incidents | IncidentsView.vue | incidents |
| 4 | /agents | AgentsView.vue | agents |
| 5 | /slo | SLOView.vue | slo |
| 6 | /logs | LogStreamView.vue | logs |
| 7 | /fleet | FleetView.vue | fleet |
| 8 | /hosts | InfraHostsView.vue | infra-hosts |
| 9 | /deployments | DeploymentsView.vue | deployments |
| 10 | /workloads | DeploymentsView.vue | workloads |
| 11 | /promotions | PromotionsView.vue | promotions |
| 12 | /docker | DockerSwarmView.vue | docker |
| 13 | /explorer | ExplorerView.vue | explorer |
| 14 | /helm | HelmCatalogView.vue | helm |
| 15 | /audit | AuditView.vue | audit |
| 16 | /security | DevSecOpsView.vue | security |
| 17 | /compliance | ComplianceView.vue | compliance |
| 18 | /drift | DriftView.vue | drift |
| 19 | /backup | BackupRestoreView.vue | backup |
| 20 | /automation | AutomationView.vue | automation |
| 21 | /runbooks | RunbooksView.vue | runbooks |
| 22 | /cost | CostFinOpsView.vue | cost |
| 23 | /capacity | CapacityView.vue | capacity |
| 24 | /tenancy | TenancyRbacView.vue | tenancy |
| 25 | /ai-hub | AIProviderHubView.vue | ai-hub |
| 26 | /changes | ChangesView.vue | changes |
| 27 | /alerts | AlertsView.vue | alerts |
| 28 | /reports | ReportsView.vue | reports |
| 29 | /catalog | ServiceCatalogView.vue | ServiceCatalog |
| 30 | /scaffolder | ScaffolderView.vue | ScaffolderTemplates |
| 31 | /ecosystem | EcosystemView.vue | ecosystem |
| 32 | /plugins | PluginsView.vue | plugins |
| 33 | /settings | SettingsView.vue | settings |
| 34 | /settings/2fa-setup | TOTPSetupView.vue | totp-setup |

## API Endpoints (34+ listed, 428+ sub-routes dynamic)
| # | Method | Path | Handler | File:Line |
|---|--------|------|---------|-----------|
| 1 | GET | /healthz | health.Handler | router.go:92 |
| 2 | GET | /readyz | health.Handler | router.go:93 |
| 3 | GET | /livez | health.LivenessHandler | router.go:94 |
| 4 | GET | /metrics | promhttp.Handler | router.go:97 |
| 5 | GET | /ws | WSHub.ServeWS | router.go:122 |
| 6 | POST | /api/v1/auth/login | AuthHandler.Login | router.go:128 |
| 7 | POST | /api/v1/auth/verify-mfa | AuthHandler.VerifyMFA | router.go:129 |
| 8 | POST | /api/v1/auth/refresh | AuthHandler.RefreshToken | router.go:130 |
| 9 | POST | /api/v1/auth/recovery/verify | AuthHandler.VerifyRecoveryCode | router.go:131 |
| 10 | POST | /api/v1/auth/logout | AuthHandler.Logout | router.go:132 |
| 11-14 | * | /api/v1/auth/totp/* | AuthHandler.TOTP* | router.go:137-140 |
| 15 | POST | /api/v1/telemetry | inline logger | router.go:155 |
| 16-27 | * | /api/v1/k8s/{cluster}/* | platform.K8s.* | router.go:212-234 |
| 28 | GET | /api/v1/ai/providers | ai_handler.ListProviders | ai_handler.go:26 |
| 29 | POST | /api/v1/alerts/channels | alert_handler.CreateChannel | alert_handler.go:23 |
| 30 | POST | /api/v1/catalog/services | catalog_handler.CreateService | catalog_handler.go:34 |
| 31 | POST | /api/v1/backup/storages | backup_handler.CreateStorage | backup_handler.go:23 |
| 32 | GET | /api/v1/audit/findings | audit_handler.ListFindings | audit_handler.go:24 |
| 33 | GET | /api/v1/hosts/ | compute_host_handler.ListHosts | compute_host_handler.go:20 |
| 34 | GET | /api/v1/automation/rules | automation_handler.ListRules | automation_handler.go:24 |

## Components (18 total — all < 500 lines ✅)
Largest: PodLogViewer.vue (329 lines)

## Views (34 total — all < 400 lines ✅)
Largest: OverviewView.vue (~372 lines)

## Composables (44 total — all < 500 lines ✅)
Largest: useSettings.ts (498 lines)

## Styles (1 file)
overview-saturation-trends.css (~220 lines)

## Violations
**ZERO AGENTS.md §5 violations** — all files under 500-line limit.
