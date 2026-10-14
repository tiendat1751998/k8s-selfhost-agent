-- Migration: 060_consensus_leases.up.sql
-- Description: Create consensus_leases table for multi-replica HA distributed fencing leases

CREATE TABLE IF NOT EXISTS consensus_leases (
    cluster_id VARCHAR(128) PRIMARY KEY,
    holder_id VARCHAR(255) NOT NULL,
    fencing_token BIGINT NOT NULL DEFAULT 1,
    acquired_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_consensus_leases_expires_at ON consensus_leases(expires_at);
