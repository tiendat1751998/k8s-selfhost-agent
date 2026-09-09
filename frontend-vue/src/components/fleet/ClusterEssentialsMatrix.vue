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
@import '../../assets/styles/views/fleet.css';
</style>
