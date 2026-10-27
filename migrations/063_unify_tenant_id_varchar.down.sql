-- Migration: 063_unify_tenant_id_varchar.down.sql
-- Description: Revert tenant_id column type from VARCHAR(255) back to UUID.

-- 1. backup_history
ALTER TABLE backup_history ALTER COLUMN tenant_id DROP NOT NULL;
ALTER TABLE backup_history ALTER COLUMN tenant_id DROP DEFAULT;
ALTER TABLE backup_history ALTER COLUMN tenant_id TYPE UUID USING (
    CASE WHEN tenant_id = 'default-tenant' THEN '00000000-0000-0000-0000-000000000001'::uuid WHEN tenant_id ~ '^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$' THEN tenant_id::uuid ELSE NULL END
);
DROP INDEX IF EXISTS idx_backup_history_tenant_id;
CREATE INDEX IF NOT EXISTS idx_backup_history_tenant_id ON backup_history(tenant_id);

-- 2. change_requests
ALTER TABLE change_requests ALTER COLUMN tenant_id DROP NOT NULL;
ALTER TABLE change_requests ALTER COLUMN tenant_id DROP DEFAULT;
ALTER TABLE change_requests ALTER COLUMN tenant_id TYPE UUID USING (
    CASE WHEN tenant_id = 'default-tenant' THEN '00000000-0000-0000-0000-000000000001'::uuid WHEN tenant_id ~ '^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$' THEN tenant_id::uuid ELSE NULL END
);
DROP INDEX IF EXISTS idx_change_requests_tenant_id;
CREATE INDEX IF NOT EXISTS idx_change_requests_tenant_id ON change_requests(tenant_id);

-- 3. maintenance_windows
ALTER TABLE maintenance_windows ALTER COLUMN tenant_id DROP NOT NULL;
ALTER TABLE maintenance_windows ALTER COLUMN tenant_id DROP DEFAULT;
ALTER TABLE maintenance_windows ALTER COLUMN tenant_id TYPE UUID USING (
    CASE WHEN tenant_id = 'default-tenant' THEN '00000000-0000-0000-0000-000000000001'::uuid WHEN tenant_id ~ '^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$' THEN tenant_id::uuid ELSE NULL END
);
DROP INDEX IF EXISTS idx_maintenance_windows_tenant_id;
CREATE INDEX IF NOT EXISTS idx_maintenance_windows_tenant_id ON maintenance_windows(tenant_id);

-- 4. compliance_frameworks
ALTER TABLE compliance_frameworks ALTER COLUMN tenant_id DROP NOT NULL;
ALTER TABLE compliance_frameworks ALTER COLUMN tenant_id DROP DEFAULT;
ALTER TABLE compliance_frameworks ALTER COLUMN tenant_id TYPE UUID USING (
    CASE WHEN tenant_id = 'default-tenant' THEN '00000000-0000-0000-0000-000000000001'::uuid WHEN tenant_id ~ '^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$' THEN tenant_id::uuid ELSE NULL END
);
DROP INDEX IF EXISTS idx_compliance_frameworks_tenant_id;
CREATE INDEX IF NOT EXISTS idx_compliance_frameworks_tenant_id ON compliance_frameworks(tenant_id);

-- 5. compliance_violations
ALTER TABLE compliance_violations ALTER COLUMN tenant_id DROP NOT NULL;
ALTER TABLE compliance_violations ALTER COLUMN tenant_id DROP DEFAULT;
ALTER TABLE compliance_violations ALTER COLUMN tenant_id TYPE UUID USING (
    CASE WHEN tenant_id = 'default-tenant' THEN '00000000-0000-0000-0000-000000000001'::uuid WHEN tenant_id ~ '^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$' THEN tenant_id::uuid ELSE NULL END
);
DROP INDEX IF EXISTS idx_compliance_violations_tenant_id;
CREATE INDEX IF NOT EXISTS idx_compliance_violations_tenant_id ON compliance_violations(tenant_id);

