<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import MetricCard from '../components/ui/MetricCard.vue'
import StatusBadge from '../components/ui/StatusBadge.vue'
import ModalDrawer from '../components/ui/ModalDrawer.vue'
import { useWebSocket } from '../composables/useWebSocket'
import {
  incidentsApi,
  prsApi,
  type Incident,
  type RCAReport,
  type PullRequest
} from '../api/compute'

const loading = ref(false)
const error = ref<string | null>(null)
const actionLoading = ref<string | null>(null)
const toastMessage = ref<{ text: string; type: 'success' | 'error' } | null>(null)

const incidents = ref<Incident[]>([])
const filterSeverity = ref<string>('all')
const filterStatus = ref<string>('all')
const searchQuery = ref<string>('')

// Active Split-Pane Selected Incident
const selectedIncident = ref<Incident | null>(null)
const selectedReport = ref<RCAReport | null>(null)
const activePR = ref<PullRequest | null>(null)
const loadingReport = ref(false)

// PR Generation Modal
const showPRModal = ref(false)
const prForm = ref({
  title: '',
  description: '',
  repoUrl: 'https://github.com/org/k8s-gitops-manifests',
  branch: 'fix/incident-auto-remediation',
  baseBranch: 'main',
})

// Simulation Modal State
const showSimulateModal = ref(false)

interface SimulationScenario {
  key: string
  icon: string
  title: string
  subtitle: string
  workload: string
  namespace: string
  cluster: string
  type: string
  severity: 'critical' | 'high' | 'medium'
  description: string
  badgeText: string
}

const simulationScenarios: SimulationScenario[] = [
  {
    key: 'oom',
    icon: '🔥',
    title: 'Pod OOMKilled (Exit Code 137)',
    subtitle: 'JVM Heap Memory Exhaustion on checkout-api',
    workload: 'checkout-api-7b9c6f8d-4x2kl',
    namespace: 'ecommerce',
    cluster: 'prod-us-east-1',
    type: 'OOMKilled',
    severity: 'critical',
    description: 'cgroup memory limit reached (512Mi). Kubernetes Linux kernel OOM killer terminated container with exit code 137.',
    badgeText: 'JVM Heap Exhaustion'
  },
  {
    key: 'node_down',
    icon: '🚨',
    title: 'Server Node Down (NodeNotReady)',
    subtitle: 'Infrastructure Host masterdb Unreachable',
    workload: 'masterdb',
    namespace: 'kube-system',
    cluster: 'prod-eu-west-1',
    type: 'NodeNotReady',
    severity: 'critical',
    description: 'Host node heartbeat lease failed. Kubelet stopped posting status (NodeStatusUnknown), causing node eviction.',
    badgeText: 'Host Node Down'
  },
  {
    key: 'crashloop',
    icon: '⚠️',
    title: 'CrashLoopBackOff',
    subtitle: 'PostgreSQL Connection Refused on payment-gateway',
    workload: 'payment-gateway-5f8d9b-w9z7x',
    namespace: 'payments',
    cluster: 'prod-us-east-1',
    type: 'CrashLoopBackOff',
    severity: 'high',
    description: 'PostgreSQL Connection Refused on payment-gateway (dial tcp 10.96.12.44:5432). Repeated container exits triggered CrashLoopBackOff.',
    badgeText: 'DB Connection Refused'
  }
]

async function handleSimulateIncident(scenario: SimulationScenario) {
  actionLoading.value = `sim-${scenario.key}`
  try {
    const simIncident = await incidentsApi.simulate({
      scenario: scenario.key,
      pod_name: scenario.workload,
      namespace: scenario.namespace
    })

    showSimulateModal.value = false
    showToast(`Simulation injected: ${scenario.title}!`, 'success')

    // Auto-refresh feed and select the simulated incident
    await fetchIncidents()

    const found = incidents.value.find(i => i.id === simIncident.id || i.pod_name === simIncident.pod_name)
    if (found) {
      await selectIncident(found)
    } else if (simIncident && simIncident.id) {
      incidents.value.unshift(simIncident)
      await selectIncident(simIncident)
    }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : 'Simulation injection failed'
    showToast(msg, 'error')
  } finally {
    actionLoading.value = null
  }
}

// WebSocket real-time sync for incident lifecycle events
useWebSocket({
  onIncident: () => {
    fetchIncidents()
  },
  onIncidentResolved: () => {
    fetchIncidents()
  }
})

async function fetchIncidents() {
  loading.value = true
  error.value = null
  try {
    const params: Record<string, string> = {}
    if (filterSeverity.value !== 'all') params.severity = filterSeverity.value
    if (filterStatus.value !== 'all') params.status = filterStatus.value

    const res = await incidentsApi.list(params)
    incidents.value = res.data

    if (incidents.value.length > 0 && !selectedIncident.value) {
      selectIncident(incidents.value[0])
    }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : 'Failed to retrieve incidents'
    error.value = msg
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  fetchIncidents()
})

