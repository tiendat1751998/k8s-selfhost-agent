<script setup lang="ts">
import { computed } from 'vue'
import type { CloudSpendBreakdown } from '../../composables/useCostFinOps'
import type { ClusterCost } from '../../api/governance'

const props = defineProps<{
  cloudBreakdown: CloudSpendBreakdown[]
  clusters: ClusterCost[]
  loading?: boolean
}>()

const emit = defineEmits<{
  (e: 'refresh'): void
}>()

const totalSpend = computed(() => {
  return props.cloudBreakdown.reduce((sum, item) => sum + item.cost, 0)
})

const RADIUS = 42
const CIRCUMFERENCE = 2 * Math.PI * RADIUS

interface DonutSegment {
  provider: string
  name: string
  cost: number
  percentage: number
  color: string
  dashArray: string
  dashOffset: number
}

const donutSegments = computed<DonutSegment[]>(() => {
  let accumulatedPercent = 0
  return props.cloudBreakdown.map((item) => {
    const strokeDash = (item.percentage / 100) * CIRCUMFERENCE
    const dashOffset = -((accumulatedPercent / 100) * CIRCUMFERENCE)
    accumulatedPercent += item.percentage
    return {
      provider: item.provider,
      name: item.name,
      cost: item.cost,
      percentage: item.percentage,
      color: item.color,
      dashArray: `${strokeDash} ${CIRCUMFERENCE - strokeDash}`,
      dashOffset,
    }
  })
})

const categoryTotals = computed(() => {
  const c = props.clusters
  const cpu = c.reduce((sum, item) => sum + (item.cpu_cost || 0), 0)
  const memory = c.reduce((sum, item) => sum + (item.memory_cost || 0), 0)
  const storage = c.reduce((sum, item) => sum + (item.storage_cost || 0), 0)
  const network = c.reduce((sum, item) => sum + (item.network_cost || 0), 0)
  const total = cpu + memory + storage + network || 1

  return {
    cpu: { amount: cpu, pct: Math.round((cpu / total) * 100) },
    memory: { amount: memory, pct: Math.round((memory / total) * 100) },
    storage: { amount: storage, pct: Math.round((storage / total) * 100) },
    network: { amount: network, pct: Math.round((network / total) * 100) },
    total,
  }
})

function getProviderIcon(provider: string): string {
  const p = (provider || '').toLowerCase()
  if (p.includes('aws')) return '☁️'
  if (p.includes('gcp') || p.includes('google')) return '🌐'
  if (p.includes('azure')) return '🔷'
  if (p.includes('baremetal') || p.includes('local')) return '🖥️'
  return '⎈'
}
</script>

