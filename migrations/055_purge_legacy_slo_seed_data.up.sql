-- Migration: 055_purge_legacy_slo_seed_data.up.sql
-- Description: Remove obsolete mock tiki_* seed data from SLO tables
DELETE FROM slo_snapshots WHERE service IN ('tiki_traefik', 'tiki_drone', 'tiki_redis', 'edge_gateway', 'ci_runner', 'app_cache', 'postgres_db', 'nats');
DELETE FROM slo_definitions WHERE service IN ('tiki_traefik', 'tiki_drone', 'tiki_redis', 'edge_gateway', 'ci_runner', 'app_cache', 'postgres_db', 'nats');