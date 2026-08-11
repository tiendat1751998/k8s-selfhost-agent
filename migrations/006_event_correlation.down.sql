-- Migration: 006_event_correlation.down.sql
-- Description: Drops correlated events tables.

DROP TABLE IF EXISTS correlated_events CASCADE;
