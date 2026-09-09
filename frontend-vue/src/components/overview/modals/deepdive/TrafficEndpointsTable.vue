<script setup lang="ts">
import { ref, computed } from 'vue'
import type { NodeMetrics, TpsSnapshot } from '../../../../api/overview'

export interface DeepDiveServiceItem {
  service_name: string
  node_name: string
  requests_per_sec: number
  traffic_percent: number
  rx_bytes_per_sec: number
  tx_bytes_per_sec: number
  total_rx_bytes?: number
  total_tx_bytes?: number
  cpu_percent: number
  memory_used_mb: number
  status: 'healthy' | 'degraded' | 'down'
  container_count: number
}

interface Props {
  nodes?: NodeMetrics[]
  tpsData?: TpsSnapshot | null
  dockerContainers?: any[]
}

const props = withDefaults(defineProps<Props>(), {
  nodes: () => [],
  tpsData: null,
  dockerContainers: () => [],
})

const modalServiceSearch = ref('')
const modalServiceSortBy = ref<'name' | 'node' | 'traffic' | 'rps' | 'bandwidth' | 'cpu' | 'mem' | 'status'>('traffic')
const modalServiceSortOrder = ref<'asc' | 'desc'>('desc')

const allClusterServices = computed<DeepDiveServiceItem[]>(() => {
  const nodeMap = new Map<string, string>()
  props.nodes.forEach(n => { nodeMap.set(n.node_id.toLowerCase(), n.node_name); nodeMap.set(n.node_name.toLowerCase(), n.node_name) })

  let list: DeepDiveServiceItem[] = []
  if (props.tpsData?.services?.length) {
    list = props.tpsData.services.map(s => ({
      service_name: s.service_name,
      node_name: nodeMap.get((s.node_id || '').toLowerCase()) || nodeMap.get((s.node_name || '').toLowerCase()) || s.node_name || s.node_id || 'k8smaster',
      requests_per_sec: s.requests_per_sec || 0, traffic_percent: 0,
      rx_bytes_per_sec: s.rx_bytes_per_sec || 0, tx_bytes_per_sec: s.tx_bytes_per_sec || 0,
      total_rx_bytes: s.total_rx_bytes || 0, total_tx_bytes: s.total_tx_bytes || 0,
      cpu_percent: s.cpu_percent || 0, memory_used_mb: s.memory_used_mb || 0,
      status: (s.status as any) || 'healthy', container_count: s.container_count || 1,
    }))
  } else if (props.dockerContainers?.length) {
    list = props.dockerContainers.map(c => ({
      service_name: ((c.names || (c.name ? [c.name] : ['unknown-service']))[0] || 'service').replace(/^\//, ''),
      node_name: nodeMap.get((c.node_id || '').toLowerCase()) || c.node_id || 'k8smaster',
      requests_per_sec: 0, traffic_percent: 0, rx_bytes_per_sec: 0, tx_bytes_per_sec: 0, total_rx_bytes: 0, total_tx_bytes: 0,
      cpu_percent: c.cpu_percent || 0, memory_used_mb: c.memory_usage ? Math.round(c.memory_usage / (1024 * 1024)) : 0,
      status: c.state === 'running' ? 'healthy' : 'degraded', container_count: 1,
    }))
  }

  const totalRps = list.reduce((acc, s) => acc + s.requests_per_sec, 0)
  const totalBw = list.reduce((acc, s) => acc + s.rx_bytes_per_sec + s.tx_bytes_per_sec, 0)
  return list.map(s => ({
    ...s,
    traffic_percent: totalRps > 0 ? (s.requests_per_sec / totalRps) * 100 : (totalBw > 0 ? ((s.rx_bytes_per_sec + s.tx_bytes_per_sec) / totalBw) * 100 : (list.length ? 100 / list.length : 0)),
  }))
})

const filteredAndSortedClusterServices = computed(() => {
  let list = [...allClusterServices.value]
  const q = modalServiceSearch.value.toLowerCase().trim()
  if (q) list = list.filter(s => s.service_name.toLowerCase().includes(q) || s.node_name.toLowerCase().includes(q))
  const key = modalServiceSortBy.value, mult = modalServiceSortOrder.value === 'asc' ? 1 : -1
  return list.sort((a, b) => {
    if (key === 'name') return a.service_name.localeCompare(b.service_name) * mult
    if (key === 'node') return a.node_name.localeCompare(b.node_name) * mult
    if (key === 'traffic') return (a.traffic_percent - b.traffic_percent) * mult
    if (key === 'rps') return (a.requests_per_sec - b.requests_per_sec) * mult
    if (key === 'bandwidth') return ((a.rx_bytes_per_sec + a.tx_bytes_per_sec) - (b.rx_bytes_per_sec + b.tx_bytes_per_sec)) * mult
    if (key === 'cpu') return (a.cpu_percent - b.cpu_percent) * mult
    if (key === 'mem') return (a.memory_used_mb - b.memory_used_mb) * mult
    if (key === 'status') return a.status.localeCompare(b.status) * mult
    return 0
  })
})

function toggleModalSort(key: typeof modalServiceSortBy.value) {
  modalServiceSortOrder.value = modalServiceSortBy.value === key ? (modalServiceSortOrder.value === 'asc' ? 'desc' : 'asc') : 'desc'
  modalServiceSortBy.value = key
}

function formatBytes(bytes?: number): string {
  if (!bytes || bytes <= 0 || isNaN(bytes)) return '0 B'
  const k = 1024, sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(1))} ${sizes[i] || 'B'}`
}
function formatIoRate(rate?: number): string { return `${formatBytes(rate)}/s` }
</script>

<template>
  <section class="modal-breakdown-section glass-panel">
    <div class="breakdown-header-bar">
      <div class="breakdown-title-wrap">
        <h4 class="modal-section-title">
          <span class="breakdown-title-full">📦 Ranked Service & Workload Contributors</span>
          <span class="breakdown-title-mobile">📦 Ranked Workloads</span>
        </h4>
        <span class="badge badge-cyan font-mono">{{ filteredAndSortedClusterServices.length }} Active Services</span>
      </div>
      <div class="breakdown-actions-bar">
        <div class="table-search-input-wrap">
          <span class="search-icon">🔍</span>
          <input type="text" v-model="modalServiceSearch" placeholder="Filter services or nodes..." class="table-search-field font-mono" />
          <button v-if="modalServiceSearch" class="btn-clear-search" @click="modalServiceSearch = ''">✕</button>
        </div>
        <div class="mobile-sort-pills">
          <button v-for="c in [{ k: 'traffic', l: 'Traffic' }, { k: 'rps', l: 'RPS' }, { k: 'cpu', l: 'CPU' }, { k: 'bandwidth', l: 'BW' }]" :key="c.k" type="button" class="mobile-sort-pill" :class="{ active: modalServiceSortBy === c.k }" @click="toggleModalSort(c.k as any)">
            {{ c.l }} {{ modalServiceSortBy === c.k ? (modalServiceSortOrder === 'asc' ? '▲' : '▼') : '' }}
          </button>
        </div>
      </div>
    </div>

    <!-- Desktop Table -->
    <div class="breakdown-table-wrapper" v-if="filteredAndSortedClusterServices.length > 0">
      <table class="breakdown-table">
        <thead>
          <tr>
            <th v-for="h in [{ k: 'name', l: 'SERVICE', c: 'th-svc' }, { k: 'node', l: 'HOST NODE', c: 'th-node' }, { k: 'traffic', l: 'TRAFFIC CONTRIBUTION', c: 'th-traffic' }, { k: 'rps', l: 'REQ / S', c: 'th-rps' }, { k: 'bandwidth', l: 'BANDWIDTH', c: 'th-bw' }, { k: 'cpu', l: 'CPU & MEMORY', c: 'th-res' }, { k: 'status', l: 'STATUS', c: 'th-status' }]" :key="h.k" :class="[h.c, 'cursor-pointer']" @click="toggleModalSort(h.k as any)">
              <div class="th-content"><span>{{ h.l }}</span><span class="sort-icon">{{ modalServiceSortBy === h.k ? (modalServiceSortOrder === 'asc' ? '▲' : '▼') : '↕' }}</span></div>
            </th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="svc in filteredAndSortedClusterServices" :key="svc.service_name + '-' + svc.node_name" class="breakdown-row">
            <td class="col-svc">
              <div class="service-identity">
                <span class="node-indicator-dot" :class="svc.status === 'healthy' ? 'bg-emerald' : svc.status === 'degraded' ? 'bg-amber' : 'bg-rose'"></span>
                <div class="service-name-wrap"><span class="service-title font-mono">{{ svc.service_name }}</span><span class="service-replica-tag" v-if="svc.container_count > 1">{{ svc.container_count }} replicas</span></div>
              </div>
            </td>
            <td class="col-node"><div class="node-cell-wrap"><span class="node-server-icon">🖥️</span><span class="node-server-name font-mono">{{ svc.node_name }}</span></div></td>
            <td class="col-traffic"><div class="traffic-bar-cell"><div class="traffic-track"><div class="traffic-fill" :style="{ width: `${Math.max(4, Math.min(100, svc.traffic_percent))}%` }"></div></div><span class="traffic-label font-mono font-bold">{{ svc.traffic_percent.toFixed(1) }}%</span></div></td>
            <td class="col-rps font-mono"><span class="smooth-value" :class="svc.requests_per_sec > 0 ? 'text-emerald font-bold' : 'text-muted'">{{ svc.requests_per_sec > 0 ? svc.requests_per_sec.toFixed(1) : '0.0' }}</span><span class="text-xs text-muted" v-if="svc.requests_per_sec > 0"> rps</span></td>
            <td class="col-bw font-mono"><div class="bandwidth-stack"><span class="bw-rx text-cyan" :title="'Live: ' + formatIoRate(svc.rx_bytes_per_sec) + ' | Total: ' + formatBytes(svc.total_rx_bytes) + ' recv'">↓ {{ formatIoRate(svc.rx_bytes_per_sec) }}</span><span class="bw-tx text-violet" :title="'Live: ' + formatIoRate(svc.tx_bytes_per_sec) + ' | Total: ' + formatBytes(svc.total_tx_bytes) + ' sent'">↑ {{ formatIoRate(svc.tx_bytes_per_sec) }}</span></div></td>
            <td class="col-res font-mono"><div class="res-stack"><span class="smooth-value" :class="svc.cpu_percent > 80 ? 'text-rose font-bold' : svc.cpu_percent > 50 ? 'text-amber' : 'text-violet'">{{ svc.cpu_percent.toFixed(1) }}% CPU</span><span class="text-cyan text-xs">{{ svc.memory_used_mb >= 1024 ? (svc.memory_used_mb / 1024).toFixed(1) + ' GB' : svc.memory_used_mb.toFixed(0) + ' MB' }}</span></div></td>
            <td class="col-status"><span class="badge smooth-value" :class="svc.status === 'healthy' ? 'badge-emerald' : svc.status === 'degraded' ? 'badge-amber' : 'badge-rose'">{{ svc.status === 'healthy' ? '● HEALTHY' : svc.status === 'degraded' ? '● DEGRADED' : '● DOWN' }}</span></td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- Mobile View -->
    <div class="breakdown-mobile-list" v-if="filteredAndSortedClusterServices.length > 0">
      <div v-for="svc in filteredAndSortedClusterServices" :key="svc.service_name + '-' + svc.node_name" class="mobile-workload-card glass-panel">
        <div class="m-card-header">
          <div class="m-svc-identity">
            <span class="node-indicator-dot" :class="svc.status === 'healthy' ? 'bg-emerald' : svc.status === 'degraded' ? 'bg-amber' : 'bg-rose'"></span>
            <span class="m-svc-title font-mono font-bold">{{ svc.service_name }}</span>
            <span class="service-replica-tag" v-if="svc.container_count > 1">({{ svc.container_count }}x)</span>
          </div>
          <div class="m-header-right">
            <span class="m-traffic-pct font-mono font-bold text-cyan">{{ svc.traffic_percent.toFixed(1) }}%</span>
            <span class="badge badge-sm" :class="svc.status === 'healthy' ? 'badge-emerald' : svc.status === 'degraded' ? 'badge-amber' : 'badge-rose'">{{ svc.status === 'healthy' ? '● HEALTHY' : svc.status === 'degraded' ? '● DEGRADED' : '● DOWN' }}</span>
          </div>
        </div>
        <div class="m-traffic-track"><div class="m-traffic-fill" :style="{ width: `${Math.max(4, Math.min(100, svc.traffic_percent))}%` }"></div></div>
        <div class="m-metric-chips font-mono">
          <span class="m-chip m-chip-rps" :class="svc.requests_per_sec > 0 ? 'text-emerald font-bold' : 'text-muted'">⚡ {{ svc.requests_per_sec > 0 ? svc.requests_per_sec.toFixed(1) : '0.0' }} rps</span>
          <span class="m-chip m-chip-bw text-cyan">↓ {{ formatIoRate(svc.rx_bytes_per_sec) }} <span class="text-violet">↑ {{ formatIoRate(svc.tx_bytes_per_sec) }}</span></span>
          <span class="m-chip m-chip-res" :class="svc.cpu_percent > 80 ? 'text-rose font-bold' : svc.cpu_percent > 50 ? 'text-amber' : 'text-violet'">{{ svc.cpu_percent.toFixed(1) }}% CPU · <span class="text-cyan">{{ svc.memory_used_mb >= 1024 ? (svc.memory_used_mb / 1024).toFixed(1) + ' GB' : svc.memory_used_mb.toFixed(0) + ' MB' }}</span></span>
          <span class="m-chip m-chip-node text-muted">🖥️ {{ svc.node_name }}</span>
        </div>
      </div>
    </div>

    <!-- Empty State -->
    <div v-else class="breakdown-empty-state">
      <span class="empty-icon">🔍</span>
      <p class="empty-text">No services found matching "{{ modalServiceSearch }}"</p>
      <button class="btn btn-secondary btn-sm" type="button" @click="modalServiceSearch = ''">Clear Search Filter</button>
    </div>
  </section>
</template>

<style scoped>
@import '../../../../assets/styles/components/deep-dive-traffic.css';
</style>
