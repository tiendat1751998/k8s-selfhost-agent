-- Migration: 051_remove_seed_data.up.sql
-- Description: Remove ALL fake seeded data from the database to achieve a clean production state with ZERO fake data.

-- 1. Compliance Frameworks & Violations
DELETE FROM compliance_violations 
WHERE id::text LIKE 'c1000001%' 
   OR cluster = 'primary-cluster'
   OR framework_id IN (
       SELECT id FROM compliance_frameworks 
       WHERE id::text LIKE 'c0000001%' 
          OR name IN (
              'CIS Kubernetes Benchmark v1.8 (Level 2)', 
              'SOC 2 Type II Trust Services Criteria', 
              'HIPAA Security Safeguards (ePHI)', 
              'PCI-DSS v4.0 Requirement 6 & 10',
              'SOC2 Type II', 'ISO 27001', 'HIPAA Security Rule', 'PCI-DSS v3.2.1', 'CIS Kubernetes Benchmark'
          )
   );

DELETE FROM compliance_frameworks 
WHERE id::text LIKE 'c0000001%' 
   OR name IN (
       'CIS Kubernetes Benchmark v1.8 (Level 2)', 
       'SOC 2 Type II Trust Services Criteria', 
       'HIPAA Security Safeguards (ePHI)', 
       'PCI-DSS v4.0 Requirement 6 & 10',
       'SOC2 Type II', 'ISO 27001', 'HIPAA Security Rule', 'PCI-DSS v3.2.1', 'CIS Kubernetes Benchmark'
   );

-- 2. Platform Audit Findings & Runs
DELETE FROM audit_findings 
WHERE id::text LIKE 'e1000001%' 
   OR description IN (
       'CVE-2024-21626 (runc container breakout vulnerability) detected in worker node base runtime image.',
       'Terraform manifest modules/backup/s3.tf allows unencrypted object uploads without KMS SSE-S3 header.',
       'Kubernetes Pod Security Admission label pod-security.kubernetes.io/enforce missing on ingress namespace.',
       'Prometheus Alertmanager webhook notification target not configured for critical disaster recovery alerts.',
       'Excessive cluster admin roles bound to default service account',
       'Pods running with privileged security context',
       'Network policies not defined for ingress traffic in namespace default',
       'Image pull policy set to IfNotPresent with latest tag'
   );

DELETE FROM audit_runs 
WHERE id::text LIKE 'e0000001%' 
   OR id = 'e0000001-0000-0000-0000-000000000001';

-- 3. Disaster Recovery & Backups
DELETE FROM restore_jobs 
WHERE id::text LIKE 'd3000001%' 
   OR backup_job_id IN (SELECT id FROM backup_jobs WHERE id::text LIKE 'd2000001%');

DELETE FROM backup_jobs 
WHERE id::text LIKE 'd2000001%' 
   OR policy_id IN (SELECT id FROM backup_policies WHERE id::text LIKE 'd1000001%' OR name IN ('postgres-enterprise-primary', 'redis-cluster-persistence', 'nats-jetstream-eventstore'));

DELETE FROM backup_policies 
WHERE id::text LIKE 'd1000001%' 
   OR name IN ('postgres-enterprise-primary', 'redis-cluster-persistence', 'nats-jetstream-eventstore')
   OR storage_id IN (SELECT id FROM backup_storages WHERE id::text LIKE 'd0000001%' OR name IN ('Local NVMe Hot Store', 'AWS S3 Dual-Sync Cold Archive', 'MinIO Air-Gapped Object Vault'));

DELETE FROM backup_storages 
WHERE id::text LIKE 'd0000001%' 
   OR name IN ('Local NVMe Hot Store', 'AWS S3 Dual-Sync Cold Archive', 'MinIO Air-Gapped Object Vault');

DELETE FROM backup_history 
WHERE details::text IN ('Nightly full backup to S3', 'Hourly snapshot to EBS', 'Pre-deployment backup')
   OR target IN ('pg-prod-primary', 'redis-cache-cluster', 'mysql-analytics-db');

-- 4. GitOps Drift Records (preserving real scans from cls-dev-local)
DELETE FROM drift_records 
WHERE id::text LIKE 'f0000001%' 
   OR (cluster IN ('primary', 'edge-node-01') AND resource IN ('auth-service', 'gateway-rate-limits', 'redis-cluster', 'fluentbit-forwarder'))
   OR (cluster IN ('cls-prod-us-east-1', 'cls-prod-eu-west-1', 'cls-stg-us-east-1', 'cls-edge-tokyo'));

-- 5. Tenancy Projects, Members, and Demo Orgs (preserving default-tenant and admin user bindings)
DELETE FROM tenant_members 
WHERE id::text LIKE 'b0000001%' 
   OR username IN ('admin@k8s.local', 'sre-lead@k8s.local', 'sec-auditor@k8s.local', 'developer@k8s.local', 'admin', 'sre-team', 'dev-lead', 'viewer', 'acme-operator', 'acme-dev');

