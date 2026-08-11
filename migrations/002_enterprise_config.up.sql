-- Migration: 002_enterprise_config.sql
-- Creates tables for the Enterprise Configuration Control Plane:
--   * kubernetes_clusters   — K8s cluster registry
--   * git_providers         — GitHub/GitLab/Gitea providers
--   * cicd_integrations     — CI/CD pipeline integrations
--   * ai_providers          — AI/LLM provider configs
--   * connection_health     — Health check snapshots
--   * audit_logs            — Configuration audit trail
--   * agent_runs            — AI Agent pipeline execution history

-- ═══════════════════════════════════════════════════════════
-- 1. Container Clusters (Kubernetes / Docker / Docker Swarm)
-- ═══════════════════════════════════════════════════════════
CREATE TABLE IF NOT EXISTS container_clusters (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name            VARCHAR(255) NOT NULL UNIQUE,
    provider        VARCHAR(20)  NOT NULL DEFAULT 'kubernetes',  -- kubernetes | docker | docker_swarm
    api_endpoint    VARCHAR(500) NOT NULL DEFAULT '',            -- K8s API / Docker API / Swarm manager URL
    socket_path     VARCHAR(500) NOT NULL DEFAULT '',            -- Docker socket path (e.g. /var/run/docker.sock)
    auth_type       VARCHAR(50)  NOT NULL DEFAULT 'token',      -- token | kubeconfig | service_account | tls | socket
    auth_data       TEXT         NOT NULL DEFAULT '',            -- encrypted token or kubeconfig
    ca_cert         TEXT         NOT NULL DEFAULT '',            -- CA certificate (PEM)
    tls_cert        TEXT         NOT NULL DEFAULT '',            -- TLS client cert (Docker TLS)
    tls_key         TEXT         NOT NULL DEFAULT '',            -- TLS client key (Docker TLS, encrypted)
    status          VARCHAR(20)  NOT NULL DEFAULT 'unknown',    -- healthy | degraded | down | unknown
    nodes_count     INTEGER      NOT NULL DEFAULT 0,            -- K8s nodes / Swarm nodes
    pods_count      INTEGER      NOT NULL DEFAULT 0,            -- K8s pods / Docker containers
    services_count  INTEGER      NOT NULL DEFAULT 0,            -- Swarm services / K8s services
    cpu_usage       VARCHAR(10)  NOT NULL DEFAULT '',            -- e.g. "67%"
    memory_usage    VARCHAR(10)  NOT NULL DEFAULT '',            -- e.g. "72%"
    docker_version  VARCHAR(50)  NOT NULL DEFAULT '',            -- Docker engine version
    swarm_manager   BOOLEAN      NOT NULL DEFAULT FALSE,        -- Is this a Swarm manager node?
    metadata        JSONB        NOT NULL DEFAULT '{}',          -- labels, annotations, extra info
    last_check_at   TIMESTAMPTZ,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_clusters_provider ON container_clusters (provider);
CREATE INDEX idx_clusters_status ON container_clusters (status);
CREATE INDEX idx_clusters_name ON container_clusters (name);

-- ═══════════════════════════════════════════════════════════
-- 2. Git Providers
-- ═══════════════════════════════════════════════════════════
CREATE TABLE IF NOT EXISTS git_providers (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    type            VARCHAR(20)  NOT NULL,                       -- github | gitlab | gitea
    organization    VARCHAR(255) NOT NULL,
    api_url         VARCHAR(500) NOT NULL DEFAULT '',            -- API base URL (for self-hosted)
    access_token    TEXT         NOT NULL DEFAULT '',            -- encrypted access token
    webhook_secret  TEXT         NOT NULL DEFAULT '',            -- encrypted webhook secret
    repo_count      INTEGER      NOT NULL DEFAULT 0,
    sync_status     VARCHAR(20)  NOT NULL DEFAULT 'pending',    -- synced | syncing | failed | pending
    last_webhook_at TIMESTAMPTZ,
    metadata        JSONB        NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_git_providers_type ON git_providers (type);
CREATE INDEX idx_git_providers_sync_status ON git_providers (sync_status);
CREATE UNIQUE INDEX idx_git_providers_type_org ON git_providers (type, organization);

-- ═══════════════════════════════════════════════════════════
-- 3. CI/CD Integrations
-- ═══════════════════════════════════════════════════════════
CREATE TABLE IF NOT EXISTS cicd_integrations (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name            VARCHAR(255) NOT NULL UNIQUE,
    provider        VARCHAR(50)  NOT NULL,                       -- github_actions | gitlab_ci | jenkins | argocd
    webhook_url     VARCHAR(500) NOT NULL DEFAULT '',
    webhook_secret  TEXT         NOT NULL DEFAULT '',            -- encrypted
    status          VARCHAR(20)  NOT NULL DEFAULT 'inactive',   -- active | inactive | failing
    success_rate    DECIMAL(5,2) NOT NULL DEFAULT 0,             -- 0.00 to 100.00
    total_runs      INTEGER      NOT NULL DEFAULT 0,
    total_success   INTEGER      NOT NULL DEFAULT 0,
    total_failures  INTEGER      NOT NULL DEFAULT 0,
    last_run_at     TIMESTAMPTZ,
    last_log        TEXT         NOT NULL DEFAULT '',
    metadata        JSONB        NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_cicd_status ON cicd_integrations (status);
CREATE INDEX idx_cicd_provider ON cicd_integrations (provider);

-- ═══════════════════════════════════════════════════════════
-- 4. AI Providers
-- ═══════════════════════════════════════════════════════════
CREATE TABLE IF NOT EXISTS ai_providers (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name            VARCHAR(255) NOT NULL UNIQUE,
    provider_type   VARCHAR(50)  NOT NULL,                       -- ollama | openai | vllm
    model           VARCHAR(255) NOT NULL,                       -- llama3:8b | gpt-4o | codellama:34b
    endpoint        VARCHAR(500) NOT NULL,
    api_key         TEXT         NOT NULL DEFAULT '',            -- encrypted API key
    status          VARCHAR(20)  NOT NULL DEFAULT 'unknown',    -- healthy | degraded | down | unknown
    latency_ms      INTEGER      NOT NULL DEFAULT 0,
    total_requests  BIGINT       NOT NULL DEFAULT 0,
    total_tokens    BIGINT       NOT NULL DEFAULT 0,
    avg_latency_ms  INTEGER      NOT NULL DEFAULT 0,
    max_tokens      INTEGER      NOT NULL DEFAULT 4096,
    temperature     DECIMAL(3,2) NOT NULL DEFAULT 0.70,
    metadata        JSONB        NOT NULL DEFAULT '{}',
    last_check_at   TIMESTAMPTZ,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_ai_providers_status ON ai_providers (status);
CREATE INDEX idx_ai_providers_type ON ai_providers (provider_type);

-- ═══════════════════════════════════════════════════════════
-- 5. Connection Health Checks
-- ═══════════════════════════════════════════════════════════
CREATE TABLE IF NOT EXISTS connection_health (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    system_type     VARCHAR(20)  NOT NULL,                       -- kubernetes | git | cicd | ai
    system_id       UUID,                                        -- FK reference (nullable for system-wide checks)
    system_name     VARCHAR(255) NOT NULL DEFAULT '',
    status          VARCHAR(20)  NOT NULL DEFAULT 'unknown',    -- healthy | degraded | down | unknown
    latency_ms      INTEGER      NOT NULL DEFAULT 0,
    error_message   TEXT         NOT NULL DEFAULT '',
    details         JSONB        NOT NULL DEFAULT '{}',
    checked_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_health_system_type ON connection_health (system_type);
CREATE INDEX idx_health_status ON connection_health (status);
CREATE INDEX idx_health_checked_at ON connection_health (checked_at DESC);
-- Cleanup: keep last 1000 records per system_type (use cron or app-level pruning)

-- ═══════════════════════════════════════════════════════════
-- 6. Audit Logs
-- ═══════════════════════════════════════════════════════════
CREATE TABLE IF NOT EXISTS audit_logs (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    actor           VARCHAR(255) NOT NULL DEFAULT 'system',      -- user or system identifier
    action          VARCHAR(50)  NOT NULL,                       -- create | update | delete | test | trigger | sync | rotate
    target_type     VARCHAR(50)  NOT NULL,                       -- kubernetes | git | cicd | ai | system
    target_id       UUID,                                        -- FK to the modified resource (nullable)
    target_name     VARCHAR(500) NOT NULL DEFAULT '',            -- human-readable target identifier
    result          VARCHAR(20)  NOT NULL DEFAULT 'success',    -- success | failure | pending
    details         JSONB        NOT NULL DEFAULT '{}',          -- change diff, error details, etc.
    ip_address      VARCHAR(45)  NOT NULL DEFAULT '',            -- client IP
    user_agent      VARCHAR(500) NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_audit_actor ON audit_logs (actor);
CREATE INDEX idx_audit_action ON audit_logs (action);
CREATE INDEX idx_audit_target_type ON audit_logs (target_type);
CREATE INDEX idx_audit_result ON audit_logs (result);
CREATE INDEX idx_audit_created_at ON audit_logs (created_at DESC);
CREATE INDEX idx_audit_target ON audit_logs (target_type, target_name);

-- ═══════════════════════════════════════════════════════════
-- 7. Agent Runs (execution history)
-- ═══════════════════════════════════════════════════════════
CREATE TABLE IF NOT EXISTS agent_runs (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    incident_id     UUID         REFERENCES incidents(id) ON DELETE SET NULL,
    cluster_name    VARCHAR(255) NOT NULL DEFAULT '',
    status          VARCHAR(20)  NOT NULL DEFAULT 'running',    -- running | success | failed | cancelled
    current_step    VARCHAR(20)  NOT NULL DEFAULT 'TASK',       -- TASK | LLM | CODE | TEST | FIX | COMMIT | PR
    steps_completed JSONB        NOT NULL DEFAULT '{}',          -- { "TASK": {"status":"success","duration_ms":1200}, ... }
    total_duration  INTEGER      NOT NULL DEFAULT 0,             -- total ms
    error_message   TEXT         NOT NULL DEFAULT '',
    metadata        JSONB        NOT NULL DEFAULT '{}',
    started_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    completed_at    TIMESTAMPTZ
);

CREATE INDEX idx_agent_runs_status ON agent_runs (status);
CREATE INDEX idx_agent_runs_incident ON agent_runs (incident_id);
CREATE INDEX idx_agent_runs_started ON agent_runs (started_at DESC);

-- ═══════════════════════════════════════════════════════════
-- Utility: auto-update updated_at trigger
-- ═══════════════════════════════════════════════════════════
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Apply trigger to all config tables
CREATE TRIGGER trg_clusters_updated       BEFORE UPDATE ON container_clusters    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER trg_git_providers_updated  BEFORE UPDATE ON git_providers          FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER trg_cicd_updated           BEFORE UPDATE ON cicd_integrations     FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER trg_ai_providers_updated   BEFORE UPDATE ON ai_providers          FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
-- Also apply to existing tables from 001
CREATE TRIGGER trg_incidents_updated      BEFORE UPDATE ON incidents              FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
CREATE TRIGGER trg_gitops_prs_updated     BEFORE UPDATE ON gitops_prs            FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
