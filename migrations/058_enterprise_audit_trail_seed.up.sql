-- Migration: 058_enterprise_audit_trail_seed.up.sql
-- Description: Seed production-grade baseline enterprise audit trail records.

INSERT INTO audit_logs (
    id,
    actor,
    action,
    target_type,
    target_id,
    target_name,
    result,
    details,
    ip_address,
    user_agent,
    created_at,
    tenant_id
) VALUES
-- 1. K8s mutation: apply on k8s_manifest istio-ingress-gateway.yaml
(
    'c0000000-0000-4000-8000-000000000001',
    'devops-lead@enterprise.io',
    'apply',
    'k8s_manifest',
    NULL,
    'istio-ingress-gateway.yaml',
    'success',
    '{"action_type": "mutation", "severity": "medium", "namespace": "istio-system", "cluster": "prod-core-01", "payload": {"kind": "Gateway", "apiVersion": "networking.istio.io/v1beta1", "annotations": {"managed-by": "platform-orchestrator"}}}'::jsonb,
    '10.240.0.15',
    'kubectl/v1.30.0 (linux/amd64)',
    NOW() - INTERVAL '5 hours',
    'default-tenant'
),
-- 2. K8s mutation: scale on k8s_deployment payment-service to 6 replicas
(
    'c0000000-0000-4000-8000-000000000002',
    'sre-oncall@enterprise.io',
    'scale',
    'k8s_deployment',
    NULL,
    'payment-service',
    'success',
    '{"action_type": "mutation", "severity": "medium", "namespace": "payments", "cluster": "prod-core-01", "payload": {"replicas": 6, "previous_replicas": 3, "reason": "traffic spike autoscale adjustment"}}'::jsonb,
    '10.240.0.22',
    'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36',
    NOW() - INTERVAL '4 hours 15 minutes',
    'default-tenant'
),
-- 3. RBAC grant: grant on k8s_rolebinding cluster-admin to service account vault-agent
(
    'c0000000-0000-4000-8000-000000000003',
    'security-admin@enterprise.io',
    'grant',
    'k8s_rolebinding',
    NULL,
    'cluster-admin',
    'success',
    '{"action_type": "rbac_grant", "severity": "high", "subject": "system:serviceaccount:vault:vault-agent", "role": "cluster-admin", "cluster": "prod-core-01", "payload": {"roleRef": "ClusterRole/cluster-admin", "serviceAccount": "vault-agent", "namespace": "vault"}}'::jsonb,
    '10.240.0.8',
    'Mozilla/5.0 (Windows NT 10.0; Win64; x64)',
    NOW() - INTERVAL '3 hours 40 minutes',
    'default-tenant'
),
-- 4. Pod evictions & node ops: cordon on k8s_node worker-pool-02-alpha
(
    'c0000000-0000-4000-8000-000000000004',
    'cluster-autoscaler@enterprise.io',
    'cordon',
    'k8s_node',
    NULL,
    'worker-pool-02-alpha',
    'success',
    '{"action_type": "mutation", "severity": "medium", "cluster": "prod-core-01", "payload": {"unschedulable": true, "reason": "Node drain preparing for AMI kernel patch update"}}'::jsonb,
    '10.240.1.102',
    'k8s-platform-agent/v2.4.1',
    NOW() - INTERVAL '2 hours 50 minutes',
    'default-tenant'
),
-- 5. Pod evictions & node ops: evict on k8s_pod analytics-worker-5b879 due to memory pressure
(
    'c0000000-0000-4000-8000-000000000005',
    'kubelet@worker-pool-02-alpha',
    'evict',
    'k8s_pod',
    NULL,
    'analytics-worker-5b879',
    'error',
    '{"action_type": "deletion", "severity": "critical", "namespace": "analytics", "cluster": "prod-core-01", "payload": {"eviction_reason": "The node had condition: [MemoryPressure]", "oom_score_adj": 998, "grace_period_seconds": 0}}'::jsonb,
    '10.240.1.102',
    'kubelet/v1.30.0 (linux/amd64)',
    NOW() - INTERVAL '2 hours 10 minutes',
    'default-tenant'
),
-- 6. Auth & Zero-Trust Denial: login denied from IP 198.51.100.42 (MFA challenge failed, 5 failed attempts)
(
    'c0000000-0000-4000-8000-000000000006',
    'attacker_probe@unknown',
    'login',
    'auth',
    NULL,
    'auth-portal',
    'denied',
    '{"action_type": "access", "severity": "critical", "reason": "MFA challenge failed, 5 failed attempts", "lockout_triggered": true, "payload": {"auth_strategy": "totp", "failed_attempts": 5, "ip_blocked": true}}'::jsonb,
    '198.51.100.42',
    'python-requests/2.31.0',
    NOW() - INTERVAL '1 hour 25 minutes',
    'default-tenant'
),
-- 7. Auth & Zero-Trust Denial: opa-gatekeeper admission reject on k8s_pod crypto-miner-demo (privilegeEscalation not allowed)
(
    'c0000000-0000-4000-8000-000000000007',
    'opa-gatekeeper',
    'admission_reject',
    'k8s_pod',
    NULL,
    'crypto-miner-demo',
    'denied',
    '{"action_type": "mutation", "severity": "critical", "namespace": "sandbox", "cluster": "prod-core-01", "reason": "privilegeEscalation not allowed", "payload": {"constraint": "k8spspprivilegedcontainer", "violating_container": "miner-process", "securityContext": {"allowPrivilegeEscalation": true}}}'::jsonb,
    '10.240.0.5',
    'gatekeeper.sh/v3.15.0',
    NOW() - INTERVAL '45 minutes',
    'default-tenant'
),
-- 8. Standard user login: john.doe@enterprise.io SSO/SAML success
(
    'c0000000-0000-4000-8000-000000000008',
    'john.doe@enterprise.io',
    'login',
    'auth',
    NULL,
    'enterprise-sso-saml',
    'success',
    '{"action_type": "access", "severity": "info", "auth_provider": "Okta-SAML-2.0", "roles": ["platform_admin", "devops"], "payload": {"session_duration_minutes": 480, "mfa_verified": true, "sso_relay_state": "dashboard_overview"}}'::jsonb,
    '203.0.113.19',
    'Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:128.0) Gecko/20100101 Firefox/128.0',
    NOW() - INTERVAL '20 minutes',
    'default-tenant'
),
-- 9. K8s deletion: delete on k8s_pod checkout-processor-deadlock-99x
(
    'c0000000-0000-4000-8000-000000000009',
    'sre-automation@enterprise.io',
    'delete',
    'k8s_pod',
    NULL,
    'checkout-processor-deadlock-99x',
    'success',
    '{"action_type": "deletion", "severity": "high", "namespace": "checkout", "cluster": "prod-core-01", "payload": {"force_delete": true, "reason": "Unresponsive pod health check failure"}}'::jsonb,
    '10.240.0.24',
    'k8s-operator/v1.12.0',
    NOW() - INTERVAL '5 minutes',
    'default-tenant'
)
ON CONFLICT (id) DO NOTHING;