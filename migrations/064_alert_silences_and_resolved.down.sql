-- Migration: 064_alert_silences_and_resolved.down.sql
-- Description: Drop alert_silences table and resolved_at column.

DROP TABLE IF EXISTS alert_silences;
ALTER TABLE alert_history DROP COLUMN IF EXISTS resolved_at;
