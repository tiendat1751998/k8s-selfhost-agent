<script setup lang="ts">
import { ref } from 'vue'
import type { TeamNamespaceCost } from '../../composables/useCostFinOps'
import DataTable, { type Column } from '../ui/DataTable.vue'
import ModalDrawer from '../ui/ModalDrawer.vue'

defineProps<{ namespaces: TeamNamespaceCost[]; loading?: boolean; error?: string | null }>()
const emit = defineEmits<{
  (e: 'save-budget', namespace: string, limit: number): void
  (e: 'select-namespace', ns: TeamNamespaceCost): void
}>()

const breakdownTarget = ref<TeamNamespaceCost | null>(null)
const budgetTarget = ref<TeamNamespaceCost | null>(null)
const newBudgetLimit = ref<number>(0)

const columns: Column<TeamNamespaceCost>[] = [
  { key: 'namespace', label: 'Namespace & Team', width: '25%', sortable: true },
  { key: 'cluster', label: 'Cluster', width: '13%', sortable: true },
  { key: 'cpu_requested', label: 'CPU Request', width: '11%' },
  { key: 'memory_requested', label: 'RAM Request', width: '11%' },
  { key: 'monthly_cost', label: 'Monthly Cost', width: '12%', sortable: true },
  { key: 'budget_limit', label: 'Budget & Burn Rate', width: '14%', sortable: true },
  { key: 'actions', label: 'Actions', width: '14%', align: 'right' },
]

function getUtilColorClass(util: number): string {
  if (util > 100) return 'bg-rose'
  if (util >= 80) return 'bg-amber'
  if (util >= 40) return 'bg-emerald'
  return 'bg-cyan'
}

function openBreakdown(row: TeamNamespaceCost) {
  breakdownTarget.value = row
  emit('select-namespace', row)
}

function openBudgetModal(row: TeamNamespaceCost) {
  budgetTarget.value = row
  newBudgetLimit.value = row.budget_limit || Math.round(row.monthly_cost * 1.2)
}

function applyPreset(pct: number) {
  if (!budgetTarget.value) return
  newBudgetLimit.value = Math.round(budgetTarget.value.monthly_cost * (1 + pct / 100))
}

function handleSaveBudget() {
  if (!budgetTarget.value || newBudgetLimit.value <= 0) return
  emit('save-budget', budgetTarget.value.namespace, newBudgetLimit.value)
  budgetTarget.value = null
}
</script>