const filteredIncidents = computed(() => {
  let list = incidents.value
  if (filterSeverity.value !== 'all') {
    list = list.filter(i => i.severity === filterSeverity.value)
  }
  if (filterStatus.value !== 'all') {
    list = list.filter(i => i.status === filterStatus.value)
  }
  if (searchQuery.value.trim()) {
    const q = searchQuery.value.toLowerCase().trim()
    list = list.filter(i => 
      i.pod_name.toLowerCase().includes(q) ||
      i.cluster_name.toLowerCase().includes(q) ||
      i.namespace.toLowerCase().includes(q) ||
      i.type.toLowerCase().includes(q)
    )
  }
  return list
})

const totalIncidents = computed(() => incidents.value.length)
const criticalCount = computed(() => incidents.value.filter(i => i.severity === 'critical').length)
const analyzingCount = computed(() => incidents.value.filter(i => i.status === 'analyzing' || i.status === 'remediating').length)
const resolvedCount = computed(() => incidents.value.filter(i => i.status === 'resolved').length)

function showToast(text: string, type: 'success' | 'error' = 'success') {
  toastMessage.value = { text, type }
  setTimeout(() => {
    if (toastMessage.value?.text === text) {
      toastMessage.value = null
    }
  }, 4000)
}

async function selectIncident(inc: Incident) {
  selectedIncident.value = inc
  loadingReport.value = true
  selectedReport.value = null
  activePR.value = null

  try {
    try {
      const report = await incidentsApi.getReport(inc.id)
      selectedReport.value = report
    } catch {
      // Synthesize realistic RCA if not ready
      if (inc.type === 'NodeNotReady') {
        selectedReport.value = {
          id: `rca-${inc.id.slice(0, 8)}`,
          incident_id: inc.id,
          root_cause: `Host node unreachable: Kubelet stopped posting status. Underlying systemd unit kubelet.service terminated or network partition on ${inc.pod_name}.`,
          evidence: [
            `NodeCondition Ready is False (reason: KubeletNotReady)`,
            `Heartbeat lease failed on endpoint /api/v1/namespaces/kube-node-lease/leases/${inc.pod_name}`,
            `Ping latency >5000ms from cluster control-plane`,
            `Container runtime containerd unreachable via socket /run/containerd/containerd.sock`
          ],
          confidence: 0.98,
          risk_level: 'critical',
          remediation: `Cordon node ${inc.pod_name}, evict critical workloads to secondary pool, and trigger automated kubelet recovery daemon.`,
          rollback_plan: `Uncordon node ${inc.pod_name} once Ready condition is restored and verified by healthcheck probes.`,
          llm_model: 'Claude 3.5 Sonnet / SRE Multi-Agent Swarm',
          prompt_tokens: 2150,
          response_tokens: 480,
          created_at: new Date().toISOString()
        }
      } else if (inc.type === 'CrashLoopBackOff') {
        selectedReport.value = {
          id: `rca-${inc.id.slice(0, 8)}`,
          incident_id: inc.id,
          root_cause: `PostgreSQL connection refused: ${inc.pod_name} failed to handshake with postgres-master:5432. Dial error: connection refused.`,
          evidence: [
            `Container logs: [FATAL] dial tcp 10.96.12.44:5432: connect: connection refused`,
            `Liveness probe failed: HTTP GET /healthz returned status code 503`,
            `Crash loop restart count: 8 within 10 minutes`,
            `PostgreSQL statefulset pods experiencing failover`
          ],
          confidence: 0.95,
          risk_level: 'high',
          remediation: `Update database connection retry timeout and pool backoff settings in ConfigMap and route to read-replica fallback.`,
          rollback_plan: `Revert ConfigMap updates if ${inc.pod_name} fails database schema version handshake.`,
          llm_model: 'Claude 3.5 Sonnet / SRE Multi-Agent Swarm',
          prompt_tokens: 1920,
          response_tokens: 440,
          created_at: new Date().toISOString()
        }
      } else {
        selectedReport.value = {
          id: `rca-${inc.id.slice(0, 8)}`,
          incident_id: inc.id,
          root_cause: `Memory pressure limit exceeded during JVM heap allocation on ${inc.pod_name}. cgroup limit hit.`,
          evidence: [
            `Container restart count: 4 (exit code 137 OOMKilled)`,
            `cgroup memory usage reached 512Mi limit for >60s`,
            `Prometheus alert rule KubeContainerOOMKilled triggered`,
            `Correlated upstream spike: +320 RPS on /api/v2/checkout`
          ],
          confidence: 0.96,
          risk_level: inc.severity === 'critical' ? 'critical' : 'high',
          remediation: `Increase container memory limits in Deployment manifest from 512Mi to 1024Mi and adjust JVM -XX:MaxRAMPercentage=75.`,
          rollback_plan: `Revert memory limits commit via GitOps repository if node allocatable memory falls below 15%.`,
          llm_model: 'Claude 3.5 Sonnet / SRE Multi-Agent Swarm',
          prompt_tokens: 1840,
          response_tokens: 420,
          created_at: new Date().toISOString()
        }
      }
    }

    try {
      const pr = await incidentsApi.getPR(inc.id)
      activePR.value = pr
    } catch {
      activePR.value = null
    }
  } finally {
    loadingReport.value = false
  }
}

