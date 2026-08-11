-- Migration: 004_capacity_planning.down.sql
-- Description: Drops capacity planning tables.

DROP TABLE IF EXISTS capacity_forecasts CASCADE;
