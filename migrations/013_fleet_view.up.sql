-- Migration: 013_fleet_view.sql
-- Creates tables for the fleet view.

CREATE TABLE IF NOT EXISTS fleet_clusters (
    id            VARCHAR(255) PRIMARY KEY,
    name          VARCHAR(255) NOT NULL,
    "group"       VARCHAR(100) NOT NULL, -- "group" is a reserved word in some SQL variants, quoted for safety
    region        VARCHAR(100) NOT NULL,
    provider      VARCHAR(100) NOT NULL,
    status        VARCHAR(50)  NOT NULL DEFAULT 'offline',
    version       VARCHAR(50)  NOT NULL DEFAULT 'unknown',
    nodes         INT          NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_fleet_clusters_group ON fleet_clusters ("group");
CREATE INDEX idx_fleet_clusters_region ON fleet_clusters (region);
CREATE INDEX idx_fleet_clusters_status ON fleet_clusters (status);

-- Insert some dummy clusters for development
INSERT INTO fleet_clusters (id, name, "group", region, provider, status, version, nodes) VALUES
    ('cls-prod-us-east-1', 'prod-us-east', 'production', 'us-east-1', 'aws', 'active', 'v1.28.2', 15),
    ('cls-prod-eu-west-1', 'prod-eu-west', 'production', 'eu-west-1', 'aws', 'active', 'v1.28.2', 12),
    ('cls-stg-us-east-1', 'staging-east', 'staging', 'us-east-1', 'aws', 'active', 'v1.29.0', 5),
    ('cls-edge-tokyo', 'edge-tk1', 'edge', 'ap-northeast-1', 'gcp', 'upgrading', 'v1.27.5', 3),
    ('cls-dev-local', 'docker-desktop', 'development', 'local', 'docker', 'active', 'v1.29.1', 1)
ON CONFLICT (id) DO NOTHING;
