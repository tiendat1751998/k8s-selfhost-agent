-- Migration: 042_enterprise_governance_seeds.up.sql
-- Description: Seeds active default compliance frameworks, backup policies & storage targets, tenancy projects & RBAC matrix, audit findings, and GitOps drift records.

-- ═══════════════════════════════════════════════════════════
-- 1. Default Organization & Projects & Members (Tenancy)
-- ═══════════════════════════════════════════════════════════
INSERT INTO organizations (id, name, tier)
VALUES ('default-tenant', 'Enterprise Operations Hub', 'Enterprise')
ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name, tier = EXCLUDED.tier;

INSERT INTO projects (id, org_id, name, envs, workloads) VALUES
('proj-core-platform', 'default-tenant', 'Core Platform Infrastructure', '["dev", "staging", "prod"]'::jsonb, 18),
('proj-telemetry-sec', 'default-tenant', 'Security & Telemetry Mesh', '["staging", "prod"]'::jsonb, 12),
('proj-data-pipeline', 'default-tenant', 'Real-time Data Stream Engine', '["dev", "prod"]'::jsonb, 8)
ON CONFLICT (id) DO UPDATE SET org_id = EXCLUDED.org_id, name = EXCLUDED.name, envs = EXCLUDED.envs, workloads = EXCLUDED.workloads;

DELETE FROM tenant_members WHERE org_id = 'default-tenant';
INSERT INTO tenant_members (id, org_id, username, role, scope) VALUES
('b0000001-0000-0000-0000-000000000001', 'default-tenant', 'admin@k8s.local', 'Platform Admin', 'Global Multi-Tenant'),
('b0000001-0000-0000-0000-000000000002', 'default-tenant', 'sre-lead@k8s.local', 'DevOps Team', 'Core Platform Infrastructure'),
('b0000001-0000-0000-0000-000000000003', 'default-tenant', 'sec-auditor@k8s.local', 'Security Auditor', 'Security & Telemetry Mesh'),
('b0000001-0000-0000-0000-000000000004', 'default-tenant', 'developer@k8s.local', 'Developer', 'Real-time Data Stream Engine')
ON CONFLICT (id) DO NOTHING;

-- ═══════════════════════════════════════════════════════════
-- 2. RBAC Matrix (Roles -> Permissions)
-- ═══════════════════════════════════════════════════════════
INSERT INTO rbac_matrix (permission, roles) VALUES
('Platform Admin', '{"pods:read": true, "pods:write": true, "deployments:scale": true, "secrets:manage": true, "backups:execute": true, "nodes:drain": true, "ai:configure": true, "changes:approve": true, "audit:view": true}'::jsonb),
('DevOps Team', '{"pods:read": true, "pods:write": true, "deployments:scale": true, "secrets:manage": true, "backups:execute": true, "nodes:drain": false, "ai:configure": true, "changes:approve": true, "audit:view": true}'::jsonb),
('Developer', '{"pods:read": true, "pods:write": false, "deployments:scale": false, "secrets:manage": false, "backups:execute": false, "nodes:drain": false, "ai:configure": false, "changes:approve": false, "audit:view": true}'::jsonb),
('Viewer', '{"pods:read": true, "pods:write": false, "deployments:scale": false, "secrets:manage": false, "backups:execute": false, "nodes:drain": false, "ai:configure": false, "changes:approve": false, "audit:view": false}'::jsonb),
('Security Auditor', '{"pods:read": true, "pods:write": false, "deployments:scale": false, "secrets:manage": false, "backups:execute": false, "nodes:drain": false, "ai:configure": false, "changes:approve": false, "audit:view": true}'::jsonb)
ON CONFLICT (permission) DO UPDATE SET roles = EXCLUDED.roles;

