-- Migration: 057_tenant_isolation_remaining_tables.up.sql
-- Description: Adds tenant_id column and index to remaining tables for tenant isolation.

ALTER TABLE backup_history ADD COLUMN IF NOT EXISTS tenant_id UUID;
CREATE INDEX IF NOT EXISTS idx_backup_history_tenant_id ON backup_history(tenant_id);

ALTER TABLE change_requests ADD COLUMN IF NOT EXISTS tenant_id UUID;
CREATE INDEX IF NOT EXISTS idx_change_requests_tenant_id ON change_requests(tenant_id);

ALTER TABLE maintenance_windows ADD COLUMN IF NOT EXISTS tenant_id UUID;
CREATE INDEX IF NOT EXISTS idx_maintenance_windows_tenant_id ON maintenance_windows(tenant_id);

ALTER TABLE compliance_frameworks ADD COLUMN IF NOT EXISTS tenant_id UUID;
CREATE INDEX IF NOT EXISTS idx_compliance_frameworks_tenant_id ON compliance_frameworks(tenant_id);

ALTER TABLE compliance_violations ADD COLUMN IF NOT EXISTS tenant_id UUID;
CREATE INDEX IF NOT EXISTS idx_compliance_violations_tenant_id ON compliance_violations(tenant_id);

ALTER TABLE correlated_events ADD COLUMN IF NOT EXISTS tenant_id UUID;
CREATE INDEX IF NOT EXISTS idx_correlated_events_tenant_id ON correlated_events(tenant_id);

ALTER TABLE cluster_costs ADD COLUMN IF NOT EXISTS tenant_id UUID;
CREATE INDEX IF NOT EXISTS idx_cluster_costs_tenant_id ON cluster_costs(tenant_id);

ALTER TABLE namespace_costs ADD COLUMN IF NOT EXISTS tenant_id UUID;
CREATE INDEX IF NOT EXISTS idx_namespace_costs_tenant_id ON namespace_costs(tenant_id);

ALTER TABLE resource_waste ADD COLUMN IF NOT EXISTS tenant_id UUID;
CREATE INDEX IF NOT EXISTS idx_resource_waste_tenant_id ON resource_waste(tenant_id);

ALTER TABLE capacity_forecasts ADD COLUMN IF NOT EXISTS tenant_id UUID;
CREATE INDEX IF NOT EXISTS idx_capacity_forecasts_tenant_id ON capacity_forecasts(tenant_id);

ALTER TABLE drift_records ADD COLUMN IF NOT EXISTS tenant_id UUID;
CREATE INDEX IF NOT EXISTS idx_drift_records_tenant_id ON drift_records(tenant_id);

ALTER TABLE notifications ADD COLUMN IF NOT EXISTS tenant_id UUID;
CREATE INDEX IF NOT EXISTS idx_notifications_tenant_id ON notifications(tenant_id);

ALTER TABLE slo_definitions ADD COLUMN IF NOT EXISTS tenant_id UUID;
CREATE INDEX IF NOT EXISTS idx_slo_definitions_tenant_id ON slo_definitions(tenant_id);

ALTER TABLE slo_snapshots ADD COLUMN IF NOT EXISTS tenant_id UUID;
CREATE INDEX IF NOT EXISTS idx_slo_snapshots_tenant_id ON slo_snapshots(tenant_id);

ALTER TABLE promotions ADD COLUMN IF NOT EXISTS tenant_id UUID;
CREATE INDEX IF NOT EXISTS idx_promotions_tenant_id ON promotions(tenant_id);

ALTER TABLE reports ADD COLUMN IF NOT EXISTS tenant_id UUID;
CREATE INDEX IF NOT EXISTS idx_reports_tenant_id ON reports(tenant_id);

ALTER TABLE reporting ADD COLUMN IF NOT EXISTS tenant_id UUID;
CREATE INDEX IF NOT EXISTS idx_reporting_tenant_id ON reporting(tenant_id);

ALTER TABLE tags ADD COLUMN IF NOT EXISTS tenant_id UUID;
CREATE INDEX IF NOT EXISTS idx_tags_tenant_id ON tags(tenant_id);

ALTER TABLE resource_tags ADD COLUMN IF NOT EXISTS tenant_id UUID;
CREATE INDEX IF NOT EXISTS idx_resource_tags_tenant_id ON resource_tags(tenant_id);

ALTER TABLE timeline_events ADD COLUMN IF NOT EXISTS tenant_id UUID;
CREATE INDEX IF NOT EXISTS idx_timeline_events_tenant_id ON timeline_events(tenant_id);

-- projects: scoped by org_id — org_id -> tenant_id mapping needed
ALTER TABLE projects ADD COLUMN IF NOT EXISTS tenant_id UUID;
CREATE INDEX IF NOT EXISTS idx_projects_tenant_id ON projects(tenant_id);

-- tenant_members: scoped by org_id — org_id -> tenant_id mapping needed
ALTER TABLE tenant_members ADD COLUMN IF NOT EXISTS tenant_id UUID;
CREATE INDEX IF NOT EXISTS idx_tenant_members_tenant_id ON tenant_members(tenant_id);