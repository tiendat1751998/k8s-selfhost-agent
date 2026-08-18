# Phase 1 Implementation Plan — K8s Self-Host Platform

> **Goal:** Foundation Hardening — add DB Backup & Restore, Cluster Import, Enhanced Alerting, start Vue.js migration, clean tech debt
> **Estimated Total Effort:** ~82 hours

---

## 1. DB Backup & Restore Module (PostgreSQL first) — ~24h

### Domain Layer
- **`internal/domain/backup/entity.go`** — Domain models:
  - `BackupPolicy` — schedule, retention, target DB, storage backend
  - `BackupJob` — status, size, duration, error, storage path
  - `BackupStorage` — type (s3/minio/local), endpoint, credentials
  - `RestoreJob` — source backup, target, status, PITR timestamp
- **`internal/domain/backup/repository.go`** — Repository interface

### Infrastructure Layer
- **`internal/infrastructure/postgres/backup_repo.go`** — PostgreSQL repository implementation
- **`internal/infrastructure/storage/backup_s3.go`** — S3/MinIO storage adapter
- **`internal/infrastructure/storage/backup_local.go`** — Local filesystem adapter
- **`internal/infrastructure/db/postgres_backup_executor.go`** — pg_dump + WAL streaming

### Usecase Layer
- **`internal/usecase/backup/backup_usecase.go`** — Backup orchestration, cron scheduling
- **`internal/usecase/backup/restore_usecase.go`** — Restore orchestration

### Adapter Layer (HTTP)
- **`internal/adapter/http/backup_handler.go`** — REST handlers
- **`internal/adapter/http/router.go`** — Register backup routes

### Database Migration
```sql
-- migrations/027_db_backup_restore.up.sql
CREATE TABLE backup_storages (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL,
    name VARCHAR(255) NOT NULL,
    type VARCHAR(50) NOT NULL, -- 's3', 'minio', 'local'
    endpoint TEXT,
    bucket VARCHAR(255),
    credentials BYTEA, -- AES-256-GCM encrypted
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE backup_policies (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL,
    name VARCHAR(255) NOT NULL,
    db_type VARCHAR(50) NOT NULL, -- 'postgres', 'mysql', 'mongodb'
    db_host TEXT NOT NULL,
    db_port INT NOT NULL,
    db_name VARCHAR(255) NOT NULL,
    storage_id UUID REFERENCES backup_storages(id),
    schedule VARCHAR(100), -- cron expression
    retention_count INT DEFAULT 7,
    backup_type VARCHAR(50) DEFAULT 'logical', -- 'logical', 'physical', 'wal'
    enabled BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE backup_jobs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL,
    policy_id UUID REFERENCES backup_policies(id) ON DELETE CASCADE,
    status VARCHAR(50) NOT NULL DEFAULT 'pending', -- pending/running/completed/failed
    backup_type VARCHAR(50) NOT NULL,
    storage_path TEXT,
    size_bytes BIGINT,
    duration_ms BIGINT,
    error_message TEXT,
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE restore_jobs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL,
    backup_job_id UUID REFERENCES backup_jobs(id),
    target_db_host TEXT NOT NULL,
    target_db_name VARCHAR(255) NOT NULL,
    pitr_timestamp TIMESTAMPTZ, -- for point-in-time recovery
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    error_message TEXT,
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_backup_policies_tenant ON backup_policies(tenant_id);
CREATE INDEX idx_backup_jobs_policy ON backup_jobs(policy_id);
CREATE INDEX idx_backup_jobs_tenant ON backup_jobs(tenant_id);
CREATE INDEX idx_restore_jobs_tenant ON restore_jobs(tenant_id);
```

