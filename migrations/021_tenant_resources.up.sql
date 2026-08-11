-- Migration: 021_tenant_resources.sql
-- Description: Adds tenant_id to fleet_clusters and runbooks, and binds existing platform_admin.

ALTER TABLE fleet_clusters ADD COLUMN IF NOT EXISTS tenant_id VARCHAR(255) NOT NULL DEFAULT 'default-tenant';
ALTER TABLE runbooks ADD COLUMN IF NOT EXISTS tenant_id VARCHAR(255) NOT NULL DEFAULT 'default-tenant';

CREATE INDEX IF NOT EXISTS idx_fleet_clusters_tenant ON fleet_clusters (tenant_id);
CREATE INDEX IF NOT EXISTS idx_runbooks_tenant ON runbooks (tenant_id);

-- Bind initial seeded admin user ('admin@k8sselfhost.local') to the 'default-tenant' with 'platform_admin' role in tenant_bindings if not already done.
INSERT INTO tenant_bindings (user_id, tenant_id, role_id)
SELECT u.id, 'default-tenant', r.id 
FROM users u, roles r 
WHERE u.email = 'admin@k8sselfhost.local' AND r.name = 'platform_admin'
AND NOT EXISTS (
    SELECT 1 FROM tenant_bindings tb 
    JOIN users u2 ON tb.user_id = u2.id 
    JOIN roles r2 ON tb.role_id = r2.id
    WHERE u2.email = 'admin@k8sselfhost.local' AND r2.name = 'platform_admin' AND tb.tenant_id = 'default-tenant'
);
