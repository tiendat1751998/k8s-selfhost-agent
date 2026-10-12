<script setup lang="ts">
import { computed } from 'vue'
import type { DeploymentApp } from '../../api/compute'
import BaseIcon from '../ui/BaseIcon.vue'

interface Props {
  app: DeploymentApp
  weight: number
  actionLoading?: string | null
}

const props = defineProps<Props>()

const emit = defineEmits<{
  (e: 'update:weight', val: number): void
  (e: 'abortCanary', app: DeploymentApp): void
  (e: 'promoteCanary', app: DeploymentApp): void
}>()

const passingProbesText = computed(() => {
  if (props.app.replicas > 0 && props.app.readyReplicas !== undefined) {
    const pct = Math.round((props.app.readyReplicas / props.app.replicas) * 100)
    return `${pct}% Passing Probes`
  }
  if (props.app.status === 'healthy') {
    return '100% Passing Probes'
  }
  return '-- Unmeasured'
})

const errorBudgetText = computed(() => {
  const eb = (props.app as any).errorBudget ?? (props.app as any).error_budget
  if (typeof eb === 'number') {
    return `Error Budget: ${eb.toFixed(2)}%`
  }
  return 'Error Budget: --'
})
</script>

<template>
  <div class="canary-config-panel glass-panel">
    <div class="canary-meter-header">
      <div>
        <h4 class="strat-title">Canary Traffic Weight</h4>
        <p class="strat-desc">Route a subset of real user traffic to the canary container track.</p>
      </div>
      <span class="canary-weight-badge font-mono">{{ weight }}% Canary</span>
    </div>

    <!-- Visual Traffic Bar -->
    <div class="traffic-split-visual">
      <div class="traffic-stable-bar font-mono" :style="{ width: `${100 - weight}%` }">
        <span>Stable: {{ 100 - weight }}%</span>
      </div>
      <div class="traffic-canary-bar font-mono" :style="{ width: `${weight}%` }">
        <span v-if="weight >= 15">Canary: {{ weight }}%</span>
      </div>
    </div>

    <!-- Weight Slider -->
    <input
      :value="weight"
      type="range"
      min="0"
      max="100"
      step="5"
      class="canary-slider"
      @input="emit('update:weight', Number(($event.target as HTMLInputElement).value))"
    />

    <!-- Quick Presets -->
    <div class="canary-presets-row">
      <button type="button" class="preset-pill font-mono" @click="emit('update:weight', 5)">5% Smoke</button>
      <button type="button" class="preset-pill font-mono" @click="emit('update:weight', 10)">10% Initial</button>
      <button type="button" class="preset-pill font-mono" @click="emit('update:weight', 25)">25% Stage 1</button>
      <button type="button" class="preset-pill font-mono" @click="emit('update:weight', 50)">50% Half Fleet</button>
      <button type="button" class="preset-pill font-mono" @click="emit('update:weight', 100)">100% Full</button>
    </div>

    <!-- Track Versions Comparison -->
    <div class="tracks-comparison-grid font-mono">
      <div class="track-card track-stable">
        <span class="track-title">STABLE BASELINE ({{ 100 - weight }}%)</span>
        <span class="track-image">{{ app.image }}</span>
        <span class="track-status" :class="passingProbesText.includes('100%') ? 'text-emerald' : 'text-muted'">● {{ passingProbesText }}</span>
      </div>
      <div class="track-card track-canary">
        <span class="track-title">CANARY TRACK ({{ weight }}%)</span>
        <span class="track-image">{{ app.canaryVersion || `${app.image}-canary` }}</span>
        <span class="track-status" :class="errorBudgetText.includes('--') ? 'text-muted' : 'text-cyan'">● {{ errorBudgetText }}</span>
      </div>
    </div>

    <!-- Canary Promotion & Abort Actions -->
    <div class="canary-actions-bar">
      <button
        type="button"
        class="btn btn-secondary btn-sm"
        :disabled="actionLoading === 'abort'"
        @click="emit('abortCanary', app)"
      >
        <BaseIcon name="x" size="xs" /> <span>Abort Canary (0%)</span>
      </button>
      <button
        type="button"
        class="btn btn-primary btn-sm"
        :disabled="actionLoading === 'promote'"
        @click="emit('promoteCanary', app)"
      >
        <span>Promote Canary to 100%</span> <BaseIcon name="chevron-right" size="xs" />
      </button>
    </div>
  </div>
</template>

<style scoped>
@import '../../assets/styles/components/canary-modal.css';
</style>
