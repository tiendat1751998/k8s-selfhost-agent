-- Migration: 026_tenant_isolation.down.sql
-- Description: Drops tenant_id columns and indexes from core tables.

DROP INDEX IF EXISTS idx_gitops_prs_tenant;
DROP INDEX IF EXISTS idx_rca_reports_tenant;
DROP INDEX IF EXISTS idx_incidents_tenant;

ALTER TABLE gitops_prs DROP COLUMN IF EXISTS tenant_id;
ALTER TABLE rca_reports DROP COLUMN IF EXISTS tenant_id;
ALTER TABLE incidents DROP COLUMN IF EXISTS tenant_id;
