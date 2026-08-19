-- Migration: 035_service_catalog.up.sql
-- Description: Creates service_catalog table for Backstage-inspired service catalog registry.

DROP TABLE IF EXISTS service_catalog;

CREATE TABLE IF NOT EXISTS service_catalog (
    id           TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    name         TEXT NOT NULL,
    description  TEXT NOT NULL DEFAULT '',
    type         TEXT NOT NULL DEFAULT 'service',
    lifecycle    TEXT NOT NULL DEFAULT 'development',
    owner_team   TEXT NOT NULL DEFAULT '',
    owner_email  TEXT NOT NULL DEFAULT '',
    repo_url     TEXT NOT NULL DEFAULT '',
    docs_url     TEXT NOT NULL DEFAULT '',
    tags         JSONB NOT NULL DEFAULT '[]',
    annotations  JSONB NOT NULL DEFAULT '{}',
    tenant_id    TEXT NOT NULL DEFAULT 'default-tenant',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(name, tenant_id)
);
CREATE INDEX IF NOT EXISTS idx_catalog_tenant ON service_catalog(tenant_id);
CREATE INDEX IF NOT EXISTS idx_catalog_type ON service_catalog(type);
CREATE INDEX IF NOT EXISTS idx_catalog_lifecycle ON service_catalog(lifecycle);
CREATE INDEX IF NOT EXISTS idx_catalog_owner ON service_catalog(owner_team);

-- Seed real active stack service entries
INSERT INTO service_catalog (id, name, description, type, lifecycle, owner_team, owner_email, repo_url, docs_url, tags, annotations, tenant_id)
VALUES
('srv-tiki-drone', 'tiki_drone', 'Drone CI/CD automation server and pipeline build runner for platform workloads', 'worker', 'production', 'devops-team', 'devops@tiki.corp', 'https://github.com/drone/drone', 'https://docs.drone.io', '["ci-cd", "automation", "runner", "drone"]', '{"k8s.io/namespace": "ci", "k8s.io/deployment": "tiki-drone", "api.endpoint": "http://drone.internal:80", "cluster": "primary-cluster"}', 'default-tenant'),
('srv-tiki-traefik', 'tiki_traefik', 'Edge reverse proxy, API gateway & ingress controller with TLS termination', 'api', 'production', 'networking-team', 'network-sre@tiki.corp', 'https://github.com/traefik/traefik', 'https://doc.traefik.io/traefik/', '["ingress", "traefik", "gateway", "reverse-proxy", "tls"]', '{"k8s.io/namespace": "kube-system", "k8s.io/deployment": "tiki-traefik", "api.endpoint": "http://traefik.internal:8080", "cluster": "primary-cluster"}', 'default-tenant'),
('srv-tiki-redis', 'tiki_redis', 'High-throughput in-memory caching layer, session storage, and pub/sub broker', 'database', 'production', 'data-platform', 'dba@tiki.corp', 'https://github.com/redis/redis', 'https://redis.io/docs/', '["redis", "cache", "key-value", "database"]', '{"k8s.io/namespace": "storage", "k8s.io/statefulset": "tiki-redis", "api.endpoint": "redis://tiki-redis:6379", "cluster": "primary-cluster"}', 'default-tenant'),
('srv-tiki-cart', 'tiki_cart', 'Shopping cart state, session checkout orchestration, and promo calculation service', 'service', 'production', 'checkout-squad', 'cart-devs@tiki.corp', 'https://github.com/tiki/tiki-cart', 'https://docs.tiki.corp/services/cart', '["cart", "checkout", "ecommerce", "microservice", "golang"]', '{"k8s.io/namespace": "production", "k8s.io/deployment": "tiki-cart", "api.endpoint": "http://tiki-cart.production.svc:8080", "cluster": "primary-cluster"}', 'default-tenant'),
('srv-tiki-product', 'tiki_product', 'Product catalog indexing, inventory query, and semantic search API', 'api', 'production', 'catalog-squad', 'catalog-devs@tiki.corp', 'https://github.com/tiki/tiki-product', 'https://docs.tiki.corp/services/product', '["product", "catalog", "search", "inventory", "python"]', '{"k8s.io/namespace": "production", "k8s.io/deployment": "tiki-product", "api.endpoint": "http://tiki-product.production.svc:8000", "cluster": "primary-cluster"}', 'default-tenant'),
('srv-postgres-db', 'postgres_db', 'Primary PostgreSQL 16 ACID relational datastore for orders, inventory, and accounts', 'database', 'production', 'data-platform', 'dba@tiki.corp', 'https://github.com/postgres/postgres', 'https://www.postgresql.org/docs/16/', '["postgres", "sql", "database", "relational", "ha"]', '{"k8s.io/namespace": "storage", "k8s.io/statefulset": "postgres-db", "api.endpoint": "postgres://db.internal:5432", "cluster": "primary-cluster"}', 'default-tenant'),
('srv-nats', 'nats', 'High-performance cloud-native messaging bus & JetStream event persistence streaming', 'worker', 'production', 'infra-core', 'infra@tiki.corp', 'https://github.com/nats-io/nats-server', 'https://docs.nats.io/', '["nats", "jetstream", "messaging", "pubsub", "streaming"]', '{"k8s.io/namespace": "infra", "k8s.io/deployment": "nats", "api.endpoint": "nats://nats.infra:4222", "cluster": "primary-cluster"}', 'default-tenant')
ON CONFLICT (name, tenant_id) DO UPDATE SET
    description = EXCLUDED.description,
    type = EXCLUDED.type,
    lifecycle = EXCLUDED.lifecycle,
    owner_team = EXCLUDED.owner_team,
    owner_email = EXCLUDED.owner_email,
    repo_url = EXCLUDED.repo_url,
    docs_url = EXCLUDED.docs_url,
    tags = EXCLUDED.tags,
    annotations = EXCLUDED.annotations,
    updated_at = now();

