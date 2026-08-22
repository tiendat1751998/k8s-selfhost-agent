-- Migration: 042_enterprise_governance_seeds.down.sql
-- Description: Rolls back enterprise governance, security, tenancy, and disaster recovery seed data.

DELETE FROM compliance_violations WHERE id::text LIKE 'c1000001%';
DELETE FROM compliance_frameworks WHERE id::text LIKE 'c0000001%';
DELETE FROM restore_jobs WHERE id::text LIKE 'd3000001%';
DELETE FROM backup_jobs WHERE id::text LIKE 'd2000001%';
DELETE FROM backup_policies WHERE id::text LIKE 'd1000001%';
DELETE FROM backup_storages WHERE id::text LIKE 'd0000001%';
DELETE FROM audit_findings WHERE id::text LIKE 'e1000001%';
DELETE FROM audit_runs WHERE id::text LIKE 'e0000001%';
DELETE FROM drift_records WHERE id::text LIKE 'f0000001%';
DELETE FROM tenant_members WHERE id::text LIKE 'b0000001%';
DELETE FROM projects WHERE id IN ('proj-core-platform', 'proj-telemetry-sec', 'proj-data-pipeline');