### API Endpoints
| Method | Path | Description |
|---|---|---|
| `POST` | `/api/v1/backups/storages` | Create storage backend |
| `GET` | `/api/v1/backups/storages` | List storage backends |
| `POST` | `/api/v1/backups/policies` | Create/update backup policy |
| `GET` | `/api/v1/backups/policies` | List policies |
| `DELETE` | `/api/v1/backups/policies/{id}` | Delete policy |
| `POST` | `/api/v1/backups/jobs` | Trigger manual backup |
| `GET` | `/api/v1/backups/jobs` | List jobs with status |
| `GET` | `/api/v1/backups/jobs/{id}` | Get job details |
| `POST` | `/api/v1/restores` | Trigger restore |
| `GET` | `/api/v1/restores` | List restore jobs |

### Dependencies
- S3/MinIO config in `config.yaml`
- Existing encryption package (`internal/pkg/crypto`)

---

## 2. Cluster Import Enhancement — ~16h

### Files to Create/Modify
- **`internal/domain/cluster/entity.go`** — Extend with import state, health status
- **`internal/usecase/cluster/import_usecase.go`** — Kubeconfig parsing, connectivity validation
- **`internal/usecase/cluster/health_checker.go`** — Polling mechanism
- **`internal/adapter/http/cluster_handler.go`** — Add import endpoint
- **`internal/infrastructure/kubernetes/discovery.go`** — Auto-discovery of nodes/resources

### Database Migration
```sql
-- migrations/028_cluster_import_fields.up.sql
ALTER TABLE fleet_clusters ADD COLUMN IF NOT EXISTS import_method VARCHAR(50) DEFAULT 'manual';
ALTER TABLE fleet_clusters ADD COLUMN IF NOT EXISTS kubeconfig_hash VARCHAR(64);
ALTER TABLE fleet_clusters ADD COLUMN IF NOT EXISTS last_health_check TIMESTAMPTZ;
ALTER TABLE fleet_clusters ADD COLUMN IF NOT EXISTS health_status VARCHAR(50) DEFAULT 'unknown';
ALTER TABLE fleet_clusters ADD COLUMN IF NOT EXISTS discovered_resources JSONB DEFAULT '{}';
```

### API Endpoints
| Method | Path | Description |
|---|---|---|
| `POST` | `/api/v1/clusters/import` | Multipart kubeconfig upload |
| `GET` | `/api/v1/clusters/{id}/health` | Poll connectivity status |
| `POST` | `/api/v1/clusters/{id}/discover` | Trigger resource discovery |

### Dependencies
- Existing fleet/cluster domain module

---

## 3. Enhanced Alerting System — ~20h

### Files to Create/Modify
- **`internal/domain/alert/entity.go`** — AlertRule, AlertHistory, AlertCondition, NotificationChannel
- **`internal/domain/alert/repository.go`** — Repository interface
- **`internal/usecase/alert/rule_engine.go`** — Condition/threshold evaluator
- **`internal/usecase/alert/notifier.go`** — Dispatcher logic
- **`internal/adapter/http/alert_handler.go`** — REST handlers
- **`internal/infrastructure/notifier/slack.go`** — Slack webhook sender
- **`internal/infrastructure/notifier/email.go`** — SMTP email sender
- **`internal/infrastructure/notifier/webhook.go`** — Generic webhook sender
- **`internal/infrastructure/postgres/alert_repo.go`** — PostgreSQL repository

### Database Migration
```sql
-- migrations/029_enhanced_alerting.up.sql
CREATE TABLE notification_channels (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL,
    name VARCHAR(255) NOT NULL,
    type VARCHAR(50) NOT NULL, -- 'slack', 'email', 'webhook'
    config JSONB NOT NULL, -- {webhook_url, smtp_host, etc.}
    enabled BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE alert_rules (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    metric_name VARCHAR(255) NOT NULL,
    condition VARCHAR(50) NOT NULL, -- 'gt', 'lt', 'eq', 'ne'
    threshold DECIMAL NOT NULL,
    duration_seconds INT DEFAULT 300,
    severity VARCHAR(50) DEFAULT 'warning',
    channel_ids UUID[] DEFAULT '{}',
    enabled BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE alert_history (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL,
    rule_id UUID REFERENCES alert_rules(id) ON DELETE CASCADE,
    status VARCHAR(50) NOT NULL, -- 'firing', 'resolved', 'acknowledged'
    value DECIMAL,
    message TEXT,
    acknowledged_by UUID,
    acknowledged_at TIMESTAMPTZ,
    fired_at TIMESTAMPTZ DEFAULT NOW(),
    resolved_at TIMESTAMPTZ
);

CREATE INDEX idx_alert_rules_tenant ON alert_rules(tenant_id);
CREATE INDEX idx_alert_history_tenant ON alert_history(tenant_id);
CREATE INDEX idx_alert_history_rule ON alert_history(rule_id);
CREATE INDEX idx_alert_history_status ON alert_history(status);
```

