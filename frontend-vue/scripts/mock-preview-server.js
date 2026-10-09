/**
 * Mock Preview Server for Vite Dashboard Testing
 * Zero external npm dependencies - uses only Node.js built-in http & url modules.
 */

import http from 'node:http'
import { URL } from 'node:url'

const PORT = parseInt(process.env.PORT || '8080', 10)
const isTestMode = process.argv.includes('--test')

// 1. Mock Data Definitions
const metricsSummary = {
  cpu_usage: 42.8,
  memory_usage: 63.4,
  pod_count: 48,
  node_count: 3,
  status: 'healthy',
  host: {
    hostname: 'k8s-node-01.prod.internal',
    os: 'linux',
    arch: 'amd64',
    uptime_seconds: 864200,
  },
  latency: 14.2,
}

const clusters = [
  {
    id: 'production-k8s', name: 'production-k8s', group: 'production', region: 'us-east-1',
    provider: 'baremetal', orchestrator: 'kubernetes', status: 'active', version: 'v1.31.1',
    nodes: 8, health_status: 'healthy',
    discovered_resources: { namespaces: ['default', 'kube-system', 'production', 'monitoring'], crd_count: 24, api_server_latency: '12ms' },
    created_at: '2026-01-15T00:00:00Z',
  },
  {
    id: 'staging-k8s', name: 'staging-k8s', group: 'staging', region: 'us-west-2',
    provider: 'baremetal', orchestrator: 'kubernetes', status: 'active', version: 'v1.31.1',
    nodes: 3, health_status: 'healthy',
    discovered_resources: { namespaces: ['default', 'kube-system', 'staging'], crd_count: 18, api_server_latency: '18ms' },
    created_at: '2026-02-01T00:00:00Z',
  },
  {
    id: 'edge-worker-01', name: 'edge-worker-01', group: 'edge', region: 'eu-central-1',
    provider: 'k3s', orchestrator: 'kubernetes', status: 'active', version: 'v1.30.3+k3s1',
    nodes: 2, health_status: 'healthy',
    discovered_resources: { namespaces: ['default', 'kube-system', 'iot-edge'], crd_count: 8, api_server_latency: '25ms' },
    created_at: '2026-03-10T00:00:00Z',
  },
]

const incidents = [
  {
    id: 'INC-8021',
    cluster_name: 'production-k8s',
    namespace: 'production',
    pod_name: 'checkout-svc-78bd4',
    type: 'OOMKilled',
    status: 'remediating',
    severity: 'critical',
    message: 'Pod exceeded memory limit 512MiB and was terminated by Linux OOM killer',
    created_at: '2026-10-06T08:15:00Z',
    updated_at: '2026-10-06T08:20:00Z',
  },
  {
    id: 'INC-8022',
    cluster_name: 'staging-k8s',
    namespace: 'staging',
    pod_name: 'payment-relay-3f21b',
    type: 'CrashLoopBackOff',
    status: 'detected',
    severity: 'high',
    message: 'Container failing liveness probe: connection refused on port 8080',
    created_at: '2026-10-06T08:25:00Z',
    updated_at: '2026-10-06T08:27:00Z',
  },
  {
    id: 'INC-8019',
    cluster_name: 'edge-worker-01',
    namespace: 'iot-edge',
    pod_name: 'telemetry-collector-9c12a',
    type: 'HighCPU',
    status: 'resolved',
    severity: 'medium',
    message: 'Node CPU throttle limit reached during telemetry burst',
    created_at: '2026-10-05T14:10:00Z',
    updated_at: '2026-10-05T14:45:00Z',
    resolved_at: '2026-10-05T14:45:00Z',
  },
]

const auditFindings = [
  {
    id: 'CVE-2024-21626',
    category: 'cve_vulnerability',
    severity: 'critical',
    description: 'Container escape vulnerability via file descriptor leak in runc process initialization',
    remediation: 'Upgrade containerd/runc runtime to version >= 1.1.12 across all worker nodes',
    status: 'open',
    detected_at: '2026-10-06T06:00:00Z',
  },
  {
    id: 'CVE-2023-44487',
    category: 'cve_vulnerability',
    severity: 'high',
    description: 'HTTP/2 Rapid Reset DDoS vulnerability in reverse proxy ingress controller',
    remediation: 'Apply rate-limiting patch to Traefik / Envoy ingress configurations',
    status: 'open',
    detected_at: '2026-10-06T06:15:00Z',
  },
  {
    id: 'CVE-2024-38475',
    category: 'iac_misconfiguration',
    severity: 'medium',
    description: 'Privileged container securityContext detected without seccomp profile attachment',
    remediation: 'Enforce baseline Pod Security Standard admitting only restricted pods',
    status: 'open',
    detected_at: '2026-10-06T07:30:00Z',
  },
]

