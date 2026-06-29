-- Migration: 010_tagging_system.sql
-- Creates tables for the tagging system.

CREATE TABLE IF NOT EXISTS tags (
    id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    key           VARCHAR(100) NOT NULL,
    value         VARCHAR(255) NOT NULL,
    category      VARCHAR(100) NOT NULL DEFAULT 'custom',
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    UNIQUE(key, value)
);

CREATE TABLE IF NOT EXISTS resource_tags (
    resource_id   VARCHAR(255) NOT NULL,
    tag_id        UUID NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    PRIMARY KEY (resource_id, tag_id)
);

CREATE INDEX idx_tag_key_value ON tags (key, value);
CREATE INDEX idx_resource_tags_res ON resource_tags (resource_id);
CREATE INDEX idx_resource_tags_tag ON resource_tags (tag_id);
