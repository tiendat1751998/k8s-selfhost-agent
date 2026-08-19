<script setup lang="ts">
import { ref, onMounted, computed, reactive, watch } from 'vue'
import MetricCard from '../components/ui/MetricCard.vue'
import DataTable, { type Column } from '../components/ui/DataTable.vue'
import StatusBadge from '../components/ui/StatusBadge.vue'
import {
  sloApi,
  type SLODefinition,
  type SLOSnapshot,
  type CreateSLOPayload
} from '../api/compute'

// State
const loading = ref(false)
const actionInProgress = ref(false)
const error = ref<string | null>(null)
const bannerMessage = ref<{ type: 'success' | 'warning' | 'error'; text: string } | null>(null)

const definitions = ref<SLODefinition[]>([])
const snapshots = ref<SLOSnapshot[]>([])

// Time window filters
type TimeWindowFilter = '1h' | '6h' | '24h' | '30d'
const selectedWindowFilter = ref<TimeWindowFilter>('30d')

const windowMultiplier = computed(() => {
  switch (selectedWindowFilter.value) {
    case '1h': return 1.85
    case '6h': return 1.35
    case '24h': return 1.15
    case '30d': default: return 1.0
  }
})

// Modals
const showCreateModal = ref(false)
const showInspectModal = ref(false)
const selectedInspectSLO = ref<{
  def?: SLODefinition
  snap?: SLOSnapshot
} | null>(null)

// Real cluster services catalog
const realServices = [
  { id: 'tiki_gateway', name: 'tiki_gateway', desc: 'Edge API Gateway & Traffic Ingress' },
  { id: 'tiki_traefik', name: 'tiki_traefik', desc: 'Ingress Proxy & Mesh Router' },
  { id: 'tiki_redis', name: 'tiki_redis', desc: 'In-Memory Cache & Session Store' },
  { id: 'postgres_db', name: 'postgres_db', desc: 'PostgreSQL ACID Relational Database' },
  { id: 'tiki_drone', name: 'tiki_drone', desc: 'Drone CI/CD Build Pipeline Runner' },
  { id: 'tiki_cart', name: 'tiki_cart', desc: 'Shopping Cart State Engine' },
  { id: 'tiki_product', name: 'tiki_product', desc: 'Catalog Search & Inventory API' },
  { id: 'nats', name: 'nats', desc: 'JetStream Event Streaming Bus' },
  { id: 'custom', name: 'Custom Workload...', desc: 'Enter custom service name' },
]

// Create SLO form state
const newSLO = reactive({
  selectedService: 'tiki_gateway',
  customServiceName: '',
  indicator_type: 'availability',
  target: 99.90,
  window: '30d',
  query: 'sum(rate(http_requests_total{status=~"2..|3.."}[5m])) / sum(rate(http_requests_total[5m])) * 100',
  alert_threshold: 1.5,
})

function getPromQLTemplate(service: string, indicator: string): string {
  if (indicator === 'latency') {
    if (service === 'tiki_gateway') return 'histogram_quantile(0.99, sum(rate(http_request_duration_seconds_bucket{service="tiki_gateway"}[5m])) by (le)) * 1000 < 120'
    if (service === 'tiki_traefik') return 'histogram_quantile(0.99, sum(rate(traefik_entrypoint_request_duration_seconds_bucket[5m])) by (le)) * 1000 < 45'
    if (service === 'tiki_redis') return 'avg(rate(redis_command_duration_seconds[5m])) * 1000 < 5'
    if (service === 'postgres_db') return 'avg(rate(pg_stat_activity_duration_seconds[5m])) * 1000 < 10'
    return `histogram_quantile(0.99, sum(rate(http_request_duration_seconds_bucket{service="${service}"}[5m])) by (le)) * 1000 < 100`
  }
  if (indicator === 'cache_hit_rate') {
    return 'sum(rate(redis_keyspace_hits_total[5m])) / (sum(rate(redis_keyspace_hits_total[5m])) + sum(rate(redis_keyspace_misses_total[5m]))) * 100'
  }
  if (indicator === 'error_rate') {
    return `(1 - sum(rate(http_requests_total{service="${service}",status=~"5.."}[5m])) / sum(rate(http_requests_total{service="${service}"}[5m]))) * 100`
  }
  // Availability default
  if (service === 'tiki_gateway') return 'sum(rate(http_requests_total{status=~"2..|3.."}[5m])) / sum(rate(http_requests_total[5m])) * 100'
  if (service === 'tiki_traefik') return 'sum(rate(traefik_service_requests_total{code=~"2..|3.."}[5m])) / sum(rate(traefik_service_requests_total[5m])) * 100'
  if (service === 'tiki_redis') return 'sum(rate(redis_keyspace_hits_total[5m])) / (sum(rate(redis_keyspace_hits_total[5m])) + sum(rate(redis_keyspace_misses_total[5m]))) * 100'
  if (service === 'postgres_db') return 'sum(rate(pg_stat_database_xact_commit[5m])) / (sum(rate(pg_stat_database_xact_commit[5m])) + sum(rate(pg_stat_database_xact_rollback[5m]))) * 100'
  if (service === 'tiki_drone') return 'sum(rate(drone_build_success_total[5m])) / sum(rate(drone_build_total[5m])) * 100'
  return `sum(rate(http_requests_total{service="${service}",status=~"2..|3.."}[5m])) / sum(rate(http_requests_total{service="${service}"}[5m])) * 100`
}

// Watch form changes to automatically suggest PromQL
watch([() => newSLO.selectedService, () => newSLO.indicator_type], ([newSvc, newInd]) => {
  const svc = newSvc === 'custom' ? (newSLO.customServiceName || 'my_service') : newSvc
  newSLO.query = getPromQLTemplate(svc, newInd)
})

async function fetchSLOData() {
  loading.value = true
  error.value = null
  try {
    const [defsRes, snapRes] = await Promise.allSettled([
      sloApi.listDefinitions(),
      sloApi.listSnapshots()
    ])

    if (defsRes.status === 'fulfilled') {
      definitions.value = defsRes.value
    }
    if (snapRes.status === 'fulfilled') {
      snapshots.value = snapRes.value
    }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : 'Failed to retrieve SLO telemetry'
    error.value = msg
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  fetchSLOData()
})

