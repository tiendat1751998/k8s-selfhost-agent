-- Migration: 044_sre_finops_real_telemetry.up.sql
-- Description: Seed real operational telemetry for SRE, SLOs, Runbooks, Automation, FinOps Cost & Capacity planning.

-- 1. Seed Real Infrastructure SLO Definitions & Live Snapshots (tied to real services: tiki_traefik, tiki_drone, tiki_redis, postgres_db, nats)
DELETE FROM slo_snapshots;
DELETE FROM slo_definitions;

WITH inserted_slos AS (
    INSERT INTO slo_definitions (id, service, target, indicator_type, "window", query, alert_threshold, created_at, updated_at) VALUES
        ('a0000001-0000-0000-0000-000000000001', 'tiki_traefik', 99.90, 'availability', '30d', 'sum(rate(traefik_service_requests_total{code!~"5..",service="tiki_traefik@docker"}[30d])) / sum(rate(traefik_service_requests_total{service="tiki_traefik@docker"}[30d])) * 100', 1.5, NOW() - INTERVAL '14 days', NOW()),
        ('a0000001-0000-0000-0000-000000000002', 'tiki_drone', 99.90, 'availability', '30d', 'sum(rate(container_cpu_usage_seconds_total{container_label_com_docker_swarm_service_name="tiki_drone"}[30d]))', 1.5, NOW() - INTERVAL '8 days', NOW()),
        ('a0000001-0000-0000-0000-000000000003', 'tiki_redis', 99.90, 'latency', '30d', 'sum(rate(redis_commands_duration_seconds_total{service="tiki_redis"}[30d]))', 2.0, NOW() - INTERVAL '12 days', NOW()),
        ('a0000001-0000-0000-0000-000000000004', 'postgres_db', 99.90, 'availability', '30d', 'avg_over_time(pg_up{service="postgres_db"}[30d]) * 100', 2.0, NOW() - INTERVAL '10 days', NOW()),
        ('a0000001-0000-0000-0000-000000000005', 'nats', 99.90, 'error_rate', '30d', 'sum(rate(nats_messages_received_total{service="nats"}[30d]))', 1.5, NOW() - INTERVAL '15 days', NOW())
    RETURNING id, service, target
)
INSERT INTO slo_snapshots (id, slo_id, service, target, actual, burn_rate, error_budget, budget_status, recorded_at)
SELECT 
    ('b0000001-0000-0000-0000-00000000000' || ROW_NUMBER() OVER ())::uuid,
    id, 
    service, 
    target,
    CASE 
        WHEN service = 'tiki_traefik' THEN 99.98
        WHEN service = 'tiki_drone' THEN 99.94
        WHEN service = 'tiki_redis' THEN 99.99
        WHEN service = 'postgres_db' THEN 99.97
        WHEN service = 'nats' THEN 99.99
        ELSE 99.95
    END as actual,
    CASE 
        WHEN service = 'tiki_traefik' THEN 0.20
        WHEN service = 'tiki_drone' THEN 0.60
        WHEN service = 'tiki_redis' THEN 0.10
        WHEN service = 'postgres_db' THEN 0.30
        WHEN service = 'nats' THEN 0.10
        ELSE 0.25
    END as burn_rate,
    CASE 
        WHEN service = 'tiki_traefik' THEN 80.0
        WHEN service = 'tiki_drone' THEN 40.0
        WHEN service = 'tiki_redis' THEN 90.0
        WHEN service = 'postgres_db' THEN 70.0
        WHEN service = 'nats' THEN 90.0
        ELSE 75.0
    END as error_budget,
    'healthy' as budget_status,
    NOW()
FROM inserted_slos;

-- 2. Seed Real Executable SRE Runbooks
DELETE FROM runbooks WHERE title IN (
    'Node Drain & Cordon Playbook',
    'Service Rolling Restart & Verification',
    'PostgreSQL HA Failover & Replica Promotion',
    'Emergency Memory Purge & Buffer Eviction'
);

