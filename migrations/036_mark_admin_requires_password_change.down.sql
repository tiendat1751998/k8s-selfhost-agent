-- Migration: 036_mark_admin_requires_password_change.down.sql
-- Description: Reverses requires_password_change column addition on users table.

ALTER TABLE users DROP COLUMN IF EXISTS requires_password_change;
