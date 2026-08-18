<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import MetricCard from '../components/ui/MetricCard.vue'
import DataTable, { type Column } from '../components/ui/DataTable.vue'
import StatusBadge from '../components/ui/StatusBadge.vue'
import {
  sloApi,
  type SLODefinition,
  type SLOSnapshot
} from '../api/compute'

const loading = ref(false)
const error = ref<string | null>(null)

const definitions = ref<SLODefinition[]>([])
const snapshots = ref<SLOSnapshot[]>([])

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

const totalSLOs = computed(() => definitions.value.length)
const healthySLOs = computed(() => snapshots.value.filter(s => s.budget_status === 'healthy').length)
const warningSLOs = computed(() => snapshots.value.filter(s => s.budget_status === 'warning').length)
const criticalSLOs = computed(() => snapshots.value.filter(s => s.budget_status === 'critical').length)

// Average burn rate
const avgBurnRate = computed(() => {
  if (snapshots.value.length === 0) return '1.0x'
  const total = snapshots.value.reduce((acc, s) => acc + (s.burn_rate || 0), 0)
  return `${(total / snapshots.value.length).toFixed(2)}x`
})

const sloColumns: Column<SLODefinition>[] = [
  { key: 'service', label: 'Service / Workload', sortable: true },
  { key: 'indicator_type', label: 'Indicator Type (SLI)', width: '180px', sortable: true },
  { key: 'target', label: 'Target Objective', width: '150px', sortable: true },
  { key: 'window', label: 'Rolling Window', width: '140px', sortable: true },
  { key: 'created_at', label: 'Defined At', width: '160px', sortable: true },
]


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
          Continuous error budget depletion tracking, multi-window burn rate indicators, and automated fast/slow breach alerting.
        </p>
      </div>

      <div class="header-actions">
        <button class="btn btn-secondary" :disabled="loading" @click="fetchSLOData">
          <span>{{ loading ? '⏳ Querying...' : '🔄 Refresh' }}</span>
        </button>
      </div>
    </div>

    <!-- Error Alert -->
    <div v-if="error" class="error-banner">
      <span class="error-icon">⚠️</span>
      <span>{{ error }}</span>
    </div>

    <!-- Metric HUD -->
    <div class="metrics-grid">
      <MetricCard
        title="Active SLOs"
        :value="totalSLOs"
        subtitle="Services tracked against target SLIs"
        icon="🎯"
        badge="OBJECTIVES"
        badge-color="cyan"
      />
      <MetricCard
        title="Healthy Error Budgets"
        :value="`${healthySLOs}/${snapshots.length || totalSLOs}`"
        subtitle="Services with >20% remaining budget"
        icon="🛡️"
        badge="HEALTHY"
        badge-color="emerald"
        trend="Within Budget"
        trend-type="positive"
      />
      <MetricCard
        title="Budget Warnings"
        :value="warningSLOs"
        subtitle="Error budget consumption > 80%"
        icon="⚠️"
        :badge="warningSLOs > 0 ? 'WARNING' : 'ZERO'"
        :badge-color="warningSLOs > 0 ? 'amber' : 'emerald'"
        :trend="warningSLOs > 0 ? 'Elevated Failure Rate' : 'Optimal Traffic'"
        :trend-type="warningSLOs > 0 ? 'negative' : 'positive'"
      />
      <MetricCard
        title="Average Burn Rate"
        :value="avgBurnRate"
        subtitle="1.0x = exact budget consumption over window"
        icon="🔥"
        :badge="criticalSLOs > 0 ? 'EXHAUSTING' : 'NOMINAL'"
        :badge-color="criticalSLOs > 0 ? 'rose' : 'emerald'"
        :trend="criticalSLOs > 0 ? 'Fast Burn Alert Active' : 'Normal Rate'"
        :trend-type="criticalSLOs > 0 ? 'negative' : 'positive'"
      />
    </div>

    <!-- Live Error Budget & Burn Rate Cards -->
    <div class="section-box glass-panel">
      <div class="box-header">
        <div>
          <h2 class="box-title">Real-Time Error Budget Gauges & Burn Rates</h2>
          <p class="box-subtitle">Instant health compliance calculated from Prometheus / OpenTelemetry telemetry streams</p>
        </div>
      </div>

      <div v-if="snapshots.length === 0" class="empty-state">
        <span>No active SLO snapshots recorded. Add SLO definitions or verify Prometheus telemetry adapter.</span>
      </div>

      <div v-else class="snapshots-grid">
        <div v-for="snap in snapshots" :key="snap.id" class="snapshot-card glass-panel">
          <div class="snap-top">
            <div class="snap-title-group">
              <span class="snap-service-name">{{ snap.service }}</span>
              <span class="snap-target-badge font-mono">Target: {{ (snap.target * 100).toFixed(2) }}%</span>
            </div>
            <StatusBadge :status="snap.budget_status" size="sm" />
          </div>

          <div class="snap-metrics-row">
            <div class="snap-metric">
              <span class="snap-lbl">Actual SLI</span>
              <span class="snap-val font-mono" :class="snap.actual >= snap.target ? 'text-emerald' : 'text-rose'">
                {{ (snap.actual * 100).toFixed(2) }}%
              </span>
            </div>
            <div class="snap-metric">
              <span class="snap-lbl">Remaining Budget</span>
              <span class="snap-val font-mono" :class="snap.error_budget > 20 ? 'text-emerald' : 'text-amber'">
                {{ (snap.error_budget).toFixed(1) }}%
              </span>
            </div>
            <div class="snap-metric">
              <span class="snap-lbl">Burn Rate</span>
              <span class="snap-val font-mono" :class="getBurnRateColor(snap.burn_rate)">
                {{ (snap.burn_rate).toFixed(2) }}x
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
        </div>
      </div>
    </div>

    <!-- SLO Definitions Catalog Table -->
    <div class="section-box glass-panel table-box">
      <div class="box-header" style="padding: 18px 22px; border-bottom: 1px solid var(--border-subtle);">
        <div>
          <h2 class="box-title">SLO Target Definitions Catalog</h2>
          <p class="box-subtitle">Configured Service Level Objectives with sliding compliance windows</p>
        </div>
      </div>

      <DataTable
        :columns="sloColumns"
        :data="definitions"
        :loading="loading"
        :error="error"
        empty-message="No SLO definitions configured."
        searchable
        search-placeholder="Search SLO definitions by service..."
      >
        <template #cell-indicator_type="{ row }">
          <span class="indicator-pill font-mono">{{ row.indicator_type }}</span>
        </template>

        <template #cell-target="{ row }">
          <span class="target-val font-mono text-emerald">{{ (row.target * 100).toFixed(2) }}%</span>
        </template>

        <template #cell-window="{ row }">
          <span class="window-badge font-mono">{{ row.window }}</span>
        </template>

        <template #cell-created_at="{ row }">
          <span class="font-mono text-muted">{{ formatDate(row.created_at) }}</span>
        </template>
      </DataTable>
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
  grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
  gap: 16px;
}

.snapshot-card {
  padding: 16px;
  border-radius: 14px;
  display: flex;
  flex-direction: column;
  gap: 12px;
  background: rgba(11, 15, 25, 0.6);
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
}

.snap-service-name {
  font-size: 14px;
  font-weight: 700;
  color: #fff;
}

.snap-target-badge {
  font-size: 10px;
  padding: 2px 6px;
  background: rgba(255, 255, 255, 0.05);
  border-radius: 4px;
  color: var(--text-muted);
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
  transition: width 0.3s ease;
}

.fill-emerald {
  background: var(--grad-emerald);
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

.indicator-pill {
  font-size: 11px;
  background: rgba(6, 182, 212, 0.12);
  color: #38bdf8;
  border: 1px solid rgba(6, 182, 212, 0.25);
  padding: 2px 8px;
  border-radius: 6px;
}

.target-val {
  font-weight: 700;
}

.window-badge {
  font-size: 11px;
  color: var(--text-secondary);
}

.font-mono { font-family: var(--font-mono); }
.text-emerald { color: var(--accent-emerald); }
.text-amber { color: var(--accent-amber); }
.text-rose { color: var(--accent-rose); }
.text-muted { color: var(--text-muted); }
</style>
