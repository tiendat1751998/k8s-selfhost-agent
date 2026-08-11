-- Migration: 005_drift_detection.sql
-- Creates tables for drift detection.

CREATE TABLE IF NOT EXISTS drift_records (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    cluster         VARCHAR(255) NOT NULL,
    namespace       VARCHAR(255) NOT NULL,
    resource        VARCHAR(255) NOT NULL,
    resource_kind   VARCHAR(100) NOT NULL,
    expected_state  TEXT         NOT NULL DEFAULT '',
    actual_state    TEXT         NOT NULL DEFAULT '',
    diff            TEXT         NOT NULL DEFAULT '',
    status          VARCHAR(50)  NOT NULL DEFAULT 'drifted', -- in_sync | drifted | unknown
    detected_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_drift_cluster ON drift_records (cluster);
CREATE INDEX idx_drift_status ON drift_records (status);
CREATE INDEX idx_drift_detected ON drift_records (detected_at DESC);
