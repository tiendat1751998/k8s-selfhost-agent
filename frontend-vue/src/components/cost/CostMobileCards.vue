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
.cost-mobile-cards { display: flex; flex-direction: column; gap: 8px; width: 100%; }
.stream-status, .stream-empty { padding: 20px 16px; text-align: center; font-size: 12px; color: var(--text-muted); }
.cards-list { display: flex; flex-direction: column; gap: 8px; }
.cost-card {
  padding: 10px 12px; border-radius: 10px; background: rgba(15, 23, 42, 0.65);
  border: 1px solid rgba(255, 255, 255, 0.08); display: flex; flex-direction: column; gap: 6px;
}
.cost-card:hover { border-color: rgba(244, 63, 94, 0.3); }
.card-header-row { display: flex; align-items: flex-start; justify-content: space-between; gap: 8px; }
.header-titles { display: flex; align-items: baseline; gap: 6px; overflow: hidden; flex: 1; min-width: 0; }
.workload-name { font-size: 13px; font-weight: 700; color: #fff; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.cluster-tag { font-size: 10px; flex-shrink: 0; }

.sev-pill {
  font-size: 9px; font-weight: 800; padding: 2px 5px; border-radius: 4px; letter-spacing: 0.03em; flex-shrink: 0;
}
.sev-critical { background: rgba(244, 63, 94, 0.2); color: #f43f5e; border: 1px solid rgba(244, 63, 94, 0.4); }
.sev-high { background: rgba(244, 63, 94, 0.15); color: #fb7185; border: 1px solid rgba(244, 63, 94, 0.3); }
.sev-medium { background: rgba(245, 158, 11, 0.15); color: #fbbf24; border: 1px solid rgba(245, 158, 11, 0.3); }
.sev-low { background: rgba(6, 182, 212, 0.15); color: #38bdf8; border: 1px solid rgba(6, 182, 212, 0.3); }

.card-meta-row { display: flex; align-items: center; justify-content: space-between; font-size: 11px; }
.ns-pill {
  font-size: 10px; color: var(--text-muted); padding: 1px 6px; background: rgba(255, 255, 255, 0.06);
  border-radius: 4px; max-width: 140px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
}
.savings-amount { font-size: 12px; }

.card-waste-metrics {
  display: flex; align-items: center; gap: 6px; font-size: 10px;
  background: rgba(0, 0, 0, 0.25); padding: 4px 8px; border-radius: 5px;
}
.metric-sep { color: rgba(255, 255, 255, 0.15); }

.card-action-row { padding-top: 4px; border-top: 1px solid rgba(255, 255, 255, 0.05); }
.btn-rightsize {
  width: 100%; height: 32px; min-height: 32px; border-radius: 6px; font-size: 11.5px; font-weight: 700;
  background: rgba(16, 185, 129, 0.12); border: 1px solid rgba(16, 185, 129, 0.3); color: #34d399;
  display: inline-flex; align-items: center; justify-content: center; cursor: pointer; transition: all 0.15s ease;
}
.btn-rightsize:hover { background: rgba(16, 185, 129, 0.22); filter: brightness(1.15); }
</style>