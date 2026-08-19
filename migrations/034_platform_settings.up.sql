-- Migration: 034_platform_settings.up.sql
-- Description: Creates platform_settings table for category- and tenant-scoped configuration.

CREATE TABLE IF NOT EXISTS platform_settings (
    id         TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    category   TEXT NOT NULL,
    key        TEXT NOT NULL,
    value      TEXT NOT NULL DEFAULT '',
    tenant_id  TEXT NOT NULL DEFAULT 'default-tenant',
    updated_by TEXT DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(category, key, tenant_id)
);

CREATE INDEX IF NOT EXISTS idx_settings_tenant_category ON platform_settings(tenant_id, category);
