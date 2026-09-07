import { ref, computed, watch } from 'vue'
import { useRoute } from 'vue-router'
import { jsonToYaml } from '../utils/yaml'

export interface FeatureMetric {
  title: string
  value: string | number
  trend: string
  dir: 'up' | 'down' | 'neutral'
}

export interface GenericPlatformItem {
  id: string
  name: string
  detail: string
  status: string
  tag: string
  namespace?: string
  kind?: string
  apiVersion?: string
  createdAt?: string
  labels?: Record<string, string>
  annotations?: Record<string, string>
  raw?: Record<string, unknown>
  [key: string]: unknown
}

export interface FeatureInfo {
  title: string
  category: string
  badge: string
  desc: string
  metrics: FeatureMetric[]
  items: GenericPlatformItem[]
}

export interface DynamicColumn {
  key: string
  label: string
  sortable?: boolean
  width?: string
}

export interface SchemaAction {
  id: string
  label: string
  icon?: string
  variant?: 'primary' | 'secondary' | 'danger'
}

export const PLATFORM_FEATURE_MAP: Record<string, FeatureInfo> = {
  incidents: {
    title: 'Incident Triage & Root Cause Analysis', category: 'OBSERVABILITY & OPS', badge: 'Real-Time RCA Active',
    desc: 'Autonomous SRE agent correlating Prometheus spikes, pod crashloops, and network policies to identify root causes in seconds.',
    metrics: [
      { title: 'Active Incidents', value: '0 Critical', trend: 'All Systems Normal', dir: 'up' },
      { title: 'MTTR Average', value: '3.4 mins', trend: '-42% vs last month', dir: 'down' },
      { title: 'AI Remediations', value: '28 Executed', trend: 'Autonomous SRE', dir: 'up' },
      { title: 'SLA Uptime', value: '99.99%', trend: 'Tier 1 Guaranteed', dir: 'up' }
    ],
    items: [
      { id: 'inc-301', name: 'payment-processor Ingress Throttling', detail: 'HPA auto-scaled from 4 to 12 replicas. Resolved in 2.1m.', status: 'healthy', tag: 'RESOLVED', kind: 'Incident', namespace: 'payments' },
      { id: 'inc-300', name: 'worker-node-08 NVMe I/O Spike', detail: 'Dual-Sync backup resynced to secondary target. IOPS normalized.', status: 'healthy', tag: 'RESOLVED', kind: 'Incident', namespace: 'kube-system' },
      { id: 'inc-299', name: 'CoreDNS Upstream Resolution Delay', detail: 'DNS cache tuned, response latency reduced to <2ms.', status: 'healthy', tag: 'RESOLVED', kind: 'Incident', namespace: 'kube-system' }
    ]
  },
  agents: {
    title: 'Autonomous AI SRE Agents', category: 'OBSERVABILITY & OPS', badge: 'Multi-Agent Mesh Armed',
    desc: 'Distributed AI agents performing continuous cluster health auditing, automated runbook execution, and predictive scaling.',
    metrics: [
      { title: 'Active AI Agents', value: '8 Agents', trend: 'In-Pod & Cluster-Wide', dir: 'neutral' },
      { title: 'Autonomous Decisions', value: '142 / Day', trend: '99.8% Success Rate', dir: 'up' },
      { title: 'Circuit Breaker State', value: 'NORMAL', trend: 'Zero Faults Observed', dir: 'up' },
      { title: 'Active Models In Use', value: 'DeepSeek + Ollama', trend: 'Air-Gapped & Cloud', dir: 'neutral' }
    ],
    items: [
      { id: 'agent-rca', name: 'Autonomous Incident RCA Agent', detail: 'Monitors pod exits, kernel logs, and Prometheus metrics for real-time triage.', status: 'active', tag: 'RUNNING', kind: 'AIAgent', namespace: 'ai-ops' },
      { id: 'agent-finops', name: 'FinOps Node Bin-Packing Agent', detail: 'Dynamically rebalances pods to consolidate underutilized worker nodes.', status: 'active', tag: 'OPTIMIZING', kind: 'AIAgent', namespace: 'ai-ops' },
      { id: 'agent-security', name: 'ZeroTrust Trivy CVE Guard Agent', detail: 'Continuous scanner verifying newly deployed image SHAs against vulnerability DBs.', status: 'active', tag: 'ARMED', kind: 'AIAgent', namespace: 'security' }
    ]
  },
  slo: {
    title: 'SLO & Error Budget Management', category: 'OBSERVABILITY & OPS', badge: 'PromQL SLI Tracking',
    desc: 'Measure service reliability targets, monitor burn rates, and enforce error budget policies before deploying new releases.',
    metrics: [
      { title: 'Target Availability', value: '99.95%', trend: 'Current: 99.985%', dir: 'up' },
      { title: 'Remaining Error Budget', value: '84.2%', trend: 'Burn Rate: 0.8x (Safe)', dir: 'up' },
      { title: 'Monitored SLIs', value: '18 Services', trend: 'Latency & Error Rate', dir: 'neutral' },
      { title: 'Fast Burn Alerts', value: '0 Triggered', trend: 'Within Safe Bounds', dir: 'up' }
    ],
    items: [
      { id: 'slo-api', name: 'API Gateway Ingress Latency (P99 < 100ms)', detail: 'Current P99: 42ms. Error budget consumption: 4.1%.', status: 'healthy', tag: 'HEALTHY', kind: 'ServiceLevelObjective', namespace: 'gateway' },
      { id: 'slo-db', name: 'PostgreSQL Query Success Rate (> 99.9%)', detail: 'Current Rate: 99.994%. Error budget consumption: 1.2%.', status: 'healthy', tag: 'HEALTHY', kind: 'ServiceLevelObjective', namespace: 'databases' },
      { id: 'slo-auth', name: 'OAuth SSO Authentication Success Rate', detail: 'Current Rate: 99.98%. Zero degraded periods in last 30 days.', status: 'healthy', tag: 'HEALTHY', kind: 'ServiceLevelObjective', namespace: 'auth' }
    ]
  },
  fleet: {
    title: 'Fleet Multi-Cluster Management', category: 'COMPUTE & FLEET', badge: 'Multi-Region Mesh',
    desc: 'Centralized control plane managing hybrid and multi-cloud Kubernetes clusters with unified policy distribution.',
    metrics: [
      { title: 'Managed Clusters', value: '3 Active', trend: 'US-East, EU-West, Edge', dir: 'neutral' },
      { title: 'Total Worker Nodes', value: '34 Nodes', trend: 'Bare-Metal & Cloud', dir: 'up' },
      { title: 'Control Plane Latency', value: '--', trend: 'Direct WireGuard Mesh', dir: 'up' },
      { title: 'Policy Synchronization', value: '100%', trend: 'Kyverno & OPA Active', dir: 'up' }
    ],
    items: [
      { id: 'cluster-us-east', name: 'prod-us-east-1 (Primary Bare-Metal)', detail: '16 Nodes, 284 Pods, Kubernetes v1.31.0, NVMe Dual-Sync Storage.', status: 'healthy', tag: 'PRIMARY', kind: 'ManagedCluster' },
      { id: 'cluster-eu-west', name: 'prod-eu-west-1 (Secondary Cloud)', detail: '12 Nodes, 198 Pods, Kubernetes v1.31.0, S3 Hot-Standby Storage.', status: 'healthy', tag: 'SECONDARY', kind: 'ManagedCluster' },
      { id: 'cluster-staging', name: 'staging-us-east (CI/CD Sandbox)', detail: '6 Nodes, 86 Pods, Kubernetes v1.31.0, ephemeral preview environments.', status: 'healthy', tag: 'STAGING', kind: 'ManagedCluster' }
    ]
  },
  deployments: {
    title: 'Zero-Downtime Deployments & Rollouts', category: 'COMPUTE & FLEET', badge: 'Canary & Blue-Green',
    desc: 'Manage progressive delivery, automated rollback triggers, and GitOps synchronization across workloads.',
    metrics: [
      { title: 'Active Deployments', value: '46 Services', trend: 'Zero Downtime Guaranteed', dir: 'neutral' },
      { title: 'Canary Rollouts Live', value: '2 Active', trend: '10% Traffic Split', dir: 'up' },
      { title: 'Auto-Rollback Triggers', value: 'Armed', trend: 'Error Rate > 1% Triggers Abort', dir: 'up' },
      { title: 'Deploy Frequency', value: '18 / Day', trend: '+24% Velocity', dir: 'up' }
    ],
    items: [
      { id: 'dep-payment', name: 'payment-service (v2.4.1)', detail: 'Canary rollout at 25% traffic split. P99 latency: 38ms. Error rate: 0.00%.', status: 'active', tag: 'CANARY', kind: 'DeploymentRollout', namespace: 'production' },
      { id: 'dep-auth', name: 'auth-gateway (v3.1.0)', detail: 'Blue/Green deployment completed. 100% traffic shifted to Green.', status: 'healthy', tag: 'STABLE', kind: 'DeploymentRollout', namespace: 'production' },
      { id: 'dep-analytics', name: 'ai-inferencer (v1.8.4)', detail: 'GPU-accelerated inference deployment running across worker-gpu-pool.', status: 'healthy', tag: 'STABLE', kind: 'DeploymentRollout', namespace: 'ai-inference' }
    ]
  },
  promotions: {
    title: 'Promotion Pipeline (Dev → Staging → Prod)', category: 'COMPUTE & FLEET', badge: 'GitOps Pipeline',
    desc: 'Structured environment promotion workflows with automated integration gates, security verification, and release sign-offs.',
    metrics: [
      { title: 'Pipeline Stages', value: '3 Enforced', trend: 'Dev → Stage → Prod', dir: 'neutral' },
      { title: 'Pending Promotions', value: '1 In Review', trend: 'Requires QA Sign-Off', dir: 'neutral' },
      { title: 'Security Gate Status', value: 'PASSED', trend: '0 Critical CVEs', dir: 'up' },
      { title: 'Mean Promotion Time', value: '4.2 mins', trend: 'Automated Manifest Sync', dir: 'down' }
    ],
    items: [
      { id: 'promo-101', name: 'Release 2026.08.2: Core Microservices', detail: 'Promoted from Staging to Production. 12 services synchronized.', status: 'healthy', tag: 'PROD LIVE', kind: 'ReleasePromotion', namespace: 'pipeline' },
      { id: 'promo-102', name: 'Release 2026.08.3: Gateway API Update', detail: 'Currently in Staging. Running automated end-to-end integration tests.', status: 'active', tag: 'STAGING', kind: 'ReleasePromotion', namespace: 'pipeline' }
    ]
  },
  docker: {
    title: 'Docker Swarm & Engine Control', category: 'COMPUTE & FLEET', badge: 'Hybrid Container Mesh',
    desc: 'Monitor standalone Docker engines, Docker Swarm services, and rootless container daemons across edge nodes.',
    metrics: [
      { title: 'Docker Engines', value: '6 Nodes', trend: 'Docker v27.1.0', dir: 'neutral' },
      { title: 'Swarm Services', value: '14 Active', trend: 'Overlay Network Mesh', dir: 'up' },
      { title: 'Container Density', value: '88 Containers', trend: 'Memory Usage: 44%', dir: 'neutral' },
      { title: 'Health Status', value: '100% Healthy', trend: 'Zero CrashLoop', dir: 'up' }
    ],
    items: [
      { id: 'swarm-edge-1', name: 'edge-gateway-swarm-01', detail: 'Ingress reverse proxy running across 3 manager nodes and 3 worker nodes.', status: 'healthy', tag: 'HEALTHY', kind: 'SwarmService' },
      { id: 'swarm-db-cache', name: 'redis-cache-swarm-service', detail: 'Replicated in-memory cache cluster distributed across zone-a and zone-b.', status: 'healthy', tag: 'HEALTHY', kind: 'SwarmService' }
    ]
  },
  explorer: {
    title: 'Kubernetes Cluster Resource Explorer', category: 'COMPUTE & FLEET', badge: 'Live CRD Browser',
    desc: 'Deep inspection of all Kubernetes API resources, Custom Resource Definitions (CRDs), events, and network topology.',
    metrics: [
      { title: 'Discovered CRDs', value: '64 Types', trend: 'KEDA, Kyverno, Velero', dir: 'neutral' },
      { title: 'Total Namespaces', value: '14 Isolated', trend: 'Multi-Tenant Enforced', dir: 'neutral' },
      { title: 'Cluster Events / Sec', value: '24 / sec', trend: 'Indexed in Real-Time', dir: 'neutral' },
      { title: 'Resource Quotas', value: 'Normal', trend: 'Within 60% Limits', dir: 'up' }
    ],
    items: [
      { id: 'crd-scaledobject', name: 'ScaledObject (keda.sh/v1alpha1)', detail: 'Defines autonomous autoscaling rules driven by external Prometheus metrics.', status: 'healthy', tag: 'CRD', kind: 'CustomResourceDefinition', apiVersion: 'apiextensions.k8s.io/v1' },
      { id: 'crd-cert', name: 'Certificate (cert-manager.io/v1)', detail: 'Automated Let\'s Encrypt and internal Vault PKI certificate renewals.', status: 'healthy', tag: 'CRD', kind: 'CustomResourceDefinition', apiVersion: 'apiextensions.k8s.io/v1' }
    ]
  },
  compliance: {
    title: 'Compliance & CIS Benchmark Verification', category: 'GOVERNANCE & SECURITY', badge: 'SOC2 & CIS L2',
    desc: 'Continuous compliance audit monitoring Pod Security Standards, air-gapped network policies, and cryptographic controls.',
    metrics: [
      { title: 'Overall Score', value: '99.4%', trend: 'CIS K8s Benchmark', dir: 'up' },
      { title: 'Enforced Policies', value: '38 Rules', trend: 'Kyverno + OPA Gatekeeper', dir: 'neutral' },
      { title: 'Violations Blocked', value: '14 Today', trend: 'Root Privileges Prevented', dir: 'up' },
      { title: 'Audit Status', value: 'COMPLIANT', trend: 'SOC2 Type II Ready', dir: 'up' }
    ],
    items: [
      { id: 'comp-pss', name: 'Pod Security Standards: Restricted Profile', detail: 'Disallows root users, privilege escalation, and host namespaces.', status: 'healthy', tag: 'ENFORCED', kind: 'PolicyValidation' },
      { id: 'comp-netpol', name: 'Default Deny Ingress/Egress NetworkPolicy', detail: 'Enforces explicit inter-namespace communication boundaries.', status: 'healthy', tag: 'ENFORCED', kind: 'PolicyValidation' }
    ]
  },
  drift: {
    title: 'Configuration Drift Detection', category: 'GOVERNANCE & SECURITY', badge: 'GitOps Reconciliation',
    desc: 'Real-time detection of manual out-of-band cluster modifications vs Git repository declarations with auto-remediation.',
    metrics: [
      { title: 'Active Drift Count', value: '0 Mismatches', trend: 'Cluster In Sync with Git', dir: 'up' },
      { title: 'Reconciliation Interval', value: '30 seconds', trend: 'Fast GitOps Loop', dir: 'neutral' },
      { title: 'Auto-Remediations', value: '6 Fixed', trend: 'Reverted Manual Changes', dir: 'up' },
      { title: 'Monitored Manifests', value: '320 Objects', trend: '100% Tracked in Git', dir: 'neutral' }
    ],
    items: [
      { id: 'drift-1', name: 'deployment/payment-processor HPA Spec', detail: 'Git repository matches live cluster state. Zero divergence detected.', status: 'healthy', tag: 'SYNCED', kind: 'DriftRecord' },
      { id: 'drift-2', name: 'networkpolicy/default-deny-ingress', detail: 'Live policy matches main branch commit 7f8a912.', status: 'healthy', tag: 'SYNCED', kind: 'DriftRecord' }
    ]
  },
  automation: {
    title: 'Automation Rules & Event-Driven Engine', category: 'AUTOMATION & FINOPS', badge: 'Event-Driven Mesh',
    desc: 'Configure autonomous cluster event triggers, self-healing pod restarts, automated snapshot creation, and alert webhooks.',
    metrics: [
      { title: 'Active Rules', value: '16 Rules', trend: 'Event-Driven SRE', dir: 'neutral' },
      { title: 'Automations Fired', value: '184 Today', trend: 'Zero Human Intervention', dir: 'up' },
      { title: 'Execution Latency', value: '< 100ms', trend: 'In-Cluster Go Engine', dir: 'up' },
      { title: 'Error Rate', value: '0.00%', trend: 'Flawless Execution', dir: 'up' }
    ],
    items: [
      { id: 'auto-1', name: 'Auto-Cordon Node on Disk Pressure', detail: 'Automatically drains and cordons worker nodes exhibiting high I/O wait.', status: 'active', tag: 'ARMED', kind: 'AutomationRule' },
      { id: 'auto-2', name: 'Snapshot Resync on High Write Burst', detail: 'Triggers NVMe dual-sync snapshot whenever database writes spike above 10MB/s.', status: 'active', tag: 'ARMED', kind: 'AutomationRule' }
    ]
  },
  runbooks: {
    title: 'Automated SRE Runbooks', category: 'AUTOMATION & FINOPS', badge: 'Interactive Playbooks',
    desc: 'Step-by-step diagnostic and remediation playbooks with one-click safe execution and audit logging.',
    metrics: [
      { title: 'Available Runbooks', value: '24 Playbooks', trend: 'SRE & Disaster Recovery', dir: 'neutral' },
      { title: 'One-Click Executions', value: '42 This Week', trend: 'Average Time: 45s', dir: 'up' },
      { title: 'Dry-Run Simulation', value: 'Supported', trend: 'Safety Verification', dir: 'up' },
      { title: 'Audit Trail', value: '100% Logged', trend: 'Immutable Records', dir: 'up' }
    ],
    items: [
      { id: 'rb-drain', name: 'Safe Worker Node Cordon & Evacuate', detail: 'Gracefully evicts pods respecting PodDisruptionBudgets (PDBs) before host reboot.', status: 'healthy', tag: 'READY', kind: 'Runbook' },
      { id: 'rb-cert', name: 'Emergency TLS Certificate Key Rotation', detail: 'Rotates internal mutual TLS certificates and triggers rollout restart.', status: 'healthy', tag: 'READY', kind: 'Runbook' }
    ]
  },
  cost: {
    title: 'FinOps Cost Optimization & Bin-Packing', category: 'AUTOMATION & FINOPS', badge: 'FinOps AI Engine',
    desc: 'Track multi-cluster compute costs, identify overprovisioned CPU/Memory requests, and optimize worker node bin-packing.',
    metrics: [
      { title: 'Monthly Projected Spend', value: '$1,420', trend: '-28% vs previous month', dir: 'down' },
      { title: 'Identified Waste', value: '$380 / Mo', trend: 'Overprovisioned Pods', dir: 'neutral' },
      { title: 'Bin-Packing Efficiency', value: '88.4%', trend: '+12% Optimized', dir: 'up' },
      { title: 'Spot Instance Ratio', value: '45%', trend: 'Non-Critical Workloads', dir: 'up' }
    ],
    items: [
      { id: 'cost-1', name: 'Right-Size payment-processor Pod Requests', detail: 'Reduce CPU request from 2000m to 1000m based on 30-day historical P95 usage. Saves $68/mo.', status: 'healthy', tag: 'OPPORTUNITY', kind: 'CostOpportunity' },
      { id: 'cost-2', name: 'Consolidate worker-pool-03 to High-Density Host', detail: 'Move 14 idle dev pods to primary node. Allows shutting down 1 bare-metal worker. Saves $180/mo.', status: 'healthy', tag: 'OPPORTUNITY', kind: 'CostOpportunity' }
    ]
  },
  capacity: {
    title: 'Capacity Planning & Predictive Scaling', category: 'AUTOMATION & FINOPS', badge: 'Predictive Forecasting',
    desc: 'Forecast CPU, RAM, and NVMe storage consumption over 30/60/90 day horizons using time-series linear regressions.',
    metrics: [
      { title: 'Days Until CPU Saturation', value: '142 Days', trend: 'Healthy Capacity', dir: 'up' },
      { title: 'NVMe Dual-Sync Storage', value: '48% Used', trend: '6.4TB / 13.2TB', dir: 'neutral' },
      { title: 'Worker Node Headroom', value: '38% Buffer', trend: 'Ready for Traffic Bursts', dir: 'up' },
      { title: 'Forecast Accuracy', value: '98.2%', trend: 'ARIMA Time-Series', dir: 'up' }
    ],
    items: [
      { id: 'cap-nvme', name: 'Primary Storage Volume Growth', detail: 'Consuming ~4.2GB/day in snapshot journals. Next storage expansion recommended in Q4.', status: 'healthy', tag: 'HEALTHY', kind: 'CapacityForecast' },
      { id: 'cap-ram', name: 'Cluster Memory Headroom', detail: 'Peak memory usage observed at 74% during 10:00 UTC batch processing.', status: 'healthy', tag: 'HEALTHY', kind: 'CapacityForecast' }
    ]
  }
}

