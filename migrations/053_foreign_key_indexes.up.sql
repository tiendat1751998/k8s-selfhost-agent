-- Migration: 053_foreign_key_indexes.up.sql
-- Description: Creates indexes for foreign keys across platform tables to eliminate full table scans on JOINs and cascading deletes.

-- 1. Notification system foreign keys
CREATE INDEX IF NOT EXISTS idx_notifications_channel_id ON notifications(channel_id);

-- 2. Agent framework foreign keys
CREATE INDEX IF NOT EXISTS idx_agent_subtasks_task_id ON agent_subtasks(task_id);
CREATE INDEX IF NOT EXISTS idx_agent_executions_task_id ON agent_executions(task_id);

-- 3. Multi-tenancy foreign keys
CREATE INDEX IF NOT EXISTS idx_projects_org_id ON projects(org_id);
CREATE INDEX IF NOT EXISTS idx_tenant_members_org_id ON tenant_members(org_id);
CREATE INDEX IF NOT EXISTS idx_tenant_bindings_user_id ON tenant_bindings(user_id);
CREATE INDEX IF NOT EXISTS idx_tenant_bindings_role_id ON tenant_bindings(role_id);
CREATE INDEX IF NOT EXISTS idx_user_roles_role_id ON user_roles(role_id);

-- 4. Database backup & restore foreign keys
CREATE INDEX IF NOT EXISTS idx_backup_policies_storage_id ON backup_policies(storage_id);
CREATE INDEX IF NOT EXISTS idx_backup_policies_secondary_storage_id ON backup_policies(secondary_storage_id);
CREATE INDEX IF NOT EXISTS idx_backup_jobs_policy_id ON backup_jobs(policy_id);
CREATE INDEX IF NOT EXISTS idx_restore_jobs_backup_job_id ON restore_jobs(backup_job_id);

-- 5. Compliance foreign keys
CREATE INDEX IF NOT EXISTS idx_compliance_violations_framework_id ON compliance_violations(framework_id);