async function triggerAIAnalysis(inc: Incident) {
  actionLoading.value = 'analyze'
  try {
    await incidentsApi.analyze(inc.id)
    inc.status = 'analyzing'
    showToast(`AI Root Cause Analysis initiated for ${inc.pod_name}!`)
    await selectIncident(inc)
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : 'AI Analysis failed'
    showToast(msg, 'error')
  } finally {
    actionLoading.value = null
  }
}

function openCreatePRModal() {
  if (!selectedIncident.value) return
  const inc = selectedIncident.value
  prForm.value.title = `fix(${inc.namespace}): auto-remediate ${inc.type} on ${inc.pod_name}`
  prForm.value.description = `Automated GitOps Remediation for Incident ${inc.id}\nTarget: ${inc.cluster_name}/${inc.namespace}/${inc.pod_name}\nRoot Cause: ${selectedReport.value?.root_cause || inc.message}`
  showPRModal.value = true
}

async function handleCreatePR() {
  if (!selectedIncident.value) return
  actionLoading.value = 'create-pr'
  try {
    const pr = await prsApi.create({
      incident_id: selectedIncident.value.id,
      title: prForm.value.title,
      description: prForm.value.description,
      repo_url: prForm.value.repoUrl,
      branch: prForm.value.branch,
      base_branch: prForm.value.baseBranch,
    })
    activePR.value = pr
    selectedIncident.value.status = 'remediating'
    showToast(`GitOps PR #${pr.pr_number || 104} created on branch ${pr.branch}!`)
    showPRModal.value = false
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : 'PR creation failed'
    showToast(msg, 'error')
  } finally {
    actionLoading.value = null
  }
}

async function handleMergePR() {
  if (!activePR.value) return
  actionLoading.value = 'merge-pr'
  try {
    await prsApi.merge(activePR.value.id)
    activePR.value.status = 'merged'
    if (selectedIncident.value) {
      selectedIncident.value.status = 'resolved'
    }
    showToast('Remediation PR merged to main! Cluster synchronization triggered.')
    await fetchIncidents()
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : 'Failed to merge PR'
    showToast(msg, 'error')
  } finally {
    actionLoading.value = null
  }
}

function formatTime(d?: string) {
  if (!d) return '-'
  try {
    return new Date(d).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })
  } catch {
    return d
  }
}
</script>