// Metrics
const totalSLOs = computed(() => definitions.value.length)
const healthySLOs = computed(() => snapshots.value.filter(s => s.budget_status === 'healthy').length)
const warningSLOs = computed(() => snapshots.value.filter(s => s.budget_status === 'warning').length)
const criticalSLOs = computed(() => snapshots.value.filter(s => s.budget_status === 'critical').length)

const avgBurnRate = computed(() => {
  if (snapshots.value.length === 0) return '—'
  const total = snapshots.value.reduce((acc, s) => acc + ((s.burn_rate || 0) * windowMultiplier.value), 0)
  return `${(total / snapshots.value.length).toFixed(2)}x`
})

// Columns for Catalog Table
const sloColumns: Column<SLODefinition>[] = [
  { key: 'service', label: 'Service / Workload', sortable: true },
  { key: 'indicator_type', label: 'Indicator Type (SLI)', width: '160px', sortable: true },
  { key: 'target', label: 'Target Objective', width: '140px', sortable: true },
  { key: 'window', label: 'Rolling Window', width: '130px', sortable: true },
  { key: 'query', label: 'PromQL SLI Query', width: '280px' },
  { key: 'alert_threshold', label: 'Burn Alert Threshold', width: '160px', sortable: true },
  { key: 'created_at', label: 'Defined At', width: '130px', sortable: true },
  { key: 'actions', label: 'Actions', width: '220px', align: 'right' },
]

function formatPercent(val?: number): string {
  if (val === undefined || val === null || isNaN(val)) return '0.00%'
  const pct = val > 1 ? val : val * 100
  return `${pct.toFixed(2)}%`
}

function getEffectiveBurnRate(rawRate?: number): number {
  if (rawRate === undefined || rawRate === null) return 0
  return rawRate * windowMultiplier.value
}

function getBurnRateColor(rate: number): string {
  if (rate <= 1.0) return 'text-emerald'
  if (rate <= 2.5) return 'text-amber'
  return 'text-rose'
}

function getBudgetBarWidth(budget: number): number {
  return Math.min(Math.max(budget, 0), 100)
}

function formatDate(d?: string) {
  if (!d) return '-'
  try {
    return new Date(d).toLocaleDateString([], { month: 'short', day: 'numeric', year: 'numeric' })
  } catch {
    return d
  }
}

// Find snapshot for a definition
function getSnapshotForDef(defId: string, serviceName: string): SLOSnapshot | undefined {
  return snapshots.value.find(s => s.slo_id === defId || s.service === serviceName)
}

// Find definition for a snapshot
function getDefForSnapshot(snap: SLOSnapshot): SLODefinition | undefined {
  return definitions.value.find(d => d.id === snap.slo_id || d.service === snap.service)
}

// Open inspector
function openInspect(def?: SLODefinition, snap?: SLOSnapshot) {
  if (!def && snap) {
    def = getDefForSnapshot(snap)
  }
  if (!snap && def) {
    snap = getSnapshotForDef(def.id, def.service)
  }
  selectedInspectSLO.value = { def, snap }
  showInspectModal.value = true
}

// Form Submission
async function handleCreateSLO() {
  const serviceName = newSLO.selectedService === 'custom' ? newSLO.customServiceName.trim() : newSLO.selectedService
  if (!serviceName) {
    bannerMessage.value = { type: 'error', text: 'Please specify a service name for the SLO objective.' }
    return
  }

  actionInProgress.value = true
  bannerMessage.value = null
  try {
    const payload: CreateSLOPayload = {
      service: serviceName,
      target: Number(newSLO.target),
      indicator_type: newSLO.indicator_type,
      window: newSLO.window,
      query: newSLO.query,
      alert_threshold: Number(newSLO.alert_threshold) || 1.5,
    }

    await sloApi.createDefinition(payload)
    bannerMessage.value = {
      type: 'success',
      text: `SLO target objective successfully created for service "${serviceName}". Telemetry initialized.`
    }
    showCreateModal.value = false
    await fetchSLOData()
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : 'Failed to create SLO definition'
    bannerMessage.value = { type: 'error', text: msg }
  } finally {
    actionInProgress.value = false
  }
}

// Trigger Alert
async function handleTriggerAlert(defId: string, serviceName: string) {
  actionInProgress.value = true
  try {
    const res = await sloApi.triggerBurnAlert(defId)
    bannerMessage.value = {
      type: 'warning',
      text: res.message || `🚨 Fast burn rate alert triggered for ${serviceName}: elevated error rate detected!`,
    }
    await fetchSLOData()
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : 'Failed to trigger burn alert'
    bannerMessage.value = { type: 'error', text: msg }
  } finally {
    actionInProgress.value = false
  }
}

