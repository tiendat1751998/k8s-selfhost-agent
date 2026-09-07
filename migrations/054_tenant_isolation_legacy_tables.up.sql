-- Migration: 054_tenant_isolation_legacy_tables.up.sql
-- Description: Adds tenant_id column and index to remaining legacy tables for Zero-Trust tenant scoping.

ALTER TABLE agent_tasks 
    ADD COLUMN IF NOT EXISTS tenant_id VARCHAR(255) NOT NULL DEFAULT 'default-tenant';
CREATE INDEX IF NOT EXISTS idx_agent_tasks_tenant ON agent_tasks(tenant_id);

ALTER TABLE agent_subtasks 
    ADD COLUMN IF NOT EXISTS tenant_id VARCHAR(255) NOT NULL DEFAULT 'default-tenant';
CREATE INDEX IF NOT EXISTS idx_agent_subtasks_tenant ON agent_subtasks(tenant_id);

ALTER TABLE agent_executions 
    ADD COLUMN IF NOT EXISTS tenant_id VARCHAR(255) NOT NULL DEFAULT 'default-tenant';
CREATE INDEX IF NOT EXISTS idx_agent_executions_tenant ON agent_executions(tenant_id);

ALTER TABLE agent_project_state 
    ADD COLUMN IF NOT EXISTS tenant_id VARCHAR(255) NOT NULL DEFAULT 'default-tenant';
CREATE INDEX IF NOT EXISTS idx_agent_project_state_tenant ON agent_project_state(tenant_id);

ALTER TABLE audit_findings 
    ADD COLUMN IF NOT EXISTS tenant_id VARCHAR(255) NOT NULL DEFAULT 'default-tenant';
CREATE INDEX IF NOT EXISTS idx_audit_findings_tenant ON audit_findings(tenant_id);

ALTER TABLE audit_runs 
    ADD COLUMN IF NOT EXISTS tenant_id VARCHAR(255) NOT NULL DEFAULT 'default-tenant';
CREATE INDEX IF NOT EXISTS idx_audit_runs_tenant ON audit_runs(tenant_id);

ALTER TABLE audit_logs 
    ADD COLUMN IF NOT EXISTS tenant_id VARCHAR(255) NOT NULL DEFAULT 'default-tenant';
CREATE INDEX IF NOT EXISTS idx_audit_logs_tenant ON audit_logs(tenant_id);

ALTER TABLE automation_rules 
    ADD COLUMN IF NOT EXISTS tenant_id VARCHAR(255) NOT NULL DEFAULT 'default-tenant';
CREATE INDEX IF NOT EXISTS idx_automation_rules_tenant ON automation_rules(tenant_id);

ALTER TABLE automation_executions 
    ADD COLUMN IF NOT EXISTS tenant_id VARCHAR(255) NOT NULL DEFAULT 'default-tenant';
CREATE INDEX IF NOT EXISTS idx_automation_executions_tenant ON automation_executions(tenant_id);
