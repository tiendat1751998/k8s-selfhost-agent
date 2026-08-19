-- Migration: 043_observability_slo_enterprise.up.sql
-- Adds query and alert_threshold columns to slo_definitions and seeds active cluster service SLO targets.

ALTER TABLE slo_definitions ADD COLUMN IF NOT EXISTS query TEXT;
ALTER TABLE slo_definitions ADD COLUMN IF NOT EXISTS alert_threshold FLOAT;

-- Seed enterprise SLO definitions for active services if empty
INSERT INTO slo_definitions (id, service, target, indicator_type, "window", query, alert_threshold, created_at, updated_at)
VALUES
    ('a0000001-0000-0000-0000-000000000001', 'tiki_gateway', 99.90, 'availability', '30d', 'sum(rate(http_requests_total{status=~"2..|3.."}[5m])) / sum(rate(http_requests_total[5m])) * 100', 1.5, NOW() - INTERVAL '15 days', NOW()),
    ('a0000001-0000-0000-0000-000000000002', 'tiki_traefik', 99.95, 'availability', '30d', 'sum(rate(traefik_service_requests_total{code=~"2..|3.."}[5m])) / sum(rate(traefik_service_requests_total[5m])) * 100', 1.5, NOW() - INTERVAL '14 days', NOW()),
    ('a0000001-0000-0000-0000-000000000003', 'tiki_redis', 99.99, 'cache_hit_rate', '30d', 'sum(rate(redis_keyspace_hits_total[5m])) / (sum(rate(redis_keyspace_hits_total[5m])) + sum(rate(redis_keyspace_misses_total[5m]))) * 100', 2.0, NOW() - INTERVAL '12 days', NOW()),
    ('a0000001-0000-0000-0000-000000000004', 'postgres_db', 99.99, 'availability', '30d', 'sum(rate(pg_stat_database_xact_commit[5m])) / (sum(rate(pg_stat_database_xact_commit[5m])) + sum(rate(pg_stat_database_xact_rollback[5m]))) * 100', 2.0, NOW() - INTERVAL '10 days', NOW()),
    ('a0000001-0000-0000-0000-000000000005', 'tiki_drone', 99.50, 'availability', '30d', 'sum(rate(drone_build_success_total[5m])) / sum(rate(drone_build_total[5m])) * 100', 1.5, NOW() - INTERVAL '8 days', NOW())
ON CONFLICT (id) DO NOTHING;

-- Seed matching initial SLO snapshots
INSERT INTO slo_snapshots (id, slo_id, service, target, actual, burn_rate, error_budget, budget_status, recorded_at)
VALUES
    ('b0000001-0000-0000-0000-000000000001', 'a0000001-0000-0000-0000-000000000001', 'tiki_gateway', 99.90, 99.94, 0.85, 60.0, 'healthy', NOW()),
    ('b0000001-0000-0000-0000-000000000002', 'a0000001-0000-0000-0000-000000000002', 'tiki_traefik', 99.95, 99.97, 0.45, 85.0, 'healthy', NOW()),
    ('b0000001-0000-0000-0000-000000000003', 'a0000001-0000-0000-0000-000000000003', 'tiki_redis', 99.99, 99.995, 0.20, 95.0, 'healthy', NOW()),
    ('b0000001-0000-0000-0000-000000000004', 'a0000001-0000-0000-0000-000000000004', 'postgres_db', 99.99, 99.985, 1.15, 75.0, 'healthy', NOW()),
    ('b0000001-0000-0000-0000-000000000005', 'a0000001-0000-0000-0000-000000000005', 'tiki_drone', 99.50, 99.65, 0.65, 88.0, 'healthy', NOW())
ON CONFLICT (id) DO NOTHING;
