-- Migration: 013_fleet_view.down.sql
-- Description: Drops fleet view tables.

DROP TABLE IF EXISTS fleet_clusters CASCADE;
