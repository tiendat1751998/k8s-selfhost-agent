-- Migration: 044_sre_finops_real_telemetry.down.sql

DROP TABLE IF EXISTS _health_check;

DELETE FROM slo_snapshots WHERE service IN ('tiki_traefik', 'tiki_drone', 'tiki_redis', 'postgres_db', 'nats');
DELETE FROM slo_definitions WHERE service IN ('tiki_traefik', 'tiki_drone', 'tiki_redis', 'postgres_db', 'nats');

DELETE FROM runbooks WHERE title IN (
    'Node Drain & Cordon Playbook',
    'Service Rolling Restart & Verification',
    'PostgreSQL HA Failover & Replica Promotion',
    'Emergency Memory Purge & Buffer Eviction'
);

DELETE FROM automation_executions WHERE rule_name IN (
    'Auto-Rollback on Deployment Failure',
    'AI Root Cause Analysis on CrashLoopBackOff',
    'Node Cordon & Evacuate on Pressure',
    'Emergency Cache Purge on Memory Spike',
    'Auto-Alert P1 on SLO Error Budget Breach'
);
DELETE FROM automation_rules WHERE name IN (
    'Auto-Rollback on Deployment Failure',
    'AI Root Cause Analysis on CrashLoopBackOff',
    'Node Cordon & Evacuate on Pressure',
    'Emergency Cache Purge on Memory Spike',
    'Auto-Alert P1 on SLO Error Budget Breach'
);

DELETE FROM resource_waste WHERE cluster IN ('k8s-prod-mesh', 'swarm-edge-cluster');
DELETE FROM namespace_costs WHERE cluster IN ('k8s-prod-mesh', 'swarm-edge-cluster');
DELETE FROM cluster_costs WHERE name IN ('k8s-prod-mesh', 'swarm-edge-cluster');

DELETE FROM capacity_forecasts WHERE cluster IN ('k8s-prod-mesh', 'swarm-edge-cluster');

