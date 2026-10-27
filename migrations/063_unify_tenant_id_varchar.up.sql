-- Migration: 063_unify_tenant_id_varchar.up.sql
-- Description: Unifies tenant_id column type from UUID to VARCHAR(255) NOT NULL DEFAULT 'default-tenant'
-- across all operational tables modified in migration 057 to prevent SQLSTATE 22P02 collisions.

-- 1. backup_history
ALTER TABLE backup_history ALTER COLUMN tenant_id TYPE VARCHAR(255) USING (
    CASE WHEN tenant_id IS NULL OR tenant_id::text IN ('00000000-0000-0000-0000-000000000000', '00000000-0000-0000-0000-000000000001') THEN 'default-tenant' ELSE tenant_id::text END
);
ALTER TABLE backup_history ALTER COLUMN tenant_id SET DEFAULT 'default-tenant';
UPDATE backup_history SET tenant_id = 'default-tenant' WHERE tenant_id IS NULL OR tenant_id = '';
ALTER TABLE backup_history ALTER COLUMN tenant_id SET NOT NULL;
DROP INDEX IF EXISTS idx_backup_history_tenant_id;
CREATE INDEX IF NOT EXISTS idx_backup_history_tenant_id ON backup_history(tenant_id);

-- 2. change_requests
ALTER TABLE change_requests ALTER COLUMN tenant_id TYPE VARCHAR(255) USING (
    CASE WHEN tenant_id IS NULL OR tenant_id::text IN ('00000000-0000-0000-0000-000000000000', '00000000-0000-0000-0000-000000000001') THEN 'default-tenant' ELSE tenant_id::text END
);
ALTER TABLE change_requests ALTER COLUMN tenant_id SET DEFAULT 'default-tenant';
UPDATE change_requests SET tenant_id = 'default-tenant' WHERE tenant_id IS NULL OR tenant_id = '';
ALTER TABLE change_requests ALTER COLUMN tenant_id SET NOT NULL;
DROP INDEX IF EXISTS idx_change_requests_tenant_id;
CREATE INDEX IF NOT EXISTS idx_change_requests_tenant_id ON change_requests(tenant_id);

-- 3. maintenance_windows
ALTER TABLE maintenance_windows ALTER COLUMN tenant_id TYPE VARCHAR(255) USING (
    CASE WHEN tenant_id IS NULL OR tenant_id::text IN ('00000000-0000-0000-0000-000000000000', '00000000-0000-0000-0000-000000000001') THEN 'default-tenant' ELSE tenant_id::text END
);
ALTER TABLE maintenance_windows ALTER COLUMN tenant_id SET DEFAULT 'default-tenant';
UPDATE maintenance_windows SET tenant_id = 'default-tenant' WHERE tenant_id IS NULL OR tenant_id = '';
ALTER TABLE maintenance_windows ALTER COLUMN tenant_id SET NOT NULL;
DROP INDEX IF EXISTS idx_maintenance_windows_tenant_id;
CREATE INDEX IF NOT EXISTS idx_maintenance_windows_tenant_id ON maintenance_windows(tenant_id);

-- 4. compliance_frameworks
ALTER TABLE compliance_frameworks ALTER COLUMN tenant_id TYPE VARCHAR(255) USING (
    CASE WHEN tenant_id IS NULL OR tenant_id::text IN ('00000000-0000-0000-0000-000000000000', '00000000-0000-0000-0000-000000000001') THEN 'default-tenant' ELSE tenant_id::text END
);
ALTER TABLE compliance_frameworks ALTER COLUMN tenant_id SET DEFAULT 'default-tenant';
UPDATE compliance_frameworks SET tenant_id = 'default-tenant' WHERE tenant_id IS NULL OR tenant_id = '';
ALTER TABLE compliance_frameworks ALTER COLUMN tenant_id SET NOT NULL;
DROP INDEX IF EXISTS idx_compliance_frameworks_tenant_id;
CREATE INDEX IF NOT EXISTS idx_compliance_frameworks_tenant_id ON compliance_frameworks(tenant_id);