export function useGenericPlatform() {
  const route = useRoute()

  // Dynamic feature discovery & resolution
  const currentFeature = computed<FeatureInfo>(() => {
    const segment = route.path.replace('/', '')
    return PLATFORM_FEATURE_MAP[segment] || {
      title: `Platform Feature (${route.name?.toString() || 'Console'})`,
      category: 'ENTERPRISE PLATFORM',
      badge: 'Operational',
      desc: 'Enterprise Kubernetes cluster telemetry and control plane subsystem.',
      metrics: [
        { title: 'Subsystem State', value: 'HEALTHY', trend: 'Air-Gapped Sync', dir: 'up' },
        { title: 'Cluster Mesh', value: '3 Nodes', trend: 'Zero Latency', dir: 'neutral' },
        { title: 'Security Posture', value: 'ARMED', trend: 'ZeroTrust Enforced', dir: 'up' },
        { title: 'SLA Rating', value: '99.99%', trend: 'Operational', dir: 'up' }
      ],
      items: [
        { id: 'item-1', name: 'Core Controller Loop', detail: 'Synchronized with local Kubernetes API server.', status: 'healthy', tag: 'ACTIVE', kind: 'Controller' }
      ]
    }
  })

  // Telemetry items state
  const rawItems = ref<GenericPlatformItem[]>([])
  const isSyncing = ref(false)
  const syncError = ref<string | null>(null)
  const toastMessage = ref<{ text: string; type: 'success' | 'error' } | null>(null)

  // Watch for route changes to load feature items
  watch(
    () => currentFeature.value,
    (feature) => {
      rawItems.value = [...feature.items]
    },
    { immediate: true }
  )

  // Search, filtering, and pagination state
  const searchQuery = ref('')
  const statusFilter = ref<'all' | string>('all')
  const sortKey = ref<string>('name')
  const sortOrder = ref<'asc' | 'desc'>('asc')
  const currentPage = ref(1)
  const pageSize = ref(10)

  const filteredItems = computed(() => {
    let result = [...rawItems.value]

    if (statusFilter.value !== 'all') {
      result = result.filter(i => i.status.toLowerCase() === statusFilter.value.toLowerCase())
    }

    if (searchQuery.value.trim()) {
      const q = searchQuery.value.toLowerCase().trim()
      result = result.filter(i =>
        i.name.toLowerCase().includes(q) ||
        i.id.toLowerCase().includes(q) ||
        i.detail.toLowerCase().includes(q) ||
        i.tag.toLowerCase().includes(q)
      )
    }

    if (sortKey.value) {
      result.sort((a, b) => {
        const va = String(a[sortKey.value] || '')
        const vb = String(b[sortKey.value] || '')
        const cmp = va.localeCompare(vb, undefined, { numeric: true })
        return sortOrder.value === 'asc' ? cmp : -cmp
      })
    }

    return result
  })

  const totalPages = computed(() => Math.max(1, Math.ceil(filteredItems.value.length / pageSize.value)))

  const paginatedItems = computed(() => {
    const start = (currentPage.value - 1) * pageSize.value
    return filteredItems.value.slice(start, start + pageSize.value)
  })

  function toggleSort(key: string) {
    if (sortKey.value === key) {
      sortOrder.value = sortOrder.value === 'asc' ? 'desc' : 'asc'
    } else {
      sortKey.value = key
      sortOrder.value = 'asc'
    }
  }

  // Dynamic column discovery
  const dynamicColumns = computed<DynamicColumn[]>(() => [
    { key: 'name', label: 'Resource / Entity', sortable: true },
    { key: 'tag', label: 'Classification', sortable: true, width: '140px' },
    { key: 'detail', label: 'Telemetry Details' },
    { key: 'status', label: 'Health Status', sortable: true, width: '130px' }
  ])

  // Metadata inspection drawer state
  const isDrawerOpen = ref(false)
  const inspectedItem = ref<GenericPlatformItem | null>(null)

  function inspectItem(item: GenericPlatformItem) {
    inspectedItem.value = item
    isDrawerOpen.value = true
  }

  function closeDrawer() {
    isDrawerOpen.value = false
  }

  const inspectedYaml = computed(() => {
    if (!inspectedItem.value) return ''
    const payload = inspectedItem.value.raw || {
      apiVersion: inspectedItem.value.apiVersion || 'platform.k8s.io/v1alpha1',
      kind: inspectedItem.value.kind || 'PlatformResource',
      metadata: {
        name: inspectedItem.value.name,
        id: inspectedItem.value.id,
        namespace: inspectedItem.value.namespace || 'default',
        labels: inspectedItem.value.labels || { tag: inspectedItem.value.tag },
        annotations: inspectedItem.value.annotations || { telemetry: 'live' }
      },
      spec: {
        detail: inspectedItem.value.detail,
        status: inspectedItem.value.status
      }
    }
    return jsonToYaml(payload)
  })

  const inspectedJson = computed(() => {
    if (!inspectedItem.value) return ''
    return JSON.stringify(inspectedItem.value, null, 2)
  })

  // Schema-driven action dispatching
  const schemaActions: SchemaAction[] = [
    { id: 'configure_rules', label: 'Configure Rules', icon: '⚙️', variant: 'secondary' },
    { id: 'autonomous_sync', label: 'Run Autonomous Sync', icon: '⚡', variant: 'primary' }
  ]

  async function dispatchAction(actionId: string, item?: GenericPlatformItem) {
    isSyncing.value = true
    syncError.value = null
    try {
      if (actionId === 'autonomous_sync') {
        await new Promise(resolve => setTimeout(resolve, 350))
        toastMessage.value = { text: `Autonomous sync dispatched for ${currentFeature.value.title}.`, type: 'success' }
      } else if (actionId === 'configure_rules') {
        toastMessage.value = { text: `Opened policy rules engine for ${currentFeature.value.title}.`, type: 'success' }
      } else if (actionId === 'inspect' && item) {
        inspectItem(item)
      } else {
        toastMessage.value = { text: `Dispatched '${actionId}' successfully.`, type: 'success' }
      }
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Action execution failed'
      syncError.value = msg
      toastMessage.value = { text: msg, type: 'error' }
    } finally {
      isSyncing.value = false
      setTimeout(() => {
        toastMessage.value = null
      }, 4000)
    }
  }

  return {
    currentFeature,
    rawItems,
    filteredItems,
    paginatedItems,
    dynamicColumns,
    searchQuery,
    statusFilter,
    sortKey,
    sortOrder,
    currentPage,
    pageSize,
    totalPages,
    toggleSort,
    isDrawerOpen,
    inspectedItem,
    inspectedYaml,
    inspectedJson,
    inspectItem,
    closeDrawer,
    schemaActions,
    dispatchAction,
    isSyncing,
    syncError,
    toastMessage
  }
}