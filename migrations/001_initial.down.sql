-- Migration: 001_initial.down.sql
-- Description: Drops core initial tables (gitops_prs, rca_reports, incidents).

DROP TABLE IF EXISTS gitops_prs CASCADE;
DROP TABLE IF EXISTS rca_reports CASCADE;
DROP TABLE IF EXISTS incidents CASCADE;