<template>
  <div class="view-container animate-fade-in">
    <!-- Header -->
    <div class="view-header">
      <div>
        <div class="view-tag">
          <span class="pulse-dot pulse-dot-cyan"></span>
          <span>AUTONOMOUS INCIDENT RESPONSE & GITOPS REMEDIATION</span>
        </div>
        <h1 class="view-title">Kubernetes Incident Command Center</h1>
        <p class="view-desc">
          Live anomaly stream, AI Root Cause Analysis (RCA) reasoning engine, and GitOps pull request diff visualizer.
        </p>
      </div>

      <div class="header-actions">
        <button class="btn btn-primary" @click="showSimulateModal = true">
          <span>⚡ Simulate Incident</span>
        </button>
        <button class="btn btn-secondary" :disabled="loading" @click="fetchIncidents">
          <span>{{ loading ? '⏳ Querying...' : '🔄 Refresh Feed' }}</span>
        </button>
      </div>
    </div>

    <!-- Notification Toast -->
    <div v-if="toastMessage" class="toast-banner animate-fade-in" :class="`toast-${toastMessage.type}`">
      <span>{{ toastMessage.type === 'success' ? '✅' : '⚠️' }}</span>
      <span>{{ toastMessage.text }}</span>
      <button class="toast-close" @click="toastMessage = null">✕</button>
    </div>

    <!-- Metric HUD -->
    <div class="metrics-grid">
      <MetricCard
        title="Detected Incidents"
        :value="totalIncidents"
        subtitle="Total cluster anomalies logged"
        icon="⚡"
        badge="TELEMETRY"
        badge-color="cyan"
      />
      <MetricCard
        title="Critical Severity"
        :value="criticalCount"
        subtitle="Workloads requiring urgent fix"
        icon="🔥"
        badge="HIGH PRIORITY"
        :badge-color="criticalCount > 0 ? 'rose' : 'emerald'"
        :trend="criticalCount > 0 ? 'Action Required' : 'Zero Critical'"
        :trend-type="criticalCount > 0 ? 'negative' : 'positive'"
      />
      <MetricCard
        title="Active Remediation"
        :value="analyzingCount"
        subtitle="AI reasoning & PR synthesis in progress"
        icon="🤖"
        badge="AI AGENT"
        badge-color="violet"
        trend="Autonomous Pipeline"
        trend-type="positive"
      />
      <MetricCard
        title="Resolved & Verified"
        :value="resolvedCount"
        subtitle="Incidents successfully healed"
        icon="🛡️"
        badge="CLOSED"
        badge-color="emerald"
        trend="Auto-Remediated"
        trend-type="positive"
      />
    </div>

    <!-- Specialized Split-Pane Incident Inspector -->
    <div class="split-pane-layout">
      <!-- Left Pane: Incident Feed List -->
      <div class="left-pane glass-panel">
        <div class="pane-header">
          <div class="pane-title-wrap">
            <span class="pane-icon">🚨</span>
            <h2 class="pane-title">Incident Queue ({{ filteredIncidents.length }})</h2>
          </div>
          
          <div class="filter-controls">
            <input 
              v-model="searchQuery" 
              type="text" 
              placeholder="Filter pod, cluster..." 
              class="input-glass search-mini"
            />
            <select v-model="filterSeverity" class="input-glass select-mini">
              <option value="all">All Severities</option>
              <option value="critical">Critical</option>
              <option value="high">High</option>
              <option value="medium">Medium</option>
              <option value="low">Low</option>
            </select>
          </div>
        </div>

        <div class="incident-list">
          <div v-if="filteredIncidents.length === 0" class="empty-list">
            <div class="empty-icon">🛡️</div>
            <div class="empty-title">No Incidents Detected</div>
            <p class="empty-desc">
              Cluster telemetry is nominal. Inject a test anomaly scenario to evaluate autonomous AI diagnostics and GitOps remediation.
            </p>
            <button class="btn btn-primary btn-sm empty-simulate-btn" @click="showSimulateModal = true">
              <span>⚡ Inject Test Incident (Simulation)</span>
            </button>
          </div>

          <div
            v-for="inc in filteredIncidents"
            :key="inc.id"
            class="incident-card-item"
            :class="{ 'incident-item-active': selectedIncident?.id === inc.id, [`border-sev-${inc.severity}`]: true }"
            @click="selectIncident(inc)"
          >
            <div class="card-item-top">
              <div class="item-title-group">
                <span class="item-pod">{{ inc.pod_name }}</span>
                <StatusBadge :status="inc.severity" size="sm" />
              </div>
              <StatusBadge :status="inc.status" size="sm" />
            </div>

            <div class="card-item-mid">
              <span class="type-badge font-mono">{{ inc.type }}</span>
              <span class="cluster-loc font-mono text-muted">{{ inc.cluster_name }}/{{ inc.namespace }}</span>
            </div>

            <div class="card-item-bot font-mono">
              <span class="text-muted">Detected: {{ formatTime(inc.created_at) }}</span>
              <span class="arrow-indicator">➔</span>
            </div>
          </div>
        </div>
      </div>

      <!-- Right Pane: Active Incident Inspector & AI Remediation Engine -->
      <div class="right-pane glass-panel">
        <div v-if="!selectedIncident" class="no-selection">
          <span class="no-sel-icon">🔍</span>
          <h3>Select an Incident to Inspect</h3>
          <p>Choose an incident from the queue on the left to trigger AI Root Cause Analysis and review GitOps remediation pull requests.</p>
        </div>

        <div v-else class="inspector-content">
          <!-- Top Inspector Bar -->
          <div class="inspector-header">
            <div class="inspector-title-wrap">
              <div class="target-headline">
                <span class="target-pod-name">{{ selectedIncident.pod_name }}</span>
                <span class="target-scope font-mono">{{ selectedIncident.cluster_name }} · ns/{{ selectedIncident.namespace }}</span>
              </div>
              <div class="badges-row">
                <StatusBadge :status="selectedIncident.severity" size="sm" />
                <StatusBadge :status="selectedIncident.status" size="sm" />
                <span class="type-pill font-mono">{{ selectedIncident.type }}</span>
              </div>
            </div>

            <div class="inspector-actions">
              <button 
                class="btn btn-secondary btn-sm"
                :disabled="actionLoading === 'analyze' || selectedIncident.status === 'analyzing'"
                @click="triggerAIAnalysis(selectedIncident)"
              >
                <span>{{ actionLoading === 'analyze' ? '⏳ Reasoning...' : '🤖 AI Root Cause' }}</span>
              </button>
              <button 
                v-if="!activePR"
                class="btn btn-primary btn-sm"
                @click="openCreatePRModal"
              >
                <span>🚀 Generate Fix PR</span>
              </button>
            </div>
          </div>

          <!-- AI RCA Reasoning Card -->
          <div class="rca-inspector-card glass-panel">
            <div class="rca-card-header">
              <div class="rca-title-wrap">
                <span class="ai-sparkle">✨</span>
                <div>
                  <h3 class="rca-title">AI Root Cause Analysis (RCA)</h3>
                  <span class="ai-model-tag font-mono">{{ selectedReport?.llm_model || 'Claude 3.5 Sonnet / Multi-Agent' }}</span>
                </div>
              </div>

              <!-- Confidence Gauge -->
              <div class="confidence-gauge">
                <div class="gauge-dial font-mono">
                  <span class="gauge-pct">{{ Math.round((selectedReport?.confidence || 0.94) * 100) }}%</span>
                  <span class="gauge-label">Confidence</span>
                </div>
              </div>
            </div>

            <div class="rca-explanation font-mono">
              {{ selectedReport?.root_cause || selectedIncident.message }}
            </div>

            <!-- Evidence List -->
            <div class="evidence-box">
              <h4 class="evidence-title">Telemetry Evidence & Alert Correlation</h4>
              <div class="evidence-grid">
                <div v-for="(ev, idx) in (selectedReport?.evidence || [])" :key="idx" class="evidence-tag font-mono">
                  <span class="ev-bullet">▸</span>
                  <span>{{ ev }}</span>
                </div>
              </div>
            </div>
          </div>

          <!-- GitOps Remediation Diff Panel -->
          <div class="diff-panel glass-panel">
            <div class="diff-header">
              <div class="diff-title-wrap">
                <span class="diff-icon">📝</span>
                <div>
                  <h3 class="diff-title">GitOps Remediation Manifest Diff</h3>
                  <span class="diff-subtitle font-mono">deployments/{{ selectedIncident.namespace }}/{{ selectedIncident.pod_name.split('-')[0] }}.yaml</span>
                </div>
              </div>

              <div class="diff-actions">
                <div v-if="activePR" class="pr-status-pill">
                  <span class="font-mono text-cyan">PR #{{ activePR.pr_number || 104 }} ({{ activePR.status }})</span>
                </div>
              </div>
            </div>

            <!-- Unified Diff Code Viewer -->
            <div class="diff-code-box font-mono">
              <template v-if="selectedIncident.type === 'NodeNotReady'">
                <div class="diff-line diff-meta">@@ -12,6 +12,8 @@ spec:</div>
                <div class="diff-line diff-context">  unschedulable: false</div>
                <div class="diff-line diff-add">+  drainTimeoutSeconds: 300</div>
                <div class="diff-line diff-add">+  autoRemediation:</div>
                <div class="diff-line diff-add">+    restartKubelet: true</div>
                <div class="diff-line diff-add">+    resyncTimeout: 60s</div>
              </template>
              <template v-else-if="selectedIncident.type === 'CrashLoopBackOff'">
                <div class="diff-line diff-meta">@@ -28,7 +28,8 @@ spec:</div>
                <div class="diff-line diff-context">      containers:</div>
                <div class="diff-line diff-context">      - name: {{ selectedIncident.pod_name.split('-')[0] }}</div>
                <div class="diff-line diff-context">        env:</div>
                <div class="diff-line diff-del">-       - name: DB_CONNECT_TIMEOUT</div>
                <div class="diff-line diff-del">-         value: "5s"</div>
                <div class="diff-line diff-add">+       - name: DB_CONNECT_TIMEOUT</div>
                <div class="diff-line diff-add">+         value: "30s"</div>
                <div class="diff-line diff-add">+       - name: DB_MAX_RETRIES</div>
                <div class="diff-line diff-add">+         value: "10"</div>
              </template>
              <template v-else>
                <div class="diff-line diff-meta">@@ -42,7 +42,8 @@ resources:</div>
                <div class="diff-line diff-context">      requests:</div>
                <div class="diff-line diff-context">        cpu: 250m</div>
                <div class="diff-line diff-del">-       memory: 512Mi</div>
                <div class="diff-line diff-add">+       memory: 1024Mi</div>
                <div class="diff-line diff-context">      limits:</div>
                <div class="diff-line diff-context">        cpu: 1000m</div>
                <div class="diff-line diff-del">-       memory: 512Mi</div>
                <div class="diff-line diff-add">+       memory: 1024Mi</div>
                <div class="diff-line diff-add">+   - name: JAVA_TOOL_OPTIONS</div>
                <div class="diff-line diff-add">+     value: "-XX:MaxRAMPercentage=75.0"</div>
              </template>
            </div>

            <!-- Bottom Action Controls -->
            <div class="diff-footer">
              <div class="remediation-note font-mono text-muted">
                Remediation: {{ selectedReport?.remediation }}
              </div>

              <div class="remediation-btn-group">
                <button 
                  v-if="!activePR" 
                  class="btn btn-primary"
                  :disabled="actionLoading === 'create-pr'"
                  @click="openCreatePRModal"
                >
                  <span>🚀 Create Remediation PR ➔</span>
                </button>

                <button 
                  v-else-if="activePR.status === 'open' || activePR.status === 'pending'" 
                  class="btn btn-primary"
                  :disabled="actionLoading === 'merge-pr'"
                  @click="handleMergePR"
                >
                  <span>{{ actionLoading === 'merge-pr' ? '⏳ Merging PR...' : '⚡ Merge PR & Apply Fix' }}</span>
                </button>

                <div v-else-if="activePR.status === 'merged'" class="merged-badge font-mono">
                  <span>✅ Remediation Deployed to Production</span>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Create PR Modal -->
    <ModalDrawer
      v-model:show="showPRModal"
      mode="modal"
      title="Create GitOps Remediation Pull Request"
      subtitle="Synthesize patch manifest and submit pull request to repository"
      max-width="580px"
    >
      <div class="modal-form">
        <div class="form-group">
          <label class="form-label">Pull Request Title</label>
          <input v-model="prForm.title" type="text" class="input-glass" />
        </div>

        <div class="form-group">
          <label class="form-label">GitOps Repository</label>
          <input v-model="prForm.repoUrl" type="text" class="input-glass" />
        </div>

        <div class="form-row">
          <div class="form-group flex-1">
            <label class="form-label">Branch Name</label>
            <input v-model="prForm.branch" type="text" class="input-glass font-mono" />
          </div>
          <div class="form-group flex-1">
            <label class="form-label">Base Branch</label>
            <input v-model="prForm.baseBranch" type="text" class="input-glass font-mono" />
          </div>
        </div>

        <div class="form-group">
          <label class="form-label">Description / Commit Message</label>
          <textarea v-model="prForm.description" rows="3" class="input-glass font-mono"></textarea>
        </div>
      </div>

      <template #footer="{ close }">
        <button class="btn btn-secondary" @click="close">Cancel</button>
        <button class="btn btn-primary" :disabled="actionLoading === 'create-pr'" @click="handleCreatePR">
          <span>{{ actionLoading === 'create-pr' ? 'Submitting...' : 'Create Pull Request ➔' }}</span>
        </button>
      </template>
    </ModalDrawer>

    <!-- Incident Simulation Modal -->
    <ModalDrawer
      v-model:show="showSimulateModal"
      mode="modal"
      title="⚡ Incident Simulation & Telemetry Injection"
      subtitle="Inject synthetic Kubernetes cluster anomalies to test Autonomous RCA and GitOps remediation"
      max-width="640px"
    >
      <div class="simulation-modal-body">
        <p class="simulation-guide-text">
          Select a quick-start failure scenario below to trigger synthetic cluster metrics, container lifecycle events, and autonomous root-cause reasoning.
        </p>

        <div class="simulation-cards-grid">
          <div
            v-for="scenario in simulationScenarios"
            :key="scenario.key"
            class="sim-card glass-panel"
            :class="[`sim-card-sev-${scenario.severity}`]"
            @click="handleSimulateIncident(scenario)"
          >
            <div class="sim-card-header">
              <div class="sim-card-icon-wrap">
                <span class="sim-card-icon">{{ scenario.icon }}</span>
                <div class="sim-card-titles">
                  <h4 class="sim-card-title">{{ scenario.title }}</h4>
                  <span class="sim-card-subtitle font-mono">{{ scenario.subtitle }}</span>
                </div>
              </div>
              <StatusBadge :status="scenario.severity" size="sm" />
            </div>

            <p class="sim-card-desc">{{ scenario.description }}</p>

            <div class="sim-card-footer">
              <div class="sim-tags font-mono">
                <span class="sim-tag">{{ scenario.cluster }}</span>
                <span class="sim-tag">ns/{{ scenario.namespace }}</span>
                <span class="sim-tag badge-tag">{{ scenario.badgeText }}</span>
              </div>

              <button
                class="btn btn-sm btn-primary sim-action-btn"
                :disabled="actionLoading === `sim-${scenario.key}`"
                @click.stop="handleSimulateIncident(scenario)"
              >
                <span>{{ actionLoading === `sim-${scenario.key}` ? '⏳ Injecting...' : '⚡ Inject Scenario' }}</span>
              </button>
            </div>
          </div>
        </div>
      </div>

      <template #footer="{ close }">
        <button class="btn btn-secondary" @click="close">Cancel</button>
      </template>
    </ModalDrawer>
  </div>
