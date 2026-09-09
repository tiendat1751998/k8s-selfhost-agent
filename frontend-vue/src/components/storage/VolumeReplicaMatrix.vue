<script setup lang="ts">
import { computed } from 'vue'
import type { DistributedVolume, VolumeReplica } from '../../api/storage'

const props = defineProps<{
  volume: DistributedVolume
}>()

const healthInfo = computed(() => {
  const h = (props.volume.health || '').toLowerCase()
  if (['healthy', 'synced', 'ready'].includes(h)) return { label: 'HEALTHY', cls: 'badge-emerald' }
  if (['degraded', 'syncing', 'warning'].includes(h)) return { label: 'DEGRADED', cls: 'badge-amber' }
  return { label: 'FAULTED', cls: 'badge-rose' }
})

const replicaSlots = computed<VolumeReplica[]>(() => {
  const list = [...(props.volume.replicas || [])]
  while (list.length < 3) {
    const slotIdx = list.length + 1
    list.push({ node: `node-0${slotIdx}.cluster.local`, status: 'faulted', mode: 'Offline' })
  }
  return list.slice(0, 3)
})

function getStatusStyle(status: string) {
  const s = (status || '').toLowerCase()
  if (['synced', 'healthy', 'ready', 'active'].includes(s)) {
    return { chip: 'border-emerald', dot: 'dot-emerald', label: 'Synced' }
  }
  if (['syncing', 'degraded', 'warning', 'pending', 'rebuilding'].includes(s)) {
    return { chip: 'border-amber', dot: 'dot-amber', label: s === 'syncing' ? 'Syncing' : 'Degraded' }
  }
  return { chip: 'border-rose', dot: 'dot-rose', label: s === 'faulted' ? 'Faulted' : status || 'Faulted' }
}
</script>

<template>
  <div class="topology-matrix-wrap">
    <div class="matrix-header">
      <div>
        <span class="matrix-title">Distributed Replica Matrix</span>
        <span class="matrix-sub">3-Node Physical Quorum</span>
      </div>
      <span class="health-pill" :class="healthInfo.cls">
        <span class="health-dot" />{{ healthInfo.label }}
      </span>
    </div>

    <div class="replica-grid" role="list" aria-label="Volume Replicas">
      <div 
        v-for="(replica, idx) in replicaSlots" 
        :key="`${replica.node}-${idx}`"
        class="replica-chip"
        :class="getStatusStyle(replica.status).chip"
        role="listitem"
      >
        <div class="chip-top">
          <div class="node-ident">
            <svg class="node-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <rect x="2" y="2" width="20" height="8" rx="2" ry="2"/>
              <rect x="2" y="14" width="20" height="8" rx="2" ry="2"/>
              <line x1="6" y1="6" x2="6.01" y2="6"/><line x1="6" y1="18" x2="6.01" y2="18"/>
            </svg>
            <span class="node-name font-mono" :title="replica.node">{{ replica.node }}</span>
          </div>
          <span class="mode-badge font-mono">{{ replica.mode }}</span>
        </div>

        <div class="chip-bottom">
          <div class="status-indicator">
            <span class="pulse-dot" :class="getStatusStyle(replica.status).dot" />
            <span class="status-text font-mono">{{ getStatusStyle(replica.status).label }}</span>
          </div>
          <span v-if="replica.ip" class="node-ip font-mono">{{ replica.ip }}</span>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
@import '../../assets/styles/components/storage.css';
</style>
