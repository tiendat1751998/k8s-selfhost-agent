-- Migration: 025_tenancy_tables.sql
-- Description: Creates organizations, projects, members, and rbac_matrix tables.

CREATE TABLE IF NOT EXISTS organizations (
    id   VARCHAR(255) PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    tier VARCHAR(50) NOT NULL DEFAULT 'Standard'
);

CREATE TABLE IF NOT EXISTS projects (
    id        VARCHAR(255) PRIMARY KEY,
    org_id    VARCHAR(255) REFERENCES organizations(id) ON DELETE CASCADE,
    name      VARCHAR(255) NOT NULL,
    envs      JSONB NOT NULL DEFAULT '[]'::jsonb,
    workloads INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS tenant_members (
    id       UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    org_id   VARCHAR(255) REFERENCES organizations(id) ON DELETE CASCADE,
    username VARCHAR(255) NOT NULL,
    role     VARCHAR(100) NOT NULL,
    scope    VARCHAR(255) NOT NULL
);

CREATE TABLE IF NOT EXISTS rbac_matrix (
    permission VARCHAR(255) PRIMARY KEY,
    roles      JSONB NOT NULL DEFAULT '{}'::jsonb
);

-- Seed initial organizations
INSERT INTO organizations (id, name, tier) VALUES
('org-google', 'Google DeepMind', 'Enterprise'),
('org-acme', 'Acme Corp', 'Standard')
ON CONFLICT (id) DO NOTHING;

-- Seed initial projects
INSERT INTO projects (id, org_id, name, envs, workloads) VALUES
('proj-healing', 'org-google', 'K8s Self Healing', '["dev", "staging", "prod"]'::jsonb, 12),
('proj-aiops', 'org-google', 'AIOps Analyzer', '["dev", "staging"]'::jsonb, 6),
('proj-web', 'org-acme', 'Acme Web Portal', '["dev", "prod"]'::jsonb, 4),
('proj-db', 'org-acme', 'Acme DB Store', '["prod"]'::jsonb, 3)
ON CONFLICT (id) DO NOTHING;

-- Seed initial members
INSERT INTO tenant_members (org_id, username, role, scope) VALUES
('org-google', 'admin', 'Platform Admin', 'Global'),
('org-google', 'sre-team', 'DevOps Team', 'Google DeepMind'),
('org-google', 'dev-lead', 'Developer', 'K8s Self Healing'),
('org-google', 'viewer', 'Viewer', 'Google DeepMind'),
('org-acme', 'acme-operator', 'DevOps Team', 'Acme Corp'),
('org-acme', 'acme-dev', 'Developer', 'Acme Web Portal')
ON CONFLICT DO NOTHING;

-- Seed initial RBAC matrix mapping
INSERT INTO rbac_matrix (permission, roles) VALUES
('Clusters Read', '{"Platform Admin": true, "Org Admin": true, "DevOps Team": true, "Developer": true, "Viewer": true}'::jsonb),
('Clusters Write', '{"Platform Admin": true, "Org Admin": true, "DevOps Team": true, "Developer": false, "Viewer": false}'::jsonb),
('Clusters Delete', '{"Platform Admin": true, "Org Admin": false, "DevOps Team": false, "Developer": false, "Viewer": false}'::jsonb),
('Deployments Deploy', '{"Platform Admin": true, "Org Admin": true, "DevOps Team": true, "Developer": true, "Viewer": false}'::jsonb),
('Deployments Scale', '{"Platform Admin": true, "Org Admin": true, "DevOps Team": true, "Developer": true, "Viewer": false}'::jsonb),
('Deployments Rollback', '{"Platform Admin": true, "Org Admin": true, "DevOps Team": true, "Developer": false, "Viewer": false}'::jsonb),
('GitOps Create PR', '{"Platform Admin": true, "Org Admin": true, "DevOps Team": true, "Developer": true, "Viewer": false}'::jsonb),
('GitOps Approve PR', '{"Platform Admin": true, "Org Admin": true, "DevOps Team": true, "Developer": false, "Viewer": false}'::jsonb),
('GitOps Merge PR', '{"Platform Admin": true, "Org Admin": false, "DevOps Team": true, "Developer": false, "Viewer": false}'::jsonb),
('AI Ops Analyze', '{"Platform Admin": true, "Org Admin": true, "DevOps Team": true, "Developer": true, "Viewer": true}'::jsonb),
('AI Ops Remediate', '{"Platform Admin": true, "Org Admin": true, "DevOps Team": true, "Developer": false, "Viewer": false}'::jsonb)
ON CONFLICT (permission) DO NOTHING;