-- ═══════════════════════════════════════════════════════════
-- 3. Compliance Frameworks & Policy Violations (SOC2 & CIS L2)
-- ═══════════════════════════════════════════════════════════
INSERT INTO compliance_frameworks (id, name, icon, total_checks, passed_checks, failed_checks, score, last_scan_at, updated_at) VALUES
('c0000001-0000-0000-0000-000000000001', 'CIS Kubernetes Benchmark v1.8 (Level 2)', '🛡️', 64, 59, 5, 92.19, NOW() - INTERVAL '1 hour', NOW()),
('c0000001-0000-0000-0000-000000000002', 'SOC 2 Type II Trust Services Criteria', '🔒', 48, 45, 3, 93.75, NOW() - INTERVAL '2 hours', NOW()),
('c0000001-0000-0000-0000-000000000003', 'HIPAA Security Safeguards (ePHI)', '🏥', 36, 34, 2, 94.44, NOW() - INTERVAL '4 hours', NOW()),
('c0000001-0000-0000-0000-000000000004', 'PCI-DSS v4.0 Requirement 6 & 10', '💳', 42, 40, 2, 95.24, NOW() - INTERVAL '6 hours', NOW())
ON CONFLICT (name) DO UPDATE SET
    icon = EXCLUDED.icon,
    total_checks = EXCLUDED.total_checks,
    passed_checks = EXCLUDED.passed_checks,
    failed_checks = EXCLUDED.failed_checks,
    score = EXCLUDED.score,
    last_scan_at = EXCLUDED.last_scan_at,
    updated_at = NOW();

DELETE FROM compliance_violations WHERE cluster = 'primary-cluster';
INSERT INTO compliance_violations (id, framework_id, severity, policy, resource, namespace, cluster, message, resolved, detected_at) VALUES
('c1000001-0000-0000-0000-000000000001', 'c0000001-0000-0000-0000-000000000001', 'high', 'CIS-5.2.2: Ensure container root filesystem is read-only', 'deployment/ingress-nginx-controller', 'ingress-nginx', 'primary-cluster', 'Container running with readOnlyRootFilesystem=false without required security context isolation', false, NOW() - INTERVAL '45 minutes'),
('c1000001-0000-0000-0000-000000000002', 'c0000001-0000-0000-0000-000000000001', 'medium', 'CIS-5.1.5: Disallow default service account automounting token', 'serviceaccount/default', 'default', 'primary-cluster', 'ServiceAccount automountServiceAccountToken is true on non-system workload namespace', false, NOW() - INTERVAL '1 hour'),
('c1000001-0000-0000-0000-000000000003', 'c0000001-0000-0000-0000-000000000002', 'critical', 'SOC2-CC6.1: Enforce mTLS encryption in-transit for all mesh ingress', 'ingress/payment-gateway', 'payments', 'primary-cluster', 'Ingress lacks TLS 1.3 strict cipher configuration and cert-manager automated rotation', false, NOW() - INTERVAL '2 hours'),
('c1000001-0000-0000-0000-000000000004', 'c0000001-0000-0000-0000-000000000002', 'critical', 'SOC2-CC6.6: Disallow privileged containers across tenant namespaces', 'deployment/metrics-aggregator', 'telemetry', 'primary-cluster', 'Pod spec contains securityContext.privileged: true violating non-root workload policy', false, NOW() - INTERVAL '3 hours'),
('c1000001-0000-0000-0000-000000000005', 'c0000001-0000-0000-0000-000000000002', 'high', 'SOC2-CC6.8: Verify container image signature via Cosign/Notary', 'deployment/order-processor', 'ecommerce', 'primary-cluster', 'Container image digest lacks cryptographic signature verification against enterprise cosign public key', false, NOW() - INTERVAL '4 hours'),
('c1000001-0000-0000-0000-000000000006', 'c0000001-0000-0000-0000-000000000004', 'high', 'PCI-DSS-6.4.3: Prevent unauthenticated secret exposure in environment', 'deployment/billing-service', 'finance', 'primary-cluster', 'Database credentials injected via plain env vars instead of HashiCorp Vault Secrets CSI driver', false, NOW() - INTERVAL '5 hours'),
('c1000001-0000-0000-0000-000000000007', 'c0000001-0000-0000-0000-000000000003', 'medium', 'HIPAA-164.312(a)(1): Audit logging active for ePHI database access', 'statefulset/patient-db', 'healthcare', 'primary-cluster', 'Postgres pgAudit extension inactive on production patient metadata cluster', false, NOW() - INTERVAL '6 hours')
ON CONFLICT (id) DO NOTHING;

