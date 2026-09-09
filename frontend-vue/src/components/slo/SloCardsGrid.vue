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
@import '../../assets/styles/views/slo.css';
</style>