-- 5. compliance_violations
ALTER TABLE compliance_violations ALTER COLUMN tenant_id TYPE VARCHAR(255) USING (
    CASE WHEN tenant_id IS NULL OR tenant_id::text IN ('00000000-0000-0000-0000-000000000000', '00000000-0000-0000-0000-000000000001') THEN 'default-tenant' ELSE tenant_id::text END
);
ALTER TABLE compliance_violations ALTER COLUMN tenant_id SET DEFAULT 'default-tenant';
UPDATE compliance_violations SET tenant_id = 'default-tenant' WHERE tenant_id IS NULL OR tenant_id = '';
ALTER TABLE compliance_violations ALTER COLUMN tenant_id SET NOT NULL;
DROP INDEX IF EXISTS idx_compliance_violations_tenant_id;
CREATE INDEX IF NOT EXISTS idx_compliance_violations_tenant_id ON compliance_violations(tenant_id);

-- 6. correlated_events
ALTER TABLE correlated_events ALTER COLUMN tenant_id TYPE VARCHAR(255) USING (
    CASE WHEN tenant_id IS NULL OR tenant_id::text IN ('00000000-0000-0000-0000-000000000000', '00000000-0000-0000-0000-000000000001') THEN 'default-tenant' ELSE tenant_id::text END
);
ALTER TABLE correlated_events ALTER COLUMN tenant_id SET DEFAULT 'default-tenant';
UPDATE correlated_events SET tenant_id = 'default-tenant' WHERE tenant_id IS NULL OR tenant_id = '';
ALTER TABLE correlated_events ALTER COLUMN tenant_id SET NOT NULL;
DROP INDEX IF EXISTS idx_correlated_events_tenant_id;
CREATE INDEX IF NOT EXISTS idx_correlated_events_tenant_id ON correlated_events(tenant_id);

-- 7. cluster_costs
ALTER TABLE cluster_costs ALTER COLUMN tenant_id TYPE VARCHAR(255) USING (
    CASE WHEN tenant_id IS NULL OR tenant_id::text IN ('00000000-0000-0000-0000-000000000000', '00000000-0000-0000-0000-000000000001') THEN 'default-tenant' ELSE tenant_id::text END
);
ALTER TABLE cluster_costs ALTER COLUMN tenant_id SET DEFAULT 'default-tenant';
UPDATE cluster_costs SET tenant_id = 'default-tenant' WHERE tenant_id IS NULL OR tenant_id = '';
ALTER TABLE cluster_costs ALTER COLUMN tenant_id SET NOT NULL;
DROP INDEX IF EXISTS idx_cluster_costs_tenant_id;
CREATE INDEX IF NOT EXISTS idx_cluster_costs_tenant_id ON cluster_costs(tenant_id);

-- 8. namespace_costs
ALTER TABLE namespace_costs ALTER COLUMN tenant_id TYPE VARCHAR(255) USING (
    CASE WHEN tenant_id IS NULL OR tenant_id::text IN ('00000000-0000-0000-0000-000000000000', '00000000-0000-0000-0000-000000000001') THEN 'default-tenant' ELSE tenant_id::text END
);
ALTER TABLE namespace_costs ALTER COLUMN tenant_id SET DEFAULT 'default-tenant';
UPDATE namespace_costs SET tenant_id = 'default-tenant' WHERE tenant_id IS NULL OR tenant_id = '';
ALTER TABLE namespace_costs ALTER COLUMN tenant_id SET NOT NULL;
DROP INDEX IF EXISTS idx_namespace_costs_tenant_id;
CREATE INDEX IF NOT EXISTS idx_namespace_costs_tenant_id ON namespace_costs(tenant_id);

