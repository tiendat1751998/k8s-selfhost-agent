-- Migration: 002_enterprise_config.down.sql
-- Description: Drops triggers, helper function, and enterprise config tables.

DROP TRIGGER IF EXISTS trg_gitops_prs_updated ON gitops_prs;
DROP TRIGGER IF EXISTS trg_incidents_updated ON incidents;
DROP TRIGGER IF EXISTS trg_ai_providers_updated ON ai_providers;
DROP TRIGGER IF EXISTS trg_cicd_updated ON cicd_integrations;
DROP TRIGGER IF EXISTS trg_git_providers_updated ON git_providers;
DROP TRIGGER IF EXISTS trg_clusters_updated ON container_clusters;

DROP FUNCTION IF EXISTS update_updated_at_column();

DROP TABLE IF EXISTS agent_runs CASCADE;
DROP TABLE IF EXISTS audit_logs CASCADE;
DROP TABLE IF EXISTS connection_health CASCADE;
DROP TABLE IF EXISTS ai_providers CASCADE;
DROP TABLE IF EXISTS cicd_integrations CASCADE;
DROP TABLE IF EXISTS git_providers CASCADE;
DROP TABLE IF EXISTS container_clusters CASCADE;