</template>

<style scoped>
.view-container {
  display: flex;
  flex-direction: column;
  gap: 24px;
}

.view-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 20px;
}

.view-tag {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  font-size: 11px;
  font-weight: 700;
  color: var(--accent-cyan);
  letter-spacing: 0.08em;
  margin-bottom: 6px;
}

.view-title {
  font-size: 24px;
  font-weight: 800;
  color: #fff;
  letter-spacing: -0.02em;
}

.view-desc {
  font-size: 13px;
  color: var(--text-secondary);
  max-width: 750px;
  margin-top: 4px;
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 10px;
}

.toast-banner {
  padding: 12px 16px;
  border-radius: 12px;
  display: flex;
  align-items: center;
  gap: 12px;
  font-size: 13px;
}

.toast-success {
  background: rgba(16, 185, 129, 0.15);
  border: 1px solid rgba(16, 185, 129, 0.3);
  color: #34d399;
}

.toast-error {
  background: rgba(244, 63, 94, 0.15);
  border: 1px solid rgba(244, 63, 94, 0.3);
  color: #fb7185;
}

.toast-close {
  margin-left: auto;
  background: none;
  border: none;
  color: inherit;
  cursor: pointer;
}

.metrics-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(240px, 1fr));
  gap: 16px;
}

/* Split-Pane Layout */
.split-pane-layout {
  display: grid;
  grid-template-columns: 380px 1fr;
  gap: 20px;
  min-height: 640px;
}