DELETE FROM projects 
WHERE id IN ('proj-core-platform', 'proj-telemetry-sec', 'proj-data-pipeline', 'proj-healing', 'proj-aiops', 'proj-web', 'proj-db');

DELETE FROM organizations 
WHERE id IN ('org-google', 'org-acme', 'audit-org', 'audit-corp', 'audit-corp-2');

-- 6. Automation Rules & Executions
DELETE FROM automation_executions 
WHERE rule_name IN (
    'Auto-Rollback on Deployment Failure',
    'AI Root Cause Analysis on CrashLoopBackOff',
    'Node Cordon & Evacuate on Pressure',
    'Emergency Cache Purge on Memory Spike',
    'Auto-Alert P1 on SLO Error Budget Breach',
    'Auto-scale on High CPU',
    'Restart Crashed Pods',
    'Notify Slack on Pod Crash',
    'Rollback Failed Deployments'
) OR rule_id IN (
    SELECT id FROM automation_rules 
    WHERE name IN (
        'Auto-Rollback on Deployment Failure',
        'AI Root Cause Analysis on CrashLoopBackOff',
        'Node Cordon & Evacuate on Pressure',
        'Emergency Cache Purge on Memory Spike',
        'Auto-Alert P1 on SLO Error Budget Breach',
        'Auto-scale on High CPU',
        'Restart Crashed Pods',
        'Notify Slack on Pod Crash',
        'Rollback Failed Deployments'
    )
);

DELETE FROM automation_rules 
WHERE name IN (
    'Auto-Rollback on Deployment Failure',
    'AI Root Cause Analysis on CrashLoopBackOff',
    'Node Cordon & Evacuate on Pressure',
    'Emergency Cache Purge on Memory Spike',
    'Auto-Alert P1 on SLO Error Budget Breach',
    'Auto-scale on High CPU',
    'Restart Crashed Pods',
    'Notify Slack on Pod Crash',
    'Rollback Failed Deployments'
);

-- 7. Runbooks
DELETE FROM runbooks 
WHERE title IN (
    'Node Drain & Cordon Playbook',
    'Service Rolling Restart & Verification',
    'PostgreSQL HA Failover & Replica Promotion',
    'Emergency Memory Purge & Buffer Eviction',
    'High CPU Investigation',
    'Pod CrashLoopBackOff Recovery',
    'Database Failover Procedure',
    'TLS Certificate Renewal'
);

-- 8. Service Catalog
DELETE FROM service_catalog 
WHERE id IN ('srv-tiki-drone', 'srv-tiki-traefik', 'srv-tiki-redis', 'srv-tiki-cart', 'srv-tiki-product', 'srv-postgres-db', 'srv-nats')
   OR name IN ('tiki_drone', 'tiki_traefik', 'tiki_redis', 'tiki_cart', 'tiki_product', 'postgres_db', 'nats', 'Payment Service', 'Order Service', 'Auth Service', 'Inventory DB', 'Notification Worker');

-- 9. Runtime Plugins
DELETE FROM plugins 
WHERE id IN ('plg-trivy-sec', 'plg-grafana-embed', 'plg-prom-alertbridge', 'plg-vector-logs')
   OR name IN ('Trivy Security Scanner', 'Grafana Dashboard Embed', 'Prometheus AlertBridge', 'Vector Log Processor');

-- 10. Ecosystem Tools (real tools auto-detected dynamically on scan)
DELETE FROM ecosystem_tools 
WHERE id IN ('eco-docker-engine', 'eco-postgres-16', 'eco-redis-8', 'eco-nats-jetstream', 'eco-traefik-v3', 'eco-drone-ci')
   OR name IN ('Docker Engine', 'PostgreSQL 16', 'Redis 8', 'NATS JetStream', 'Traefik v3.1', 'Drone CI');

-- 11. FinOps Costs & Resource Waste (computed dynamically)
DELETE FROM resource_waste;
DELETE FROM namespace_costs;
DELETE FROM cluster_costs;

-- 12. Capacity Forecasts (computed dynamically)
DELETE FROM capacity_forecasts;

-- 13. Observability SLO Definitions & Snapshots (clean fake services, retain real Docker services)
DELETE FROM slo_snapshots 
WHERE service IN ('payment-api', 'user-service', 'order-service', 'inventory-api', 'notification-svc', 'analytics-engine', 'tiki_gateway')
   OR slo_id IN (
       SELECT id FROM slo_definitions 
       WHERE service IN ('payment-api', 'user-service', 'order-service', 'inventory-api', 'notification-svc', 'analytics-engine', 'tiki_gateway')
   );

DELETE FROM slo_definitions 
WHERE service IN ('payment-api', 'user-service', 'order-service', 'inventory-api', 'notification-svc', 'analytics-engine', 'tiki_gateway');

-- Clean up any residual temporary artifacts
DROP TABLE IF EXISTS _health_check;
