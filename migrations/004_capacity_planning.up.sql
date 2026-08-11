-- Migration: 004_capacity_planning.sql
-- Creates tables for capacity planning.

CREATE TABLE IF NOT EXISTS capacity_forecasts (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    cluster         VARCHAR(255) NOT NULL,
    resource_type   VARCHAR(50)  NOT NULL, -- cpu | memory | storage
    current_usage   DECIMAL(5,2) NOT NULL DEFAULT 0,
    forecast_7d     DECIMAL(5,2) NOT NULL DEFAULT 0,
    forecast_30d    DECIMAL(5,2) NOT NULL DEFAULT 0,
    forecast_90d    DECIMAL(5,2) NOT NULL DEFAULT 0,
    exhaustion_at   TIMESTAMPTZ,
    status          VARCHAR(20)  NOT NULL DEFAULT 'healthy', -- healthy | warning | critical
    recorded_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_capacity_cluster ON capacity_forecasts (cluster);
CREATE INDEX idx_capacity_recorded ON capacity_forecasts (recorded_at DESC);