-- ═══════════════════════════════════════════════════════════
-- 4. Disaster Recovery & Backup Engines (Postgres, Redis, NATS)
-- ═══════════════════════════════════════════════════════════
INSERT INTO backup_storages (id, tenant_id, name, type, endpoint, bucket, credentials, created_at, updated_at) VALUES
('d0000001-0000-0000-0000-000000000001', 'default-tenant', 'Local NVMe Hot Store', 'local', '/var/data/backups', 'nvme0n1-snapshots', '{}'::jsonb, NOW(), NOW()),
('d0000001-0000-0000-0000-000000000002', 'default-tenant', 'AWS S3 Dual-Sync Cold Archive', 's3', 'https://s3.us-east-1.amazonaws.com', 'k8s-dr-snapshots-us-east', '{}'::jsonb, NOW(), NOW()),
('d0000001-0000-0000-0000-000000000003', 'default-tenant', 'MinIO Air-Gapped Object Vault', 'minio', 'http://minio.storage.svc.cluster.local:9000', 'enterprise-vault-dr', '{}'::jsonb, NOW(), NOW())
ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name, type = EXCLUDED.type, endpoint = EXCLUDED.endpoint, bucket = EXCLUDED.bucket;

INSERT INTO backup_policies (id, tenant_id, name, db_type, db_host, db_port, db_name, storage_id, secondary_storage_id, schedule, retention_count, backup_type, compression_level, encryption_enabled, enabled, created_at, updated_at) VALUES
('d1000001-0000-0000-0000-000000000001', 'default-tenant', 'postgres-enterprise-primary', 'postgres', 'postgres.db.svc.cluster.local', 5432, 'k8s_production', 'd0000001-0000-0000-0000-000000000001', 'd0000001-0000-0000-0000-000000000002', '0 */4 * * *', 30, 'full', 3, true, true, NOW(), NOW()),
('d1000001-0000-0000-0000-000000000002', 'default-tenant', 'redis-cluster-persistence', 'redis', 'redis-cluster.cache.svc.cluster.local', 6379, '0', 'd0000001-0000-0000-0000-000000000001', NULL, '0 */2 * * *', 24, 'snapshot', 3, true, true, NOW(), NOW()),
('d1000001-0000-0000-0000-000000000003', 'default-tenant', 'nats-jetstream-eventstore', 'nats', 'nats.messaging.svc.cluster.local', 4222, 'EVENTS_STREAM', 'd0000001-0000-0000-0000-000000000002', NULL, '0 */1 * * *', 48, 'full', 3, true, true, NOW(), NOW())
ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name, db_type = EXCLUDED.db_type, db_host = EXCLUDED.db_host, db_port = EXCLUDED.db_port, db_name = EXCLUDED.db_name, schedule = EXCLUDED.schedule, retention_count = EXCLUDED.retention_count, backup_type = EXCLUDED.backup_type, enabled = EXCLUDED.enabled;

