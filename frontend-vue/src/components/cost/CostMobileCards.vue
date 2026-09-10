<template>
  <div class="cost-mobile-cards">
    <!-- Stream Segmented Tabs: Namespaces vs Waste Findings -->
    <div class="stream-tabs font-mono">
      <button
        type="button"
        class="stream-tab-btn"
        :class="{ active: activeTab === 'namespaces' }"
        @click="activeTab = 'namespaces'"
      >
        <span>📁 Namespaces ({{ (namespaces || []).length }})</span>
      </button>
      <button
        type="button"
        class="stream-tab-btn"
        :class="{ active: activeTab === 'waste' }"
        @click="activeTab = 'waste'"
      >
        <span>⚠️ Waste Findings ({{ (wasteAlerts || []).length }})</span>
      </button>
    </div>

    <!-- Loading State -->
    <div v-if="loading" class="stream-status font-mono">
      <span class="spin-icon">⏳</span> Loading FinOps cost telemetry...
    </div>

    <!-- Tab 1: High-Density Namespace Cards Stream (~68px/item) -->
    <div v-else-if="activeTab === 'namespaces'">
      <div v-if="(namespaces || []).length === 0" class="stream-empty glass-panel font-mono">
        <span>✅</span> No namespace cost allocations recorded.
      </div>
      <div v-else class="cards-list">
        <div
          v-for="item in (namespaces || [])"
          :key="item.namespace"
          class="mobile-cost-card glass-panel cursor-pointer"
          role="button"
          tabindex="0"
          @click="$emit('selectNamespace', item)"
          @keydown.enter="$emit('selectNamespace', item)"
        >
          <!-- Top Row: Name + Team + Spend -->
          <div class="card-row-top">
            <div class="card-title-meta">
              <span class="card-name font-mono" :title="item.namespace">{{ item.namespace }}</span>
              <span class="card-team-badge font-mono">{{ item.team }}</span>
            </div>
            <span class="card-cost-pill font-mono">${{ item.monthly_cost.toLocaleString() }}/mo</span>
          </div>

          <!-- Bottom Row: Cluster + CPU/RAM + Budget bar -->
          <div class="card-row-bot font-mono">
            <span class="card-sub-info">
              {{ item.cluster }} · CPU: {{ item.cpu_requested }} · RAM: {{ item.memory_requested }}
            </span>
            <div class="card-budget-mini" :title="`Budget Burn: ${item.budget_utilization}%`">
              <div class="card-budget-bar">
                <div
                  class="card-budget-bar-fill"
                  :style="{ width: `${Math.min(100, item.budget_utilization)}%` }"
                  :class="getUtilColorClass(item.budget_utilization)"
                ></div>
              </div>
              <span
                class="card-budget-pct"
                :class="item.budget_status === 'exceeded' ? 'text-rose' : item.budget_status === 'warning' ? 'text-amber' : 'text-emerald'"
              >
                {{ item.budget_utilization }}%
              </span>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Tab 2: High-Density Waste Alerts Stream (~68px/item) -->
    <div v-else-if="activeTab === 'waste'">
      <div v-if="(wasteAlerts || []).length === 0" class="stream-empty glass-panel font-mono">
        <span>✅</span> No resource waste detected. Cluster requests are optimal.
      </div>
      <div v-else class="cards-list">
        <div
          v-for="item in (wasteAlerts || [])"
          :key="item.id"
          class="mobile-cost-card glass-panel"
        >
          <div class="card-row-top">
            <div class="card-title-meta">
              <span class="card-name font-mono" :title="item.resource">{{ item.resource }}</span>
              <span class="sev-pill font-mono" :class="`sev-${(item.severity || 'medium').toLowerCase()}`">
                {{ (item.severity || 'HIGH').toUpperCase() }}
              </span>
            </div>
            <span class="card-cost-pill font-mono text-rose">${{ item.wasted_cost }}/mo</span>
          </div>

          <div class="card-row-bot font-mono">
            <span class="card-sub-info">
              {{ item.namespace }} · CPU {{ item.cpu_util ?? 0 }}% · RAM {{ item.mem_util ?? 0 }}%
            </span>
            <button
              type="button"
              class="btn-rightsize-mini font-mono"
              title="Right-size idle resource"
              @click.stop="$emit('rightSize', item.id)"
            >
              <span>⚡ Right-Size</span>
            </button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import type { ResourceWaste } from '../../api/governance'
import type { TeamNamespaceCost } from '../../composables/useCostFinOps'

defineProps<{
  namespaces?: TeamNamespaceCost[]
  wasteAlerts?: ResourceWaste[]
  loading?: boolean
}>()

defineEmits<{
  (e: 'rightSize', id: string): void
  (e: 'selectNamespace', ns: TeamNamespaceCost): void
}>()

const activeTab = ref<'namespaces' | 'waste'>('namespaces')

function getUtilColorClass(util: number): string {
  if (util > 100) return 'bg-rose'
  if (util >= 80) return 'bg-amber'
  if (util >= 40) return 'bg-emerald'
  return 'bg-cyan'
}
</script>

<style scoped>
@import '../../assets/styles/views/cost.css';
@import '../../assets/styles/components/cost-drawers.css';
</style>
