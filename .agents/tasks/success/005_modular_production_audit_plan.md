# Task 005: Phased Bite-Sized Production Audit & Polish Plan

**Status:** IN_PROGRESS  
**Phases:** 6 Modular Phases  
**Current Phase:** Phase 1 (Auth, Multi-Tenancy & User Management)

---

## 📋 6 Phased Modules

### [ ] Phase 1: Auth, Multi-Tenancy & User Management
- Backend: `internal/domain/user/`, `internal/usecase/auth/`, `internal/infrastructure/postgres/user_repo.go`, `tenancy_repo.go`, `internal/adapter/http/middleware/auth.go`
- Frontend: `frontend-vue/src/views/LoginView.vue`, `TOTPSetupView.vue`, `src/stores/authStore.ts`

### [ ] Phase 2: Kubernetes Explorer, Fleet & Workloads
- Backend: `internal/domain/explorer/`, `internal/domain/deployment/`, `internal/domain/fleet/`
- Frontend: `frontend-vue/src/domain/explorer/`, `ExplorerView.vue`, `DeploymentsView.vue`, `FleetView.vue`

### [ ] Phase 3: Database Backup & Dual-Target Storage Engine
- Backend: `internal/domain/backup/`, `internal/infrastructure/backup/drivers/`, `dualsync/`, `storage/`
- Frontend: `frontend-vue/src/views/BackupRestoreView.vue`

### [ ] Phase 4: SRE Observability, Live Logs & Alert Engine
- Backend: `internal/domain/alert/`, `internal/domain/nodemetrics/`, `internal/infrastructure/logging/`, `telegram/`
- Frontend: `frontend-vue/src/views/OverviewView.vue`, `AlertsView.vue`, `LogStreamView.vue`, `IncidentsView.vue`

### [ ] Phase 5: DevSecOps, Vulnerability Scanning & IaC
- Backend: `internal/infrastructure/security/`, `internal/infrastructure/iac/`
- Frontend: `frontend-vue/src/views/DevSecOpsView.vue`, `ComplianceView.vue`, `AuditView.vue`

### [ ] Phase 6: Service Catalog, Scaffolder & Plugins
- Backend: `internal/domain/catalog/`, `internal/domain/scaffold/`, `internal/domain/plugin/`
- Frontend: `frontend-vue/src/views/ServiceCatalogView.vue`, `ScaffolderView.vue`, `PluginsView.vue`, `AIProviderHubView.vue`
