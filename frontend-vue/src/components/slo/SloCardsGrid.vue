<script setup lang="ts">
import StatusBadge from '../ui/StatusBadge.vue'
import type { SLODefinition, SLOSnapshot } from '../../api/compute'

const props = defineProps<{
  definitions: SLODefinition[]
  snapshots: SLOSnapshot[]
  selectedWindowFilter: string
  actionInProgress?: boolean
}>()

const emit = defineEmits<{
  createSlo: []
  inspect: [payload: { def?: SLODefinition; snap?: SLOSnapshot }]
  triggerAlert: [sloId: string, service: string]
  deleteSlo: [sloId: string, service: string]
}>()

function getDefForSnapshot(snap: SLOSnapshot): SLODefinition | undefined {
  return props.definitions.find(d => d.id === snap.slo_id || d.service === snap.service)
}

function formatPercent(val?: number): string {
  if (val === undefined || val === null || isNaN(val)) return '0.00%'
  const pct = val > 1 ? val : val * 100
  return `${pct.toFixed(2)}%`
}

function getEffectiveBurnRate(rawRate?: number): number {
  if (rawRate === undefined || rawRate === null || isNaN(rawRate)) return 0
  return rawRate
}

function getBurnRateColor(rate: number): string {
  if (rate <= 1.0) return 'text-emerald'
  if (rate <= 2.5) return 'text-amber'
  return 'text-rose'
}

function getBurnRateLabel(rate: number): string {
  if (rate <= 1.0) return 'Nominal'
  if (rate <= 2.5) return 'Elevated'
  return 'Fast Burn'
}

function getBudgetBarWidth(budget?: number): number {
  if (budget === undefined || budget === null || isNaN(budget)) return 0
  return Math.max(0, Math.min(100, budget))
}
</script>