-- 9. resource_waste
ALTER TABLE resource_waste ALTER COLUMN tenant_id TYPE VARCHAR(255) USING (
    CASE WHEN tenant_id IS NULL OR tenant_id::text IN ('00000000-0000-0000-0000-000000000000', '00000000-0000-0000-0000-000000000001') THEN 'default-tenant' ELSE tenant_id::text END
);
ALTER TABLE resource_waste ALTER COLUMN tenant_id SET DEFAULT 'default-tenant';
UPDATE resource_waste SET tenant_id = 'default-tenant' WHERE tenant_id IS NULL OR tenant_id = '';
ALTER TABLE resource_waste ALTER COLUMN tenant_id SET NOT NULL;
DROP INDEX IF EXISTS idx_resource_waste_tenant_id;
CREATE INDEX IF NOT EXISTS idx_resource_waste_tenant_id ON resource_waste(tenant_id);

-- 10. capacity_forecasts
ALTER TABLE capacity_forecasts ALTER COLUMN tenant_id TYPE VARCHAR(255) USING (
    CASE WHEN tenant_id IS NULL OR tenant_id::text IN ('00000000-0000-0000-0000-000000000000', '00000000-0000-0000-0000-000000000001') THEN 'default-tenant' ELSE tenant_id::text END
);
ALTER TABLE capacity_forecasts ALTER COLUMN tenant_id SET DEFAULT 'default-tenant';
UPDATE capacity_forecasts SET tenant_id = 'default-tenant' WHERE tenant_id IS NULL OR tenant_id = '';
ALTER TABLE capacity_forecasts ALTER COLUMN tenant_id SET NOT NULL;
DROP INDEX IF EXISTS idx_capacity_forecasts_tenant_id;
CREATE INDEX IF NOT EXISTS idx_capacity_forecasts_tenant_id ON capacity_forecasts(tenant_id);

-- 11. drift_records
ALTER TABLE drift_records ALTER COLUMN tenant_id TYPE VARCHAR(255) USING (
    CASE WHEN tenant_id IS NULL OR tenant_id::text IN ('00000000-0000-0000-0000-000000000000', '00000000-0000-0000-0000-000000000001') THEN 'default-tenant' ELSE tenant_id::text END
);
ALTER TABLE drift_records ALTER COLUMN tenant_id SET DEFAULT 'default-tenant';
UPDATE drift_records SET tenant_id = 'default-tenant' WHERE tenant_id IS NULL OR tenant_id = '';
ALTER TABLE drift_records ALTER COLUMN tenant_id SET NOT NULL;
DROP INDEX IF EXISTS idx_drift_records_tenant_id;
CREATE INDEX IF NOT EXISTS idx_drift_records_tenant_id ON drift_records(tenant_id);

-- 12. notifications
ALTER TABLE notifications ALTER COLUMN tenant_id TYPE VARCHAR(255) USING (
    CASE WHEN tenant_id IS NULL OR tenant_id::text IN ('00000000-0000-0000-0000-000000000000', '00000000-0000-0000-0000-000000000001') THEN 'default-tenant' ELSE tenant_id::text END
);
ALTER TABLE notifications ALTER COLUMN tenant_id SET DEFAULT 'default-tenant';
UPDATE notifications SET tenant_id = 'default-tenant' WHERE tenant_id IS NULL OR tenant_id = '';
ALTER TABLE notifications ALTER COLUMN tenant_id SET NOT NULL;
DROP INDEX IF EXISTS idx_notifications_tenant_id;
CREATE INDEX IF NOT EXISTS idx_notifications_tenant_id ON notifications(tenant_id);

-- 13. slo_definitions
ALTER TABLE slo_definitions ALTER COLUMN tenant_id TYPE VARCHAR(255) USING (
    CASE WHEN tenant_id IS NULL OR tenant_id::text IN ('00000000-0000-0000-0000-000000000000', '00000000-0000-0000-0000-000000000001') THEN 'default-tenant' ELSE tenant_id::text END
);
ALTER TABLE slo_definitions ALTER COLUMN tenant_id SET DEFAULT 'default-tenant';
UPDATE slo_definitions SET tenant_id = 'default-tenant' WHERE tenant_id IS NULL OR tenant_id = '';
ALTER TABLE slo_definitions ALTER COLUMN tenant_id SET NOT NULL;
DROP INDEX IF EXISTS idx_slo_definitions_tenant_id;
CREATE INDEX IF NOT EXISTS idx_slo_definitions_tenant_id ON slo_definitions(tenant_id);