<template>
  <div class="section-card glass-panel">
    <div class="section-top">
      <div>
        <h2 class="section-title">Multi-Cloud Infrastructure Cost Breakdown</h2>
        <p class="section-subtitle">Aggregated multi-cloud spend distribution, resource categories, and cluster billing</p>
      </div>
      <div v-if="cloudBreakdown.length > 0" class="total-spend-pill font-mono">
        Total: ${{ totalSpend.toLocaleString() }} / mo
      </div>
    </div>

    <div v-if="cloudBreakdown.length > 0" class="charts-container">
      <!-- Donut Chart & Cloud Legend -->
      <div class="donut-section glass-panel">
        <div class="donut-chart-wrapper">
          <svg class="donut-svg" viewBox="0 0 100 100">
            <circle class="donut-bg" cx="50" cy="50" :r="RADIUS" />
            <circle
              v-for="seg in donutSegments"
              :key="seg.provider"
              class="donut-segment"
              cx="50"
              cy="50"
              :r="RADIUS"
              :stroke="seg.color"
              :stroke-dasharray="seg.dashArray"
              :stroke-dashoffset="seg.dashOffset"
            />
          </svg>
          <div class="donut-center-text">
            <span class="donut-center-val font-mono">${{ totalSpend.toLocaleString() }}</span>
            <span class="donut-center-label">RUN-RATE</span>
          </div>
        </div>

        <div class="donut-legend">
          <div
            v-for="item in cloudBreakdown"
            :key="item.provider"
            class="legend-item font-mono"
          >
            <div class="legend-color-dot" :style="{ backgroundColor: item.color }"></div>
            <div class="legend-info">
              <div class="legend-name-row">
                <span class="legend-name">{{ item.name }}</span>
                <span class="legend-pct">{{ item.percentage }}%</span>
              </div>
              <div class="legend-meta text-muted">
                <span>${{ item.cost.toLocaleString() }}</span>
                <span>• {{ item.clusterCount }} {{ item.clusterCount === 1 ? 'cluster' : 'clusters' }}</span>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- Resource Category Breakdown Bars -->
      <div class="categories-section glass-panel">
        <h3 class="subsection-title">Spend by Resource Architecture</h3>
        <p class="subsection-desc">Distribution of monthly compute, memory, persistence, and networking costs</p>

        <div class="stacked-bar-container">
          <div
            class="stacked-slice bg-cyan"
            :style="{ width: `${categoryTotals.cpu.pct}%` }"
            :title="`Compute (CPU): ${categoryTotals.cpu.pct}%`"
          ></div>
          <div
            class="stacked-slice bg-violet"
            :style="{ width: `${categoryTotals.memory.pct}%` }"
            :title="`Memory (RAM): ${categoryTotals.memory.pct}%`"
          ></div>
          <div
            class="stacked-slice bg-amber"
            :style="{ width: `${categoryTotals.storage.pct}%` }"
            :title="`Storage (PV): ${categoryTotals.storage.pct}%`"
          ></div>
          <div
            class="stacked-slice bg-emerald"
            :style="{ width: `${categoryTotals.network.pct}%` }"
            :title="`Network Ingress: ${categoryTotals.network.pct}%`"
          ></div>
        </div>

        <div class="categories-grid font-mono">
          <div class="category-card">
            <div class="cat-label text-cyan">Compute (CPU)</div>
            <div class="cat-cost">${{ categoryTotals.cpu.amount.toLocaleString() }}</div>
            <div class="cat-pct text-muted">{{ categoryTotals.cpu.pct }}% of spend</div>
          </div>

          <div class="category-card">
            <div class="cat-label text-violet">Memory (RAM)</div>
            <div class="cat-cost">${{ categoryTotals.memory.amount.toLocaleString() }}</div>
            <div class="cat-pct text-muted">{{ categoryTotals.memory.pct }}% of spend</div>
          </div>

          <div class="category-card">
            <div class="cat-label text-amber">Storage (PV)</div>
            <div class="cat-cost">${{ categoryTotals.storage.amount.toLocaleString() }}</div>
            <div class="cat-pct text-muted">{{ categoryTotals.storage.pct }}% of spend</div>
          </div>

          <div class="category-card">
            <div class="cat-label text-emerald">Network Ingress</div>
            <div class="cat-cost">${{ categoryTotals.network.amount.toLocaleString() }}</div>
            <div class="cat-pct text-muted">{{ categoryTotals.network.pct }}% of spend</div>
          </div>
        </div>
      </div>
    </div>

    <!-- Cluster Breakdown Cards -->
    <div v-if="clusters.length > 0" class="clusters-cost-grid">
      <div v-for="cluster in clusters" :key="cluster.id" class="cluster-cost-card glass-panel">
        <div class="cluster-cost-top">
          <div class="provider-icon-box">{{ getProviderIcon(cluster.provider) }}</div>
          <div class="cluster-cost-meta">
            <h3 class="cluster-name">{{ cluster.name }}</h3>
            <span class="provider-name font-mono text-muted">{{ cluster.provider.toUpperCase() }}</span>
          </div>
          <div class="cluster-monthly-val font-mono">
            ${{ cluster.monthly_cost.toLocaleString() }} <small>/mo</small>
          </div>
        </div>

        <div class="cost-components-grid font-mono">
          <div class="component-item">
            <span class="comp-k">Compute (CPU):</span>
            <span class="comp-v text-cyan">${{ cluster.cpu_cost.toLocaleString() }}</span>
          </div>
          <div class="component-item">
            <span class="comp-k">Memory (RAM):</span>
            <span class="comp-v text-violet">${{ cluster.memory_cost.toLocaleString() }}</span>
          </div>
          <div class="component-item">
            <span class="comp-k">Storage (PV):</span>
            <span class="comp-v text-amber">${{ cluster.storage_cost.toLocaleString() }}</span>
          </div>
          <div class="component-item">
            <span class="comp-k">Network Ingress:</span>
            <span class="comp-v text-emerald">${{ cluster.network_cost.toLocaleString() }}</span>
          </div>
        </div>
      </div>
    </div>

    <div v-else-if="!loading" class="empty-state-box glass-panel">
      <span class="empty-icon">💵</span>
      <h3 class="empty-title">No Cluster Cost Telemetry</h3>
      <p class="empty-desc">No cluster billing data discovered. Connect OpenCost, Kubecost, or cloud billing exports.</p>
      <button class="btn btn-secondary btn-sm" @click="emit('refresh')">
        <span>🔄 Refresh Metrics</span>
      </button>
    </div>
  </div>
</template>
