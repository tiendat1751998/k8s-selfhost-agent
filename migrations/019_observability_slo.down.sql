-- Migration: 019_observability_slo.down.sql
-- Description: Drops slo_snapshots and slo_definitions tables.

DROP TABLE IF EXISTS slo_snapshots CASCADE;
DROP TABLE IF EXISTS slo_definitions CASCADE;