-- 14. slo_snapshots
ALTER TABLE slo_snapshots ALTER COLUMN tenant_id TYPE VARCHAR(255) USING (
    CASE WHEN tenant_id IS NULL OR tenant_id::text IN ('00000000-0000-0000-0000-000000000000', '00000000-0000-0000-0000-000000000001') THEN 'default-tenant' ELSE tenant_id::text END
);
ALTER TABLE slo_snapshots ALTER COLUMN tenant_id SET DEFAULT 'default-tenant';
UPDATE slo_snapshots SET tenant_id = 'default-tenant' WHERE tenant_id IS NULL OR tenant_id = '';
ALTER TABLE slo_snapshots ALTER COLUMN tenant_id SET NOT NULL;
DROP INDEX IF EXISTS idx_slo_snapshots_tenant_id;
CREATE INDEX IF NOT EXISTS idx_slo_snapshots_tenant_id ON slo_snapshots(tenant_id);

-- 15. promotions
ALTER TABLE promotions ALTER COLUMN tenant_id TYPE VARCHAR(255) USING (
    CASE WHEN tenant_id IS NULL OR tenant_id::text IN ('00000000-0000-0000-0000-000000000000', '00000000-0000-0000-0000-000000000001') THEN 'default-tenant' ELSE tenant_id::text END
);
ALTER TABLE promotions ALTER COLUMN tenant_id SET DEFAULT 'default-tenant';
UPDATE promotions SET tenant_id = 'default-tenant' WHERE tenant_id IS NULL OR tenant_id = '';
ALTER TABLE promotions ALTER COLUMN tenant_id SET NOT NULL;
DROP INDEX IF EXISTS idx_promotions_tenant_id;
CREATE INDEX IF NOT EXISTS idx_promotions_tenant_id ON promotions(tenant_id);

-- 16. reports
ALTER TABLE reports ALTER COLUMN tenant_id TYPE VARCHAR(255) USING (
    CASE WHEN tenant_id IS NULL OR tenant_id::text IN ('00000000-0000-0000-0000-000000000000', '00000000-0000-0000-0000-000000000001') THEN 'default-tenant' ELSE tenant_id::text END
);
ALTER TABLE reports ALTER COLUMN tenant_id SET DEFAULT 'default-tenant';
UPDATE reports SET tenant_id = 'default-tenant' WHERE tenant_id IS NULL OR tenant_id = '';
ALTER TABLE reports ALTER COLUMN tenant_id SET NOT NULL;
DROP INDEX IF EXISTS idx_reports_tenant_id;
CREATE INDEX IF NOT EXISTS idx_reports_tenant_id ON reports(tenant_id);

-- 17. reporting (conditional if table exists)
DO $$ BEGIN 
    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = 'public' AND table_name = 'reporting') THEN 
        ALTER TABLE reporting ALTER COLUMN tenant_id TYPE VARCHAR(255) USING (
            CASE WHEN tenant_id IS NULL OR tenant_id::text IN ('00000000-0000-0000-0000-000000000000', '00000000-0000-0000-0000-000000000001') THEN 'default-tenant' ELSE tenant_id::text END
        );
        ALTER TABLE reporting ALTER COLUMN tenant_id SET DEFAULT 'default-tenant';
        UPDATE reporting SET tenant_id = 'default-tenant' WHERE tenant_id IS NULL OR tenant_id = '';
        ALTER TABLE reporting ALTER COLUMN tenant_id SET NOT NULL;
        DROP INDEX IF EXISTS idx_reporting_tenant_id;
        CREATE INDEX IF NOT EXISTS idx_reporting_tenant_id ON reporting(tenant_id);
    END IF; 
END $$;

