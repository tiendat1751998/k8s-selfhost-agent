-- Migration: 032_clean_seed_data.up.sql
-- Description: Clean ALL seed/demo data from the database, keeping only real operational data and admin user.

-- 1. Ensure default tenant/organization exists for real platform admin & operational data
INSERT INTO organizations (id, name, tier)
VALUES ('default-tenant', 'Default Organization', 'Enterprise')
ON CONFLICT (id) DO NOTHING;

-- Update tenant_id default for core tables to 'default-tenant'
ALTER TABLE incidents ALTER COLUMN tenant_id SET DEFAULT 'default-tenant';
ALTER TABLE rca_reports ALTER COLUMN tenant_id SET DEFAULT 'default-tenant';
ALTER TABLE gitops_prs ALTER COLUMN tenant_id SET DEFAULT 'default-tenant';

-- 2. Agent data: clean tasks, subtasks, executions, runs, and reset project state (023)
DELETE FROM agent_executions;
DELETE FROM agent_subtasks;
DELETE FROM agent_tasks;
DELETE FROM agent_runs;
DELETE FROM agent_project_state;
INSERT INTO agent_project_state (id, current_phase, current_module, current_feature, repository_health, technical_debt, architecture_score, quality_score, updated_at)
VALUES ('latest', '', '', '', 1.0, 0.0, 0.99, 1.0, NOW())
ON CONFLICT (id) DO UPDATE SET
    current_phase = '',
    current_module = '',
    current_feature = '',
    current_task_id = NULL,
    current_subtask_id = NULL,
    repository_health = 1.0,
    technical_debt = 0.0,
    architecture_score = 0.99,
    quality_score = 1.0,
    updated_at = NOW();

-- 3. Cost & Backup seed data (022)
DELETE FROM correlated_events;
DELETE FROM promotions;
DELETE FROM cluster_costs;
DELETE FROM namespace_costs;
DELETE FROM resource_waste;
DELETE FROM backup_history;

-- 4. Observability SLO seeds (024)
DELETE FROM slo_snapshots;
DELETE FROM slo_definitions;

-- 5. Platform Audit seeds (014) & audit logs/runs
DELETE FROM audit_findings;
DELETE FROM audit_runs;
DELETE FROM audit_logs;

-- 6. Tenancy seed data (021, 025)
-- Delete demo tenant projects, members, and demo orgs
DELETE FROM projects;
DELETE FROM tenant_members;
DELETE FROM organizations WHERE id IN ('org-google', 'org-acme', 'audit-org', 'audit-corp', 'audit-corp-2');

-- 7. Dummy fleet clusters (013) & drift records (005)
DELETE FROM drift_records WHERE cluster IN ('cls-prod-us-east-1', 'cls-prod-eu-west-1', 'cls-stg-us-east-1', 'cls-edge-tokyo', 'cls-dev-local') OR cluster NOT IN (SELECT id FROM fleet_clusters WHERE id NOT IN ('cls-prod-us-east-1', 'cls-prod-eu-west-1', 'cls-stg-us-east-1', 'cls-edge-tokyo', 'cls-dev-local'));
DELETE FROM fleet_clusters WHERE id IN ('cls-prod-us-east-1', 'cls-prod-eu-west-1', 'cls-stg-us-east-1', 'cls-edge-tokyo', 'cls-dev-local');

-- 8. Other demo/test data: compliance, automation, alerts, backups, notifications, changes, reports, tags, providers
DELETE FROM compliance_violations;
DELETE FROM compliance_frameworks;
DELETE FROM automation_executions;
DELETE FROM automation_rules;
DELETE FROM alert_history;
DELETE FROM alert_rules;
DELETE FROM notifications;
DELETE FROM notification_history;
DELETE FROM notification_channels;
DELETE FROM restore_jobs;
DELETE FROM backup_jobs;
DELETE FROM backup_policies;
DELETE FROM backup_storages;
DELETE FROM change_requests;
DELETE FROM maintenance_windows;
DELETE FROM reports;
DELETE FROM resource_tags;
DELETE FROM tags;
DELETE FROM timeline_events;
DELETE FROM deployment_events;
DELETE FROM runbooks;
DELETE FROM service_catalog;
DELETE FROM capacity_forecasts;
DELETE FROM connection_health;
DELETE FROM container_clusters;
DELETE FROM cicd_integrations;
DELETE FROM git_providers;
DELETE FROM ai_providers;
DELETE FROM explorer_resources;
DELETE FROM rca_reports;
DELETE FROM gitops_prs;
DELETE FROM incidents;

-- 9. Clean demo users while keeping real admin user (admin@k8s.local)
DELETE FROM users WHERE email != 'admin@k8s.local';

-- Deduplicate tenant_bindings
DELETE FROM tenant_bindings WHERE id NOT IN (
    SELECT DISTINCT ON (user_id, tenant_id, role_id) id
    FROM tenant_bindings
    ORDER BY user_id, tenant_id, role_id, created_at ASC
);

-- Ensure real admin (admin@k8s.local) has proper bindings
INSERT INTO user_roles (user_id, role_id)
SELECT u.id, r.id FROM users u, roles r
WHERE u.email = 'admin@k8s.local' AND r.name = 'platform_admin'
ON CONFLICT DO NOTHING;

INSERT INTO tenant_bindings (user_id, tenant_id, role_id)
SELECT u.id, 'default-tenant', r.id FROM users u, roles r
WHERE u.email = 'admin@k8s.local' AND r.name = 'platform_admin'
AND NOT EXISTS (
    SELECT 1 FROM tenant_bindings tb
    WHERE tb.user_id = u.id AND tb.tenant_id = 'default-tenant' AND tb.role_id = r.id
);