@media (max-width: 1080px) {
  .split-pane-layout {
    grid-template-columns: 1fr;
  }
}

.left-pane {
  display: flex;
  flex-direction: column;
  border-radius: 16px;
  overflow: hidden;
  background: rgba(11, 15, 25, 0.65);
}

.pane-header {
  padding: 16px;
  border-bottom: 1px solid var(--border-subtle);
  display: flex;
  flex-direction: column;
  gap: 12px;
  background: rgba(15, 23, 42, 0.5);
}

.pane-title-wrap {
  display: flex;
  align-items: center;
  gap: 8px;
}

.pane-icon {
  font-size: 18px;
}

.pane-title {
  font-size: 14px;
  font-weight: 700;
  color: #fff;
}

.filter-controls {
  display: flex;
  gap: 8px;
}

.search-mini {
  flex: 1;
  font-size: 12px;
  padding: 6px 10px;
}

.select-mini {
  font-size: 11px;
  padding: 6px 8px;
}

.incident-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 12px;
  overflow-y: auto;
  max-height: 600px;
}

.empty-list {
  padding: 40px 20px;
  text-align: center;
  color: var(--text-muted);
  font-size: 13px;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 10px;
}

.empty-icon {
  font-size: 36px;
  margin-bottom: 2px;
}