### API Endpoints
| Method | Path | Description |
|---|---|---|
| `POST` | `/api/v1/alerts/channels` | Create notification channel |
| `GET` | `/api/v1/alerts/channels` | List channels |
| `POST` | `/api/v1/alerts/rules` | Create alert rule |
| `GET` | `/api/v1/alerts/rules` | List rules |
| `PUT` | `/api/v1/alerts/rules/{id}` | Update rule |
| `DELETE` | `/api/v1/alerts/rules/{id}` | Delete rule |
| `GET` | `/api/v1/alerts/history` | List alert history |
| `POST` | `/api/v1/alerts/history/{id}/acknowledge` | Acknowledge alert |

### Dependencies
- Existing observability/Prometheus metrics integration

---

## 4. Vue.js Migration Foundation — ~16h

### Files to Create
- **`frontend-vue/package.json`** — Vite + Vue 3 + Pinia + TypeScript
- **`frontend-vue/vite.config.ts`**
- **`frontend-vue/src/main.ts`** — App entry
- **`frontend-vue/src/App.vue`**
- **`frontend-vue/src/components/base/`** — BasePanel, BaseTable, BaseModal, BaseBadge
- **`frontend-vue/src/stores/app.ts`** — Pinia wrapper for existing AppState
- **`frontend-vue/src/composables/useWebSocket.ts`** — Wrap ws-client.js
- **`frontend-vue/src/composables/useApi.ts`** — Wrap api-client.js
- **`frontend-vue/src/composables/useAuth.ts`**
- **`frontend-vue/src/views/settings/SettingsView.vue`** — First migrated module

### Strategy
- New `frontend-vue/` directory alongside existing `frontend/`
- Vite dev server proxies API calls to Go backend
- First module (Settings) as proof of concept
- Existing CSS design tokens ported as-is

### Dependencies
- None — can run in parallel with backend tasks

---

## 5. Technical Debt Cleanup — ~6h

| Task | Action | Files |
|---|---|---|
| Remove frontend mocks | Delete | `frontend/analyze_mock.js`, `frontend/find_mock_arrays.js` |
| Sync agent definitions | Align Go (15) and Python (10) agent roles | `internal/domain/agent/`, `orchestrator-agent/app/agent.py` |
| Create skills directory | `mkdir .agents/skills/` | `.agents/skills/` |
| Clean root test file | Delete or move | `test_deployments.go` → `scripts/` |

### Dependencies
- Frontend mocks deletion depends on API completeness

---

## Task Dependency Graph

```
                    ┌──────────────────┐
                    │  Tech Debt (5)   │
                    │     ~6h          │
                    └──────────────────┘
                           (independent)

┌──────────────┐    ┌──────────────────┐    ┌──────────────┐
│ DB Backup (1)│    │ Cluster Import(2)│    │ Vue.js (4)   │
│    ~24h      │    │     ~16h         │    │   ~16h       │
└──────┬───────┘    └────────┬─────────┘    └──────────────┘
       │                     │                (independent)
       │                     │
       └──────────┬──────────┘
                  ▼
         ┌────────────────┐
         │ Alerting (3)   │
         │    ~20h        │
         │ (depends on    │
         │  observability)│
         └────────────────┘
```

**Parallel execution possible:**
- Tasks 1, 2, 4, 5 can start simultaneously
- Task 3 can start after observability review
