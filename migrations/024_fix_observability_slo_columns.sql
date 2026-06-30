-- Migration: 024_fix_observability_slo_columns.sql
-- Fixes missing service and target columns on slo_snapshots due to CREATE TABLE IF NOT EXISTS conflict
-- and seeds the initial production-like SLO definitions and snapshots.

-- 1. Alter table to ensure missing columns are present
ALTER TABLE slo_snapshots ADD COLUMN IF NOT EXISTS service VARCHAR(255);
ALTER TABLE slo_snapshots ADD COLUMN IF NOT EXISTS target FLOAT;

-- 2. Clean up any existing records to prevent conflicts during seed
DELETE FROM slo_snapshots;
DELETE FROM slo_definitions;

-- 3. Seed SLO Definitions and Snapshots
WITH inserted_slos AS (
    INSERT INTO slo_definitions (service, target, indicator_type, "window") VALUES
        ('payment-api', 99.95, 'availability', '30d'),
        ('user-service', 99.90, 'availability', '30d'),
        ('order-service', 99.95, 'availability', '30d'),
        ('inventory-api', 99.90, 'availability', '30d'),
        ('notification-svc', 99.50, 'availability', '30d'),
        ('analytics-engine', 99.90, 'availability', '30d')
    RETURNING id, service, target
)
INSERT INTO slo_snapshots (slo_id, service, target, actual, burn_rate, error_budget, budget_status, recorded_at)
SELECT 
    id, 
    service, 
    target,
    CASE 
        WHEN service = 'payment-api' THEN 99.92
        WHEN service = 'user-service' THEN 99.97
        WHEN service = 'order-service' THEN 99.88
        WHEN service = 'inventory-api' THEN 99.93
        WHEN service = 'notification-svc' THEN 99.78
        ELSE 99.85
    END as actual,
    CASE 
        WHEN service = 'payment-api' THEN 1.8
        WHEN service = 'user-service' THEN 0.3
        WHEN service = 'order-service' THEN 2.4
        WHEN service = 'inventory-api' THEN 0.7
        WHEN service = 'notification-svc' THEN 0.2
        ELSE 1.5
    END as burn_rate,
    CASE 
        WHEN service = 'payment-api' THEN 32.0
        WHEN service = 'user-service' THEN 85.0
        WHEN service = 'order-service' THEN 12.0
        WHEN service = 'inventory-api' THEN 67.0
        WHEN service = 'notification-svc' THEN 94.0
        ELSE 45.0
    END as error_budget,
    CASE 
        WHEN service = 'order-service' THEN 'critical'
        WHEN service IN ('payment-api', 'analytics-engine') THEN 'warning'
        ELSE 'healthy'
    END as budget_status,
    NOW()
FROM inserted_slos;