-- 18. tags
ALTER TABLE tags ALTER COLUMN tenant_id TYPE VARCHAR(255) USING (
    CASE WHEN tenant_id IS NULL OR tenant_id::text IN ('00000000-0000-0000-0000-000000000000', '00000000-0000-0000-0000-000000000001') THEN 'default-tenant' ELSE tenant_id::text END
);
ALTER TABLE tags ALTER COLUMN tenant_id SET DEFAULT 'default-tenant';
UPDATE tags SET tenant_id = 'default-tenant' WHERE tenant_id IS NULL OR tenant_id = '';
ALTER TABLE tags ALTER COLUMN tenant_id SET NOT NULL;
DROP INDEX IF EXISTS idx_tags_tenant_id;
CREATE INDEX IF NOT EXISTS idx_tags_tenant_id ON tags(tenant_id);

-- 19. resource_tags
ALTER TABLE resource_tags ALTER COLUMN tenant_id TYPE VARCHAR(255) USING (
    CASE WHEN tenant_id IS NULL OR tenant_id::text IN ('00000000-0000-0000-0000-000000000000', '00000000-0000-0000-0000-000000000001') THEN 'default-tenant' ELSE tenant_id::text END
);
ALTER TABLE resource_tags ALTER COLUMN tenant_id SET DEFAULT 'default-tenant';
UPDATE resource_tags SET tenant_id = 'default-tenant' WHERE tenant_id IS NULL OR tenant_id = '';
ALTER TABLE resource_tags ALTER COLUMN tenant_id SET NOT NULL;
DROP INDEX IF EXISTS idx_resource_tags_tenant_id;
CREATE INDEX IF NOT EXISTS idx_resource_tags_tenant_id ON resource_tags(tenant_id);

-- 20. timeline_events
ALTER TABLE timeline_events ALTER COLUMN tenant_id TYPE VARCHAR(255) USING (
    CASE WHEN tenant_id IS NULL OR tenant_id::text IN ('00000000-0000-0000-0000-000000000000', '00000000-0000-0000-0000-000000000001') THEN 'default-tenant' ELSE tenant_id::text END
);
ALTER TABLE timeline_events ALTER COLUMN tenant_id SET DEFAULT 'default-tenant';
UPDATE timeline_events SET tenant_id = 'default-tenant' WHERE tenant_id IS NULL OR tenant_id = '';
ALTER TABLE timeline_events ALTER COLUMN tenant_id SET NOT NULL;
DROP INDEX IF EXISTS idx_timeline_events_tenant_id;
CREATE INDEX IF NOT EXISTS idx_timeline_events_tenant_id ON timeline_events(tenant_id);

-- 21. projects
ALTER TABLE projects ALTER COLUMN tenant_id TYPE VARCHAR(255) USING (
    CASE WHEN tenant_id IS NULL OR tenant_id::text IN ('00000000-0000-0000-0000-000000000000', '00000000-0000-0000-0000-000000000001') THEN 'default-tenant' ELSE tenant_id::text END
);
ALTER TABLE projects ALTER COLUMN tenant_id SET DEFAULT 'default-tenant';
UPDATE projects SET tenant_id = 'default-tenant' WHERE tenant_id IS NULL OR tenant_id = '';
ALTER TABLE projects ALTER COLUMN tenant_id SET NOT NULL;
DROP INDEX IF EXISTS idx_projects_tenant_id;
CREATE INDEX IF NOT EXISTS idx_projects_tenant_id ON projects(tenant_id);

-- 22. tenant_members
ALTER TABLE tenant_members ALTER COLUMN tenant_id TYPE VARCHAR(255) USING (
    CASE WHEN tenant_id IS NULL OR tenant_id::text IN ('00000000-0000-0000-0000-000000000000', '00000000-0000-0000-0000-000000000001') THEN 'default-tenant' ELSE tenant_id::text END
);
ALTER TABLE tenant_members ALTER COLUMN tenant_id SET DEFAULT 'default-tenant';
UPDATE tenant_members SET tenant_id = 'default-tenant' WHERE tenant_id IS NULL OR tenant_id = '';
ALTER TABLE tenant_members ALTER COLUMN tenant_id SET NOT NULL;
DROP INDEX IF EXISTS idx_tenant_members_tenant_id;
CREATE INDEX IF NOT EXISTS idx_tenant_members_tenant_id ON tenant_members(tenant_id);
