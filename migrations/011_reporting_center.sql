-- Migration: 011_reporting_center.sql
-- Creates tables for the reporting center.

CREATE TABLE IF NOT EXISTS reports (
    id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    title         VARCHAR(500) NOT NULL,
    type          VARCHAR(50)  NOT NULL, -- operational | security | compliance | cost | incident
    format        VARCHAR(20)  NOT NULL, -- pdf | excel | csv
    status        VARCHAR(50)  NOT NULL DEFAULT 'pending', -- pending | generating | completed | failed
    file_url      TEXT         NOT NULL DEFAULT '',
    created_by    VARCHAR(255) NOT NULL,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    expires_at    TIMESTAMPTZ  NOT NULL
);

CREATE INDEX idx_reports_type ON reports (type);
CREATE INDEX idx_reports_status ON reports (status);
CREATE INDEX idx_reports_created ON reports (created_at DESC);
