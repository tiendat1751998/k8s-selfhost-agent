-- DOWN migration for 021_tenant_resources.sql

DELETE FROM tenant_bindings WHERE tenant_id = 'default-tenant' AND user_id IN (SELECT id FROM users WHERE email = 'admin@k8sselfhost.local') AND role_id IN (SELECT id FROM roles WHERE name = 'platform_admin');

DROP INDEX IF EXISTS idx_runbooks_tenant;
DROP INDEX IF EXISTS idx_fleet_clusters_tenant;

ALTER TABLE runbooks DROP COLUMN IF EXISTS tenant_id;
ALTER TABLE fleet_clusters DROP COLUMN IF EXISTS tenant_id;
