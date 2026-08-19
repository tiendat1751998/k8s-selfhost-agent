CREATE TABLE IF NOT EXISTS compute_hosts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    host_type VARCHAR(50) NOT NULL DEFAULT 'docker', -- docker, k8s
    endpoint VARCHAR(512) NOT NULL, -- tcp://ip:port or unix:///var/run/docker.sock
    tls_enabled BOOLEAN NOT NULL DEFAULT false,
    tls_ca TEXT,
    tls_cert TEXT,
    tls_key TEXT, -- encrypted with crypto.Encrypt
    api_version VARCHAR(20) DEFAULT '',
    status VARCHAR(50) NOT NULL DEFAULT 'pending', -- connected, disconnected, pending, error
    last_health_check TIMESTAMPTZ,
    labels JSONB DEFAULT '{}',
    tenant_id VARCHAR(255) NOT NULL DEFAULT 'default-tenant',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(name, tenant_id)
);

CREATE INDEX IF NOT EXISTS idx_compute_hosts_tenant ON compute_hosts(tenant_id);
