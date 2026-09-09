-- Migration: 054_tenant_isolation_legacy_tables.down.sql
-- Description: Drops tenant_id columns and indexes from legacy tables.

DROP INDEX IF EXISTS idx_automation_executions_tenant;
ALTER TABLE automation_executions DROP COLUMN IF EXISTS tenant_id;

DROP INDEX IF EXISTS idx_automation_rules_tenant;
ALTER TABLE automation_rules DROP COLUMN IF EXISTS tenant_id;

DROP INDEX IF EXISTS idx_audit_logs_tenant;
ALTER TABLE audit_logs DROP COLUMN IF EXISTS tenant_id;

DROP INDEX IF EXISTS idx_audit_runs_tenant;
ALTER TABLE audit_runs DROP COLUMN IF EXISTS tenant_id;

DROP INDEX IF EXISTS idx_audit_findings_tenant;
ALTER TABLE audit_findings DROP COLUMN IF EXISTS tenant_id;

DROP INDEX IF EXISTS idx_agent_project_state_tenant;
ALTER TABLE agent_project_state DROP COLUMN IF EXISTS tenant_id;

DROP INDEX IF EXISTS idx_agent_executions_tenant;
ALTER TABLE agent_executions DROP COLUMN IF EXISTS tenant_id;

DROP INDEX IF EXISTS idx_agent_subtasks_tenant;
ALTER TABLE agent_subtasks DROP COLUMN IF EXISTS tenant_id;

DROP INDEX IF EXISTS idx_agent_tasks_tenant;
ALTER TABLE agent_tasks DROP COLUMN IF EXISTS tenant_id;
