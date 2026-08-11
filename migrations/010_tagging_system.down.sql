-- Migration: 010_tagging_system.down.sql
-- Description: Drops tagging system tables.

DROP TABLE IF EXISTS resource_tags CASCADE;
DROP TABLE IF EXISTS tags CASCADE;
