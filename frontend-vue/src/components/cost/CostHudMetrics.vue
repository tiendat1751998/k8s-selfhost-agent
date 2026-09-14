<script setup lang="ts">
import BaseIcon from '../ui/BaseIcon.vue'

defineProps<{
  totalMonthlyCost: number
  totalDailyCost: number
  wasteSaved: number
  totalWastedCost: number
  wastePercentage: number
  projectedCost: number
  spotRatio: number
  spotSavings: number
}>()
</script>

<template>
  <div class="cost-hud-ribbon" role="region" aria-label="FinOps Cost KPI Ribbon">
    <!-- Metric 1: Monthly Spend & Daily Run-rate -->
    <div class="hud-ribbon-card glass-panel">
      <div class="hud-card-top">
        <div class="hud-label-group">
          <BaseIcon name="dollar-sign" size="xs" class="hud-icon text-emerald" />
          <span class="hud-label">Monthly Spend</span>
        </div>
        <span class="hud-pill badge-emerald font-mono">RUN-RATE</span>
      </div>
      <div class="hud-card-bottom font-mono">
        <span class="hud-metric-val font-bold text-white">${{ totalMonthlyCost.toLocaleString() }}</span>
        <span class="hud-metric-sub text-muted">${{ totalDailyCost.toLocaleString() }}/d</span>
      </div>
    </div>

    <!-- Metric 2: Idle Resource Waste & Remediated -->
    <div class="hud-ribbon-card glass-panel">
      <div class="hud-card-top">
        <div class="hud-label-group">
          <BaseIcon name="trending-up" size="xs" class="hud-icon text-amber" />
          <span class="hud-label">Actionable Waste</span>
        </div>
        <span class="hud-pill font-mono" :class="totalWastedCost > 0 ? 'badge-rose' : 'badge-emerald'">
          {{ totalWastedCost > 0 ? `${wastePercentage}% IDLE` : 'OPTIMIZED' }}
        </span>
      </div>
      <div class="hud-card-bottom font-mono">
        <span class="hud-metric-val font-bold" :class="totalWastedCost > 0 ? 'text-rose' : 'text-emerald'">
          ${{ totalWastedCost.toLocaleString() }}
        </span>
        <span class="hud-metric-sub text-muted">${{ wasteSaved.toLocaleString() }} saved</span>
      </div>
    </div>

    <!-- Metric 3: Projected Trajectory Forecast -->
    <div class="hud-ribbon-card glass-panel">
      <div class="hud-card-top">
        <div class="hud-label-group">
          <BaseIcon name="activity" size="xs" class="hud-icon text-cyan" />
          <span class="hud-label">Monthly Forecast</span>
        </div>
        <span class="hud-pill badge-cyan font-mono">+4.2% P95</span>
      </div>
      <div class="hud-card-bottom font-mono">
        <span class="hud-metric-val font-bold text-white">${{ projectedCost.toLocaleString() }}</span>
        <span class="hud-metric-sub text-cyan">run-rate model</span>
      </div>
    </div>

    <!-- Metric 4: Spot Instance Utilization & Savings -->
    <div class="hud-ribbon-card glass-panel">
      <div class="hud-card-top">
        <div class="hud-label-group">
          <BaseIcon name="zap" size="xs" class="hud-icon text-violet" />
          <span class="hud-label">Spot Node Ratio</span>
        </div>
        <span class="hud-pill badge-violet font-mono">SPOT SAVINGS</span>
      </div>
      <div class="hud-card-bottom font-mono">
        <span class="hud-metric-val font-bold text-violet">{{ spotRatio }}%</span>
        <span class="hud-metric-sub text-muted">+${{ spotSavings.toLocaleString() }}/mo saved</span>
      </div>
    </div>
  </div>
</template>

