-- Migration: 012_health_center.sql
-- Creates tables for the platform health center.

CREATE TABLE IF NOT EXISTS platform_health (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    component       VARCHAR(100) NOT NULL UNIQUE,
    status          VARCHAR(50)  NOT NULL DEFAULT 'unknown', -- healthy | degraded | down | unknown
    message         TEXT         NOT NULL DEFAULT '',
    last_checked_at TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- Insert default components
INSERT INTO platform_health (component, status, message) VALUES
    ('frontend', 'healthy', 'Frontend is serving traffic normally'),
    ('backend', 'healthy', 'API is responsive'),
    ('websocket', 'healthy', 'Real-time connections are stable'),
    ('ai', 'healthy', 'AI Copilot and RCA Engine are operational'),
    ('gitops', 'healthy', 'Syncing with Git providers'),
    ('database', 'healthy', 'Primary database is online')
ON CONFLICT (component) DO NOTHING;
