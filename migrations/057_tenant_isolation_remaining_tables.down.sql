-- Migration: 057_tenant_isolation_remaining_tables.down.sql
-- Description: Drops tenant_id columns and indexes from remaining tables.

DROP INDEX IF EXISTS idx_tenant_members_tenant_id;
ALTER TABLE tenant_members DROP COLUMN IF EXISTS tenant_id;

DROP INDEX IF EXISTS idx_projects_tenant_id;
ALTER TABLE projects DROP COLUMN IF EXISTS tenant_id;

DROP INDEX IF EXISTS idx_timeline_events_tenant_id;
ALTER TABLE timeline_events DROP COLUMN IF EXISTS tenant_id;

DROP INDEX IF EXISTS idx_resource_tags_tenant_id;
ALTER TABLE resource_tags DROP COLUMN IF EXISTS tenant_id;

DROP INDEX IF EXISTS idx_tags_tenant_id;
ALTER TABLE tags DROP COLUMN IF EXISTS tenant_id;

DROP INDEX IF EXISTS idx_reporting_tenant_id;
ALTER TABLE reporting DROP COLUMN IF EXISTS tenant_id;

DROP INDEX IF EXISTS idx_reports_tenant_id;
ALTER TABLE reports DROP COLUMN IF EXISTS tenant_id;

DROP INDEX IF EXISTS idx_promotions_tenant_id;
ALTER TABLE promotions DROP COLUMN IF EXISTS tenant_id;

DROP INDEX IF EXISTS idx_slo_snapshots_tenant_id;
ALTER TABLE slo_snapshots DROP COLUMN IF EXISTS tenant_id;

DROP INDEX IF EXISTS idx_slo_definitions_tenant_id;
ALTER TABLE slo_definitions DROP COLUMN IF EXISTS tenant_id;

DROP INDEX IF EXISTS idx_notifications_tenant_id;
ALTER TABLE notifications DROP COLUMN IF EXISTS tenant_id;

DROP INDEX IF EXISTS idx_drift_records_tenant_id;
ALTER TABLE drift_records DROP COLUMN IF EXISTS tenant_id;

DROP INDEX IF EXISTS idx_capacity_forecasts_tenant_id;
ALTER TABLE capacity_forecasts DROP COLUMN IF EXISTS tenant_id;

DROP INDEX IF EXISTS idx_resource_waste_tenant_id;
ALTER TABLE resource_waste DROP COLUMN IF EXISTS tenant_id;

DROP INDEX IF EXISTS idx_namespace_costs_tenant_id;
ALTER TABLE namespace_costs DROP COLUMN IF EXISTS tenant_id;

DROP INDEX IF EXISTS idx_cluster_costs_tenant_id;
ALTER TABLE cluster_costs DROP COLUMN IF EXISTS tenant_id;

DROP INDEX IF EXISTS idx_correlated_events_tenant_id;
ALTER TABLE correlated_events DROP COLUMN IF EXISTS tenant_id;

DROP INDEX IF EXISTS idx_compliance_violations_tenant_id;
ALTER TABLE compliance_violations DROP COLUMN IF EXISTS tenant_id;

DROP INDEX IF EXISTS idx_compliance_frameworks_tenant_id;
ALTER TABLE compliance_frameworks DROP COLUMN IF EXISTS tenant_id;

DROP INDEX IF EXISTS idx_maintenance_windows_tenant_id;
ALTER TABLE maintenance_windows DROP COLUMN IF EXISTS tenant_id;

DROP INDEX IF EXISTS idx_change_requests_tenant_id;
ALTER TABLE change_requests DROP COLUMN IF EXISTS tenant_id;

DROP INDEX IF EXISTS idx_backup_history_tenant_id;
ALTER TABLE backup_history DROP COLUMN IF EXISTS tenant_id;