-- Migration: 017_automation.down.sql
-- Description: Drops automation executions and rules tables.

DROP TABLE IF EXISTS automation_executions CASCADE;
DROP TABLE IF EXISTS automation_rules CASCADE;
