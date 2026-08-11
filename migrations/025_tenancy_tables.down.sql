-- DOWN migration for 025_tenancy_tables.sql

DELETE FROM rbac_matrix WHERE permission IN ('Clusters Read', 'Clusters Write', 'Clusters Delete', 'Deployments Deploy', 'Deployments Scale', 'Deployments Rollback', 'GitOps Create PR', 'GitOps Approve PR', 'GitOps Merge PR', 'AI Ops Analyze', 'AI Ops Remediate');
DELETE FROM tenant_members WHERE username IN ('admin', 'sre-team', 'dev-lead', 'viewer', 'acme-operator', 'acme-dev');
DELETE FROM projects WHERE id IN ('proj-healing', 'proj-aiops', 'proj-web', 'proj-db');
DELETE FROM organizations WHERE id IN ('org-google', 'org-acme');

DROP TABLE IF EXISTS rbac_matrix;
DROP TABLE IF EXISTS tenant_members;
DROP TABLE IF EXISTS projects;
DROP TABLE IF EXISTS organizations;