INSERT INTO backup_jobs (id, tenant_id, policy_id, status, backup_type, storage_path, local_storage_path, cloud_storage_path, size_bytes, compressed_size_bytes, duration_ms, checksum_sha256, wal_start_lsn, wal_end_lsn, verification_status, verified_at, error_message, created_at, updated_at) VALUES
('d2000001-0000-0000-0000-000000000001', 'default-tenant', 'd1000001-0000-0000-0000-000000000001', 'verified', 'full', 's3://k8s-dr-snapshots-us-east/postgres/2026/08/pg_dump_20260819_0000.zst', '/var/data/backups/pg_dump_20260819_0000.zst', 's3://k8s-dr-snapshots-us-east/postgres/2026/08/pg_dump_20260819_0000.zst', 1503238553, 401604608, 14250, 'a9f2571c890123ef456789abcde0123456789abcdef0123456789abcdef01234', '0/16B30F8', '0/16B9520', 'verified', NOW() - INTERVAL '1 hour', NULL, NOW() - INTERVAL '1 hour', NOW() - INTERVAL '1 hour'),
('d2000001-0000-0000-0000-000000000002', 'default-tenant', 'd1000001-0000-0000-0000-000000000002', 'verified', 'snapshot', '/var/data/backups/redis_dump_20260819_0100.rdb.zst', '/var/data/backups/redis_dump_20260819_0100.rdb.zst', NULL, 251658240, 50331648, 3120, 'b8e1923a789456bc123456def7890123456789abcdef0123456789abcdef0123', NULL, NULL, 'verified', NOW() - INTERVAL '30 minutes', NULL, NOW() - INTERVAL '30 minutes', NOW() - INTERVAL '30 minutes'),
('d2000001-0000-0000-0000-000000000003', 'default-tenant', 'd1000001-0000-0000-0000-000000000003', 'verified', 'full', 's3://k8s-dr-snapshots-us-east/nats/2026/08/nats_jetstream_20260819_0130.tar.zst', NULL, 's3://k8s-dr-snapshots-us-east/nats/2026/08/nats_jetstream_20260819_0130.tar.zst', 545259520, 115343360, 6800, 'c7d3841e987654ba321098fed6543210987654abcdef0123456789abcdef0123', NULL, NULL, 'verified', NOW() - INTERVAL '15 minutes', NULL, NOW() - INTERVAL '15 minutes', NOW() - INTERVAL '15 minutes')
ON CONFLICT (id) DO NOTHING;

INSERT INTO restore_jobs (id, tenant_id, backup_job_id, target_db_host, target_db_name, pitr_timestamp, dry_run, source_storage_type, verification_log, status, error_message, created_at, updated_at) VALUES
('d3000001-0000-0000-0000-000000000001', 'default-tenant', 'd2000001-0000-0000-0000-000000000001', 'postgres.staging.svc.cluster.local:5432', 'k8s_staging_verify', NOW() - INTERVAL '2 hours', false, 's3', 'WAL streaming replay successful. 1,420 tables and 28 schemas restored without integrity mismatch.', 'completed', NULL, NOW() - INTERVAL '2 hours', NOW() - INTERVAL '2 hours')
ON CONFLICT (id) DO NOTHING;

-- ═══════════════════════════════════════════════════════════
-- 5. Platform Audit Runs & Findings (Trivy CVE & Checkov IaC)
-- ═══════════════════════════════════════════════════════════
INSERT INTO audit_runs (id, status, start_time, end_time, findings_count) VALUES
('e0000001-0000-0000-0000-000000000001', 'completed', NOW() - INTERVAL '30 minutes', NOW() - INTERVAL '28 minutes', 4)
ON CONFLICT (id) DO UPDATE SET status = EXCLUDED.status, start_time = EXCLUDED.start_time, end_time = EXCLUDED.end_time, findings_count = EXCLUDED.findings_count;

DELETE FROM audit_findings WHERE id IN (
    'e1000001-0000-0000-0000-000000000001',
    'e1000001-0000-0000-0000-000000000002',
    'e1000001-0000-0000-0000-000000000003',
    'e1000001-0000-0000-0000-000000000004'
);
INSERT INTO audit_findings (id, category, severity, description, remediation, status, detected_at) VALUES
('e1000001-0000-0000-0000-000000000001', 'cve_vulnerability', 'critical', 'CVE-2024-21626 (runc container breakout vulnerability) detected in worker node base runtime image.', 'Upgrade runc package to >= v1.1.12 across all Linux host compute nodes.', 'open', NOW() - INTERVAL '2 hours'),
('e1000001-0000-0000-0000-000000000002', 'iac_misconfiguration', 'high', 'Terraform manifest modules/backup/s3.tf allows unencrypted object uploads without KMS SSE-S3 header.', 'Add aws_s3_bucket_server_side_encryption_configuration with AES256 or KMS key.', 'open', NOW() - INTERVAL '3 hours'),
('e1000001-0000-0000-0000-000000000003', 'iac_misconfiguration', 'high', 'Kubernetes Pod Security Admission label pod-security.kubernetes.io/enforce missing on ingress namespace.', 'Apply pod-security.kubernetes.io/enforce=restricted label to namespace manifest.', 'open', NOW() - INTERVAL '4 hours'),
('e1000001-0000-0000-0000-000000000004', 'missing_integration', 'medium', 'Prometheus Alertmanager webhook notification target not configured for critical disaster recovery alerts.', 'Configure Alertmanager receiver channel with Telegram/Slack webhook endpoint in alerts hub.', 'open', NOW() - INTERVAL '5 hours')
ON CONFLICT (id) DO NOTHING;

