-- Migration: 014_platform_audit.sql
-- Creates tables for the platform audit mode.

CREATE TABLE IF NOT EXISTS audit_findings (
    id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    category      VARCHAR(100) NOT NULL,
    severity      VARCHAR(50)  NOT NULL,
    description   TEXT         NOT NULL,
    remediation   TEXT         NOT NULL,
    status        VARCHAR(50)  NOT NULL DEFAULT 'open',
    detected_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    resolved_at   TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS audit_runs (
    id             UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    status         VARCHAR(50)  NOT NULL DEFAULT 'running',
    start_time     TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    end_time       TIMESTAMPTZ,
    findings_count INT          NOT NULL DEFAULT 0
);

CREATE INDEX idx_audit_findings_status ON audit_findings (status);
CREATE INDEX idx_audit_findings_category ON audit_findings (category);
CREATE INDEX idx_audit_runs_start ON audit_runs (start_time DESC);

-- Insert dummy findings
INSERT INTO audit_findings (category, severity, description, remediation, status) VALUES
    ('missing_integration', 'high', 'Slack integration is configured but token is invalid.', 'Update Slack API token in Settings.', 'open'),
    ('disconnected_cluster', 'critical', 'Edge cluster "edge-tk1" has missed 3 heartbeats.', 'Check network connectivity to cluster or restart agent.', 'open'),
    ('missing_dashboard', 'medium', 'No dashboard configured for "payments" namespace.', 'Create a Grafana dashboard for payments metrics.', 'open')
ON CONFLICT DO NOTHING;
