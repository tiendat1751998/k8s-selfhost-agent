<script setup lang="ts">
import { ref, onMounted, watch, computed } from 'vue'
import { clusterApi, type EssentialItem } from '../../api/cluster'

const props = withDefaults(defineProps<{ clusterId: string; clusterName?: string; compact?: boolean }>(), { compact: false })
const emit = defineEmits<{ (e: 'bootstrap', itemId?: string): void }>()

const loading = ref(false)
const error = ref<string | null>(null)
const essentials = ref<EssentialItem[]>([])

const defaultDefs: EssentialItem[] = [
  { id: 'metrics-server', name: 'Metrics Server', status: 'not_found', description: 'Enables CPU & memory metrics collection for pods and nodes.' },
  { id: 'local-storage', name: 'Local StorageClass', status: 'not_found', description: 'Dynamic local persistent volume storage provisioning.' },
  { id: 'agent-daemonset', name: 'Agent DaemonSet', status: 'not_found', description: 'Node monitoring daemon for real-time telemetry streaming.' },
  { id: 'master-taints', name: 'Master Taints', status: 'pending', description: 'Controls control-plane workload scheduling.' },
]

async function loadEssentials() {
  if (!props.clusterId) return
  loading.value = true
  error.value = null
  try {
    const data = await clusterApi.getEssentialsStatus(props.clusterId)
    if (data && Array.isArray(data.essentials) && data.essentials.length > 0) {
      essentials.value = defaultDefs.map(def => {
        const found = data.essentials.find(i => i.id === def.id || i.name?.toLowerCase().includes(def.name.toLowerCase().split(' ')[0]))
        return found ? { ...def, ...found } : def
      })
    } else {
      essentials.value = defaultDefs
    }
  } catch (err: unknown) {
    error.value = err instanceof Error ? err.message : 'Unable to query essentials status'
    essentials.value = defaultDefs
  } finally {
    loading.value = false
  }
}

watch(() => props.clusterId, () => { loadEssentials() })
onMounted(() => { loadEssentials() })

function getBadge(status: string) {
  const s = (status || '').toLowerCase()
  if (['installed', 'running', 'ready', 'active', 'ok'].includes(s)) return { label: '🟢 Installed / Running', class: 'badge-emerald', ready: true }
  if (['pending', 'in_progress', 'configuring'].includes(s)) return { label: '🟡 Pending', class: 'badge-amber', ready: false }
  return { label: '🔴 Not Found', class: 'badge-rose', ready: false }
}

const readyCount = computed(() => essentials.value.filter(i => getBadge(i.status).ready).length)
defineExpose({ refresh: loadEssentials })
</script>

<template>
  <div class="essentials-matrix glass-panel" :class="{ 'is-compact': compact }">
    <div class="matrix-header">
      <div>
        <div class="matrix-title-row">
          <span class="matrix-icon">⚡</span>
          <h4 class="matrix-title">Cluster Essentials Readiness Matrix</h4>
          <span class="matrix-counter font-mono">⚡ {{ readyCount }} / {{ essentials.length }} READY</span>
        </div>
        <p class="matrix-subtitle">Core Kubernetes cluster primitives required for telemetry, storage, and workloads.</p>
      </div>
      <button class="btn btn-secondary btn-xs" :disabled="loading" @click="loadEssentials">
        <span>{{ loading ? '⏳ Probing...' : '🔄 Re-check' }}</span>
      </button>
    </div>

    <div v-if="error" class="matrix-error font-mono">⚠️ {{ error }}</div>

    <div class="matrix-list">
      <div v-for="item in essentials" :key="item.id" class="matrix-row" :class="{ 'is-ready': getBadge(item.status).ready }">
        <div class="matrix-info">
          <div class="item-name-row">
            <span class="item-name font-mono">{{ item.name }}</span>
            <span class="badge font-mono" :class="getBadge(item.status).class">{{ getBadge(item.status).label }}</span>
          </div>
          <p class="item-desc">{{ item.description }}</p>
        </div>
        <div class="matrix-action">
          <button v-if="!getBadge(item.status).ready" class="btn btn-primary btn-xs" @click="emit('bootstrap', item.id)">
            <span>⚡ Install</span>
          </button>
          <span v-else class="ready-text font-mono text-emerald">✓ Active</span>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.essentials-matrix { padding: 16px; border-radius: 14px; display: flex; flex-direction: column; gap: 12px; background: rgba(11, 15, 25, 0.7); border: 1px solid var(--border-subtle); }
.matrix-header { display: flex; justify-content: space-between; align-items: flex-start; gap: 10px; }
.matrix-title-row { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.matrix-icon { font-size: 16px; color: var(--accent-cyan); }
.matrix-title { font-size: 14px; font-weight: 700; color: #fff; }
.matrix-counter { font-size: 10px; font-weight: 700; padding: 2px 6px; border-radius: 4px; background: rgba(6, 182, 212, 0.15); color: var(--accent-cyan); border: 1px solid rgba(6, 182, 212, 0.3); }
.matrix-subtitle { font-size: 11.5px; color: var(--text-muted); margin-top: 2px; }
.matrix-error { font-size: 11px; color: #fb7185; background: rgba(244, 63, 94, 0.1); border: 1px solid rgba(244, 63, 94, 0.25); padding: 5px 8px; border-radius: 6px; }
.matrix-list { display: flex; flex-direction: column; gap: 8px; }
.matrix-row { display: flex; justify-content: space-between; align-items: center; padding: 9px 12px; border-radius: 9px; background: rgba(0, 0, 0, 0.25); border: 1px solid var(--border-subtle); transition: all 0.15s ease; }
.matrix-row:hover { border-color: var(--border-medium); background: rgba(0, 0, 0, 0.35); }
.matrix-row.is-ready { border-color: rgba(16, 185, 129, 0.2); }
.matrix-info { display: flex; flex-direction: column; gap: 3px; }
.item-name-row { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.item-name { font-size: 13px; font-weight: 700; color: #fff; }
.item-desc { font-size: 11px; color: var(--text-secondary); line-height: 1.35; }
.matrix-action { flex-shrink: 0; margin-left: 12px; }
.ready-text { font-size: 11.5px; font-weight: 600; }
.font-mono { font-family: var(--font-mono); }
.text-emerald { color: var(--accent-emerald); }
@media (max-width: 640px) {
  .matrix-row { flex-direction: column; align-items: flex-start; gap: 8px; }
  .matrix-action { width: 100%; margin-left: 0; }
  .matrix-action .btn { width: 100%; }
}
</style>
