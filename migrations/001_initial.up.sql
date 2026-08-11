-- Migration: 001_initial.sql
-- Creates the core tables for the K8S Self-Healing system.

-- Enable UUID extension
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Incidents table (aggregate root)
CREATE TABLE IF NOT EXISTS incidents (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    cluster_name VARCHAR(255) NOT NULL,
    namespace   VARCHAR(255) NOT NULL,
    pod_name    VARCHAR(255) NOT NULL,
    type        VARCHAR(50)  NOT NULL,
    status      VARCHAR(50)  NOT NULL DEFAULT 'detected',
    severity    VARCHAR(20)  NOT NULL DEFAULT 'medium',
    message     TEXT         NOT NULL DEFAULT '',
    raw_data    JSONB        NOT NULL DEFAULT '{}',
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    resolved_at TIMESTAMPTZ
);

CREATE INDEX idx_incidents_status ON incidents (status);
CREATE INDEX idx_incidents_type ON incidents (type);
CREATE INDEX idx_incidents_severity ON incidents (severity);
CREATE INDEX idx_incidents_namespace ON incidents (namespace);
CREATE INDEX idx_incidents_cluster ON incidents (cluster_name);
CREATE INDEX idx_incidents_pod_type ON incidents (namespace, pod_name, type);
CREATE INDEX idx_incidents_created_at ON incidents (created_at DESC);

-- RCA Reports table
CREATE TABLE IF NOT EXISTS rca_reports (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    incident_id     UUID         NOT NULL REFERENCES incidents(id) ON DELETE CASCADE,
    root_cause      TEXT         NOT NULL,
    evidence        JSONB        NOT NULL DEFAULT '[]',
    confidence      DECIMAL(4,3) NOT NULL CHECK (confidence >= 0 AND confidence <= 1),
    risk_level      VARCHAR(20)  NOT NULL,
    remediation     TEXT         NOT NULL,
    rollback_plan   TEXT         NOT NULL DEFAULT '',
    llm_model       VARCHAR(100) NOT NULL DEFAULT '',
    prompt_tokens   INTEGER      NOT NULL DEFAULT 0,
    response_tokens INTEGER      NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_rca_reports_incident ON rca_reports (incident_id);
CREATE INDEX idx_rca_reports_confidence ON rca_reports (confidence DESC);

-- GitOps Pull Requests table
CREATE TABLE IF NOT EXISTS gitops_prs (
    id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    incident_id  UUID         NOT NULL REFERENCES incidents(id) ON DELETE CASCADE,
    provider     VARCHAR(20)  NOT NULL,
    repo_url     VARCHAR(500) NOT NULL,
    branch       VARCHAR(255) NOT NULL,
    base_branch  VARCHAR(255) NOT NULL DEFAULT 'main',
    title        VARCHAR(500) NOT NULL,
    description  TEXT         NOT NULL DEFAULT '',
    pr_url       VARCHAR(500) NOT NULL DEFAULT '',
    pr_number    INTEGER      NOT NULL DEFAULT 0,
    status       VARCHAR(20)  NOT NULL DEFAULT 'pending',
    files_changed JSONB       NOT NULL DEFAULT '[]',
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    merged_at    TIMESTAMPTZ
);

CREATE INDEX idx_gitops_prs_incident ON gitops_prs (incident_id);
CREATE INDEX idx_gitops_prs_status ON gitops_prs (status);