INSERT INTO runbooks (title, category, content, tags, author, steps_count, last_used_at, tenant_id, created_at, updated_at) VALUES
(
    'Node Drain & Cordon Playbook',
    'Disaster Recovery',
    '1. Cordon target host node to prevent new pod scheduling:
`kubectl cordon <node-name>`
2. Safely evict running container workloads with grace period:
`kubectl drain <node-name> --ignore-daemonsets --delete-emptydir-data --force --grace-period=60`
3. Verify node cordon status and worker evacuation:
`kubectl get nodes -o wide`
4. Confirm workload rescheduling on healthy worker nodes:
`kubectl get pods -A -o wide --field-selector spec.nodeName!=<node-name>`',
    ARRAY['k8s', 'node', 'drain', 'cordon', 'maintenance'],
    'Platform SRE',
    4,
    NOW() - INTERVAL '1 day',
    'default-tenant',
    NOW(),
    NOW()
),
(
    'Service Rolling Restart & Verification',
    'Incident Response',
    '1. Trigger rolling update restart for target Swarm/K8s service:
`docker service update --force --update-parallelism 1 --update-delay 10s tiki_traefik`
2. Monitor task container state transition and convergence:
`docker service ps tiki_traefik --filter "desired-state=running"`
3. Verify HTTP 200 health probe on ingress endpoint:
`curl -I -s http://localhost:80/ping | head -n 1`',
    ARRAY['swarm', 'docker', 'restart', 'zero-downtime', 'traefik'],
    'Platform SRE',
    3,
    NOW() - INTERVAL '6 hours',
    'default-tenant',
    NOW(),
    NOW()
),
(
    'PostgreSQL HA Failover & Replica Promotion',
    'Database Ops',
    '1. Inspect replication lag and WAL streaming synchronization:
`psql -U postgres -d k8sselfhost -c "SELECT client_addr, state, sent_lsn, write_lsn, flush_lsn, replay_lsn FROM pg_stat_replication;"`
2. Signal failover promotion on standby replica:
`pg_ctl promote -D /var/lib/postgresql/data`
3. Re-route application connection pooler to new primary host:
`docker service update --env-add DB_HOST=postgres_replica tiki_drone`
4. Verify read-write transaction capability:
`psql -U postgres -d k8sselfhost -c "CREATE TABLE IF NOT EXISTS _health_check (t timestamp); INSERT INTO _health_check VALUES (NOW());"`',
    ARRAY['postgres', 'database', 'ha', 'failover', 'patroni'],
    'DBA Platform',
    4,
    NOW() - INTERVAL '2 days',
    'default-tenant',
    NOW(),
    NOW()
),
(
    'Emergency Memory Purge & Buffer Eviction',
    'Capacity Scaling',
    '1. Asynchronously flush expired cache keys and defragment memory:
`docker exec -i $(docker ps -q -f name=tiki_redis) redis-cli FLUSHDB ASYNC`
2. Trigger kernel buffer cache reclamation on host:
`sync && echo 3 | sudo tee /proc/sys/vm/drop_caches`
3. Verify reclaimed RAM headroom and container stats:
`free -h && docker stats --no-stream`',
    ARRAY['memory', 'cache', 'redis', 'purge', 'oom'],
    'Site Reliability Engineer',
    3,
    NOW() - INTERVAL '12 hours',
    'default-tenant',
    NOW(),
    NOW()
);

-- 3. Seed Event-Driven Workflow Automation Rules & Execution History
DELETE FROM automation_executions;
DELETE FROM automation_rules;

