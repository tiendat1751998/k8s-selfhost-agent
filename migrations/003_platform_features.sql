-- Migration: 003_platform_features.sql
-- Creates tables for new platform features:
--   * notification_channels    — Multi-channel notification config
--   * notification_history     — Delivery audit log
--   * automation_rules         — Event-driven automation rules
--   * automation_executions    — Execution history
--   * compliance_frameworks    — Compliance framework results
--   * compliance_violations    — Policy violations
--   * runbooks                 — Operational runbook library
--   * slo_definitions          — Service Level Objectives
--   * slo_snapshots            — SLO compliance snapshots
--   * deployment_events        — Deployment timeline events
--   * service_catalog          — Platform engineering service catalog

-- ═══════════════════════════════════════════════════════════
-- 1. Notification Channels
-- ═══════════════════════════════════════════════════════════
CREATE TABLE IF NOT EXISTS notification_channels (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    type            VARCHAR(20)  NOT NULL,                       -- slack | email | teams | telegram | webhook
    name            VARCHAR(255) NOT NULL,
    webhook_url     TEXT         NOT NULL DEFAULT '',
    config          JSONB        NOT NULL DEFAULT '{}',          -- channel-specific config (e.g. smtp host, bot token)
    enabled         BOOLEAN      NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_notif_channels_type ON notification_channels (type);

-- ═══════════════════════════════════════════════════════════
-- 2. Notification History
-- ═══════════════════════════════════════════════════════════
CREATE TABLE IF NOT EXISTS notification_history (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    channel_id      UUID         REFERENCES notification_channels(id) ON DELETE SET NULL,
    title           VARCHAR(500) NOT NULL,
    severity        VARCHAR(20)  NOT NULL DEFAULT 'info',        -- info | warning | critical
    message         TEXT         NOT NULL DEFAULT '',
    status          VARCHAR(20)  NOT NULL DEFAULT 'pending',     -- pending | delivered | failed | retried
    error_detail    TEXT         NOT NULL DEFAULT '',
    read            BOOLEAN      NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_notif_history_status ON notification_history (status);
CREATE INDEX idx_notif_history_read ON notification_history (read);
CREATE INDEX idx_notif_history_created ON notification_history (created_at DESC);

-- ═══════════════════════════════════════════════════════════
-- 3. Automation Rules
-- ═══════════════════════════════════════════════════════════
CREATE TABLE IF NOT EXISTS automation_rules (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name            VARCHAR(255) NOT NULL,
    trigger_type    VARCHAR(50)  NOT NULL,                       -- pod_restart | node_pressure | deployment_failure | high_cpu | slo_breach | error_rate
    trigger_config  JSONB        NOT NULL DEFAULT '{}',          -- condition, threshold, target
    action_type     VARCHAR(50)  NOT NULL,                       -- generate_rca | send_notification | rollback | scale_deployment | create_incident
    action_config   JSONB        NOT NULL DEFAULT '{}',          -- action-specific config
    enabled         BOOLEAN      NOT NULL DEFAULT TRUE,
    executions      INTEGER      NOT NULL DEFAULT 0,
    last_triggered  TIMESTAMPTZ,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_auto_rules_enabled ON automation_rules (enabled);
CREATE INDEX idx_auto_rules_trigger ON automation_rules (trigger_type);

-- ═══════════════════════════════════════════════════════════
-- 4. Automation Executions
-- ═══════════════════════════════════════════════════════════
CREATE TABLE IF NOT EXISTS automation_executions (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    rule_id         UUID         REFERENCES automation_rules(id) ON DELETE CASCADE,
    rule_name       VARCHAR(255) NOT NULL,
    trigger_event   TEXT         NOT NULL DEFAULT '',
    action_taken    TEXT         NOT NULL DEFAULT '',
    result          VARCHAR(20)  NOT NULL DEFAULT 'success',     -- success | failure
    error_detail    TEXT         NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_auto_exec_rule ON automation_executions (rule_id);
CREATE INDEX idx_auto_exec_created ON automation_executions (created_at DESC);

-- ═══════════════════════════════════════════════════════════
-- 5. Compliance Frameworks
-- ═══════════════════════════════════════════════════════════
CREATE TABLE IF NOT EXISTS compliance_frameworks (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name            VARCHAR(255) NOT NULL UNIQUE,                -- CIS Kubernetes Benchmark, SOC 2, PCI-DSS, HIPAA
    icon            VARCHAR(10)  NOT NULL DEFAULT '🛡️',
    total_checks    INTEGER      NOT NULL DEFAULT 0,
    passed_checks   INTEGER      NOT NULL DEFAULT 0,
    failed_checks   INTEGER      NOT NULL DEFAULT 0,
    score           DECIMAL(5,2) NOT NULL DEFAULT 0,             -- percentage
    last_scan_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- ═══════════════════════════════════════════════════════════
-- 6. Compliance Violations
-- ═══════════════════════════════════════════════════════════
CREATE TABLE IF NOT EXISTS compliance_violations (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    framework_id    UUID         REFERENCES compliance_frameworks(id) ON DELETE CASCADE,
    severity        VARCHAR(20)  NOT NULL,                       -- critical | high | medium | low
    policy          VARCHAR(500) NOT NULL,
    resource        VARCHAR(500) NOT NULL,
    namespace       VARCHAR(255) NOT NULL DEFAULT '',
    cluster         VARCHAR(255) NOT NULL DEFAULT '',
    message         TEXT         NOT NULL DEFAULT '',
    resolved        BOOLEAN      NOT NULL DEFAULT FALSE,
    detected_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_compliance_viol_sev ON compliance_violations (severity);
CREATE INDEX idx_compliance_viol_resolved ON compliance_violations (resolved);

-- ═══════════════════════════════════════════════════════════
-- 7. Runbooks
-- ═══════════════════════════════════════════════════════════
CREATE TABLE IF NOT EXISTS runbooks (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    title           VARCHAR(500) NOT NULL,
    category        VARCHAR(100) NOT NULL,                       -- Application | Infrastructure | Database | Security | Network
    content         TEXT         NOT NULL DEFAULT '',             -- Markdown content
    tags            TEXT[]       NOT NULL DEFAULT '{}',
    author          VARCHAR(255) NOT NULL DEFAULT '',
    steps_count     INTEGER      NOT NULL DEFAULT 0,
    last_used_at    TIMESTAMPTZ,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_runbooks_category ON runbooks (category);

-- ═══════════════════════════════════════════════════════════
-- 8. SLO Definitions
-- ═══════════════════════════════════════════════════════════
CREATE TABLE IF NOT EXISTS slo_definitions (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    service         VARCHAR(255) NOT NULL,
    target          DECIMAL(6,3) NOT NULL,                       -- e.g. 99.950
    indicator_type  VARCHAR(50)  NOT NULL DEFAULT 'availability', -- availability | latency | error_rate
    "window"        VARCHAR(20)  NOT NULL DEFAULT '30d',          -- rolling window
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_slo_service ON slo_definitions (service, indicator_type);

-- ═══════════════════════════════════════════════════════════
-- 9. SLO Snapshots
-- ═══════════════════════════════════════════════════════════
CREATE TABLE IF NOT EXISTS slo_snapshots (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    slo_id          UUID         REFERENCES slo_definitions(id) ON DELETE CASCADE,
    actual          DECIMAL(6,3) NOT NULL,                       -- e.g. 99.920
    burn_rate       DECIMAL(5,2) NOT NULL DEFAULT 0,
    error_budget    DECIMAL(5,2) NOT NULL DEFAULT 100,           -- remaining percentage
    budget_status   VARCHAR(20)  NOT NULL DEFAULT 'healthy',     -- healthy | warning | critical
    recorded_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_slo_snap_slo ON slo_snapshots (slo_id);
CREATE INDEX idx_slo_snap_recorded ON slo_snapshots (recorded_at DESC);

-- ═══════════════════════════════════════════════════════════
-- 10. Deployment Events (Timeline)
-- ═══════════════════════════════════════════════════════════
CREATE TABLE IF NOT EXISTS deployment_events (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    type            VARCHAR(30)  NOT NULL,                       -- deployment | rollback | incident | commit | config
    title           VARCHAR(500) NOT NULL,
    detail          TEXT         NOT NULL DEFAULT '',
    namespace       VARCHAR(255) NOT NULL DEFAULT '',
    cluster         VARCHAR(255) NOT NULL DEFAULT '',
    metadata        JSONB        NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_deploy_events_type ON deployment_events (type);
CREATE INDEX idx_deploy_events_created ON deployment_events (created_at DESC);

-- ═══════════════════════════════════════════════════════════
-- 11. Service Catalog (Platform Engineering)
-- ═══════════════════════════════════════════════════════════
CREATE TABLE IF NOT EXISTS service_catalog (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name            VARCHAR(255) NOT NULL UNIQUE,
    icon            VARCHAR(10)  NOT NULL DEFAULT '🔗',
    description     TEXT         NOT NULL DEFAULT '',
    category        VARCHAR(100) NOT NULL DEFAULT 'general',     -- web | api | worker | cronjob | stateful | ml
    tags            TEXT[]       NOT NULL DEFAULT '{}',
    deploy_count    INTEGER      NOT NULL DEFAULT 0,
    template_config JSONB        NOT NULL DEFAULT '{}',          -- default resources, env, replicas
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_svc_catalog_category ON service_catalog (category);