<template>
  <div class="section-box glass-panel">
    <!-- Section Header -->
    <div class="box-header">
      <div>
        <h2 class="box-title">Real-Time Error Budget Gauges & Burn Rates</h2>
        <p class="box-subtitle">Live compliance calculated from active Prometheus & OpenTelemetry telemetry streams</p>
      </div>
      <div v-if="definitions.length > 0" class="header-badges">
        <span class="badge badge-cyan">{{ snapshots.length }} Live Services</span>
      </div>
    </div>

    <!-- Empty State when 0 SLOs defined -->
    <div v-if="definitions.length === 0" class="slo-empty-state">
      <div class="empty-icon-wrap">
        <span class="empty-hero-icon">🎯</span>
      </div>
      <h3 class="empty-headline">No Service Level Objectives (SLOs) Defined</h3>
      <p class="empty-explanation">
        Service Level Objectives (SLOs) establish target reliability thresholds using Service Level Indicators (SLIs). 
        The <strong>Error Budget</strong> represents the allowable margin for unreliability (<code>100% - Target</code>). 
        When errors consume budget faster than scheduled (<code>Burn Rate &gt; 1.0x</code>), multi-window burn rate alerts protect user experience.
      </p>
      <div class="empty-sre-features">
        <div class="sre-pill"><span class="sre-icon">📊</span><span class="sre-text">Google SRE Error Budgeting</span></div>
        <div class="sre-pill"><span class="sre-icon">🔥</span><span class="sre-text">Multi-Window Burn Rate Alerts</span></div>
        <div class="sre-pill"><span class="sre-icon">⚡</span><span class="sre-text">PromQL Telemetry SLI Queries</span></div>
      </div>
      <button class="btn btn-primary create-first-btn" @click="$emit('createSlo')">
        <span>➕ Create First SLO</span>
      </button>
    </div>

    <!-- Active SLO Cards Grid -->
    <div v-else class="snapshots-grid">
      <div v-for="snap in snapshots" :key="snap.id" class="snapshot-card glass-panel">
        <!-- Card Top: Service Name & Target -->
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

        <!-- Key SLI Metrics Row -->
        <div class="snap-metrics-row">
          <div class="snap-metric">
            <span class="snap-lbl">Target SLI</span>
            <span class="snap-val font-mono text-muted">{{ formatPercent(snap.target) }}</span>
          </div>
          <div class="snap-metric">
            <span class="snap-lbl">Actual SLI</span>
            <span class="snap-val font-mono" :class="snap.actual >= snap.target ? 'text-emerald' : 'text-rose'">
              {{ formatPercent(snap.actual) }}
            </span>
          </div>
          <div class="snap-metric">
            <span class="snap-lbl">Remaining Budget</span>
            <span class="snap-val font-mono" :class="snap.error_budget > 20 ? 'text-emerald' : snap.error_budget > 0 ? 'text-amber' : 'text-rose'">
              {{ snap.error_budget.toFixed(1) }}%
            </span>
          </div>
          <div class="snap-metric">
            <span class="snap-lbl">{{ selectedWindowFilter }} Burn</span>
            <span class="snap-val font-mono" :class="getBurnRateColor(getEffectiveBurnRate(snap.burn_rate))">
              {{ getEffectiveBurnRate(snap.burn_rate).toFixed(2) }}x
              <span class="burn-rate-sub">({{ getBurnRateLabel(getEffectiveBurnRate(snap.burn_rate)) }})</span>
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
          <div class="gauge-labels font-mono">
            <span>0% Exhausted</span>
            <span>{{ snap.error_budget.toFixed(1) }}% Remaining</span>
            <span>100% Intact</span>
          </div>
        </div>

        <!-- Card Actions Footer: Standard 30-32px Buttons -->
        <div class="card-actions-row">
          <button 
            class="btn btn-secondary btn-card-action" 
            title="Inspect SLI Telemetry" 
            @click="$emit('inspect', { snap })"
          >
            <span>🔍 Inspect</span>
          </button>
          <button 
            class="btn btn-secondary btn-card-action" 
            title="Simulate Burn Rate Alert"
            :disabled="actionInProgress"
            @click="$emit('triggerAlert', snap.slo_id, snap.service)"
          >
            <span>⚡ Simulate Burn</span>
          </button>
          <button 
            class="btn btn-danger btn-card-action" 
            title="Delete SLO"
            :disabled="actionInProgress"
            @click="$emit('deleteSlo', snap.slo_id, snap.service)"
          >
            <span>🗑️ Delete</span>
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.section-box {
  border-radius: 12px;
  background: var(--bg-card);
  border: 1px solid var(--border-subtle);
  overflow: hidden;
}

.box-header {
  padding: 16px 20px;
  border-bottom: 1px solid var(--border-subtle);
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
}

