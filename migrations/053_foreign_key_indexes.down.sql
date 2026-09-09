-- Migration: 053_foreign_key_indexes.down.sql
-- Description: Drops foreign key indexes created in 053_foreign_key_indexes.up.sql.

DROP INDEX IF EXISTS idx_notifications_channel_id;
DROP INDEX IF EXISTS idx_agent_subtasks_task_id;
DROP INDEX IF EXISTS idx_agent_executions_task_id;
DROP INDEX IF EXISTS idx_projects_org_id;
DROP INDEX IF EXISTS idx_tenant_members_org_id;
DROP INDEX IF EXISTS idx_tenant_bindings_user_id;
DROP INDEX IF EXISTS idx_tenant_bindings_role_id;
DROP INDEX IF EXISTS idx_user_roles_role_id;
DROP INDEX IF EXISTS idx_backup_policies_storage_id;
DROP INDEX IF EXISTS idx_backup_policies_secondary_storage_id;
DROP INDEX IF EXISTS idx_backup_jobs_policy_id;
DROP INDEX IF EXISTS idx_restore_jobs_backup_job_id;
DROP INDEX IF EXISTS idx_compliance_violations_framework_id;