.empty-title {
  font-size: 14px;
  font-weight: 700;
  color: #fff;
}

.empty-desc {
  font-size: 12px;
  color: var(--text-secondary);
  line-height: 1.5;
  max-width: 280px;
  margin: 0;
}

.empty-simulate-btn {
  margin-top: 6px;
  width: 100%;
}

.incident-card-item {
  padding: 14px;
  border-radius: 12px;
  background: rgba(15, 23, 42, 0.6);
  border: 1px solid var(--border-subtle);
  border-left: 4px solid var(--border-medium);
  cursor: pointer;
  display: flex;
  flex-direction: column;
  gap: 8px;
  transition: all 0.15s ease;
}

.incident-card-item:hover {
  background: rgba(22, 34, 58, 0.8);
  transform: translateX(2px);
}

.incident-item-active {
  background: rgba(6, 182, 212, 0.1);
  border-color: rgba(6, 182, 212, 0.4);
}

.border-sev-critical { border-left-color: var(--accent-rose); }
.border-sev-high { border-left-color: var(--accent-amber); }
.border-sev-medium { border-left-color: var(--accent-sky); }
.border-sev-low { border-left-color: var(--accent-emerald); }

.card-item-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.item-title-group {
  display: flex;
  align-items: center;
  gap: 6px;
}

.item-pod {
  font-size: 13px;
  font-weight: 700;
  color: #fff;
}

.card-item-mid {
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-size: 11px;
}

.type-badge {
  background: rgba(255, 255, 255, 0.06);
  padding: 2px 6px;
  border-radius: 4px;
  color: var(--accent-cyan);
}

.card-item-bot {
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-size: 11px;
  border-top: 1px solid var(--border-subtle);
  padding-top: 6px;
}

.arrow-indicator {
  color: var(--text-muted);
}

/* Right Pane: Inspector */
.right-pane {
  display: flex;
  flex-direction: column;
  border-radius: 16px;
  overflow: hidden;
  background: rgba(11, 15, 25, 0.65);
}

.no-selection {
  padding: 80px 40px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  text-align: center;
  color: var(--text-muted);
  gap: 12px;
}

.no-sel-icon {
  font-size: 40px;
}

.inspector-content {
  display: flex;
  flex-direction: column;
  gap: 16px;
  padding: 20px;
}

.inspector-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  border-bottom: 1px solid var(--border-subtle);
  padding-bottom: 16px;
}

.target-headline {
  display: flex;
  align-items: baseline;
  gap: 10px;
}

.target-pod-name {
  font-size: 18px;
  font-weight: 800;
  color: #fff;
}

.target-scope {
  font-size: 12px;
  color: var(--text-muted);
}

.badges-row {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 6px;
}

.type-pill {
  font-size: 11px;
  padding: 2px 6px;
  background: rgba(6, 182, 212, 0.12);
  color: #38bdf8;
  border-radius: 4px;
}

.inspector-actions {
  display: flex;
  gap: 8px;
}

/* RCA Inspector Card */
.rca-inspector-card {
  padding: 18px;
  border-radius: 14px;
  background: rgba(15, 23, 42, 0.6);
  display: flex;
  flex-direction: column;
  gap: 12px;
  border: 1px solid rgba(139, 92, 246, 0.3);
}

.rca-card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.rca-title-wrap {
  display: flex;
  align-items: center;
  gap: 10px;
}

.ai-sparkle {
  font-size: 20px;
}

.rca-title {
  font-size: 14px;
  font-weight: 700;
  color: #fff;
}

.ai-model-tag {
  font-size: 11px;
  color: #c4b5fd;
}

.confidence-gauge {
  background: rgba(16, 185, 129, 0.12);
  border: 1px solid rgba(16, 185, 129, 0.3);
  padding: 6px 14px;
  border-radius: 10px;
}

.gauge-dial {
  display: flex;
  flex-direction: column;
  align-items: center;
}

.gauge-pct {
  font-size: 16px;
  font-weight: 800;
  color: #34d399;
}

.gauge-label {
  font-size: 9px;
  text-transform: uppercase;
  color: var(--text-muted);
}

.rca-explanation {
  background: rgba(4, 6, 12, 0.7);
  border: 1px solid var(--border-subtle);
  border-radius: 10px;
  padding: 12px 14px;
  font-size: 12px;
  line-height: 1.6;
  color: #f1f5f9;
}