<template>
  <div class="section-card glass-panel">
    <div class="section-top">
      <div>
        <h2 class="section-title">Namespace Cost Allocations & Budget Governance</h2>
        <p class="section-subtitle">Unit economics attribution per team/namespace with CPU/RAM quotas and proactive budget monitoring</p>
      </div>
      <div class="section-meta-tags">
        <span class="badge badge-cyan">{{ namespaces.length }} Namespaces</span>
      </div>
    </div>

    <DataTable
      :columns="columns"
      :data="namespaces"
      :loading="loading"
      :error="error"
      searchable
      search-placeholder="Search namespace, team, or cluster..."
      empty-message="No namespace cost allocations recorded."
    >
      <template #cell-namespace="{ row }">
        <div class="ns-cell">
          <div class="ns-title-row">
            <span class="ns-name font-mono font-semibold" :title="row.namespace">{{ row.namespace }}</span>
            <span class="team-tag font-mono">{{ row.team }}</span>
          </div>
          <span class="ns-cluster font-mono text-muted">{{ row.cluster }}</span>
        </div>
      </template>

      <template #cell-cluster="{ row }">
        <span class="cluster-badge font-mono">{{ row.cluster }}</span>
      </template>

      <template #cell-cpu_requested="{ row }">
        <span class="font-mono text-cyan">{{ row.cpu_requested }}</span>
      </template>

      <template #cell-memory_requested="{ row }">
        <span class="font-mono text-violet">{{ row.memory_requested }}</span>
      </template>

      <template #cell-monthly_cost="{ row }">
        <span class="font-mono font-bold text-emerald">${{ row.monthly_cost.toLocaleString() }}</span>
      </template>

      <template #cell-budget_limit="{ row }">
        <div class="budget-cell">
          <div class="budget-labels font-mono">
            <span class="burn-val">${{ row.monthly_cost.toLocaleString() }}</span>
            <span class="cap-val text-muted">/ ${{ row.budget_limit.toLocaleString() }}</span>
          </div>
          <div class="util-cell">
            <div class="util-bar-bg">
              <div
                class="util-bar-fill"
                :style="{ width: `${Math.min(100, row.budget_utilization)}%` }"
                :class="getUtilColorClass(row.budget_utilization)"
              ></div>
            </div>
            <span class="util-num font-mono" :class="row.budget_status === 'exceeded' ? 'text-rose' : ''">
              {{ row.budget_utilization }}%
            </span>
          </div>
        </div>
      </template>

      <template #cell-actions="{ row }">
        <div class="action-btn-group">
          <button
            class="btn-compact-32 btn-action-opt"
            title="Right-size and optimize resource quotas"
            @click="openBudgetModal(row)"
          >
            <span>🔍 Optimize</span>
          </button>
          <button
            class="btn-compact-32 btn-action-breakdown"
            title="Inspect detailed cost breakdown"
            @click="openBreakdown(row)"
          >
            <span>📊 Breakdown</span>
          </button>
        </div>
      </template>
    </DataTable>

    <!-- Modal: Namespace Breakdown -->
    <ModalDrawer
      :show="breakdownTarget !== null"
      :title="`Cost Breakdown: ${breakdownTarget?.namespace || ''}`"
      :subtitle="`Assigned Team: ${breakdownTarget?.team || ''} | Cluster: ${breakdownTarget?.cluster || ''}`"
      mode="drawer"
      placement="right"
      max-width="500px"
      @close="breakdownTarget = null"
    >
      <div v-if="breakdownTarget" class="breakdown-details font-mono">
        <div class="breakdown-hero glass-panel">
          <div class="hero-label">CURRENT MONTHLY SPEND</div>
          <div class="hero-val font-mono">${{ breakdownTarget.monthly_cost.toLocaleString() }} <small>/mo</small></div>
          <div class="hero-sub">
            Budget Burn: <span :class="breakdownTarget.budget_utilization > 100 ? 'text-rose' : 'text-emerald'">
              {{ breakdownTarget.budget_utilization }}% of ${{ breakdownTarget.budget_limit.toLocaleString() }}
            </span>
          </div>
        </div>

        <div class="resource-split-section">
          <h4 class="subhead">Resource Component Breakdown</h4>
          <div class="breakdown-bars">
            <div class="split-row">
              <div class="split-meta">
                <span class="text-cyan">Compute (CPU) - {{ breakdownTarget.cpu_requested }}</span>
                <span>${{ Math.round(breakdownTarget.monthly_cost * 0.45).toLocaleString() }}</span>
              </div>
              <div class="util-bar-bg"><div class="util-bar-fill bg-cyan" style="width: 45%"></div></div>
            </div>
            <div class="split-row">
              <div class="split-meta">
                <span class="text-violet">Memory (RAM) - {{ breakdownTarget.memory_requested }}</span>
                <span>${{ Math.round(breakdownTarget.monthly_cost * 0.35).toLocaleString() }}</span>
              </div>
              <div class="util-bar-bg"><div class="util-bar-fill bg-violet" style="width: 35%"></div></div>
            </div>
            <div class="split-row">
              <div class="split-meta">
                <span class="text-amber">Storage (PV / PVCs)</span>
                <span>${{ Math.round(breakdownTarget.monthly_cost * 0.2).toLocaleString() }}</span>
              </div>
              <div class="util-bar-bg"><div class="util-bar-fill bg-amber" style="width: 20%"></div></div>
            </div>
          </div>
        </div>

        <div class="recommendations-box glass-panel">
          <h4 class="rec-title">⚡ FinOps Right-Sizing Insights</h4>
          <p class="rec-desc">
            Historical CPU utilization sits at <strong>{{ breakdownTarget.utilization }}%</strong>.
            Downscaling requests by 20% recovers approx.
            <strong class="text-emerald">${{ Math.round(breakdownTarget.monthly_cost * 0.2).toLocaleString() }}/mo</strong>.
          </p>
        </div>
      </div>
    </ModalDrawer>

    <!-- Modal: Budget Limit Configuration -->
    <ModalDrawer
      :show="budgetTarget !== null"
      :title="`Budget Limit: ${budgetTarget?.namespace || ''}`"
      subtitle="Adjust team allocation threshold and burn-rate alert trigger"
      mode="modal"
      max-width="440px"
      @close="budgetTarget = null"
    >
      <div v-if="budgetTarget" class="budget-dialog-content">
        <div class="current-spend-row">
          <span class="text-muted">Current Monthly Spend:</span>
          <span class="font-mono font-bold text-emerald">${{ budgetTarget.monthly_cost.toLocaleString() }}</span>
        </div>
        <div class="form-group">
          <label class="form-label" for="budget-input">Monthly Budget Cap (USD):</label>
          <div class="input-wrapper">
            <span class="currency-prefix">$</span>
            <input id="budget-input" v-model.number="newBudgetLimit" type="number" min="100" step="100" class="budget-input font-mono" />
          </div>
        </div>
        <div class="preset-buttons">
          <span class="preset-label text-muted">Quick Presets:</span>
          <div class="preset-group">
            <button class="btn btn-secondary btn-xs" @click="applyPreset(10)">+10%</button>
            <button class="btn btn-secondary btn-xs" @click="applyPreset(25)">+25%</button>
            <button class="btn btn-secondary btn-xs" @click="newBudgetLimit = 5000">$5,000</button>
            <button class="btn btn-secondary btn-xs" @click="newBudgetLimit = 10000">$10,000</button>
          </div>
        </div>
      </div>
      <template #footer>
        <div class="modal-actions">
          <button class="btn btn-secondary" @click="budgetTarget = null">Cancel</button>
          <button class="btn btn-primary" :disabled="newBudgetLimit <= 0" @click="handleSaveBudget">Save Budget Cap 💾</button>
        </div>
      </template>
    </ModalDrawer>
  </div>
</template>

<style scoped>
@import '../../assets/styles/views/cost.css';
@import '../../assets/styles/components/cost-drawers.css';

:deep(.table-scroll-wrapper) {
  overflow-x: hidden !important;
  width: 100% !important;
}

:deep(.data-table) {
  table-layout: fixed !important;
  width: 100% !important;
}

.ns-name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 100%;
  display: block;
}
</style>
