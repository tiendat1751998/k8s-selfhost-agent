-- Migration: 016_runbooks.sql
-- Creates tables for operational runbooks.

CREATE TABLE IF NOT EXISTS runbooks (
    id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    title         VARCHAR(255) NOT NULL,
    category      VARCHAR(100) NOT NULL,
    content       TEXT NOT NULL,
    tags          TEXT[] DEFAULT '{}',
    author        VARCHAR(100) NOT NULL,
    steps_count   INT NOT NULL DEFAULT 0,
    last_used_at  TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_runbooks_category ON runbooks (category);
CREATE INDEX idx_runbooks_created_at ON runbooks (created_at DESC);