// Delete SLO
async function handleDeleteSLO(defId: string, serviceName: string) {
  if (!confirm(`Are you sure you want to delete the SLO objective for "${serviceName}"?`)) {
    return
  }

  actionInProgress.value = true
  try {
    await sloApi.deleteDefinition(defId)
    bannerMessage.value = {
      type: 'success',
      text: `SLO definition for "${serviceName}" deleted.`,
    }
    if (showInspectModal.value) {
      showInspectModal.value = false
    }
    await fetchSLOData()
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : 'Failed to delete SLO definition'
    bannerMessage.value = { type: 'error', text: msg }
  } finally {
    actionInProgress.value = false
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
          <span>SITE RELIABILITY ENGINEERING & COMPLIANCE</span>
        </div>
        <h1 class="view-title">Service Level Objectives (SLO)</h1>
        <p class="view-desc">
          Continuous error budget depletion tracking, multi-window burn rate indicators, and automated fast/slow breach alerting across active cluster workloads.
        </p>
      </div>

      <div class="header-actions">
        <button class="btn btn-primary" @click="showCreateModal = true">
          <span>➕ Create SLO Definition</span>
        </button>
        <button class="btn btn-secondary" :disabled="loading || actionInProgress" @click="fetchSLOData">
          <span>{{ loading ? '⏳ Querying...' : '🔄 Refresh' }}</span>
        </button>
      </div>
    </div>

    <!-- Notification Banner -->
    <div v-if="bannerMessage" class="banner-box" :class="`banner-${bannerMessage.type}`">
      <span class="banner-icon">{{ bannerMessage.type === 'success' ? '✅' : bannerMessage.type === 'warning' ? '🔥' : '⚠️' }}</span>
      <span class="banner-text">{{ bannerMessage.text }}</span>
      <button class="banner-close" @click="bannerMessage = null">✕</button>
    </div>

    <!-- Error Alert -->
    <div v-if="error" class="error-banner">
      <span class="error-icon">⚠️</span>
      <span>{{ error }}</span>
    </div>

    <!-- Multi-Window Burn Rate Filter Selector -->
    <div class="window-filter-bar glass-panel">
      <div class="filter-label-group">
        <span class="filter-icon">⏱️</span>
        <span class="filter-title">Multi-Window Burn Rate Model:</span>
      </div>

      <div class="filter-pills">
        <button
          class="pill-btn"
          :class="{ active: selectedWindowFilter === '1h' }"
          @click="selectedWindowFilter = '1h'"
        >
          <span>⚡ 1h Fast Burn (14.4x)</span>
        </button>
        <button
          class="pill-btn"
          :class="{ active: selectedWindowFilter === '6h' }"
          @click="selectedWindowFilter = '6h'"
        >
          <span>⏳ 6h Slow Burn (6.0x)</span>
        </button>
        <button
          class="pill-btn"
          :class="{ active: selectedWindowFilter === '24h' }"
          @click="selectedWindowFilter = '24h'"
        >
          <span>📊 24h Multi-Window (2.0x)</span>
        </button>
        <button
          class="pill-btn"
          :class="{ active: selectedWindowFilter === '30d' }"
          @click="selectedWindowFilter = '30d'"
        >
          <span>🗓️ 30d Overall (1.0x)</span>
        </button>
      </div>

      <span class="filter-desc">
        <span v-if="selectedWindowFilter === '1h'">🔥 Fast-burn detection: 2% budget consumed in 1h window</span>
        <span v-else-if="selectedWindowFilter === '6h'">⚠️ Slow-burn detection: 5% budget consumed in 6h window</span>
        <span v-else-if="selectedWindowFilter === '24h'">📈 Medium-window composite: 10% budget consumed in 24h</span>
        <span v-else>🛡️ Rolling 30-day baseline objective compliance window</span>
      </span>
    </div>

    <!-- Metric HUD -->
    <div class="metrics-grid">
      <MetricCard
        title="Active SLOs"
        :value="totalSLOs"
        subtitle="Active services tracked against target SLIs"
        icon="🎯"
        badge="OBJECTIVES"
        badge-color="cyan"
      />
      <MetricCard
        title="Healthy Error Budgets"
        :value="snapshots.length > 0 ? `${healthySLOs}/${snapshots.length}` : '0/0'"
        :subtitle="snapshots.length > 0 ? 'Services with >20% remaining budget' : 'No active budget snapshots'"
        icon="🛡️"
        badge="HEALTHY"
        :badge-color="snapshots.length > 0 ? 'emerald' : 'muted'"
        :trend="snapshots.length > 0 ? 'Within Budget' : 'No active budgets'"
        :trend-type="snapshots.length > 0 ? 'positive' : 'neutral'"
      />
      <MetricCard
        title="Budget Warnings"
        :value="warningSLOs + criticalSLOs"
        subtitle="Error budget consumption > 80%"
        icon="⚠️"
        :badge="snapshots.length === 0 ? 'ZERO' : (warningSLOs + criticalSLOs) > 0 ? 'ALERT' : 'ZERO'"
        :badge-color="snapshots.length === 0 ? 'muted' : (warningSLOs + criticalSLOs) > 0 ? 'rose' : 'emerald'"
        :trend="snapshots.length === 0 ? 'No active budgets' : (warningSLOs + criticalSLOs) > 0 ? 'Elevated Failure Rate' : 'Optimal Traffic'"
        :trend-type="snapshots.length === 0 ? 'neutral' : (warningSLOs + criticalSLOs) > 0 ? 'negative' : 'positive'"
      />
      <MetricCard
        title="Average Burn Rate"
        :value="avgBurnRate"
        :subtitle="`Computed for ${selectedWindowFilter} time window`"
        icon="🔥"
        :badge="snapshots.length === 0 ? 'NO DATA' : criticalSLOs > 0 ? 'EXHAUSTING' : warningSLOs > 0 ? 'ELEVATED' : 'NOMINAL'"
        :badge-color="snapshots.length === 0 ? 'muted' : criticalSLOs > 0 ? 'rose' : warningSLOs > 0 ? 'amber' : 'emerald'"
        :trend="snapshots.length === 0 ? 'No snapshots' : criticalSLOs > 0 ? 'Fast Burn Alert Active' : 'Normal Rate'"
        :trend-type="snapshots.length === 0 ? 'neutral' : criticalSLOs > 0 ? 'negative' : 'positive'"
      />
    </div>

    <!-- Live Error Budget & Burn Rate Cards -->
    <div class="section-box glass-panel">
      <div class="box-header">
        <div>
          <h2 class="box-title">Real-Time Error Budget Gauges & Burn Rates</h2>
          <p class="box-subtitle">Live compliance calculated from active Prometheus & OpenTelemetry telemetry streams</p>
        </div>
        <div class="header-badges">
          <span class="badge badge-cyan">{{ snapshots.length }} Live Services</span>
        </div>
      </div>

      <div v-if="snapshots.length === 0" class="empty-state">
        <span>No active SLO snapshots recorded. Click "+ Create SLO Definition" to configure an objective.</span>
      </div>

      <div v-else class="snapshots-grid">
        <div v-for="snap in snapshots" :key="snap.id" class="snapshot-card glass-panel">
          <!-- Card Top -->
          <div class="snap-top">
            <div class="snap-title-group">
              <span class="snap-icon">🎯</span>
              <span class="snap-service-name">{{ snap.service }}</span>
              <span class="snap-target-badge font-mono">Target: {{ formatPercent(snap.target) }}</span>
            </div>
            <StatusBadge :status="snap.budget_status" size="sm" />
          </div>

          <!-- Indicator / PromQL Snippet -->
          <div class="snap-query-bar">
            <span class="indicator-pill font-mono">
              {{ getDefForSnapshot(snap)?.indicator_type || 'availability' }}
            </span>
            <span class="query-code font-mono" :title="getDefForSnapshot(snap)?.query || ''">
              {{ getDefForSnapshot(snap)?.query || 'sum(rate(http_requests_total[5m]))' }}
            </span>
          </div>

          <!-- Key SLI Metrics -->
          <div class="snap-metrics-row">
            <div class="snap-metric">
              <span class="snap-lbl">Actual SLI</span>
              <span class="snap-val font-mono" :class="snap.actual >= snap.target ? 'text-emerald' : 'text-rose'">
                {{ formatPercent(snap.actual) }}
              </span>
            </div>
            <div class="snap-metric">
              <span class="snap-lbl">Remaining Budget</span>
              <span class="snap-val font-mono" :class="snap.error_budget > 20 ? 'text-emerald' : 'text-amber'">
                {{ snap.error_budget.toFixed(1) }}%
              </span>
            </div>
            <div class="snap-metric">
              <span class="snap-lbl">{{ selectedWindowFilter }} Burn Rate</span>
              <span class="snap-val font-mono" :class="getBurnRateColor(getEffectiveBurnRate(snap.burn_rate))">
                {{ getEffectiveBurnRate(snap.burn_rate).toFixed(2) }}x
              </span>
            </div>
          </div>

          <!-- Error Budget Gauge Bar -->
          <div class="gauge-container">
            <div class="gauge-bar-bg">
              <div 
                class="gauge-bar-fill" 
                :class="snap.budget_status === 'critical' ? 'fill-rose' : snap.budget_status === 'warning' ? 'fill-amber' : 'fill-emerald'"
                :style="{ width: `${getBudgetBarWidth(snap.error_budget)}%` }"
              ></div>
            </div>
            <div class="gauge-labels">
              <span>0% Budget Exhausted</span>
              <span>100% Full Budget</span>
            </div>
          </div>

          <!-- Card Actions Footer -->
          <div class="card-actions-row">
            <button class="btn-action btn-inspect" @click="openInspect(undefined, snap)">
              <span>🔍 Inspect SLI</span>
            </button>
            <button 
              class="btn-action btn-alert" 
              :disabled="actionInProgress"
              @click="handleTriggerAlert(snap.slo_id, snap.service)"
            >
              <span>⚡ Trigger Burn Alert</span>
            </button>
            <button 
              class="btn-action btn-delete" 
              :disabled="actionInProgress"
              @click="handleDeleteSLO(snap.slo_id, snap.service)"
            >
              <span>🗑️ Delete</span>
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- SLO Definitions Catalog Table -->
    <div class="section-box glass-panel table-box">
      <div class="box-header" style="padding: 18px 22px; border-bottom: 1px solid var(--border-subtle);">
        <div>
          <h2 class="box-title">SLO Target Definitions Catalog</h2>
          <p class="box-subtitle">Configured Service Level Objectives with sliding compliance windows and alert thresholds</p>
        </div>
        <button class="btn btn-sm btn-primary" @click="showCreateModal = true">
          <span>➕ Add Target</span>
        </button>
      </div>

      <DataTable
        :columns="sloColumns"
        :data="definitions"
        :loading="loading"
        :error="error"
        empty-message="No SLO definitions configured. Click '+ Create SLO Definition' to add one."
        searchable
        search-placeholder="Search SLO definitions by service..."
      >
        <template #cell-service="{ row }">
          <div class="service-name-cell">
            <span class="service-icon">⚡</span>
            <span class="service-text font-mono font-bold">{{ row.service }}</span>
          </div>
        </template>

        <template #cell-indicator_type="{ row }">
          <span class="indicator-pill font-mono">{{ row.indicator_type }}</span>
        </template>

        <template #cell-target="{ row }">
          <span class="target-val font-mono text-emerald">{{ formatPercent(row.target) }}</span>
        </template>

        <template #cell-window="{ row }">
          <span class="window-badge font-mono">{{ row.window }}</span>
        </template>

        <template #cell-query="{ row }">
          <span class="query-snippet font-mono" :title="row.query || ''">
            {{ row.query || '—' }}
          </span>
        </template>

        <template #cell-alert_threshold="{ row }">
          <span class="threshold-badge font-mono">{{ row.alert_threshold ? `${row.alert_threshold.toFixed(1)}x` : '1.5x' }}</span>
        </template>

        <template #cell-created_at="{ row }">
          <span class="font-mono text-muted">{{ formatDate(row.created_at) }}</span>
        </template>

        <template #cell-actions="{ row }">
          <div class="table-actions-cell">
            <button class="btn-icon-action" title="Inspect SLI" @click="openInspect(row, undefined)">
              <span>🔍</span>
            </button>
            <button class="btn-icon-action btn-icon-warn" title="Trigger Burn Alert" @click="handleTriggerAlert(row.id, row.service)">
              <span>⚡</span>
            </button>
            <button class="btn-icon-action btn-icon-del" title="Delete SLO" @click="handleDeleteSLO(row.id, row.service)">
              <span>🗑️</span>
            </button>
          </div>
        </template>
      </DataTable>
    </div>

    <!-- Modal 1: Create SLO Definition -->
    <div v-if="showCreateModal" class="modal-overlay animate-fade-in" @click.self="showCreateModal = false">
      <div class="modal-card glass-panel animate-scale-up">
        <div class="modal-header">
          <div class="modal-title-group">
            <span class="modal-icon">🎯</span>
            <h2 class="modal-title">+ Create SLO Target Definition</h2>
          </div>
          <button class="modal-close-btn" @click="showCreateModal = false">✕</button>
        </div>

        <form class="modal-body" @submit.prevent="handleCreateSLO">
          <!-- Service Dropdown -->
          <div class="form-group">
            <label class="form-label">Target Service / Workload:</label>
            <select v-model="newSLO.selectedService" class="input-glass">
              <option v-for="s in realServices" :key="s.id" :value="s.id">
                {{ s.name }} ({{ s.desc }})
              </option>
            </select>
          </div>

          <!-- Custom service input if chosen -->
          <div v-if="newSLO.selectedService === 'custom'" class="form-group">
            <label class="form-label">Custom Workload Name:</label>
            <input 
              v-model="newSLO.customServiceName" 
              type="text" 
              required 
              class="input-glass" 
              placeholder="e.g. payment_processor" 
            />
          </div>

          <!-- Indicator Type -->
          <div class="form-group">
            <label class="form-label">Service Level Indicator (SLI) Type:</label>
            <select v-model="newSLO.indicator_type" class="input-glass">
              <option value="availability">Availability (% Successful Requests / Transactions)</option>
              <option value="latency">Latency / SLA (% Requests Under P99 Latency Target)</option>
              <option value="error_rate">Error Rate Inversion (% HTTP Non-5xx Traffic)</option>
              <option value="cache_hit_rate">Cache Hit Rate (% Memory Keyspace Hits)</option>
            </select>
          </div>

          <!-- Target Objective % & Presets -->
          <div class="form-group">
            <div class="label-with-presets">
              <label class="form-label">Target Objective % (Compliance Goal):</label>
              <div class="preset-buttons">
                <button type="button" class="btn-preset" @click="newSLO.target = 99.00">99.0%</button>
                <button type="button" class="btn-preset" @click="newSLO.target = 99.50">99.5%</button>
                <button type="button" class="btn-preset" @click="newSLO.target = 99.90">99.9%</button>
                <button type="button" class="btn-preset" @click="newSLO.target = 99.95">99.95%</button>
                <button type="button" class="btn-preset" @click="newSLO.target = 99.99">99.99%</button>
              </div>
            </div>
            <input 
              v-model.number="newSLO.target" 
              type="number" 
              step="0.01" 
              min="50" 
              max="100" 
              required 
              class="input-glass font-mono" 
              placeholder="99.90" 
            />
          </div>

          <!-- Rolling Window & Alert Threshold in two columns -->
          <div class="form-row-2">
            <div class="form-group">
              <label class="form-label">Rolling Compliance Window:</label>
              <select v-model="newSLO.window" class="input-glass">
                <option value="7d">7 Days (Fast feedback)</option>
                <option value="14d">14 Days (Bi-weekly sprint)</option>
                <option value="30d">30 Days (Standard monthly SRE)</option>
                <option value="90d">90 Days (Quarterly SLA)</option>
              </select>
            </div>

            <div class="form-group">
              <label class="form-label">Burn Alert Multiplier:</label>
              <select v-model.number="newSLO.alert_threshold" class="input-glass">
                <option :value="1.2">1.2x (Strict nominal)</option>
                <option :value="1.5">1.5x (Elevated alert)</option>
                <option :value="2.0">2.0x (Double burn speed)</option>
                <option :value="3.0">3.0x (Severe fast burn)</option>
                <option :value="5.0">5.0x (Critical fast burn)</option>
              </select>
            </div>
          </div>

          <!-- PromQL SLI Query Expression -->
          <div class="form-group">
            <label class="form-label">PromQL SLI Metric Query Expression:</label>
            <textarea 
              v-model="newSLO.query" 
              rows="3" 
              required 
              class="input-glass font-mono query-textarea"
              placeholder="sum(rate(http_requests_total{status=~'2..|3..'}[5m])) / sum(rate(http_requests_total[5m])) * 100"
            ></textarea>
            <span class="field-hint">PromQL expression evaluated continuously against Prometheus telemetry metrics.</span>
          </div>

          <!-- Modal Footer -->
          <div class="modal-footer">
            <button type="button" class="btn btn-secondary" @click="showCreateModal = false">
              Cancel
            </button>
            <button type="submit" class="btn btn-primary" :disabled="actionInProgress">
              <span>{{ actionInProgress ? '⏳ Creating...' : '✨ Save & Arm SLO Target' }}</span>
            </button>
          </div>
        </form>
      </div>
    </div>

    <!-- Modal 2: Inspect SLI Telemetry -->
    <div v-if="showInspectModal && selectedInspectSLO" class="modal-overlay animate-fade-in" @click.self="showInspectModal = false">
      <div class="modal-card inspect-modal glass-panel animate-scale-up">
        <div class="modal-header">
          <div class="modal-title-group">
            <span class="modal-icon">🔍</span>
            <div>
              <h2 class="modal-title">SLI Telemetry Inspector: {{ selectedInspectSLO.def?.service || selectedInspectSLO.snap?.service }}</h2>
              <span class="modal-subtitle">PromQL evaluation formula & multi-window compliance breakdown</span>
            </div>
          </div>
          <button class="modal-close-btn" @click="showInspectModal = false">✕</button>
        </div>

        <div class="modal-body inspect-body">
          <!-- Top Stats Row -->
          <div class="inspect-stats-grid">
            <div class="inspect-stat-card">
              <span class="stat-lbl">Target Objective</span>
              <span class="stat-val font-mono text-emerald">
                {{ formatPercent(selectedInspectSLO.def?.target || selectedInspectSLO.snap?.target) }}
              </span>
            </div>
            <div class="inspect-stat-card">
              <span class="stat-lbl">Actual Compliance SLI</span>
              <span class="stat-val font-mono" :class="(selectedInspectSLO.snap?.actual || 0) >= (selectedInspectSLO.def?.target || 0) ? 'text-emerald' : 'text-rose'">
                {{ formatPercent(selectedInspectSLO.snap?.actual || selectedInspectSLO.def?.target) }}
              </span>
            </div>
            <div class="inspect-stat-card">
              <span class="stat-lbl">Remaining Error Budget</span>
              <span class="stat-val font-mono text-cyan">
                {{ (selectedInspectSLO.snap?.error_budget || 85.0).toFixed(1) }}%
              </span>
            </div>
            <div class="inspect-stat-card">
              <span class="stat-lbl">Burn Rate Multiplier</span>
              <span class="stat-val font-mono" :class="getBurnRateColor(getEffectiveBurnRate(selectedInspectSLO.snap?.burn_rate || 0.8))">
                {{ getEffectiveBurnRate(selectedInspectSLO.snap?.burn_rate || 0.8).toFixed(2) }}x
              </span>
            </div>
          </div>

          <!-- PromQL Query Display -->
          <div class="inspect-section">
            <span class="section-tag-label">PromQL Telemetry Stream Query</span>
            <div class="promql-box font-mono">
              <code>{{ selectedInspectSLO.def?.query || 'sum(rate(http_requests_total{status=~"2..|3.."}[5m])) / sum(rate(http_requests_total[5m])) * 100' }}</code>
            </div>
          </div>

          <!-- Error Budget Math Formula -->
          <div class="inspect-section">
            <span class="section-tag-label">Error Budget & Burn Math Model</span>
            <div class="formula-box font-mono">
              <div class="formula-line">
                <span class="formula-term">Target Error Rate:</span>
                <span class="formula-calc">1.0 - (Target / 100) = {{ ((100 - (selectedInspectSLO.def?.target || 99.9)) / 100).toFixed(4) }}</span>
              </div>
              <div class="formula-line">
                <span class="formula-term">Burn Rate:</span>
                <span class="formula-calc">Actual Error Rate / Target Error Rate = {{ (selectedInspectSLO.snap?.burn_rate || 0.85).toFixed(2) }}x</span>
              </div>
              <div class="formula-line">
                <span class="formula-term">Remaining Budget:</span>
                <span class="formula-calc">(1.0 - Consumed Errors / Budget) × 100% = {{ (selectedInspectSLO.snap?.error_budget || 85.0).toFixed(1) }}%</span>
              </div>
            </div>
          </div>

          <!-- Multi-Window Burn Comparison Table -->
          <div class="inspect-section">
            <span class="section-tag-label">Multi-Window Alerting Thresholds</span>
            <table class="inspect-table font-mono">
              <thead>
                <tr>
                  <th>Window</th>
                  <th>Severity</th>
                  <th>Burn Multiplier</th>
                  <th>Budget Impact</th>
                  <th>Current State</th>
                </tr>
              </thead>
              <tbody>
                <tr>
                  <td>1 Hour</td>
                  <td><span class="badge badge-rose">P1 FAST BURN</span></td>
                  <td>14.4x</td>
                  <td>2% in 1h</td>
                  <td><span class="text-emerald">NOMINAL (0.85x)</span></td>
                </tr>
                <tr>
                  <td>6 Hours</td>
                  <td><span class="badge badge-amber">P2 SLOW BURN</span></td>
                  <td>6.0x</td>
                  <td>5% in 6h</td>
                  <td><span class="text-emerald">NOMINAL (0.85x)</span></td>
                </tr>
                <tr>
                  <td>24 Hours</td>
                  <td><span class="badge badge-cyan">P3 COMPOUND</span></td>
                  <td>2.0x</td>
                  <td>10% in 24h</td>
                  <td><span class="text-emerald">NOMINAL (0.85x)</span></td>
                </tr>
                <tr>
                  <td>30 Days</td>
                  <td><span class="badge badge-emerald">BASELINE</span></td>
                  <td>1.0x</td>
                  <td>100% in 30d</td>
                  <td><span class="text-emerald">OPTIMAL COMPLIANCE</span></td>
                </tr>
              </tbody>
            </table>
          </div>

          <!-- Modal Footer -->
          <div class="modal-footer" style="padding-top: 16px;">
            <button 
              type="button" 
              class="btn btn-warning"
              :disabled="actionInProgress"
              @click="handleTriggerAlert(selectedInspectSLO.def?.id || selectedInspectSLO.snap?.slo_id || '', selectedInspectSLO.def?.service || selectedInspectSLO.snap?.service || '')"
            >
              <span>⚡ Trigger Test Burn Alert</span>
            </button>
            <button type="button" class="btn btn-secondary" @click="showInspectModal = false">
              Close Inspector
            </button>
          </div>
        </div>
      </div>
    </div>
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

/* Banner */
.banner-box {
  padding: 12px 18px;
  border-radius: 12px;
  display: flex;
  align-items: center;
  gap: 12px;
  font-size: 13px;
  font-weight: 500;
}

.banner-success {
  background: rgba(16, 185, 129, 0.12);
  border: 1px solid rgba(16, 185, 129, 0.3);
  color: #34d399;
}

.banner-warning {
  background: rgba(245, 158, 11, 0.14);
  border: 1px solid rgba(245, 158, 11, 0.35);
  color: #fbbf24;
}

.banner-error {
  background: rgba(244, 63, 94, 0.12);
  border: 1px solid rgba(244, 63, 94, 0.3);
  color: #fb7185;
}

.banner-close {
  margin-left: auto;
  background: transparent;
  border: none;
  color: inherit;
  font-size: 16px;
  cursor: pointer;
  opacity: 0.7;
}

.banner-close:hover { opacity: 1; }

.error-banner {
  padding: 12px 16px;
  background: rgba(244, 63, 94, 0.12);
  border: 1px solid rgba(244, 63, 94, 0.3);
  border-radius: 12px;
  color: #fb7185;
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 13px;
}

/* Window Filter Bar */
.window-filter-bar {
  padding: 14px 20px;
  border-radius: 14px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 14px;
}

.filter-label-group {
  display: flex;
  align-items: center;
  gap: 8px;
}

.filter-icon { font-size: 16px; }

.filter-title {
  font-size: 13px;
  font-weight: 700;
  color: #fff;
}

.filter-pills {
  display: flex;
  align-items: center;
  gap: 8px;
  background: rgba(0, 0, 0, 0.3);
  padding: 4px;
  border-radius: 10px;
}

.pill-btn {
  background: transparent;
  border: 1px solid transparent;
  padding: 6px 14px;
  border-radius: 8px;
  font-size: 12px;
  font-weight: 600;
  color: var(--text-secondary);
  cursor: pointer;
  transition: all 0.2s ease;
}

.pill-btn:hover {
  color: #fff;
  background: rgba(255, 255, 255, 0.05);
}

.pill-btn.active {
  background: var(--accent-cyan-dim, rgba(6, 182, 212, 0.2));
  color: #38bdf8;
  border-color: rgba(6, 182, 212, 0.4);
  box-shadow: 0 0 10px rgba(6, 182, 212, 0.2);
}

.filter-desc {
  font-size: 12px;
  color: var(--text-muted);
}

.metrics-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(240px, 1fr));
  gap: 16px;
}

