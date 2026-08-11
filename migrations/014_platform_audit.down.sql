-- Migration: 014_platform_audit.down.sql
-- Description: Drops platform audit tables.

DROP TABLE IF EXISTS audit_runs CASCADE;
DROP TABLE IF EXISTS audit_findings CASCADE;