-- 6. correlated_events
ALTER TABLE correlated_events ALTER COLUMN tenant_id DROP NOT NULL;
ALTER TABLE correlated_events ALTER COLUMN tenant_id DROP DEFAULT;
ALTER TABLE correlated_events ALTER COLUMN tenant_id TYPE UUID USING (
    CASE WHEN tenant_id = 'default-tenant' THEN '00000000-0000-0000-0000-000000000001'::uuid WHEN tenant_id ~ '^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$' THEN tenant_id::uuid ELSE NULL END
);
DROP INDEX IF EXISTS idx_correlated_events_tenant_id;
CREATE INDEX IF NOT EXISTS idx_correlated_events_tenant_id ON correlated_events(tenant_id);

-- 7. cluster_costs
ALTER TABLE cluster_costs ALTER COLUMN tenant_id DROP NOT NULL;
ALTER TABLE cluster_costs ALTER COLUMN tenant_id DROP DEFAULT;
ALTER TABLE cluster_costs ALTER COLUMN tenant_id TYPE UUID USING (
    CASE WHEN tenant_id = 'default-tenant' THEN '00000000-0000-0000-0000-000000000001'::uuid WHEN tenant_id ~ '^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$' THEN tenant_id::uuid ELSE NULL END
);
DROP INDEX IF EXISTS idx_cluster_costs_tenant_id;
CREATE INDEX IF NOT EXISTS idx_cluster_costs_tenant_id ON cluster_costs(tenant_id);

-- 8. namespace_costs
ALTER TABLE namespace_costs ALTER COLUMN tenant_id DROP NOT NULL;
ALTER TABLE namespace_costs ALTER COLUMN tenant_id DROP DEFAULT;
ALTER TABLE namespace_costs ALTER COLUMN tenant_id TYPE UUID USING (
    CASE WHEN tenant_id = 'default-tenant' THEN '00000000-0000-0000-0000-000000000001'::uuid WHEN tenant_id ~ '^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$' THEN tenant_id::uuid ELSE NULL END
);
DROP INDEX IF EXISTS idx_namespace_costs_tenant_id;
CREATE INDEX IF NOT EXISTS idx_namespace_costs_tenant_id ON namespace_costs(tenant_id);

-- 9. resource_waste
ALTER TABLE resource_waste ALTER COLUMN tenant_id DROP NOT NULL;
ALTER TABLE resource_waste ALTER COLUMN tenant_id DROP DEFAULT;
ALTER TABLE resource_waste ALTER COLUMN tenant_id TYPE UUID USING (
    CASE WHEN tenant_id = 'default-tenant' THEN '00000000-0000-0000-0000-000000000001'::uuid WHEN tenant_id ~ '^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$' THEN tenant_id::uuid ELSE NULL END
);
DROP INDEX IF EXISTS idx_resource_waste_tenant_id;
CREATE INDEX IF NOT EXISTS idx_resource_waste_tenant_id ON resource_waste(tenant_id);

-- 10. capacity_forecasts
ALTER TABLE capacity_forecasts ALTER COLUMN tenant_id DROP NOT NULL;
ALTER TABLE capacity_forecasts ALTER COLUMN tenant_id DROP DEFAULT;
ALTER TABLE capacity_forecasts ALTER COLUMN tenant_id TYPE UUID USING (
    CASE WHEN tenant_id = 'default-tenant' THEN '00000000-0000-0000-0000-000000000001'::uuid WHEN tenant_id ~ '^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$' THEN tenant_id::uuid ELSE NULL END
);
DROP INDEX IF EXISTS idx_capacity_forecasts_tenant_id;
CREATE INDEX IF NOT EXISTS idx_capacity_forecasts_tenant_id ON capacity_forecasts(tenant_id);

-- 11. drift_records
ALTER TABLE drift_records ALTER COLUMN tenant_id DROP NOT NULL;
ALTER TABLE drift_records ALTER COLUMN tenant_id DROP DEFAULT;
ALTER TABLE drift_records ALTER COLUMN tenant_id TYPE UUID USING (
    CASE WHEN tenant_id = 'default-tenant' THEN '00000000-0000-0000-0000-000000000001'::uuid WHEN tenant_id ~ '^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$' THEN tenant_id::uuid ELSE NULL END
);
DROP INDEX IF EXISTS idx_drift_records_tenant_id;
CREATE INDEX IF NOT EXISTS idx_drift_records_tenant_id ON drift_records(tenant_id);

