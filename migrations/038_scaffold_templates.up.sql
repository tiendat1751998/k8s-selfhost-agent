-- Migration: 038_scaffold_templates.up.sql
-- Description: Creates scaffold_templates table for 1-click application scaffolding.

DROP TABLE IF EXISTS scaffold_templates;

CREATE TABLE IF NOT EXISTS scaffold_templates (
    id             TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    name           TEXT NOT NULL,
    description    TEXT NOT NULL DEFAULT '',
    category       TEXT NOT NULL DEFAULT 'web',
    framework      TEXT NOT NULL DEFAULT 'go-chi',
    manifest_yaml  TEXT NOT NULL DEFAULT '',
    helm_values    TEXT NOT NULL DEFAULT '',
    docker_compose TEXT NOT NULL DEFAULT '',
    variables      JSONB NOT NULL DEFAULT '[]',
    tags           JSONB NOT NULL DEFAULT '[]',
    built_in       BOOLEAN NOT NULL DEFAULT false,
    tenant_id      TEXT NOT NULL DEFAULT 'default-tenant',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(name, tenant_id)
);

CREATE INDEX IF NOT EXISTS idx_scaffold_tenant ON scaffold_templates(tenant_id);
CREATE INDEX IF NOT EXISTS idx_scaffold_category ON scaffold_templates(category);
CREATE INDEX IF NOT EXISTS idx_scaffold_framework ON scaffold_templates(framework);
CREATE INDEX IF NOT EXISTS idx_scaffold_builtin ON scaffold_templates(built_in);