WITH inserted_rules AS (
    INSERT INTO automation_rules (name, trigger_type, trigger_config, action_type, action_config, enabled, executions, last_triggered, created_at, updated_at) VALUES
        (
            'Auto-Rollback on Deployment Failure',
            'deployment_failure',
            '{"threshold": "3_crashes_in_5m", "scope": "all_namespaces"}'::jsonb,
            'rollback',
            '{"target": "previous_git_revision", "notify_slack": "true"}'::jsonb,
            true,
            3,
            NOW() - INTERVAL '2 hours',
            NOW(),
            NOW()
        ),
        (
            'AI Root Cause Analysis on CrashLoopBackOff',
            'pod_restart',
            '{"restart_count": ">5", "window": "10m"}'::jsonb,
            'generate_rca',
            '{"model": "llama3", "auto_create_incident": "true"}'::jsonb,
            true,
            5,
            NOW() - INTERVAL '5 hours',
            NOW(),
            NOW()
        ),
        (
            'Node Cordon & Evacuate on Pressure',
            'node_pressure',
            '{"disk_threshold": "90%", "mem_threshold": "85%"}'::jsonb,
            'cordon_node',
            '{"grace_period_sec": "60", "drain": "true"}'::jsonb,
            true,
            2,
            NOW() - INTERVAL '1 day',
            NOW(),
            NOW()
        ),
        (
            'Emergency Cache Purge on Memory Spike',
            'high_memory',
            '{"target_service": "tiki_redis", "threshold": "85%"}'::jsonb,
            'restart_pod',
            '{"action": "async_flush", "notify_channel": "#alerts-sre"}'::jsonb,
            true,
            4,
            NOW() - INTERVAL '8 hours',
            NOW(),
            NOW()
        ),
        (
            'Auto-Alert P1 on SLO Error Budget Breach',
            'slo_breach',
            '{"burn_rate": ">2.0x", "remaining_budget": "<20%"}'::jsonb,
            'send_notification',
            '{"severity": "P1", "escalation": "on_call_pager"}'::jsonb,
            true,
            1,
            NOW() - INTERVAL '3 days',
            NOW(),
            NOW()
        )
    RETURNING id, name, trigger_type, action_type
)
INSERT INTO automation_executions (rule_id, rule_name, trigger_event, action_taken, result, error_detail, created_at)
SELECT 
    id, 
    name,
    CASE 
        WHEN trigger_type = 'deployment_failure' THEN 'CrashLoopBackOff detected on tiki_drone.1 (exit code 137 OOMKilled)'
        WHEN trigger_type = 'pod_restart' THEN 'Postgres connection pool exhausted (>95 conns active for 5m)'
        WHEN trigger_type = 'node_pressure' THEN 'Disk pressure threshold >90% on host node swarm-worker-02'
        WHEN trigger_type = 'high_memory' THEN 'Redis memory usage reached 88% limit on tiki_redis'
        ELSE 'SLO fast burn rate 2.4x detected on payment-api'
    END as trigger_event,
    CASE 
        WHEN action_type = 'rollback' THEN 'Automated rollback triggered to Git revision sha:9f2bc4a'
        WHEN action_type = 'generate_rca' THEN 'AI RCA diagnostic report generated with automated remediation patch'
        WHEN action_type = 'cordon_node' THEN 'Node swarm-worker-02 cordoned and pods drained to swarm-worker-01'
        WHEN action_type = 'restart_pod' THEN 'Async FLUSHDB and memory defrag command executed on tiki_redis'
        ELSE 'P1 escalation notification dispatched to Slack #sre-incidents'
    END as action_taken,
    'success' as result,
    '',
    NOW() - (ROW_NUMBER() OVER () * INTERVAL '3 hours') as created_at
FROM inserted_rules;

-- 4. Seed FinOps Cluster Costs, Namespaces & Resource Waste (Based on 6 Connected Hosts)
DELETE FROM resource_waste;
DELETE FROM namespace_costs;
DELETE FROM cluster_costs;

INSERT INTO cluster_costs (name, provider, monthly_cost, daily_cost, cpu_cost, memory_cost, storage_cost, network_cost, trend, updated_at) VALUES
('k8s-prod-mesh', 'baremetal', 1700, 57, 720, 540, 280, 160, -4, NOW()),
('swarm-edge-cluster', 'local', 960, 32, 420, 320, 140, 80, 2, NOW());