const logEntries = [
  {
    timestamp: '2026-10-06T08:30:00.123Z',
    cluster_id: 'production-k8s',
    namespace: 'production',
    pod_name: 'checkout-svc-78bd4',
    container_name: 'checkout',
    log_level: 'INFO',
    message: 'Handling HTTP POST /api/checkout/cart (duration=12ms, status=200)',
  },
  {
    timestamp: '2026-10-06T08:30:01.456Z',
    cluster_id: 'production-k8s',
    namespace: 'production',
    pod_name: 'checkout-svc-78bd4',
    container_name: 'checkout',
    log_level: 'WARN',
    message: 'Memory heap approaching high threshold: 468MiB / 512MiB',
  },
  {
    timestamp: '2026-10-06T08:30:02.789Z',
    cluster_id: 'staging-k8s',
    namespace: 'staging',
    pod_name: 'payment-relay-3f21b',
    container_name: 'relay',
    log_level: 'ERROR',
    message: 'Failed to connect to upstream payment gateway: connection timeout',
  },
]

const explorerResources = [
  {
    id: 'pod-checkout-78bd4',
    kind: 'Pod',
    name: 'checkout-svc-78bd4',
    namespace: 'production',
    cluster: 'production-k8s',
    status: 'Running',
    labels: { app: 'checkout', env: 'production' },
    age: '3d',
  },
  {
    id: 'pod-api-gw-7f9d1',
    kind: 'Pod',
    name: 'api-gateway-7f9d1',
    namespace: 'production',
    cluster: 'production-k8s',
    status: 'Running',
    labels: { app: 'api-gateway', env: 'production' },
    age: '7d',
  },
  {
    id: 'deploy-checkout-svc',
    kind: 'Deployment',
    name: 'checkout-svc',
    namespace: 'production',
    cluster: 'production-k8s',
    status: '3/3 Ready',
    labels: { app: 'checkout' },
    age: '14d',
  },
  {
    id: 'svc-checkout-lb',
    kind: 'Service',
    name: 'checkout-svc',
    namespace: 'production',
    cluster: 'production-k8s',
    status: 'Active',
    labels: { app: 'checkout' },
    age: '14d',
  },
  {
    id: 'node-prod-01',
    kind: 'Node',
    name: 'k8s-node-01.prod.internal',
    cluster: 'production-k8s',
    status: 'Ready',
    labels: { 'node-role.kubernetes.io/control-plane': '' },
    age: '45d',
  },
]

const namespaces = ['default', 'kube-system', 'production', 'staging', 'iot-edge']

const systemOverview = {
  nodes: [
    {
      node_id: 'node-prod-01',
      node_name: 'k8s-node-01.prod.internal',
      role: 'control-plane',
      status: 'ready',
      cpu_percent: 35.5,
      memory_used: 8589934592,
      memory_total: 17179869184,
      memory_percent: 50.0,
      disk_used: 53687091200,
      disk_total: 214748364800,
      disk_percent: 25.0,
      network_rx_bytes: 1048576,
      network_tx_bytes: 2097152,
      container_count: 12,
      running_count: 12,
      uptime: 864200,
      load_avg: [0.45, 0.52, 0.48],
    },
    {
      node_id: 'node-prod-02',
      node_name: 'k8s-node-02.prod.internal',
      role: 'worker',
      status: 'ready',
      cpu_percent: 48.2,
      memory_used: 12884901888,
      memory_total: 17179869184,
      memory_percent: 75.0,
      disk_used: 75161927680,
      disk_total: 214748364800,
      disk_percent: 35.0,
      network_rx_bytes: 5242880,
      network_tx_bytes: 4194304,
      container_count: 16,
      running_count: 16,
      uptime: 864200,
      load_avg: [1.12, 0.95, 0.88],
    },
  ],
  containers: [],
  total_nodes: 2,
  healthy_nodes: 2,
  total_containers: 28,
  running_containers: 28,
  total_cpu_percent: 41.8,
  total_mem_percent: 62.5,
  total_disk_percent: 30.0,
  requests_per_sec: 1250,
  alerts: [],
  collected_at: new Date().toISOString(),
}

