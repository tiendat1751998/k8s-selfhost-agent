<script setup lang="ts">
import type { SLODefinition, SLOSnapshot } from '../../api/compute'

const props = defineProps<{
  definitions: SLODefinition[]
  snapshots: SLOSnapshot[]
  formatPercent: (val?: number) => string
  getEffectiveBurnRate: (rate?: number) => number
  getBurnRateColor: (rate: number) => string
  getBudgetBarWidth: (budget?: number) => number
  getSnapshotForDef: (defId: string, serviceName: string) => SLOSnapshot | undefined
}>()

const emit = defineEmits<{
  (e: 'inspect', def: SLODefinition, snap?: SLOSnapshot): void
  (e: 'edit', def: SLODefinition): void
  (e: 'delete', defId: string, serviceName: string): void
}>()
</script>

<template>
  <div class="mobile-stream-container">
    <div class="mobile-stream-header">
      <span class="mobile-stream-title">📱 Touch-Optimized SLO Workload Stream</span>
      <span class="mobile-stream-count font-mono">{{ definitions.length }} Objectives</span>
    </div>

    <div v-if="definitions.length === 0" class="mobile-stream-empty">
      <span>No active SLO definitions configured.</span>
    </div>

    <div v-else class="mobile-cards-stream">
      <div 
        v-for="def in definitions" 
        :key="def.id" 
        class="mobile-stream-item glass-panel"
      >
        <!-- Left: Service identity & status dot -->
        <div class="mobile-item-left">
          <div class="mobile-item-title-row">
            <span 
              class="mobile-status-dot" 
              :class="getSnapshotForDef(def.id, def.service)?.budget_status || 'healthy'"
            ></span>
            <span class="mobile-service-name font-mono">{{ def.service }}</span>
          </div>
          <div class="mobile-item-sub font-mono">
            <span class="mobile-sli-tag">{{ def.indicator_type }}</span>
            <span class="mobile-window-tag">{{ def.window }}</span>
          </div>
        </div>

        <!-- Middle: Actual vs Target & Mini Gauge -->
        <div class="mobile-item-mid">
          <div class="mobile-metric-line font-mono">
            <span class="mobile-actual text-emerald">
              {{ formatPercent(getSnapshotForDef(def.id, def.service)?.actual || def.target) }}
            </span>
            <span class="mobile-slash">/</span>
            <span class="mobile-target text-muted">{{ formatPercent(def.target) }}</span>
          </div>
          <div class="mobile-mini-gauge">
            <div 
              class="mobile-gauge-fill" 
              :class="getSnapshotForDef(def.id, def.service)?.budget_status === 'critical' ? 'fill-rose' : getSnapshotForDef(def.id, def.service)?.budget_status === 'warning' ? 'fill-amber' : 'fill-emerald'"
              :style="{ width: `${getBudgetBarWidth(getSnapshotForDef(def.id, def.service)?.error_budget ?? 100)}%` }"
            ></div>
          </div>
          <div class="mobile-burn-val font-mono" :class="getBurnRateColor(getEffectiveBurnRate(getSnapshotForDef(def.id, def.service)?.burn_rate))">
            {{ getEffectiveBurnRate(getSnapshotForDef(def.id, def.service)?.burn_rate).toFixed(2) }}x burn
          </div>
        </div>

        <!-- Right: Touch Actions -->
        <div class="mobile-item-actions">
          <button 
            type="button"
            class="mobile-action-btn mobile-btn-inspect" 
            title="Inspect SLI"
            aria-label="Inspect SLI"
            @click="emit('inspect', def, getSnapshotForDef(def.id, def.service))"
          >
            <span>📈</span>
          </button>
          <button 
            type="button"
            class="mobile-action-btn mobile-btn-edit" 
            title="Edit SLO"
            aria-label="Edit SLO"
            @click="emit('edit', def)"
          >
            <span>✏️</span>
          </button>
          <button 
            type="button"
            class="mobile-action-btn mobile-btn-del" 
            title="Delete SLO"
            aria-label="Delete SLO"
            @click="emit('delete', def.id, def.service)"
          >
            <span>🗑️</span>
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
@import '../../assets/styles/views/slo.css';
</style>
