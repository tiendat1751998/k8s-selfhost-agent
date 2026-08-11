-- Migration: 020_auth_rbac.sql
-- Description: Creates user and RBAC tables for enterprise multi-tenancy.

CREATE TABLE IF NOT EXISTS users (
    id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    email         VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    first_name    VARCHAR(100),
    last_name     VARCHAR(100),
    status        VARCHAR(50) DEFAULT 'active',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS roles (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name        VARCHAR(50) UNIQUE NOT NULL,
    description TEXT,
    permissions JSONB NOT NULL DEFAULT '[]'::jsonb
);

CREATE TABLE IF NOT EXISTS user_roles (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role_id UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, role_id)
);

CREATE TABLE IF NOT EXISTS tenant_bindings (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    tenant_id   VARCHAR(255) NOT NULL, -- Logical tenant / organization ID
    role_id     UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Add encrypted credentials for clusters
ALTER TABLE fleet_clusters ADD COLUMN IF NOT EXISTS encrypted_token TEXT;

-- Seed basic roles
INSERT INTO roles (name, description, permissions) VALUES 
('platform_admin', 'Full access to all platform resources', '["*"]'),
('tenant_admin', 'Full access to tenant-scoped resources', '["tenant:*"]'),
('viewer', 'Read-only access to tenant resources', '["tenant:read"]')
ON CONFLICT (name) DO NOTHING;

-- Seed an initial admin user (password: admin)
-- The hash below is a bcrypt hash for 'admin' (cost 10).
INSERT INTO users (email, password_hash, first_name, last_name) VALUES 
('admin@k8sselfhost.local', '$2a$10$xseVQz9NZxRiL6XHVHfIxec2sPO3vH2gfRjgRCXXJfrc64/hCDzzq', 'Platform', 'Admin')
ON CONFLICT (email) DO NOTHING;

-- Bind the admin user to the platform_admin role
INSERT INTO user_roles (user_id, role_id)
SELECT u.id, r.id FROM users u, roles r 
WHERE u.email = 'admin@k8sselfhost.local' AND r.name = 'platform_admin'
ON CONFLICT DO NOTHING;