const mockDeployments = [
  { id: 'dep-ingress-traefik', name: 'ingress-traefik', team: 'Platform Ops', env: 'production', image: 'traefik:v3.1.2', target: 'production-k8s', namespace: 'kube-system', type: 'kubernetes', replicas: 3, readyReplicas: 3, status: 'healthy', cpu: '150m', memory: '256MiB', port: 443, strategy: 'RollingUpdate' },
  { id: 'dep-api-gateway', name: 'api-gateway', team: 'Core API', env: 'production', image: 'envoyproxy/envoy:v1.31.0', target: 'production-k8s', namespace: 'production', type: 'kubernetes', replicas: 4, readyReplicas: 4, status: 'healthy', cpu: '320m', memory: '512MiB', port: 8080, strategy: 'Canary', canaryWeight: 20, canaryVersion: 'envoyproxy/envoy:v1.32.0' },
  { id: 'dep-auth-service', name: 'auth-service', team: 'Security Team', env: 'production', image: 'auth-service:v1.18.4', target: 'production-k8s', namespace: 'production', type: 'kubernetes', replicas: 2, readyReplicas: 2, status: 'healthy', cpu: '180m', memory: '384MiB', port: 9090, strategy: 'BlueGreen', blueGreenActive: 'blue' },
  { id: 'dep-redis-master', name: 'redis-master', team: 'Data Infra', env: 'production', image: 'redis:7.2-alpine', target: 'staging-k8s', namespace: 'staging', type: 'kubernetes', replicas: 1, readyReplicas: 1, status: 'healthy', cpu: '90m', memory: '192MiB', port: 6379, strategy: 'RollingUpdate' },
  { id: 'dep-postgres-operator', name: 'postgres-operator', team: 'Database Ops', env: 'production', image: 'ghcr.io/zalando/postgres-operator:v1.12.0', target: 'production-k8s', namespace: 'production', type: 'kubernetes', replicas: 2, readyReplicas: 2, status: 'healthy', cpu: '210m', memory: '420MiB', port: 5432, strategy: 'RollingUpdate' },
]

const mockDockerServices = [
  { id: 'svc-telemetry-relay', name: 'telemetry-relay', image: 'fluent-bit:3.1', replicas: 2, ports: ['2020:2020'], updated_at: '2026-10-06T00:00:00Z' },
]

const mockDockerContainers = [
  { id: 'c-edge-prom', name: 'prometheus-node-exporter', image: 'prom/node-exporter:v1.8.2', status: 'Up 12 days', state: 'running', created: '2026-09-24T00:00:00Z' },
]

// 2. Response Helpers
function sendJson(res, statusCode, data) {
  const payload = JSON.stringify(data)
  res.writeHead(statusCode, {
    'Content-Type': 'application/json; charset=utf-8',
    'Access-Control-Allow-Origin': '*',
    'Access-Control-Allow-Methods': 'GET, POST, PUT, DELETE, PATCH, OPTIONS',
    'Access-Control-Allow-Headers': 'Content-Type, Authorization, X-Tenant-ID',
  })
  res.end(payload)
}

function handleCorsPreflight(res) {
  res.writeHead(204, {
    'Access-Control-Allow-Origin': '*',
    'Access-Control-Allow-Methods': 'GET, POST, PUT, DELETE, PATCH, OPTIONS',
    'Access-Control-Allow-Headers': 'Content-Type, Authorization, X-Tenant-ID',
    'Access-Control-Max-Age': '86400',
  })
  res.end()
}