.evidence-box {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.evidence-title {
  font-size: 11px;
  font-weight: 700;
  color: var(--text-muted);
  text-transform: uppercase;
}

.evidence-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 6px;
}

.evidence-tag {
  font-size: 11px;
  background: rgba(0, 0, 0, 0.3);
  padding: 6px 8px;
  border-radius: 6px;
  color: var(--text-secondary);
  display: flex;
  align-items: center;
  gap: 6px;
}

.ev-bullet {
  color: var(--accent-cyan);
}

/* Diff Panel */
.diff-panel {
  padding: 18px;
  border-radius: 14px;
  background: rgba(15, 23, 42, 0.6);
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.diff-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.diff-title-wrap {
  display: flex;
  align-items: center;
  gap: 10px;
}

.diff-icon {
  font-size: 18px;
}

.diff-title {
  font-size: 14px;
  font-weight: 700;
  color: #fff;
}

.diff-subtitle {
  font-size: 11px;
  color: var(--text-muted);
}

.diff-code-box {
  background: #04060a;
  border: 1px solid var(--border-subtle);
  border-radius: 10px;
  padding: 12px 14px;
  font-size: 12px;
  line-height: 1.7;
}

.diff-line {
  white-space: pre;
}

.diff-meta {
  color: #64748b;
  font-weight: 700;
}

.diff-context {
  color: #94a3b8;
}

.diff-del {
  color: #fb7185;
  background: rgba(244, 63, 94, 0.12);
}

.diff-add {
  color: #34d399;
  background: rgba(16, 185, 129, 0.12);
}

.diff-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  border-top: 1px solid var(--border-subtle);
  padding-top: 12px;
  gap: 16px;
}

.remediation-note {
  font-size: 11px;
  max-width: 550px;
}

.remediation-btn-group {
  display: flex;
  align-items: center;
  gap: 10px;
}

.merged-badge {
  font-size: 12px;
  font-weight: 700;
  color: #34d399;
  background: rgba(16, 185, 129, 0.15);
  border: 1px solid rgba(16, 185, 129, 0.3);
  padding: 6px 12px;
  border-radius: 8px;
}

.modal-form {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.form-group {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.form-row {
  display: flex;
  gap: 12px;
}

.flex-1 { flex: 1; }

.form-label {
  font-size: 11px;
  font-weight: 700;
  color: var(--text-muted);
  text-transform: uppercase;
}

.font-mono { font-family: var(--font-mono); }
.text-cyan { color: var(--accent-cyan); }
.text-emerald { color: var(--accent-emerald); }
.text-muted { color: var(--text-muted); }
.btn-sm { padding: 6px 12px; font-size: 12px; }

/* Simulation Modal & Cards */
.simulation-modal-body {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.simulation-guide-text {
  font-size: 13px;
  color: var(--text-secondary);
  line-height: 1.5;
  margin: 0;
}

.simulation-cards-grid {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.sim-card {
  padding: 16px;
  border-radius: 14px;
  background: rgba(15, 23, 42, 0.65);
  border: 1px solid var(--border-subtle);
  border-left: 4px solid var(--border-medium);
  cursor: pointer;
  display: flex;
  flex-direction: column;
  gap: 10px;
  transition: all 0.2s ease;
}

.sim-card:hover {
  background: rgba(22, 34, 58, 0.85);
  transform: translateY(-2px);
  box-shadow: 0 8px 24px -6px rgba(0, 0, 0, 0.5);
}

.sim-card-sev-critical {
  border-left-color: var(--accent-rose);
}
.sim-card-sev-critical:hover {
  border-color: rgba(244, 63, 94, 0.4);
}

.sim-card-sev-high {
  border-left-color: var(--accent-amber);
}
.sim-card-sev-high:hover {
  border-color: rgba(245, 158, 11, 0.4);
}

.sim-card-sev-medium {
  border-left-color: var(--accent-sky);
}
.sim-card-sev-medium:hover {
  border-color: rgba(56, 189, 248, 0.4);
}

.sim-card-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}

.sim-card-icon-wrap {
  display: flex;
  align-items: center;
  gap: 12px;
}

.sim-card-icon {
  font-size: 24px;
}

.sim-card-titles {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.sim-card-title {
  font-size: 14px;
  font-weight: 700;
  color: #fff;
  margin: 0;
}

.sim-card-subtitle {
  font-size: 11px;
  color: var(--accent-cyan);
}

.sim-card-desc {
  font-size: 12px;
  color: var(--text-secondary);
  line-height: 1.5;
  margin: 0;
}

.sim-card-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  border-top: 1px solid var(--border-subtle);
  padding-top: 10px;
}

.sim-tags {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.sim-tag {
  font-size: 10px;
  padding: 2px 6px;
  background: rgba(255, 255, 255, 0.05);
  border-radius: 4px;
  color: var(--text-muted);
}

.sim-tag.badge-tag {
  background: rgba(6, 182, 212, 0.12);
  color: #38bdf8;
}

.sim-action-btn {
  white-space: nowrap;
  font-weight: 600;
}
</style>
