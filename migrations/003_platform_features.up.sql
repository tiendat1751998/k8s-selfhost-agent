-- Migration: 003_platform_features.sql
-- NOTE: Tables runbooks, automation_*, notification_*, slo_* moved to migrations 016-019.
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
