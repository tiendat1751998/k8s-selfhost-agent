-- Migration: 007_change_management.sql
-- Creates tables for change management.

CREATE TABLE IF NOT EXISTS change_requests (
    id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    title         VARCHAR(500) NOT NULL,
    description   TEXT         NOT NULL DEFAULT '',
    type          VARCHAR(20)  NOT NULL DEFAULT 'standard', -- standard | emergency
    status        VARCHAR(20)  NOT NULL DEFAULT 'pending',  -- pending | approved | rejected | deployed
    requester     VARCHAR(255) NOT NULL,
    approver      VARCHAR(255),
    cluster       VARCHAR(255) NOT NULL,
    namespace     VARCHAR(255) NOT NULL,
    resource      VARCHAR(255) NOT NULL,
    scheduled_at  TIMESTAMPTZ,
    approved_at   TIMESTAMPTZ,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_change_status ON change_requests (status);
CREATE INDEX idx_change_created ON change_requests (created_at DESC);

CREATE TABLE IF NOT EXISTS maintenance_windows (
    id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    title         VARCHAR(500) NOT NULL,
    cluster       VARCHAR(255) NOT NULL,
    start_at      TIMESTAMPTZ  NOT NULL,
    end_at        TIMESTAMPTZ  NOT NULL,
    active        BOOLEAN      NOT NULL DEFAULT TRUE,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_maint_cluster ON maintenance_windows (cluster);
CREATE INDEX idx_maint_active ON maintenance_windows (active);
