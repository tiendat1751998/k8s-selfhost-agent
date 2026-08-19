-- Migration: 037_plugins.up.sql
-- Description: Creates plugins table for runtime frontend plugin system.

DROP TABLE IF EXISTS plugins;

CREATE TABLE IF NOT EXISTS plugins (
    id           TEXT PRIMARY KEY DEFAULT gen_random_uuid()::text,
    name         TEXT NOT NULL,
    version      TEXT NOT NULL DEFAULT '1.0.0',
    description  TEXT NOT NULL DEFAULT '',
    author       TEXT NOT NULL DEFAULT '',
    enabled      BOOLEAN NOT NULL DEFAULT true,
    entry_point  TEXT NOT NULL DEFAULT '',
    icon         TEXT NOT NULL DEFAULT '',
    category     TEXT NOT NULL DEFAULT 'devtools',
    permissions  JSONB NOT NULL DEFAULT '[]',
    config       JSONB NOT NULL DEFAULT '{}',
    tenant_id    TEXT NOT NULL DEFAULT 'default-tenant',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(name, tenant_id)
);
CREATE INDEX IF NOT EXISTS idx_plugins_tenant ON plugins(tenant_id);
CREATE INDEX IF NOT EXISTS idx_plugins_enabled ON plugins(enabled);
CREATE INDEX IF NOT EXISTS idx_plugins_category ON plugins(category);

-- Seed active operational JS extension plugins
INSERT INTO plugins (id, name, version, description, author, enabled, entry_point, icon, category, permissions, config, tenant_id)
VALUES
('plg-trivy-sec', 'Trivy Security Scanner', 'v1.4.2', 'Injects live CVE vulnerability analysis, SBOM component graphs, and image scan metrics directly into pod detail drawers.', 'Aqua Security & Platform SRE', true, 'https://cdn.jsdelivr.net/npm/@k8s-plugins/trivy-scanner/dist/index.js', '🛡️', 'security', '["audit:read", "k8s:read", "cve:read"]', '{"scanner_endpoint": "http://trivy.security:4954", "auto_scan_on_mount": "true", "min_severity": "HIGH"}', 'default-tenant'),

('plg-grafana-embed', 'Grafana Dashboard Embed', 'v2.1.0', 'Seamlessly embeds contextual Grafana latency, throughput, and CPU/memory panels into deployment and service views.', 'Grafana Labs Community', true, 'https://cdn.jsdelivr.net/npm/@k8s-plugins/grafana-embed/dist/index.js', '📈', 'monitoring', '["metrics:read", "dashboards:view"]', '{"grafana_url": "http://grafana.monitoring:3000", "theme": "dark", "refresh_interval": "15s"}', 'default-tenant'),

('plg-prom-alertbridge', 'Prometheus AlertBridge', 'v1.2.0', 'Connects Alertmanager active firings, alert routing rules, and Prometheus SLI breach warnings to real-time notification toasts.', 'Prometheus Authors', true, 'https://cdn.jsdelivr.net/npm/@k8s-plugins/alert-bridge/dist/index.js', '🔥', 'monitoring', '["alerts:read", "notifications:send"]', '{"alertmanager_url": "http://alertmanager:9093", "poll_interval_sec": "10", "dedup_window": "60s"}', 'default-tenant'),

('plg-vector-logs', 'Vector Log Processor', 'v1.1.5', 'High-throughput client-side log parsing, regex highlighting, and structured JSON stream extraction for container logs.', 'Timber.io & Vector OSS', true, 'https://cdn.jsdelivr.net/npm/@k8s-plugins/vector-logs/dist/index.js', '📜', 'devtools', '["logs:read", "k8s:read"]', '{"buffer_lines": "5000", "ansi_highlighting": "true", "filter_heartbeats": "true"}', 'default-tenant')
ON CONFLICT (name, tenant_id) DO UPDATE SET
    version = EXCLUDED.version,
    description = EXCLUDED.description,
    author = EXCLUDED.author,
    enabled = EXCLUDED.enabled,
    entry_point = EXCLUDED.entry_point,
    icon = EXCLUDED.icon,
    category = EXCLUDED.category,
    permissions = EXCLUDED.permissions,
    config = EXCLUDED.config,
    updated_at = now();