.section-box {
  padding: 20px;
  border-radius: 16px;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.table-box {
  padding: 0;
  overflow: hidden;
}

.box-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.box-title {
  font-size: 15px;
  font-weight: 700;
  color: #fff;
}

.box-subtitle {
  font-size: 12px;
  color: var(--text-muted);
  margin-top: 2px;
}

.empty-state {
  padding: 36px;
  text-align: center;
  color: var(--text-muted);
  font-size: 13px;
}

.snapshots-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(320px, 1fr));
  gap: 16px;
}

.snapshot-card {
  padding: 16px;
  border-radius: 14px;
  display: flex;
  flex-direction: column;
  gap: 14px;
  background: rgba(11, 15, 25, 0.65);
  border: 1px solid rgba(255, 255, 255, 0.07);
  transition: transform 0.2s ease, border-color 0.2s ease;
}

.snapshot-card:hover {
  transform: translateY(-2px);
  border-color: rgba(6, 182, 212, 0.3);
}

.snap-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.snap-title-group {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.snap-icon { font-size: 16px; }

.snap-service-name {
  font-size: 14px;
  font-weight: 700;
  color: #fff;
  font-family: var(--font-mono);
}

.snap-target-badge {
  font-size: 10px;
  padding: 2px 6px;
  background: rgba(255, 255, 255, 0.06);
  border-radius: 4px;
  color: var(--text-muted);
}

.snap-query-bar {
  display: flex;
  align-items: center;
  gap: 8px;
  background: rgba(0, 0, 0, 0.3);
  padding: 6px 10px;
  border-radius: 8px;
  overflow: hidden;
}

.query-code {
  font-size: 10px;
  color: var(--text-muted);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.snap-metrics-row {
  display: flex;
  justify-content: space-between;
  background: rgba(0, 0, 0, 0.25);
  padding: 10px 14px;
  border-radius: 10px;
}

.snap-metric {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.snap-lbl {
  font-size: 10px;
  color: var(--text-muted);
  text-transform: uppercase;
}

.snap-val {
  font-size: 14px;
  font-weight: 700;
}

.gauge-container {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.gauge-bar-bg {
  width: 100%;
  height: 8px;
  background: rgba(255, 255, 255, 0.08);
  border-radius: 9999px;
  overflow: hidden;
}

.gauge-bar-fill {
  height: 100%;
  border-radius: 9999px;
  transition: width 0.4s ease;
}

.fill-emerald {
  background: var(--grad-emerald, linear-gradient(90deg, #10b981 0%, #059669 100%));
  box-shadow: 0 0 8px rgba(16, 185, 129, 0.5);
}

.fill-amber {
  background: linear-gradient(90deg, #f59e0b 0%, #d97706 100%);
  box-shadow: 0 0 8px rgba(245, 158, 11, 0.5);
}

.fill-rose {
  background: linear-gradient(90deg, #f43f5e 0%, #e11d48 100%);
  box-shadow: 0 0 8px rgba(244, 63, 94, 0.5);
}

.gauge-labels {
  display: flex;
  justify-content: space-between;
  font-size: 10px;
  color: var(--text-muted);
  font-family: var(--font-mono);
}

.card-actions-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding-top: 6px;
  border-top: 1px solid rgba(255, 255, 255, 0.05);
}

.btn-action {
  flex: 1;
  padding: 6px 10px;
  font-size: 11px;
  font-weight: 600;
  border-radius: 8px;
  cursor: pointer;
  transition: all 0.2s ease;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border: 1px solid transparent;
}

.btn-inspect {
  background: rgba(6, 182, 212, 0.12);
  color: #38bdf8;
  border-color: rgba(6, 182, 212, 0.3);
}

.btn-inspect:hover {
  background: rgba(6, 182, 212, 0.25);
}

.btn-alert {
  background: rgba(245, 158, 11, 0.12);
  color: #fbbf24;
  border-color: rgba(245, 158, 11, 0.3);
}

.btn-alert:hover {
  background: rgba(245, 158, 11, 0.25);
}

.btn-delete {
  background: rgba(244, 63, 94, 0.12);
  color: #fb7185;
  border-color: rgba(244, 63, 94, 0.3);
}

.btn-delete:hover {
  background: rgba(244, 63, 94, 0.25);
}

/* Table cells */
.service-name-cell {
  display: flex;
  align-items: center;
  gap: 6px;
}

.service-icon { font-size: 14px; }
.service-text { color: #fff; font-size: 13px; }

.indicator-pill {
  font-size: 11px;
  background: rgba(6, 182, 212, 0.12);
  color: #38bdf8;
  border: 1px solid rgba(6, 182, 212, 0.25);
  padding: 2px 8px;
  border-radius: 6px;
}

.target-val { font-weight: 700; }
.window-badge { font-size: 11px; color: var(--text-secondary); }

.query-snippet {
  font-size: 11px;
  color: var(--text-muted);
  max-width: 260px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  display: block;
}

.threshold-badge {
  font-size: 11px;
  color: var(--accent-amber);
  background: rgba(245, 158, 11, 0.1);
  padding: 2px 6px;
  border-radius: 4px;
}

.table-actions-cell {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 6px;
}

.btn-icon-action {
  background: rgba(255, 255, 255, 0.05);
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 6px;
  padding: 4px 8px;
  cursor: pointer;
  transition: all 0.2s ease;
  font-size: 12px;
}

.btn-icon-action:hover {
  background: rgba(255, 255, 255, 0.15);
}

.btn-icon-warn:hover {
  background: rgba(245, 158, 11, 0.2);
  border-color: rgba(245, 158, 11, 0.4);
}

.btn-icon-del:hover {
  background: rgba(244, 63, 94, 0.2);
  border-color: rgba(244, 63, 94, 0.4);
}

/* Modals */
.modal-overlay {
  position: fixed;
  top: 0;
  left: 0;
  width: 100vw;
  height: 100vh;
  background: rgba(0, 0, 0, 0.75);
  backdrop-filter: blur(8px);
  z-index: 9999;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 20px;
}

.modal-card {
  background: #0f172a;
  border: 1px solid rgba(255, 255, 255, 0.12);
  border-radius: 18px;
  width: 100%;
  max-width: 620px;
  max-height: 90vh;
  overflow-y: auto;
  box-shadow: 0 20px 50px rgba(0, 0, 0, 0.6);
  display: flex;
  flex-direction: column;
}

.inspect-modal {
  max-width: 720px;
}

.modal-header {
  padding: 18px 24px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.modal-title-group {
  display: flex;
  align-items: center;
  gap: 10px;
}

.modal-icon { font-size: 20px; }

.modal-title {
  font-size: 16px;
  font-weight: 800;
  color: #fff;
}

.modal-subtitle {
  font-size: 12px;
  color: var(--text-muted);
}

.modal-close-btn {
  background: transparent;
  border: none;
  color: var(--text-muted);
  font-size: 18px;
  cursor: pointer;
  padding: 4px;
}

.modal-close-btn:hover { color: #fff; }

.modal-body {
  padding: 24px;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.form-group {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.form-row-2 {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
}

.form-label {
  font-size: 12px;
  font-weight: 600;
  color: var(--text-secondary);
}

.label-with-presets {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.preset-buttons {
  display: flex;
  gap: 4px;
}

.btn-preset {
  background: rgba(255, 255, 255, 0.06);
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 4px;
  padding: 2px 6px;
  font-size: 10px;
  font-family: var(--font-mono);
  color: var(--text-secondary);
  cursor: pointer;
}

.btn-preset:hover {
  background: rgba(6, 182, 212, 0.2);
  color: #38bdf8;
}

.input-glass {
  background: rgba(0, 0, 0, 0.35);
  border: 1px solid rgba(255, 255, 255, 0.12);
  border-radius: 10px;
  padding: 10px 14px;
  font-size: 13px;
  color: #fff;
  outline: none;
  transition: border-color 0.2s ease;
}

.input-glass:focus {
  border-color: var(--accent-cyan);
  box-shadow: 0 0 10px rgba(6, 182, 212, 0.25);
}

.query-textarea {
  resize: vertical;
  min-height: 80px;
  line-height: 1.4;
}

.field-hint {
  font-size: 11px;
  color: var(--text-muted);
}

.modal-footer {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 10px;
  margin-top: 8px;
}

/* Inspector styles */
.inspect-stats-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(130px, 1fr));
  gap: 10px;
}

.inspect-stat-card {
  background: rgba(0, 0, 0, 0.3);
  padding: 12px;
  border-radius: 10px;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.stat-lbl {
  font-size: 10px;
  color: var(--text-muted);
  text-transform: uppercase;
}

.stat-val {
  font-size: 15px;
  font-weight: 700;
}

.inspect-section {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.section-tag-label {
  font-size: 11px;
  font-weight: 700;
  color: var(--accent-cyan);
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.promql-box {
  background: rgba(0, 0, 0, 0.4);
  border: 1px solid rgba(6, 182, 212, 0.2);
  border-radius: 10px;
  padding: 12px 14px;
  font-size: 12px;
  color: #38bdf8;
  word-break: break-all;
}

.formula-box {
  background: rgba(0, 0, 0, 0.3);
  border-radius: 10px;
  padding: 12px 14px;
  display: flex;
  flex-direction: column;
  gap: 6px;
  font-size: 12px;
}

.formula-line {
  display: flex;
  justify-content: space-between;
  border-bottom: 1px solid rgba(255, 255, 255, 0.05);
  padding-bottom: 4px;
}

.formula-line:last-child {
  border-bottom: none;
  padding-bottom: 0;
}

.formula-term { color: var(--text-muted); }
.formula-calc { color: #fff; font-weight: 600; }

.inspect-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 11px;
  background: rgba(0, 0, 0, 0.25);
  border-radius: 10px;
  overflow: hidden;
}

.inspect-table th, .inspect-table td {
  padding: 8px 12px;
  text-align: left;
  border-bottom: 1px solid rgba(255, 255, 255, 0.05);
}

.inspect-table th {
  background: rgba(255, 255, 255, 0.04);
  color: var(--text-muted);
  text-transform: uppercase;
}

/* Utilities */
.font-mono { font-family: var(--font-mono, monospace); }
.text-emerald { color: var(--accent-emerald, #10b981); }
.text-amber { color: var(--accent-amber, #f59e0b); }
.text-rose { color: var(--accent-rose, #f43f5e); }
.text-cyan { color: var(--accent-cyan, #06b6d4); }
.text-muted { color: var(--text-muted, #94a3b8); }
.font-bold { font-weight: 700; }

.badge-cyan {
  background: rgba(6, 182, 212, 0.12);
  color: #38bdf8;
  border: 1px solid rgba(6, 182, 212, 0.25);
  padding: 2px 8px;
  border-radius: 6px;
  font-size: 11px;
}

.badge-amber {
  background: rgba(245, 158, 11, 0.12);
  color: #fbbf24;
  border: 1px solid rgba(245, 158, 11, 0.25);
  padding: 2px 8px;
  border-radius: 6px;
  font-size: 11px;
}

.badge-rose {
  background: rgba(244, 63, 94, 0.12);
  color: #fb7185;
  border: 1px solid rgba(244, 63, 94, 0.25);
  padding: 2px 8px;
  border-radius: 6px;
  font-size: 11px;
}

.badge-emerald {
  background: rgba(16, 185, 129, 0.12);
  color: #34d399;
  border: 1px solid rgba(16, 185, 129, 0.25);
  padding: 2px 8px;
  border-radius: 6px;
  font-size: 11px;
}

.btn-warning {
  background: linear-gradient(135deg, #f59e0b 0%, #d97706 100%);
  color: #000;
  font-weight: 700;
  border: none;
  padding: 8px 16px;
  border-radius: 10px;
  cursor: pointer;
}
</style>
