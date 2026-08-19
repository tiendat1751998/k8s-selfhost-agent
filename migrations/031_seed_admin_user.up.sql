-- WARNING: FOR DEVELOPMENT / TESTING ONLY!
-- DO NOT USE DEFAULT CREDENTIALS (admin@k8s.local / admin123) IN PRODUCTION ENVIRONMENTS.
-- Seed admin user: admin@k8s.local / admin123
-- bcrypt hash for "admin123" at cost 10
INSERT INTO users (id, email, password_hash, first_name, last_name, status)
VALUES (
  'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11',
  'admin@k8s.local',
  '$2a$10$8T/QtlixRTiJ3kR1EHcz/udFIT6QRfYOJcy95uxdODsOZBiinEfSO',
  'Platform',
  'Admin',
  'active'
) ON CONFLICT (email) DO NOTHING;

-- Bind user to platform_admin role (use existing role by name lookup)
INSERT INTO user_roles (user_id, role_id)
SELECT 
  'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11'::uuid,
  r.id
FROM roles r
WHERE r.name = 'platform_admin'
ON CONFLICT DO NOTHING;

-- Bind user to default tenant with platform_admin role
INSERT INTO tenant_bindings (user_id, tenant_id, role_id)
SELECT
  'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11'::uuid,
  'default-tenant',
  r.id
FROM roles r
WHERE r.name = 'platform_admin'
ON CONFLICT DO NOTHING;