-- ═══════════════════════════════════════════════════════════
-- 6. GitOps IaC Drift Records
-- ═══════════════════════════════════════════════════════════
DELETE FROM drift_records WHERE id IN (
    'f0000001-0000-0000-0000-000000000001',
    'f0000001-0000-0000-0000-000000000002',
    'f0000001-0000-0000-0000-000000000003',
    'f0000001-0000-0000-0000-000000000004'
);
INSERT INTO drift_records (id, cluster, namespace, resource, resource_kind, expected_state, actual_state, diff, status, detected_at) VALUES
('f0000001-0000-0000-0000-000000000001', 'primary', 'production', 'auth-service', 'Deployment', 'apiVersion: apps/v1
kind: Deployment
metadata:
  name: auth-service
  namespace: production
spec:
  replicas: 3
  template:
    spec:
      containers:
      - name: auth
        image: registry.k8s.local/auth:v2.4.0', 'apiVersion: apps/v1
kind: Deployment
metadata:
  name: auth-service
  namespace: production
spec:
  replicas: 5
  template:
    spec:
      containers:
      - name: auth
        image: registry.k8s.local/auth:v2.4.1-hotfix', '--- Expected (Git Repository)
+++ Actual (Live Kubernetes)
@@ -6,3 +6,3 @@
-  replicas: 3
+  replicas: 5
-        image: registry.k8s.local/auth:v2.4.0
+        image: registry.k8s.local/auth:v2.4.1-hotfix', 'drifted', NOW() - INTERVAL '30 minutes'),
('f0000001-0000-0000-0000-000000000002', 'primary', 'ingress-nginx', 'gateway-rate-limits', 'ConfigMap', 'apiVersion: v1
kind: ConfigMap
metadata:
  name: gateway-rate-limits
  namespace: ingress-nginx
data:
  rate-limit-per-ip: "1000"', 'apiVersion: v1
kind: ConfigMap
metadata:
  name: gateway-rate-limits
  namespace: ingress-nginx
data:
  rate-limit-per-ip: "5000"', '--- Expected (Git Repository)
+++ Actual (Live Kubernetes)
@@ -6,1 +6,1 @@
-  rate-limit-per-ip: "1000"
+  rate-limit-per-ip: "5000"', 'drifted', NOW() - INTERVAL '1 hour'),
('f0000001-0000-0000-0000-000000000003', 'primary', 'cache', 'redis-cluster', 'Service', 'apiVersion: v1
kind: Service
metadata:
  name: redis-cluster
  namespace: cache
spec:
  ports:
  - port: 6379
    targetPort: 6379', 'apiVersion: v1
kind: Service
metadata:
  name: redis-cluster
  namespace: cache
spec:
  ports:
  - port: 6379
    targetPort: 6379', '', 'in_sync', NOW() - INTERVAL '2 hours'),
('f0000001-0000-0000-0000-000000000004', 'edge-node-01', 'logging', 'fluentbit-forwarder', 'DaemonSet', 'apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: fluentbit-forwarder
  namespace: logging
spec:
  template:
    spec:
      containers:
      - name: fluentbit
        image: fluent/fluent-bit:3.0.4', 'apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: fluentbit-forwarder
  namespace: logging
spec:
  template:
    spec:
      containers:
      - name: fluentbit
        image: fluent/fluent-bit:3.0.4', '', 'in_sync', NOW() - INTERVAL '3 hours')
ON CONFLICT (id) DO NOTHING;
