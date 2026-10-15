-- Migration: 060_purge_demo_seed_data.up.sql
-- Description: Purge all synthetic demo/seed records from database tables for clean enterprise state.

-- 1. Purge synthetic audit trail records
DELETE FROM audit_logs
WHERE id IN (
    'c0000000-0000-4000-8000-000000000001',
    'c0000000-0000-4000-8000-000000000002',
    'c0000000-0000-4000-8000-000000000003',
    'c0000000-0000-4000-8000-000000000004',
    'c0000000-0000-4000-8000-000000000005',
    'c0000000-0000-4000-8000-000000000006',
    'c0000000-0000-4000-8000-000000000007',
    'c0000000-0000-4000-8000-000000000008',
    'c0000000-0000-4000-8000-000000000009'
)
OR actor LIKE '%@enterprise.io'
OR target_name = 'crypto-miner-demo'
OR target_name = 'istio-ingress-gateway.yaml';

-- 2. Purge fake incidents (including DESKTOP-GKK38T1 local PC host anomalies and demo services)
DELETE FROM incidents
WHERE pod_name = 'DESKTOP-GKK38T1'
   OR pod_name IN ('checkout-api', 'payment-gateway', 'masterdb')
   OR message LIKE '%DESKTOP-GKK38T1%'
   OR raw_data->>'title' LIKE '%DESKTOP-GKK38T1%'
   OR raw_data->>'resource_name' IN ('DESKTOP-GKK38T1', 'checkout-api', 'payment-gateway', 'masterdb');

-- 3. Purge demo SLO definitions and snapshots
DELETE FROM slo_snapshots
WHERE service IN ('checkout-api', 'payment-gateway', 'postgres_db', 'traefik', 'postgres', 'nats');

DELETE FROM slo_definitions
WHERE service IN ('checkout-api', 'payment-gateway', 'postgres_db', 'traefik', 'postgres', 'nats');

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_name = 'slo_health_samples') THEN
        DELETE FROM slo_health_samples
        WHERE service_name IN ('checkout-api', 'payment-gateway', 'postgres_db', 'traefik', 'postgres', 'nats');
    END IF;
END $$;

-- 4. Purge demo services from service_catalog
DELETE FROM service_catalog
WHERE name IN ('checkout-api', 'payment-gateway', 'catalog-service', 'auth-service', 'notification-worker');

-- 5. Purge demo compliance violations, drift records, and audit findings
DELETE FROM compliance_violations
WHERE resource IN ('crypto-miner-demo', 'istio-ingress-gateway.yaml')
   OR message LIKE '%crypto-miner-demo%'
   OR policy LIKE '%crypto-miner-demo%';

DELETE FROM drift_records
WHERE resource IN ('crypto-miner-demo', 'istio-ingress-gateway.yaml');

DELETE FROM audit_findings
WHERE description LIKE '%crypto-miner-demo%'
   OR description LIKE '%istio-ingress-gateway.yaml%';