.box-title { font-size: 16px; font-weight: 700; color: #fff; letter-spacing: -0.01em; }
.box-subtitle { font-size: 12px; color: var(--text-secondary); margin-top: 2px; }

/* Empty State */
.slo-empty-state {
  padding: 48px 24px;
  display: flex;
  flex-direction: column;
  align-items: center;
  text-align: center;
  max-width: 680px;
  margin: 0 auto;
}
.empty-icon-wrap {
  width: 52px; height: 52px; border-radius: 14px;
  background: rgba(37, 99, 235, 0.12); border: 1px solid rgba(59, 130, 246, 0.25);
  display: flex; align-items: center; justify-content: center; margin-bottom: 14px;
}
.empty-hero-icon { font-size: 26px; }
.empty-headline { font-size: 17px; font-weight: 700; color: #fff; margin-bottom: 8px; }
.empty-explanation { font-size: 13px; color: var(--text-secondary); line-height: 1.5; margin-bottom: 18px; }
.empty-explanation code { background: rgba(0, 0, 0, 0.3); padding: 2px 6px; border-radius: 4px; font-family: var(--font-mono); color: #93c5fd; }
.empty-sre-features { display: flex; gap: 8px; flex-wrap: wrap; justify-content: center; margin-bottom: 20px; }
.sre-pill {
  display: inline-flex; align-items: center; gap: 6px; padding: 5px 10px;
  border-radius: 8px; background: #0b0f19; border: 1px solid var(--border-subtle);
  font-size: 12px; color: var(--text-secondary);
}
.sre-icon { font-size: 13px; }
.create-first-btn { padding: 9px 18px; font-size: 13px; }

/* Snapshots Grid */
.snapshots-grid {
  padding: 16px;
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(340px, 1fr));
  gap: 14px;
}

.snapshot-card {
  padding: 14px;
  border-radius: 10px;
  background: rgba(11, 15, 25, 0.5);
  border: 1px solid var(--border-subtle);
  display: flex;
  flex-direction: column;
  gap: 10px;
  transition: border-color 0.15s ease, background 0.15s ease;
}
.snapshot-card:hover { background: #161f30; border-color: var(--border-medium); }

.snap-top { display: flex; align-items: center; justify-content: space-between; gap: 8px; }
.snap-title-group { display: flex; align-items: center; gap: 6px; flex-wrap: wrap; }
.snap-icon { font-size: 15px; }
.snap-service-name { font-size: 13.5px; font-weight: 700; color: #fff; }
.snap-target-badge { font-size: 10.5px; color: var(--text-muted); background: rgba(255, 255, 255, 0.05); padding: 2px 6px; border-radius: 4px; }

.snap-query-bar {
  display: flex; align-items: center; gap: 6px; background: #0b0f19;
  padding: 5px 8px; border-radius: 6px; border: 1px solid var(--border-subtle); overflow: hidden;
}
.indicator-pill {
  font-size: 9.5px; padding: 2px 5px; border-radius: 4px;
  background: rgba(59, 130, 246, 0.12); color: #60a5fa; text-transform: uppercase; font-weight: 700; white-space: nowrap;
}
.query-code { font-size: 10.5px; color: var(--text-muted); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }

.snap-metrics-row {
  display: grid; grid-template-columns: repeat(4, 1fr); gap: 6px;
  background: rgba(11, 15, 25, 0.4); padding: 7px 8px; border-radius: 6px;
}
.snap-metric { display: flex; flex-direction: column; gap: 2px; }
.snap-lbl { font-size: 9.5px; color: var(--text-muted); text-transform: uppercase; letter-spacing: 0.02em; }
.snap-val { font-size: 12.5px; font-weight: 700; }
.burn-rate-sub { font-size: 9.5px; font-weight: 400; margin-left: 2px; }

/* Gauge Container */
.gauge-container { display: flex; flex-direction: column; gap: 3px; }
.gauge-bar-bg { width: 100%; height: 5px; background: #0b0f19; border-radius: 9999px; overflow: hidden; }
.gauge-bar-fill { height: 100%; border-radius: 9999px; transition: width 0.3s ease; }
.fill-emerald { background: #10b981; }
.fill-amber { background: #f59e0b; }
.fill-rose { background: #f43f5e; }
.gauge-labels { display: flex; justify-content: space-between; font-size: 9px; color: var(--text-muted); }

/* Card Actions: Standard 30-32px Heights */
.card-actions-row {
  display: flex;
  align-items: center;
  gap: 8px;
  border-top: 1px solid var(--border-subtle);
  padding-top: 8px;
}

.btn-card-action {
  height: 32px;
  min-height: 32px;
  padding: 0 10px;
  font-size: 11.5px;
  font-weight: 600;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 6px;
  line-height: 1;
  flex: 1;
  white-space: nowrap;
}

/* Mobile Responsive Overrides */
@media (max-width: 640px) {
  .box-header { padding: 10px 12px; }
  .box-subtitle { display: none; }
  .snapshots-grid {
    grid-template-columns: 1fr;
    padding: 6px;
    gap: 8px;
  }
  .snapshot-card {
    padding: 10px 12px;
    gap: 7px;
  }
  .snap-metrics-row {
    padding: 5px 7px;
    gap: 4px;
  }
  .snap-val {
    font-size: 11.5px;
  }
  .card-actions-row {
    padding-top: 6px;
    gap: 6px;
  }
  .btn-card-action {
    height: 30px;
    min-height: 30px;
    font-size: 11px;
    padding: 0 4px;
  }
}

.text-emerald { color: #10b981; }
.text-amber { color: #f59e0b; }
.text-rose { color: #f43f5e; }
.text-muted { color: var(--text-muted); }
.font-mono { font-family: var(--font-mono); }
</style>