-- 12. notifications
ALTER TABLE notifications ALTER COLUMN tenant_id DROP NOT NULL;
ALTER TABLE notifications ALTER COLUMN tenant_id DROP DEFAULT;
ALTER TABLE notifications ALTER COLUMN tenant_id TYPE UUID USING (
    CASE WHEN tenant_id = 'default-tenant' THEN '00000000-0000-0000-0000-000000000001'::uuid WHEN tenant_id ~ '^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$' THEN tenant_id::uuid ELSE NULL END
);
DROP INDEX IF EXISTS idx_notifications_tenant_id;
CREATE INDEX IF NOT EXISTS idx_notifications_tenant_id ON notifications(tenant_id);

-- 13. slo_definitions
ALTER TABLE slo_definitions ALTER COLUMN tenant_id DROP NOT NULL;
ALTER TABLE slo_definitions ALTER COLUMN tenant_id DROP DEFAULT;
ALTER TABLE slo_definitions ALTER COLUMN tenant_id TYPE UUID USING (
    CASE WHEN tenant_id = 'default-tenant' THEN '00000000-0000-0000-0000-000000000001'::uuid WHEN tenant_id ~ '^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$' THEN tenant_id::uuid ELSE NULL END
);
DROP INDEX IF EXISTS idx_slo_definitions_tenant_id;
CREATE INDEX IF NOT EXISTS idx_slo_definitions_tenant_id ON slo_definitions(tenant_id);

-- 14. slo_snapshots
ALTER TABLE slo_snapshots ALTER COLUMN tenant_id DROP NOT NULL;
ALTER TABLE slo_snapshots ALTER COLUMN tenant_id DROP DEFAULT;
ALTER TABLE slo_snapshots ALTER COLUMN tenant_id TYPE UUID USING (
    CASE WHEN tenant_id = 'default-tenant' THEN '00000000-0000-0000-0000-000000000001'::uuid WHEN tenant_id ~ '^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$' THEN tenant_id::uuid ELSE NULL END
);
DROP INDEX IF EXISTS idx_slo_snapshots_tenant_id;
CREATE INDEX IF NOT EXISTS idx_slo_snapshots_tenant_id ON slo_snapshots(tenant_id);

-- 15. promotions
ALTER TABLE promotions ALTER COLUMN tenant_id DROP NOT NULL;
ALTER TABLE promotions ALTER COLUMN tenant_id DROP DEFAULT;
ALTER TABLE promotions ALTER COLUMN tenant_id TYPE UUID USING (
    CASE WHEN tenant_id = 'default-tenant' THEN '00000000-0000-0000-0000-000000000001'::uuid WHEN tenant_id ~ '^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$' THEN tenant_id::uuid ELSE NULL END
);
DROP INDEX IF EXISTS idx_promotions_tenant_id;
CREATE INDEX IF NOT EXISTS idx_promotions_tenant_id ON promotions(tenant_id);

-- 16. reports
ALTER TABLE reports ALTER COLUMN tenant_id DROP NOT NULL;
ALTER TABLE reports ALTER COLUMN tenant_id DROP DEFAULT;
ALTER TABLE reports ALTER COLUMN tenant_id TYPE UUID USING (
    CASE WHEN tenant_id = 'default-tenant' THEN '00000000-0000-0000-0000-000000000001'::uuid WHEN tenant_id ~ '^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$' THEN tenant_id::uuid ELSE NULL END
);
DROP INDEX IF EXISTS idx_reports_tenant_id;
CREATE INDEX IF NOT EXISTS idx_reports_tenant_id ON reports(tenant_id);

