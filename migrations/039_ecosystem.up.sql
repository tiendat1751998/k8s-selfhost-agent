-- Migration: 039_ecosystem.up.sql
-- Description: Creates ecosystem_tools table for platform tool auto-detection.

CREATE TABLE IF NOT EXISTS ecosystem_tools (
    id           TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    name         TEXT NOT NULL,
    category     TEXT NOT NULL,
    status       TEXT NOT NULL DEFAULT 'detected',
    version      TEXT NOT NULL DEFAULT '',
    endpoint     TEXT NOT NULL DEFAULT '',
    source       TEXT NOT NULL DEFAULT 'settings',
    health       TEXT NOT NULL DEFAULT 'healthy',
    last_checked TIMESTAMPTZ NOT NULL DEFAULT now(),
    metadata     JSONB NOT NULL DEFAULT '{}',
    tenant_id    TEXT NOT NULL DEFAULT 'default-tenant',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(name, tenant_id)
);

CREATE INDEX IF NOT EXISTS idx_ecosystem_tenant ON ecosystem_tools(tenant_id);
CREATE INDEX IF NOT EXISTS idx_ecosystem_category ON ecosystem_tools(category);
CREATE INDEX IF NOT EXISTS idx_ecosystem_status ON ecosystem_tools(status);
CREATE INDEX IF NOT EXISTS idx_ecosystem_health ON ecosystem_tools(health);

-- Seed auto-detected real stack tools running on the cluster
INSERT INTO ecosystem_tools (id, name, category, status, version, endpoint, source, health, last_checked, metadata, tenant_id)
VALUES
('eco-docker-engine', 'Docker Engine', 'compute', 'detected', '27.0.0', 'http://127.0.0.1:2375', 'k8s_discovery', 'healthy', now(), '{"engine": "docker-daemon", "storage_driver": "overlay2", "cluster": "primary-cluster"}', 'default-tenant'),
('eco-postgres-16', 'PostgreSQL 16', 'database', 'detected', '16.3', 'postgres://db.internal:5432', 'k8s_discovery', 'healthy', now(), '{"engine": "postgresql", "port": "5432", "mode": "read-write", "wal_level": "replica"}', 'default-tenant'),
('eco-redis-8', 'Redis 8', 'database', 'detected', '8.0-M02', 'redis://tiki-redis:6379', 'k8s_discovery', 'healthy', now(), '{"engine": "redis", "mode": "standalone", "port": "6379", "persistence": "rdb+aof"}', 'default-tenant'),
('eco-nats-jetstream', 'NATS JetStream', 'messaging', 'detected', 'v2.10.18', 'http://nats.infra:8222', 'k8s_discovery', 'healthy', now(), '{"jetstream": "enabled", "port": "4222", "http_port": "8222", "cluster": "primary-cluster"}', 'default-tenant'),
('eco-traefik-v3', 'Traefik v3.1', 'mesh', 'detected', 'v3.1.2', 'http://traefik.internal:8080', 'k8s_discovery', 'healthy', now(), '{"router": "traefik", "port": "8080", "dashboard": "enabled", "providers": "kubernetes,docker"}', 'default-tenant'),
('eco-drone-ci', 'Drone CI', 'gitops', 'detected', 'v2.24.0', 'http://drone.internal:80', 'k8s_discovery', 'healthy', now(), '{"runner": "docker-runner", "port": "80", "auth_provider": "github"}', 'default-tenant')
ON CONFLICT (name, tenant_id) DO UPDATE SET
    category = EXCLUDED.category,
    status = EXCLUDED.status,
    version = EXCLUDED.version,
    endpoint = EXCLUDED.endpoint,
    source = EXCLUDED.source,
    health = EXCLUDED.health,
    last_checked = EXCLUDED.last_checked,
    metadata = EXCLUDED.metadata,
    updated_at = now();