// 3. Request Handler
export function createMockServer() {
  return http.createServer((req, res) => {
    if (req.method === 'OPTIONS') {
      return handleCorsPreflight(res)
    }

    const parsedUrl = new URL(req.url || '/', `http://${req.headers.host || 'localhost'}`)
    const pathname = parsedUrl.pathname.replace(/\/+$/, '') || '/'

    // Health Check
    if (pathname === '/health' || pathname === '/api/health' || pathname === '/api/v1/health') {
      return sendJson(res, 200, { status: 'ok' })
    }

    // Telemetry Summary
    if (
      pathname === '/api/metrics/summary' ||
      pathname === '/api/v1/metrics/summary' ||
      pathname === '/metrics/summary'
    ) {
      return sendJson(res, 200, metricsSummary)
    }

    // Clusters List
    if (
      pathname === '/api/v1/clusters' ||
      pathname === '/api/clusters' ||
      pathname === '/clusters' ||
      pathname === '/api/v1/fleet' ||
      pathname === '/api/v1/fleet/clusters' ||
      pathname === '/api/fleet' ||
      pathname === '/api/fleet/clusters' ||
      pathname === '/fleet' ||
      pathname === '/fleet/clusters'
    ) {
      return sendJson(res, 200, { data: clusters, total: clusters.length })
    }

    // Incidents List
    if (
      pathname === '/api/v1/incidents' ||
      pathname === '/api/incidents' ||
      pathname === '/incidents'
    ) {
      return sendJson(res, 200, {
        data: incidents,
        total: incidents.length,
        limit: 50,
        offset: 0,
      })
    }

    // Audit Findings
    if (
      pathname === '/api/v1/audit/findings' ||
      pathname === '/api/audit/findings' ||
      pathname === '/audit/findings'
    ) {
      return sendJson(res, 200, { data: auditFindings, total: auditFindings.length })
    }

    // Logs Search / Query
    if (
      pathname === '/api/v1/logs/query' ||
      pathname === '/api/logs/query' ||
      pathname === '/logs/query' ||
      pathname === '/api/v1/logs/search' ||
      pathname === '/api/logs/search' ||
      pathname === '/logs/search'
    ) {
      return sendJson(res, 200, {
        entries: logEntries,
        total_count: logEntries.length,
        has_more: false,
      })
    }

    // Resource Explorer
    if (
      pathname === '/api/v1/explorer' ||
      pathname === '/api/explorer' ||
      pathname === '/explorer' ||
      pathname.startsWith('/api/v1/explorer/') ||
      pathname.startsWith('/api/explorer/')
    ) {
      return sendJson(res, 200, {
        data: explorerResources,
        namespaces,
        total: explorerResources.length,
      })
    }

    // Overview & TPS
    if (
      pathname === '/api/v1/overview' ||
      pathname === '/api/overview' ||
      pathname === '/overview'
    ) {
      return sendJson(res, 200, { data: systemOverview })
    }

    if (
      pathname === '/api/v1/overview/nodes' ||
      pathname === '/api/overview/nodes' ||
      pathname === '/overview/nodes'
    ) {
      return sendJson(res, 200, { data: systemOverview.nodes })
    }

    // Deployments & Workloads
    if (pathname === '/api/v1/deployments' || pathname === '/api/deployments' || pathname === '/deployments') {
      return sendJson(res, 200, { data: mockDeployments, total: mockDeployments.length })
    }

    // Docker Services & Containers
    if (pathname === '/api/v1/docker/services' || pathname === '/api/docker/services' || pathname === '/docker/services') {
      return sendJson(res, 200, { data: mockDockerServices, total: mockDockerServices.length })
    }

    if (pathname === '/api/v1/docker/containers' || pathname === '/api/docker/containers' || pathname === '/docker/containers') {
      return sendJson(res, 200, { data: mockDockerContainers, total: mockDockerContainers.length })
    }

    // Fallback for other GET / POST API calls
    return sendJson(res, 200, { status: 'ok', data: [] })
  })
}

// 4. Execution & Self-Test Routine
if (isTestMode) {
  const server = createMockServer()
  server.listen(0, '127.0.0.1', () => {
    const address = server.address()
    const testPort = typeof address === 'object' && address ? address.port : 8080
    const testUrl = `http://127.0.0.1:${testPort}/health`

    http.get(testUrl, (res) => {
      let body = ''
      res.on('data', (chunk) => {
        body += chunk
      })
      res.on('end', () => {
        try {
          const parsed = JSON.parse(body)
          if (res.statusCode === 200 && parsed.status === 'ok') {
            console.log(`[PASS] Mock server self-test succeeded: GET /health returned 200 OK (${body})`)
            server.close(() => process.exit(0))
          } else {
            console.error(`[FAIL] Expected status 200 and { status: 'ok' }, got ${res.statusCode}: ${body}`)
            server.close(() => process.exit(1))
          }
        } catch (err) {
          console.error(`[FAIL] JSON parse error:`, err)
          server.close(() => process.exit(1))
        }
      })
    }).on('error', (err) => {
      console.error(`[FAIL] Self-test request failed:`, err)
      server.close(() => process.exit(1))
    })
  })
} else {
  const server = createMockServer()
  server.listen(PORT, '0.0.0.0', () => {
    console.log(`Mock Preview Server listening on http://localhost:${PORT}`)
  })
}
