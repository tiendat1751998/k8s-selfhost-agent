-- Migration: 059_fix_audit_logs_tenant_id_type.down.sql
-- Description: Revert audit_logs tenant_id back to UUID.

ALTER TABLE audit_logs ALTER COLUMN tenant_id DROP DEFAULT;
ALTER TABLE audit_logs ALTER COLUMN tenant_id TYPE UUID USING (
    CASE WHEN tenant_id ~ '^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$' 
    THEN tenant_id::uuid 
    ELSE NULL END
);
