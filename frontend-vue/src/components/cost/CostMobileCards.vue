<template>
  <div class="cost-mobile-cards">
    <div v-if="loading" class="stream-status font-mono">
      <span class="spin-icon">⏳</span> Loading optimization findings...
    </div>
    <div v-else-if="wasteAlerts.length === 0" class="stream-empty glass-panel font-mono">
      <span class="empty-icon">✅</span>
      <p class="empty-text">No resource waste detected. Cluster requests are optimal.</p>
    </div>
    <div v-else class="cards-list">
      <div
        v-for="item in wasteAlerts"
        :key="item.id"
        class="cost-card glass-panel"
      >
        <!-- Top Row: Workload & Severity -->
        <div class="card-header-row">
          <div class="header-titles">
            <span class="workload-name font-mono" :title="item.resource">{{ item.resource }}</span>
            <span class="cluster-tag font-mono text-muted">@ {{ item.cluster || 'cluster' }}</span>
          </div>
          <span class="sev-pill font-mono" :class="`sev-${(item.severity || 'medium').toLowerCase()}`">
            {{ (item.severity || 'HIGH').toUpperCase() }}
          </span>
        </div>

        <!-- Mid Row: Namespace & Savings -->
        <div class="card-meta-row font-mono">
          <span class="ns-pill">📁 {{ item.namespace }}</span>
          <span class="savings-amount text-rose font-bold">${{ item.wasted_cost }}/mo idle</span>
        </div>

        <!-- Metric Waste Breakdown -->
        <div class="card-waste-metrics font-mono">
          <span class="metric-item">
            CPU Util: <strong :class="(item.cpu_util ?? 0) < 20 ? 'text-rose' : 'text-amber'">{{ item.cpu_util ?? 0 }}%</strong>
          </span>
          <span class="metric-sep">|</span>
          <span class="metric-item">
            RAM Util: <strong :class="(item.mem_util ?? 0) < 30 ? 'text-rose' : 'text-amber'">{{ item.mem_util ?? 0 }}%</strong>
          </span>
          <span class="metric-sep">|</span>
          <span class="metric-item text-muted">{{ formatWasteType(item.type) }}</span>
        </div>

        <!-- Action Row (32px touch button) -->
        <div class="card-action-row">
          <button
            type="button"
            class="btn-rightsize"
            @click="$emit('rightSize', item.id)"
          >
            <span>⚡ Right-Size</span>
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import type { ResourceWaste } from '../../api/governance'

defineProps<{
  wasteAlerts: ResourceWaste[]
  loading?: boolean
}>()

defineEmits<{
  (e: 'rightSize', id: string): void
}>()

function formatWasteType(t?: string): string {
  if (!t) return 'IDLE RES'
  return t.replace(/_/g, ' ').toUpperCase()
}
</script>

<style scoped>
@import '../../assets/styles/views/cost.css';
</style>
