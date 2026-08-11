-- DOWN migration for 024_fix_observability_slo_columns.sql

DELETE FROM slo_snapshots WHERE service IN ('payment-api', 'user-service', 'order-service', 'inventory-api', 'notification-svc', 'analytics-engine');
DELETE FROM slo_definitions WHERE service IN ('payment-api', 'user-service', 'order-service', 'inventory-api', 'notification-svc', 'analytics-engine');

ALTER TABLE slo_snapshots DROP COLUMN IF EXISTS target;
ALTER TABLE slo_snapshots DROP COLUMN IF EXISTS service;
