-- Migration: 064_alert_silences_and_resolved.up.sql
-- Description: Create alert_silences table and add resolved_at to alert_history.

CREATE TABLE IF NOT EXISTS alert_silences (
    id VARCHAR(50) PRIMARY KEY,
    tenant_id VARCHAR(255) NOT NULL DEFAULT 'default-tenant',
    alert_id VARCHAR(100) NOT NULL,
    comment TEXT DEFAULT '',
    created_by VARCHAR(100) DEFAULT '',
    duration_minutes INT NOT NULL DEFAULT 60,
    starts_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    ends_at TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_alert_silences_tenant ON alert_silences(tenant_id);
CREATE INDEX IF NOT EXISTS idx_alert_silences_alert ON alert_silences(alert_id);
CREATE INDEX IF NOT EXISTS idx_alert_silences_ends_at ON alert_silences(ends_at);

ALTER TABLE alert_history ADD COLUMN IF NOT EXISTS resolved_at TIMESTAMP WITH TIME ZONE;
