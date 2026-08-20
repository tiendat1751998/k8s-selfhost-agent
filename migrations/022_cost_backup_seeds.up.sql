-- Migration: 022_cost_backup_seeds.sql

CREATE TABLE IF NOT EXISTS cluster_costs (
    id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name          VARCHAR(255) NOT NULL UNIQUE,
    provider      VARCHAR(50)  NOT NULL DEFAULT 'kubernetes',
    monthly_cost  INTEGER      NOT NULL DEFAULT 0,
    daily_cost    INTEGER      NOT NULL DEFAULT 0,
    cpu_cost      INTEGER      NOT NULL DEFAULT 0,
    memory_cost   INTEGER      NOT NULL DEFAULT 0,
    storage_cost  INTEGER      NOT NULL DEFAULT 0,
    network_cost  INTEGER      NOT NULL DEFAULT 0,
    trend         INTEGER      NOT NULL DEFAULT 0,
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS namespace_costs (
    id               UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    namespace        VARCHAR(255) NOT NULL,
    cluster          VARCHAR(255) NOT NULL,
    cpu_requested    VARCHAR(50)  NOT NULL,
    memory_requested VARCHAR(50)  NOT NULL,
    monthly_cost     INTEGER      NOT NULL DEFAULT 0,
    utilization      INTEGER      NOT NULL DEFAULT 0,
    updated_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS resource_waste (
    id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    type         VARCHAR(255) NOT NULL, -- Underutilized Pod | Orphaned PVC | Unused ConfigMap | Over-provisioned Deploy
    resource     VARCHAR(255) NOT NULL,
    namespace    VARCHAR(255) NOT NULL,
    cluster      VARCHAR(255) NOT NULL,
    cpu_util     INTEGER,
    mem_util     INTEGER,
    wasted_cost  INTEGER      NOT NULL DEFAULT 0,
    severity     VARCHAR(20)  NOT NULL DEFAULT 'medium', -- critical | high | medium | low
    updated_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS backup_history (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    timestamp   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    action      VARCHAR(50)  NOT NULL, -- backup | restore
    target      VARCHAR(255) NOT NULL,
    status      VARCHAR(50)  NOT NULL, -- success | failed | running
    duration    VARCHAR(50)  NOT NULL,
    size        VARCHAR(50)  NOT NULL,
    details     JSONB        NOT NULL DEFAULT '{}'
);

-- Seed event correlation
INSERT INTO correlated_events (title, root_cause, severity, event_count, cluster, namespace, status)
VALUES
('Payment API Gateway Timeout Cascading Failure', 'OOMKill on payment-db-redis-0 causing connection pool exhaustion in payment-api, leading to 504 Gateway Timeouts at Ingress.', 'critical', 142, 'production-us-east', 'payments', 'active'),
('High CPU on Authentication Service', 'Unoptimized crypto hashing triggered by sudden burst of login requests from 3 IP addresses.', 'warning', 28, 'production-eu-west', 'security', 'active')
ON CONFLICT DO NOTHING;

-- Seed costs
INSERT INTO cluster_costs (name, provider, monthly_cost, daily_cost, cpu_cost, memory_cost, storage_cost, network_cost, trend)
VALUES
('production-us-east', 'aws', 4250, 142, 1800, 1200, 750, 500, -3),
('production-eu-west', 'gcp', 1850, 62, 750, 550, 350, 200, 8),
('dev-cluster-local', 'azure', 920, 31, 400, 280, 140, 100, -1)
ON CONFLICT (name) DO NOTHING;

INSERT INTO namespace_costs (namespace, cluster, cpu_requested, memory_requested, monthly_cost, utilization)
VALUES
('production', 'production-us-east', '12.5 cores', '48 GiB', 2100, 78),
('staging', 'production-us-east', '8.0 cores', '32 GiB', 1350, 45),
('monitoring', 'production-us-east', '4.0 cores', '16 GiB', 680, 62),
('kube-system', 'production-us-east', '2.0 cores', '8 GiB', 320, 88),
('dev-apps', 'production-eu-west', '6.0 cores', '24 GiB', 980, 32),
('qa-testing', 'production-eu-west', '4.0 cores', '16 GiB', 620, 18)
ON CONFLICT DO NOTHING;

INSERT INTO resource_waste (type, resource, namespace, cluster, cpu_util, mem_util, wasted_cost, severity)
VALUES
('Underutilized Pod', 'analytics-worker-7f8a', 'production', 'production-us-east', 3, 8, 145, 'high'),
('Underutilized Pod', 'cache-warmer-5d2b', 'staging', 'production-us-east', 5, 12, 89, 'medium'),
('Orphaned PVC', 'data-vol-old-migration', 'production', 'production-us-east', NULL, NULL, 210, 'high'),
('Unused ConfigMap', 'legacy-config-v2', 'staging', 'production-eu-west', NULL, NULL, 0, 'low'),
('Over-provisioned Deploy', 'payment-gateway', 'production', 'production-us-east', 8, 15, 320, 'critical')
ON CONFLICT DO NOTHING;

-- Seed backups
INSERT INTO backup_history (action, target, status, duration, size, details)
VALUES
('backup', 'production-namespace', 'success', '45s', '1.2 GB', '{"cluster":"production-us-east","type":"full"}'),
('backup', 'payments-db', 'success', '12s', '250 MB', '{"cluster":"production-us-east","type":"incremental"}'),
('restore', 'qa-testing', 'success', '1m 15s', '800 MB', '{"cluster":"production-eu-west","type":"full"}')
ON CONFLICT DO NOTHING;
