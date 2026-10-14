-- Migration: 061_audit_and_composite_indexes.down.sql
-- Description: Drop composite indexes created in 061

DROP INDEX IF EXISTS idx_automation_exec_rule_time;
DROP INDEX IF EXISTS idx_audit_logs_tenant_created_at;
DROP INDEX IF EXISTS idx_alert_history_tenant_time;
DROP INDEX IF EXISTS idx_slo_samples_tenant_service_time;
DROP INDEX IF EXISTS idx_incidents_tenant_created_at;