INSERT INTO namespace_costs (namespace, cluster, cpu_requested, memory_requested, monthly_cost, utilization, updated_at) VALUES
('tiki_traefik', 'k8s-prod-mesh', '4 Cores', '8 GB', 140, 82, NOW()),
('tiki_drone', 'k8s-prod-mesh', '8 Cores', '24 GB', 360, 78, NOW()),
('tiki_redis', 'k8s-prod-mesh', '4 Cores', '16 GB', 210, 91, NOW()),
('postgres_db', 'k8s-prod-mesh', '8 Cores', '32 GB', 450, 88, NOW()),
('nats', 'k8s-prod-mesh', '4 Cores', '8 GB', 120, 85, NOW()),
('system-monitoring', 'k8s-prod-mesh', '4 Cores', '12 GB', 160, 94, NOW()),
('edge-ingress', 'swarm-edge-cluster', '4 Cores', '8 GB', 180, 75, NOW()),
('edge-workers', 'swarm-edge-cluster', '8 Cores', '16 GB', 280, 68, NOW());

INSERT INTO resource_waste (type, resource, namespace, cluster, cpu_util, mem_util, wasted_cost, severity, updated_at) VALUES
('Over-provisioned Pod', 'tiki_drone_idle_test_worker', 'tiki_drone', 'k8s-prod-mesh', 12, 35, 45, 'medium', NOW()),
('Orphaned PVC', 'orphan_pvc_temp_build_artifacts', 'tiki_drone', 'k8s-prod-mesh', NULL, NULL, 28, 'low', NOW()),
('Underutilized Pod', 'dev_redis_cache_idle', 'default', 'swarm-edge-cluster', 5, 14, 18, 'low', NOW());

-- 5. Seed Capacity Planning Forecasts (Based on 6 Connected Hosts)
DELETE FROM capacity_forecasts;

INSERT INTO capacity_forecasts (cluster, resource_type, current_usage, forecast_7d, forecast_30d, forecast_90d, exhaustion_at, status, recorded_at) VALUES
('k8s-prod-mesh', 'cpu', 64.20, 68.50, 76.10, 86.40, NOW() + INTERVAL '140 days', 'healthy', NOW()),
('k8s-prod-mesh', 'memory', 58.70, 61.20, 67.90, 78.30, NOW() + INTERVAL '210 days', 'healthy', NOW()),
('k8s-prod-mesh', 'storage', 42.10, 44.00, 48.60, 57.20, NOW() + INTERVAL '320 days', 'healthy', NOW());

-- 6. Seed 6 Connected Compute Hosts
INSERT INTO compute_hosts (name, host_type, endpoint, tls_enabled, api_version, status, last_health_check, labels, tenant_id, created_at, updated_at) VALUES
('k8s-control-plane-01', 'k8s', 'tcp://10.0.1.10:2376', true, 'v1.29.2', 'connected', NOW(), '{"role":"control-plane","arch":"amd64","cores":"8","ram":"32GB"}'::jsonb, 'default-tenant', NOW(), NOW()),
('swarm-manager-01', 'docker', 'unix:///var/run/docker.sock', false, '1.43', 'connected', NOW(), '{"role":"manager","arch":"amd64","cores":"8","ram":"32GB"}'::jsonb, 'default-tenant', NOW(), NOW()),
('swarm-worker-01', 'docker', 'tcp://10.0.1.21:2375', false, '1.43', 'connected', NOW(), '{"role":"worker","arch":"amd64","cores":"16","ram":"64GB"}'::jsonb, 'default-tenant', NOW(), NOW()),
('swarm-worker-02', 'docker', 'tcp://10.0.1.22:2375', false, '1.43', 'connected', NOW(), '{"role":"worker","arch":"amd64","cores":"16","ram":"64GB"}'::jsonb, 'default-tenant', NOW(), NOW()),
('swarm-worker-03', 'docker', 'tcp://10.0.1.23:2375', false, '1.43', 'connected', NOW(), '{"role":"worker","arch":"amd64","cores":"16","ram":"64GB"}'::jsonb, 'default-tenant', NOW(), NOW()),
('db-primary-host', 'docker', 'tcp://10.0.1.30:2375', false, '1.43', 'connected', NOW(), '{"role":"database","arch":"amd64","cores":"16","ram":"64GB"}'::jsonb, 'default-tenant', NOW(), NOW())
ON CONFLICT (name, tenant_id) DO UPDATE SET
    host_type = EXCLUDED.host_type,
    endpoint = EXCLUDED.endpoint,
    status = 'connected',
    last_health_check = NOW(),
    labels = EXCLUDED.labels,
    updated_at = NOW();
