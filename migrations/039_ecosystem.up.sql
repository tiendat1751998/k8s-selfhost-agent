-- Migration: 039_ecosystem.up.sql
-- Description: Creates ecosystem_tools table for platform tool auto-detection.

CREATE TABLE IF NOT EXISTS ecosystem_tools (
    id           TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    name         TEXT NOT NULL,
    category     TEXT NOT NULL,
    status       TEXT NOT NULL DEFAULT 'detected',
    version      TEXT NOT NULL DEFAULT '',
    endpoint     TEXT NOT NULL DEFAULT '',
    source       TEXT NOT NULL DEFAULT 'settings',
    health       TEXT NOT NULL DEFAULT 'healthy',
    last_checked TIMESTAMPTZ NOT NULL DEFAULT now(),
    metadata     JSONB NOT NULL DEFAULT '{}',
    tenant_id    TEXT NOT NULL DEFAULT 'default-tenant',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(name, tenant_id)
);

CREATE INDEX IF NOT EXISTS idx_ecosystem_tenant ON ecosystem_tools(tenant_id);
CREATE INDEX IF NOT EXISTS idx_ecosystem_category ON ecosystem_tools(category);
CREATE INDEX IF NOT EXISTS idx_ecosystem_status ON ecosystem_tools(status);
CREATE INDEX IF NOT EXISTS idx_ecosystem_health ON ecosystem_tools(health);
