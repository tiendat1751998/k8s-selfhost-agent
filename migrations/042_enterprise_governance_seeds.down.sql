-- Migration: 042_enterprise_governance_seeds.down.sql
-- Description: Rolls back enterprise governance, security, tenancy, and disaster recovery seed data.

DELETE FROM compliance_violations WHERE id LIKE 'c1000001%';
DELETE FROM compliance_frameworks WHERE id LIKE 'c0000001%';
DELETE FROM restore_jobs WHERE id LIKE 'd3000001%';
DELETE FROM backup_jobs WHERE id LIKE 'd2000001%';
DELETE FROM backup_policies WHERE id LIKE 'd1000001%';
DELETE FROM backup_storages WHERE id LIKE 'd0000001%';
DELETE FROM audit_findings WHERE id LIKE 'e1000001%';
DELETE FROM audit_runs WHERE id LIKE 'e0000001%';
DELETE FROM drift_records WHERE id LIKE 'f0000001%';
DELETE FROM tenant_members WHERE id LIKE 'b0000001%';
DELETE FROM projects WHERE id IN ('proj-core-platform', 'proj-telemetry-sec', 'proj-data-pipeline');
