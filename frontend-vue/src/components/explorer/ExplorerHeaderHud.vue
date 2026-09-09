<script setup lang="ts">
import type { K8sNamespace, ResourceKind } from '../../api/k8s'
import type { Cluster } from '../../api/compute'

withDefaults(
  defineProps<{
    clusters: Cluster[]
    selectedCluster: string
    namespaces: K8sNamespace[]
    selectedNamespace: string
    selectedKind: ResourceKind
    searchQuery: string
    totalCount: number
    healthyCount: number
    warnCount: number
    podCount: number
    loading?: boolean
  }>(),
  { clusters: () => [], namespaces: () => [], loading: false }
)

const emit = defineEmits<{
  (e: 'update:selectedCluster', val: string): void
  (e: 'update:selectedNamespace', val: string): void
  (e: 'update:selectedKind', val: ResourceKind): void
  (e: 'update:searchQuery', val: string): void
  (e: 'create'): void
  (e: 'refresh'): void
}>()

const filterChips: { kind: ResourceKind; label: string; icon: string }[] = [
  { kind: 'pods', label: 'Pods', icon: '🚀' },
  { kind: 'deployments', label: 'Deployments', icon: '📦' },
  { kind: 'services', label: 'Services', icon: '🔌' },
  { kind: 'configmaps', label: 'ConfigMaps', icon: '🗺️' },
  { kind: 'ingresses', label: 'Ingresses', icon: '🌐' },
  { kind: 'nodes', label: 'Nodes', icon: '🖥️' },
  { kind: 'persistentvolumeclaims', label: 'PV/PVC', icon: '💾' },
]
</script>

<template>
  <div class="explorer-hud-container glass-panel">
    <div class="hud-top-row">
      <div class="hud-selectors">
        <div class="selector-group">
          <label class="hud-label">Cluster</label>
          <select :value="selectedCluster" class="input-glass font-mono hud-select" @change="emit('update:selectedCluster', ($event.target as HTMLSelectElement).value)">
            <option v-for="c in clusters" :key="c.id || c.name" :value="c.name || c.id">🌐 {{ c.name || c.id }}</option>
            <option v-if="clusters.length === 0" :value="selectedCluster">🌐 {{ selectedCluster || 'primary-cluster' }}</option>
          </select>
        </div>
        <div class="selector-group">
          <label class="hud-label">Namespace</label>
          <select :value="selectedNamespace" class="input-glass font-mono hud-select" @change="emit('update:selectedNamespace', ($event.target as HTMLSelectElement).value)">
            <option value="all">🌐 All Namespaces</option>
            <option v-for="ns in namespaces" :key="ns.name" :value="ns.name">📁 {{ ns.name }}</option>
          </select>
        </div>
      </div>

      <div class="hud-telemetry-cards">
        <div class="telemetry-card"><span class="telemetry-label">Total</span><span class="telemetry-val font-mono">{{ totalCount }}</span></div>
        <div class="telemetry-card text-emerald"><span class="telemetry-label">Healthy</span><span class="telemetry-val font-mono">{{ healthyCount }}</span></div>
        <div class="telemetry-card text-amber"><span class="telemetry-label">Warnings</span><span class="telemetry-val font-mono">{{ warnCount }}</span></div>
        <div class="telemetry-card text-cyan"><span class="telemetry-label">Pods</span><span class="telemetry-val font-mono">{{ podCount }}</span></div>
      </div>

      <div class="hud-actions">
        <button type="button" class="btn btn-secondary btn-xs font-mono" :disabled="loading" @click="emit('refresh')">{{ loading ? '⏳' : '🔄' }} Refresh</button>
        <button type="button" class="btn btn-primary btn-xs font-mono" @click="emit('create')">➕ New</button>
      </div>
    </div>

    <div class="hud-bottom-row">
      <div class="filter-chips-scroller">
        <button
          v-for="chip in filterChips"
          :key="chip.kind"
          type="button"
          class="hud-chip font-mono"
          :class="{ 'is-active': selectedKind === chip.kind }"
          @click="emit('update:selectedKind', chip.kind)"
        >
          <span>{{ chip.icon }}</span><span>{{ chip.label }}</span>
        </button>
      </div>
      <div class="hud-search-box">
        <input :value="searchQuery" type="text" placeholder="Filter resources..." class="input-glass font-mono hud-search-input" @input="emit('update:searchQuery', ($event.target as HTMLInputElement).value)" />
        <button v-if="searchQuery" type="button" class="search-clear-btn" @click="emit('update:searchQuery', '')">✕</button>
      </div>
    </div>
  </div>
</template>

<style scoped>
@import '../../assets/styles/views/explorer.css';
</style>
