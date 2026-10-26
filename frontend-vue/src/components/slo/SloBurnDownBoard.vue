<script setup lang="ts">
import BaseIcon from '../ui/BaseIcon.vue'
import ActionDropdown, { type ActionItem } from '../ui/ActionDropdown.vue'
import type { EnrichedSLOItem, FleetReliabilityPulse } from '../../composables/useSLOMonitor'
import type { SLODefinition, SLOSnapshot } from '../../api/compute'

const props = defineProps<{
  items: EnrichedSLOItem[]
  pulse: FleetReliabilityPulse
  loading: boolean
  error: string | null
}>()

const emit = defineEmits<{
  (e: 'inspect', def: SLODefinition, snap?: SLOSnapshot): void
  (e: 'triggerAlert', id: string, service: string): void
  (e: 'edit', def: SLODefinition): void
  (e: 'delete', id: string, service: string): void
  (e: 'create'): void
}>()

function getCardActions(_item: EnrichedSLOItem): ActionItem[] {
  return [
    { id: 'edit', label: 'Edit Target Objective', icon: 'edit' },
    { id: 'sep', label: '', separator: true },
    { id: 'delete', label: 'Delete SLO Target', icon: 'trash', variant: 'danger' },
  ]
}

function handleCardAction(actionId: string, item: EnrichedSLOItem) {
  if (actionId === 'edit') emit('edit', item.def)
  else if (actionId === 'delete') emit('delete', item.id, item.service)
}

function formatIndicatorType(type: string): string {
  if (!type) return 'AVAILABILITY'
  return type.replace(/_/g, ' ').toUpperCase()
}

function getBurnVelocityInfo(item: EnrichedSLOItem): { label: string; cls: string } {
  if (item.isFastBurn || item.burn_rate >= 14.4) {
    return { label: `${item.burn_rate.toFixed(1)}x Fast Burn`, cls: 'pill-burn-fast' }
  }
  if (item.burn_rate > 1.0) {
    return { label: `${item.burn_rate.toFixed(1)}x Elevated`, cls: 'pill-burn-elevated' }
  }
  return { label: `${item.burn_rate.toFixed(1)}x Normal`, cls: 'pill-burn-normal' }
}

function getSegmentClass(segIndex: number, errorBudget: number): string {
  const segThreshold = segIndex * 5
  if (errorBudget < segThreshold) return 'seg-empty'
  if (errorBudget >= 50) return 'seg-filled-online'
  if (errorBudget >= 20) return 'seg-filled-warning'
  return 'seg-filled-offline'
}
</script>

