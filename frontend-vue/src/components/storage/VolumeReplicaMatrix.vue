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
.topology-matrix-wrap {
  padding: 12px;
  border-radius: var(--rounded-lg, 8px);
  background: var(--color-surface-card, #1e2329);
  border: 1px solid var(--color-hairline, #2b3139);
}
.matrix-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 10px;
}
.matrix-title {
  display: block;
  font-size: 11px;
  font-weight: 700;
  color: var(--color-on-dark, #ffffff);
  text-transform: uppercase;
  letter-spacing: 0.05em;
}
.matrix-sub {
  font-size: 10px;
  color: var(--color-muted, #707a8a);
  font-family: var(--font-mono, monospace);
}
.health-pill {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 2px 7px;
  border-radius: 9999px;
  font-size: 10px;
  font-weight: 700;
  font-family: var(--font-mono, monospace);
}
.health-dot { width: 6px; height: 6px; border-radius: 50%; background: currentColor; }
.badge-emerald { background: rgba(14, 203, 129, 0.15); color: #0ecb81; border: 1px solid rgba(14, 203, 129, 0.3); }
.badge-amber { background: rgba(252, 213, 53, 0.15); color: #fcd535; border: 1px solid rgba(252, 213, 53, 0.3); }
.badge-rose { background: rgba(246, 70, 93, 0.15); color: #f6465d; border: 1px solid rgba(246, 70, 93, 0.3); }

.replica-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 8px;
}
@media (max-width: 640px) { .replica-grid { grid-template-columns: 1fr; } }

.replica-chip {
  padding: 8px 10px;
  border-radius: var(--rounded-md, 6px);
  background: rgba(11, 14, 17, 0.6);
  border: 1px solid var(--color-hairline, #2b3139);
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.border-emerald { border-color: rgba(14, 203, 129, 0.4); box-shadow: inset 0 0 10px rgba(14, 203, 129, 0.05); }
.border-amber { border-color: rgba(252, 213, 53, 0.4); box-shadow: inset 0 0 10px rgba(252, 213, 53, 0.05); }
.border-rose { border-color: rgba(246, 70, 93, 0.4); box-shadow: inset 0 0 10px rgba(246, 70, 93, 0.05); }

.chip-top, .chip-bottom { display: flex; align-items: center; justify-content: space-between; gap: 4px; }
.node-ident { display: flex; align-items: center; gap: 5px; overflow: hidden; }
.node-icon { width: 13px; height: 13px; color: var(--color-muted, #707a8a); flex-shrink: 0; }
.node-name { font-size: 11px; font-weight: 600; color: var(--color-body, #eaecef); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.mode-badge { font-size: 9px; padding: 1px 5px; border-radius: 4px; background: rgba(255, 255, 255, 0.08); color: #a0aec0; }

.status-indicator { display: flex; align-items: center; gap: 4px; }
.pulse-dot { width: 6px; height: 6px; border-radius: 50%; }
.dot-emerald { background: #0ecb81; box-shadow: 0 0 6px #0ecb81; }
.dot-amber { background: #fcd535; box-shadow: 0 0 6px #fcd535; }
.dot-rose { background: #f6465d; box-shadow: 0 0 6px #f6465d; }
.status-text { font-size: 10px; color: var(--color-body, #eaecef); }
.node-ip { font-size: 9px; color: var(--color-muted, #707a8a); }
</style>
