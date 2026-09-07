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
.explorer-hud-container { display: flex; flex-direction: column; gap: 10px; padding: 12px 16px; border-radius: 12px; }
.hud-top-row { display: flex; align-items: center; justify-content: space-between; gap: 12px; flex-wrap: wrap; }
.hud-selectors { display: flex; align-items: center; gap: 10px; }
.selector-group { display: flex; align-items: center; gap: 6px; }
.hud-label { font-size: 0.72rem; color: #94a3b8; font-family: var(--font-mono); text-transform: uppercase; }
.hud-select { padding: 4px 8px; font-size: 0.75rem; min-width: 120px; }
.hud-telemetry-cards { display: flex; align-items: center; gap: 8px; }
.telemetry-card { display: flex; align-items: baseline; gap: 4px; background: rgba(255, 255, 255, 0.03); border: 1px solid rgba(255, 255, 255, 0.06); padding: 2px 7px; border-radius: 6px; }
.telemetry-label { font-size: 0.68rem; color: #64748b; text-transform: uppercase; }
.telemetry-val { font-size: 0.82rem; font-weight: 700; }
.hud-actions { display: flex; align-items: center; gap: 6px; }
.hud-bottom-row { display: flex; align-items: center; justify-content: space-between; gap: 12px; flex-wrap: wrap; }
.filter-chips-scroller { display: flex; align-items: center; gap: 5px; overflow-x: auto; padding-bottom: 2px; }
.hud-chip { display: inline-flex; align-items: center; gap: 5px; padding: 3px 9px; border-radius: 9999px; background: rgba(255, 255, 255, 0.04); border: 1px solid rgba(255, 255, 255, 0.08); color: #94a3b8; font-size: 0.74rem; cursor: pointer; white-space: nowrap; }
.hud-chip:hover { border-color: rgba(6, 182, 212, 0.35); color: #e2e8f0; }
.hud-chip.is-active { background: rgba(6, 182, 212, 0.15); border-color: #06b6d4; color: #38bdf8; font-weight: 600; }
.hud-search-box { position: relative; display: flex; align-items: center; flex: 1; max-width: 260px; min-width: 160px; }
.hud-search-input { width: 100%; padding: 4px 24px 4px 8px; font-size: 0.75rem; }
.search-clear-btn { position: absolute; right: 6px; background: none; border: none; color: #64748b; cursor: pointer; font-size: 0.72rem; }
</style>