<template>
  <div class="slo-burndown-board animate-fade-in">
    <!-- Top Fleet Reliability Pulse Strip (4 KPIs Desktop, 2x2 Micro-Grid on < 640px Mobile) -->
    <div class="fleet-pulse-strip" role="region" aria-label="Fleet Reliability Pulse">
      <div class="pulse-card">
        <div class="pulse-card-header">
          <div class="pulse-card-title-wrap">
            <BaseIcon name="activity" size="xs" class="pulse-icon text-online" />
            <span class="pulse-card-title">Avg Error Budget</span>
          </div>
          <span class="pulse-badge font-mono" :class="pulse.avgBudgetRemainingPct >= 50 ? 'badge-online' : pulse.avgBudgetRemainingPct >= 20 ? 'badge-warning' : 'badge-offline'">
            {{ pulse.avgBudgetRemainingPct.toFixed(1) }}%
          </span>
        </div>
        <div class="pulse-metric-row">
          <span class="pulse-big-val font-mono">{{ pulse.avgBudgetRemainingPct.toFixed(1) }}%</span>
          <span class="pulse-sub font-mono">Fleet Margin</span>
        </div>
        <div class="pulse-gauge-track" aria-hidden="true">
          <div class="pulse-gauge-fill" :class="pulse.avgBudgetRemainingPct >= 50 ? 'fill-online' : pulse.avgBudgetRemainingPct >= 20 ? 'fill-warning' : 'fill-offline'" :style="{ width: `${Math.min(100, Math.max(0, pulse.avgBudgetRemainingPct))}%` }"></div>
        </div>
      </div>

      <div class="pulse-card">
        <div class="pulse-card-header">
          <div class="pulse-card-title-wrap">
            <BaseIcon name="flame" size="xs" class="pulse-icon" :class="pulse.fastBurnCount > 0 ? 'text-offline' : 'text-online'" />
            <span class="pulse-card-title">Fast-Burn Alerts</span>
          </div>
          <span class="pulse-badge font-mono" :class="pulse.fastBurnCount > 0 ? 'badge-offline' : 'badge-online'">
            {{ pulse.fastBurnCount > 0 ? 'Active Burn' : 'Nominal' }}
          </span>
        </div>
        <div class="pulse-metric-row">
          <span class="pulse-big-val font-mono" :class="pulse.fastBurnCount > 0 ? 'text-offline' : 'text-primary'">{{ pulse.fastBurnCount }}</span>
          <span class="pulse-sub font-mono">≥ Alert Threshold</span>
        </div>
      </div>

      <div class="pulse-card">
        <div class="pulse-card-header">
          <div class="pulse-card-title-wrap">
            <BaseIcon name="alert-triangle" size="xs" class="pulse-icon" :class="pulse.exhaustedCount > 0 ? 'text-offline' : 'text-online'" />
            <span class="pulse-card-title">Depleted Budgets</span>
          </div>
          <span class="pulse-badge font-mono" :class="pulse.exhaustedCount > 0 ? 'badge-offline' : 'badge-online'">
            {{ pulse.exhaustedCount > 0 ? 'Breached' : '0 Exhausted' }}
          </span>
        </div>
        <div class="pulse-metric-row">
          <span class="pulse-big-val font-mono" :class="pulse.exhaustedCount > 0 ? 'text-offline' : 'text-primary'">{{ pulse.exhaustedCount }}</span>
          <span class="pulse-sub font-mono">≤ 0% Margin Left</span>
        </div>
      </div>

      <div class="pulse-card">
        <div class="pulse-card-header">
          <div class="pulse-card-title-wrap">
            <BaseIcon name="target" size="xs" class="pulse-icon text-primary" />
            <span class="pulse-card-title">Monitored Targets</span>
          </div>
          <span class="pulse-badge font-mono badge-primary">{{ pulse.monitoredCount }} Targets</span>
        </div>
        <div class="pulse-metric-row">
          <span class="pulse-big-val font-mono">{{ pulse.monitoredCount }}</span>
          <span class="pulse-sub font-mono">{{ pulse.healthyCount }} Healthy · {{ pulse.warningCount }} Warn</span>
        </div>
      </div>
    </div>

    <!-- Error State -->
    <div v-if="error" class="board-error-panel" role="alert">
      <BaseIcon name="alert-triangle" size="sm" class="text-offline" />
      <span class="error-text">{{ error }}</span>
    </div>

    <!-- Empty State -->
    <div v-else-if="!loading && items.length === 0" class="board-empty-panel">
      <div class="empty-icon-wrap">
        <BaseIcon name="target" size="xl" class="empty-hero-icon" />
      </div>
      <h3 class="empty-headline">No SLO Targets Configured</h3>
      <p class="empty-explanation">
        Deploy reliability objectives to monitor real-time error budget burn rates and safeguard user SLAs before incidents breach contracts.
      </p>
      <button type="button" class="btn btn-primary" @click="emit('create')">
        <BaseIcon name="plus" size="xs" />
        <span>Add Target</span>
      </button>
    </div>

    <!-- Service Reliability Cards Grid -->
    <div v-else class="slo-cards-grid">
      <div
        v-for="item in items"
        :key="item.id"
        class="slo-card"
        :class="{ 'card-urgent': item.isFastBurn || item.isExhausted }"
      >
        <!-- Card Header: Service name + Indicator type badge + Rolling Window pill + Status dot -->
        <div class="card-header-row">
          <div class="card-identity">
            <span class="card-status-dot" :class="`dot-${item.budget_status}`" :title="`Status: ${item.budget_status}`"></span>
            <span class="card-service-name font-mono" :title="item.service">{{ item.service }}</span>
          </div>
          <div class="card-tags-group">
            <span class="card-sli-badge font-mono">{{ formatIndicatorType(item.indicator_type) }}</span>
            <span class="card-window-pill font-mono">{{ item.window }}</span>
          </div>
        </div>

        <!-- Hero Metric Comparison: Current SLI vs Objective Target & Burn-Rate Velocity Pill -->
        <div class="card-hero-metric">
          <div class="hero-sli-block">
            <span class="hero-label">CURRENT SLI</span>
            <div class="hero-val-line">
              <span class="hero-sli-val font-mono" :class="item.actual >= item.target ? 'text-online' : 'text-offline'">
                {{ item.actual.toFixed(2) }}%
              </span>
              <span class="hero-sli-target font-mono text-muted">
                Target {{ item.target.toFixed(2) }}%
              </span>
            </div>
          </div>
          <div class="hero-velocity-block">
            <span class="velocity-pill font-mono" :class="getBurnVelocityInfo(item).cls">
              {{ getBurnVelocityInfo(item).label }}
            </span>
          </div>
        </div>

        <!-- Segmented Error Budget Remaining Meter (0% – 100%) -->
        <div class="card-budget-section">
          <div class="budget-header-line">
            <span class="budget-title">Error Budget Remaining</span>
            <span class="budget-pct font-mono" :class="item.error_budget >= 50 ? 'text-online' : item.error_budget >= 20 ? 'text-warning' : 'text-offline'">
              {{ item.error_budget.toFixed(1) }}%
            </span>
          </div>
          <div class="segmented-track" role="progressbar" :aria-valuenow="item.error_budget" aria-valuemin="0" aria-valuemax="100" aria-label="Error Budget 20 Segment Meter">
            <span
              v-for="seg in 20"
              :key="seg"
              class="meter-seg"
              :class="getSegmentClass(seg, item.error_budget)"
            ></span>
          </div>
          <div class="budget-ticks-line font-mono text-muted">
            <span>0% Depleted</span>
            <span>Target Boundary</span>
            <span>100% Intact</span>
          </div>
        </div>

        <!-- Clean Footer (PromQL Hidden Behind Inspector, Window/Threshold meta, At most 2 inline actions + kebab) -->
        <div class="card-footer-row">
          <div class="footer-meta-block font-mono text-muted">
            <span>Alert Rate: {{ item.alert_threshold }}x</span>
            <span class="meta-dot">·</span>
            <span>{{ item.window }}</span>
          </div>
          <div class="footer-actions-block">
            <button
              type="button"
              class="card-btn card-btn-secondary"
              title="Inspect SLI Telemetry & History"
              aria-label="Inspect SLI Telemetry"
              @click="emit('inspect', item.def, item.snap)"
            >
              <BaseIcon name="trending-up" size="xs" />
              <span>Inspect</span>
            </button>
            <button
              type="button"
              class="card-btn card-btn-secondary"
              title="Test Burn Alert Trigger"
              aria-label="Test Burn Alert"
              @click="emit('triggerAlert', item.id, item.service)"
            >
              <BaseIcon name="alert-triangle" size="xs" />
              <span>Test Alert</span>
            </button>
            <ActionDropdown
              :items="getCardActions(item)"
              size="xs"
              align="right"
              aria-label="Target Actions Menu"
              @select="handleCardAction($event, item)"
            />
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
@import '../../assets/styles/views/slo-board.css';
</style>