-- 17. reporting (conditional if table exists)
DO $$ BEGIN 
    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = 'public' AND table_name = 'reporting') THEN 
        ALTER TABLE reporting ALTER COLUMN tenant_id DROP NOT NULL;
        ALTER TABLE reporting ALTER COLUMN tenant_id DROP DEFAULT;
        ALTER TABLE reporting ALTER COLUMN tenant_id TYPE UUID USING (
            CASE WHEN tenant_id = 'default-tenant' THEN '00000000-0000-0000-0000-000000000001'::uuid WHEN tenant_id ~ '^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$' THEN tenant_id::uuid ELSE NULL END
        );
        DROP INDEX IF EXISTS idx_reporting_tenant_id;
        CREATE INDEX IF NOT EXISTS idx_reporting_tenant_id ON reporting(tenant_id);
    END IF; 
END $$;

-- 18. tags
ALTER TABLE tags ALTER COLUMN tenant_id DROP NOT NULL;
ALTER TABLE tags ALTER COLUMN tenant_id DROP DEFAULT;
ALTER TABLE tags ALTER COLUMN tenant_id TYPE UUID USING (
    CASE WHEN tenant_id = 'default-tenant' THEN '00000000-0000-0000-0000-000000000001'::uuid WHEN tenant_id ~ '^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$' THEN tenant_id::uuid ELSE NULL END
);
DROP INDEX IF EXISTS idx_tags_tenant_id;
CREATE INDEX IF NOT EXISTS idx_tags_tenant_id ON tags(tenant_id);

-- 19. resource_tags
ALTER TABLE resource_tags ALTER COLUMN tenant_id DROP NOT NULL;
ALTER TABLE resource_tags ALTER COLUMN tenant_id DROP DEFAULT;
ALTER TABLE resource_tags ALTER COLUMN tenant_id TYPE UUID USING (
    CASE WHEN tenant_id = 'default-tenant' THEN '00000000-0000-0000-0000-000000000001'::uuid WHEN tenant_id ~ '^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$' THEN tenant_id::uuid ELSE NULL END
);
DROP INDEX IF EXISTS idx_resource_tags_tenant_id;
CREATE INDEX IF NOT EXISTS idx_resource_tags_tenant_id ON resource_tags(tenant_id);

-- 20. timeline_events
ALTER TABLE timeline_events ALTER COLUMN tenant_id DROP NOT NULL;
ALTER TABLE timeline_events ALTER COLUMN tenant_id DROP DEFAULT;
ALTER TABLE timeline_events ALTER COLUMN tenant_id TYPE UUID USING (
    CASE WHEN tenant_id = 'default-tenant' THEN '00000000-0000-0000-0000-000000000001'::uuid WHEN tenant_id ~ '^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$' THEN tenant_id::uuid ELSE NULL END
);
DROP INDEX IF EXISTS idx_timeline_events_tenant_id;
CREATE INDEX IF NOT EXISTS idx_timeline_events_tenant_id ON timeline_events(tenant_id);

-- 21. projects
ALTER TABLE projects ALTER COLUMN tenant_id DROP NOT NULL;
ALTER TABLE projects ALTER COLUMN tenant_id DROP DEFAULT;
ALTER TABLE projects ALTER COLUMN tenant_id TYPE UUID USING (
    CASE WHEN tenant_id = 'default-tenant' THEN '00000000-0000-0000-0000-000000000001'::uuid WHEN tenant_id ~ '^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$' THEN tenant_id::uuid ELSE NULL END
);
DROP INDEX IF EXISTS idx_projects_tenant_id;
CREATE INDEX IF NOT EXISTS idx_projects_tenant_id ON projects(tenant_id);

-- 22. tenant_members
ALTER TABLE tenant_members ALTER COLUMN tenant_id DROP NOT NULL;
ALTER TABLE tenant_members ALTER COLUMN tenant_id DROP DEFAULT;
ALTER TABLE tenant_members ALTER COLUMN tenant_id TYPE UUID USING (
    CASE WHEN tenant_id = 'default-tenant' THEN '00000000-0000-0000-0000-000000000001'::uuid WHEN tenant_id ~ '^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$' THEN tenant_id::uuid ELSE NULL END
);
DROP INDEX IF EXISTS idx_tenant_members_tenant_id;
CREATE INDEX IF NOT EXISTS idx_tenant_members_tenant_id ON tenant_members(tenant_id);
