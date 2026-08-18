-- Migration: 029_enhanced_alerting.down.sql
DROP TABLE IF EXISTS alert_history;
DROP TABLE IF EXISTS alert_rules;
DROP INDEX IF EXISTS idx_notification_channels_tenant;
ALTER TABLE notification_channels DROP COLUMN IF EXISTS tenant_id;
