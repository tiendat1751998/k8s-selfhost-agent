-- Migration: 036_mark_admin_requires_password_change.up.sql
-- Description: Adds requires_password_change column to users table and flags default seeded admin.

ALTER TABLE users ADD COLUMN IF NOT EXISTS requires_password_change BOOLEAN NOT NULL DEFAULT FALSE;

-- Mark seeded default admin user as requiring password change on first login for security
UPDATE users
SET requires_password_change = TRUE
WHERE email = 'admin@k8s.local';
