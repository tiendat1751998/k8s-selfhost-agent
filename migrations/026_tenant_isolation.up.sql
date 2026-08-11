-- Migration: 026_tenant_isolation.up.sql
-- Description: Adds tenant_id column to core tables (incidents, rca_reports, gitops_prs) referencing organizations(id).

ALTER TABLE incidents 
    ADD COLUMN IF NOT EXISTS tenant_id VARCHAR(255) NOT NULL DEFAULT 'org-google' 
    REFERENCES organizations(id) ON DELETE CASCADE;

ALTER TABLE rca_reports 
    ADD COLUMN IF NOT EXISTS tenant_id VARCHAR(255) NOT NULL DEFAULT 'org-google' 
    REFERENCES organizations(id) ON DELETE CASCADE;

ALTER TABLE gitops_prs 
    ADD COLUMN IF NOT EXISTS tenant_id VARCHAR(255) NOT NULL DEFAULT 'org-google' 
    REFERENCES organizations(id) ON DELETE CASCADE;

-- Indexes for performance on tenant-filtered queries
CREATE INDEX IF NOT EXISTS idx_incidents_tenant ON incidents (tenant_id);
CREATE INDEX IF NOT EXISTS idx_rca_reports_tenant ON rca_reports (tenant_id);
CREATE INDEX IF NOT EXISTS idx_gitops_prs_tenant ON gitops_prs (tenant_id);
