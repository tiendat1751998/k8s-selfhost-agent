-- Migration: 015_timeline.sql
-- Creates tables for the deployment timeline.

CREATE TABLE IF NOT EXISTS timeline_events (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    type        VARCHAR(50) NOT NULL,
    title       VARCHAR(255) NOT NULL,
    detail      TEXT NOT NULL,
    namespace   VARCHAR(100),
    cluster     VARCHAR(100),
    metadata    JSONB DEFAULT '{}'::jsonb,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_timeline_events_type ON timeline_events (type);
CREATE INDEX idx_timeline_events_created_at ON timeline_events (created_at DESC);
