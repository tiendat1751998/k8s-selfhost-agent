-- Migration: 061_audit_and_composite_indexes.up.sql
-- Description: Composite indexes for multi-tenant query acceleration (PERF-10, PERF-13)

CREATE INDEX IF NOT EXISTS idx_incidents_tenant_created_at ON incidents(tenant_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_slo_samples_tenant_service_time ON slo_health_samples(tenant_id, service_name, recorded_at DESC);
CREATE INDEX IF NOT EXISTS idx_alert_history_tenant_time ON alert_history(tenant_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_logs_tenant_created_at ON audit_logs(tenant_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_automation_exec_rule_time ON automation_executions(rule_id, created_at DESC);
