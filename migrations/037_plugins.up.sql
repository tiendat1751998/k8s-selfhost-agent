-- Migration: 037_plugins.up.sql
-- Description: Creates plugins table for runtime frontend plugin system.

DROP TABLE IF EXISTS plugins;

CREATE TABLE IF NOT EXISTS plugins (
    id           TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    name         TEXT NOT NULL,
    version      TEXT NOT NULL DEFAULT '1.0.0',
    description  TEXT NOT NULL DEFAULT '',
    author       TEXT NOT NULL DEFAULT '',
    enabled      BOOLEAN NOT NULL DEFAULT true,
    entry_point  TEXT NOT NULL DEFAULT '',
    icon         TEXT NOT NULL DEFAULT '',
    category     TEXT NOT NULL DEFAULT 'devtools',
    permissions  JSONB NOT NULL DEFAULT '[]',
    config       JSONB NOT NULL DEFAULT '{}',
    tenant_id    TEXT NOT NULL DEFAULT 'default-tenant',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(name, tenant_id)
);
CREATE INDEX IF NOT EXISTS idx_plugins_tenant ON plugins(tenant_id);
CREATE INDEX IF NOT EXISTS idx_plugins_enabled ON plugins(enabled);
CREATE INDEX IF NOT EXISTS idx_plugins_category ON plugins(category);
