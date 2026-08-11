-- Migration: 007_change_management.down.sql
-- Description: Drops change management tables.

DROP TABLE IF EXISTS maintenance_windows CASCADE;
DROP TABLE IF EXISTS change_requests CASCADE;
