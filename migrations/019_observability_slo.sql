-- Migration: 019_observability_slo.sql
-- Creates tables for SLO definitions and snapshots.

CREATE TABLE IF NOT EXISTS slo_definitions (
    id             UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    service        VARCHAR(255) NOT NULL,
    target         FLOAT NOT NULL,
    indicator_type VARCHAR(50) NOT NULL,
    "window"       VARCHAR(50) NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS slo_snapshots (
    id             UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    slo_id         UUID NOT NULL REFERENCES slo_definitions(id) ON DELETE CASCADE,
    service        VARCHAR(255) NOT NULL,
    target         FLOAT NOT NULL,
    actual         FLOAT NOT NULL,
    burn_rate      FLOAT NOT NULL,
    error_budget   FLOAT NOT NULL,
    budget_status  VARCHAR(50) NOT NULL,
    recorded_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_slo_snapshots_slo_id ON slo_snapshots (slo_id);
CREATE INDEX idx_slo_snapshots_recorded_at ON slo_snapshots (recorded_at DESC);
