-- Migration: 052_node_metric_rollups.up.sql
-- Description: Create node_metric_rollups table for per-node granular time-series historical telemetry

CREATE TABLE IF NOT EXISTS node_metric_rollups (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    node_id TEXT NOT NULL,
    node_name TEXT NOT NULL,
    cpu_percent DOUBLE PRECISION NOT NULL DEFAULT 0,
    cpu_peak DOUBLE PRECISION NOT NULL DEFAULT 0,
    mem_used_bytes BIGINT NOT NULL DEFAULT 0,
    mem_total_bytes BIGINT NOT NULL DEFAULT 0,
    mem_percent DOUBLE PRECISION NOT NULL DEFAULT 0,
    disk_used_bytes BIGINT NOT NULL DEFAULT 0,
    disk_total_bytes BIGINT NOT NULL DEFAULT 0,
    disk_percent DOUBLE PRECISION NOT NULL DEFAULT 0,
    rx_bytes_per_sec BIGINT NOT NULL DEFAULT 0,
    tx_bytes_per_sec BIGINT NOT NULL DEFAULT 0,
    process_count INT NOT NULL DEFAULT 0,
    container_count INT NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'online',
    resolution TEXT NOT NULL DEFAULT '1m', -- '1m', '1h', '1d'
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_node_rollups_lookup 
    ON node_metric_rollups(node_id, resolution, recorded_at DESC);
CREATE INDEX IF NOT EXISTS idx_node_rollups_tenant 
    ON node_metric_rollups(tenant_id, recorded_at DESC);
CREATE INDEX IF NOT EXISTS idx_node_rollups_recorded_at 
    ON node_metric_rollups(recorded_at DESC);
