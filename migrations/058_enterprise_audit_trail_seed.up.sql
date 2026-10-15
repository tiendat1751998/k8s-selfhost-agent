-- Migration: 058_enterprise_audit_trail_seed.up.sql
-- Description: Ensure audit_logs tenant_id column exists. Demo seed data purged.

ALTER TABLE audit_logs ADD COLUMN IF NOT EXISTS tenant_id VARCHAR(255) DEFAULT 'default-tenant';

-- Demo seed data purged: fresh setups never seed fake audit events.
SELECT 1;
