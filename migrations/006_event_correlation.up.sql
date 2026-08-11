-- Migration: 006_event_correlation.sql
-- Creates tables for event correlation engine.

CREATE TABLE IF NOT EXISTS correlated_events (
    id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    title         VARCHAR(500) NOT NULL,
    root_cause    TEXT         NOT NULL DEFAULT '',
    severity      VARCHAR(20)  NOT NULL DEFAULT 'medium', -- critical | high | medium | low
    event_ids     JSONB        NOT NULL DEFAULT '[]',     -- array of raw event strings/ids
    event_count   INTEGER      NOT NULL DEFAULT 0,
    cluster       VARCHAR(255) NOT NULL,
    namespace     VARCHAR(255) NOT NULL,
    status        VARCHAR(20)  NOT NULL DEFAULT 'active', -- active | resolved
    correlated_at TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_corr_cluster ON correlated_events (cluster);
CREATE INDEX idx_corr_status ON correlated_events (status);
CREATE INDEX idx_corr_time ON correlated_events (correlated_at DESC);
