-- Migration: 035_service_catalog.up.sql
-- Description: Creates service_catalog table for Backstage-inspired service catalog registry.

DROP TABLE IF EXISTS service_catalog;

CREATE TABLE IF NOT EXISTS service_catalog (
    id           TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    name         TEXT NOT NULL,
    description  TEXT NOT NULL DEFAULT '',
    type         TEXT NOT NULL DEFAULT 'service',
    lifecycle    TEXT NOT NULL DEFAULT 'development',
    owner_team   TEXT NOT NULL DEFAULT '',
    owner_email  TEXT NOT NULL DEFAULT '',
    repo_url     TEXT NOT NULL DEFAULT '',
    docs_url     TEXT NOT NULL DEFAULT '',
    tags         JSONB NOT NULL DEFAULT '[]',
    annotations  JSONB NOT NULL DEFAULT '{}',
    tenant_id    TEXT NOT NULL DEFAULT 'default-tenant',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(name, tenant_id)
);
CREATE INDEX IF NOT EXISTS idx_catalog_tenant ON service_catalog(tenant_id);
CREATE INDEX IF NOT EXISTS idx_catalog_type ON service_catalog(type);
CREATE INDEX IF NOT EXISTS idx_catalog_lifecycle ON service_catalog(lifecycle);
CREATE INDEX IF NOT EXISTS idx_catalog_owner ON service_catalog(owner_team);
