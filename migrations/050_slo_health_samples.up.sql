-- Migration: 050_slo_health_samples.up.sql
-- Description: Create slo_health_samples table for real live health telemetry sampling

CREATE TABLE IF NOT EXISTS slo_health_samples (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    service_name TEXT NOT NULL,
    desired_replicas INT NOT NULL DEFAULT 1,
    running_replicas INT NOT NULL DEFAULT 0,
    is_healthy BOOLEAN NOT NULL DEFAULT false,
    health_check_latency_ms INT,
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_slo_samples_service_time ON slo_health_samples(service_name, recorded_at DESC);
CREATE INDEX IF NOT EXISTS idx_slo_samples_tenant ON slo_health_samples(tenant_id);
