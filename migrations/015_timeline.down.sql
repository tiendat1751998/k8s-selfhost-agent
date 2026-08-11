-- Migration: 015_timeline.down.sql
-- Description: Drops timeline events tables.

DROP TABLE IF EXISTS timeline_events CASCADE;
