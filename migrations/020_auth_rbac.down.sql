-- Migration: 020_auth_rbac.down.sql
-- Description: Drops encrypted_token column and RBAC/Auth tables.

ALTER TABLE fleet_clusters DROP COLUMN IF EXISTS encrypted_token;
DROP TABLE IF EXISTS tenant_bindings CASCADE;
DROP TABLE IF EXISTS user_roles CASCADE;
DROP TABLE IF EXISTS roles CASCADE;
DROP TABLE IF EXISTS users CASCADE;
