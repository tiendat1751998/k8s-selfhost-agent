-- Migration: 065_purge_stale_seed_artifacts.down.sql
-- Description: Rollback for stale seed artifact purge. Purged fake seed artifacts are not restored.

ALTER TABLE compliance_violations DROP COLUMN IF EXISTS framework;
