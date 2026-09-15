-- Migration: 059_fix_audit_logs_tenant_id_type.up.sql
-- Description: Ensure audit_logs tenant_id is VARCHAR(255) to support slug/string tenant identifiers.

ALTER TABLE audit_logs ALTER COLUMN tenant_id TYPE VARCHAR(255);
ALTER TABLE audit_logs ALTER COLUMN tenant_id SET DEFAULT 'default-tenant';
