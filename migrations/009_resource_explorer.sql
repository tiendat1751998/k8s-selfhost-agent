-- Migration: 009_resource_explorer.sql
-- Creates tables for resource explorer.

CREATE TABLE IF NOT EXISTS explorer_resources (
    id            VARCHAR(255) PRIMARY KEY, -- e.g. cluster/namespace/kind/name
    kind          VARCHAR(100) NOT NULL,
    name          VARCHAR(255) NOT NULL,
    namespace     VARCHAR(255) NOT NULL DEFAULT '',
    cluster       VARCHAR(255) NOT NULL,
    status        VARCHAR(50)  NOT NULL DEFAULT 'unknown',
    labels        JSONB        NOT NULL DEFAULT '{}',
    age           VARCHAR(50)  NOT NULL DEFAULT '',
    last_seen_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_exp_res_kind ON explorer_resources (kind);
CREATE INDEX idx_exp_res_cluster ON explorer_resources (cluster);
CREATE INDEX idx_exp_res_namespace ON explorer_resources (namespace);
CREATE INDEX idx_exp_res_name ON explorer_resources (name);
