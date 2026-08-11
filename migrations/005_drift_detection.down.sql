-- Migration: 005_drift_detection.down.sql
-- Description: Drops drift detection tables.

DROP TABLE IF EXISTS drift_records CASCADE;
