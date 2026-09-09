<template>
  <div class="mobile-cards-stream">
    <div v-if="loading" class="stream-status font-mono">
      <span class="spin-icon">⏳</span> Loading resources...
    </div>
    <div v-else-if="resources.length === 0" class="stream-empty glass-panel font-mono">
      <span class="empty-icon">📦</span>
      <p class="empty-text">{{ emptyMessage || 'No Kubernetes resources found' }}</p>
    </div>
    <div v-else class="cards-list">
      <div
        v-for="res in resources"
        :key="res.metadata?.uid || `${res.metadata?.namespace}-${res.metadata?.name}`"
        class="mobile-card glass-panel"
        @click="$emit('select', res)"
      >
        <div class="card-top-row">
          <span class="kind-badge font-mono">{{ getKindAbbr(res.kind || selectedKind) }}</span>
          <span v-if="res.metadata?.namespace" class="ns-pill font-mono">{{ res.metadata.namespace }}</span>
          <span class="res-name font-mono" :title="res.metadata?.name">{{ res.metadata?.name || 'unnamed' }}</span>
          <button type="button" class="copy-btn" title="Copy name" @click.stop="copyResourceName(res.metadata?.name)">
            <span v-if="copiedName === res.metadata?.name" class="copy-ok">✓</span>
            <svg v-else viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2"><rect x="9" y="9" width="13" height="13" rx="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/></svg>
          </button>
        </div>
        <div class="card-mid-row font-mono">
          <div class="status-chip" :class="`status-${getStatusClass(res)}`">
            <span class="status-dot"></span>
            <span class="status-text">{{ getStatusText(res) }}</span>
          </div>
          <div class="metrics-meta">
            <span v-if="getReplicas(res)" class="meta-item text-cyan">⚡ {{ getReplicas(res) }}</span>
            <span class="meta-item text-muted">🕒 {{ getAge(res) }}</span>
          </div>
        </div>
        <div class="card-actions-row" @click.stop>
          <button v-if="canLog(res)" type="button" class="btn-card-action btn-logs" title="Logs" @click="$emit('logs', res)"><span>📄 Logs</span></button>
          <button v-if="canScale(res)" type="button" class="btn-card-action btn-scale" title="Scale" @click="$emit('scale', res)"><span>⚡ Scale</span></button>
          <button v-if="canRestart(res)" type="button" class="btn-card-action btn-restart" title="Restart" @click="$emit('restart', res)"><span>🔄 Restart</span></button>
          <button v-if="canDelete(res)" type="button" class="btn-card-action btn-del" title="Delete" @click="$emit('delete', res)"><span>🗑 Delete</span></button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import type { K8sResource, ResourceKind } from '../../api/k8s'

const props = withDefaults(
  defineProps<{ resources: K8sResource[]; selectedKind: ResourceKind; loading?: boolean; emptyMessage?: string }>(),
  { loading: false, emptyMessage: '' }
)

defineEmits<{
  (e: 'select', resource: K8sResource): void
  (e: 'logs', resource: K8sResource): void
  (e: 'scale', resource: K8sResource): void
  (e: 'restart', resource: K8sResource): void
  (e: 'delete', resource: K8sResource): void
}>()

const copiedName = ref<string | null>(null)
function copyResourceName(name?: string) {
  if (!name) return
  navigator.clipboard.writeText(name)
  copiedName.value = name
  setTimeout(() => { if (copiedName.value === name) copiedName.value = null }, 1500)
}

function getKindAbbr(kind?: string): string {
  const k = (kind || props.selectedKind || '').toLowerCase()
  if (k.includes('pod')) return 'POD'
  if (k.includes('deploy')) return 'DEPLOY'
  if (k.includes('stateful')) return 'STS'
  if (k.includes('daemon')) return 'DS'
  if (k.includes('cron')) return 'CRON'
  if (k.includes('job')) return 'JOB'
  if (k.includes('service') && !k.includes('account')) return 'SVC'
  if (k.includes('ingress')) return 'ING'
  if (k.includes('configmap')) return 'CM'
  if (k.includes('secret')) return 'SECRET'
  if (k.includes('claim')) return 'PVC'
  if (k.includes('volume')) return 'PV'
  if (k.includes('horizontal')) return 'HPA'
  if (k.includes('node')) return 'NODE'
  return k.slice(0, 5).toUpperCase()
}

function getStatusText(r: K8sResource): string {
  const s = r.status as Record<string, unknown> | undefined
  if (typeof s?.phase === 'string') return s.phase
  if (typeof s?.readyReplicas === 'number' && typeof s?.replicas === 'number') return `${s.readyReplicas}/${s.replicas} Ready`
  if (props.selectedKind === 'nodes') return (r.spec as { unschedulable?: boolean })?.unschedulable ? 'Unschedulable' : 'Ready'
  return 'Active'
}

function getStatusClass(r: K8sResource): 'healthy' | 'warning' | 'error' | 'neutral' {
  const s = getStatusText(r).toLowerCase()
  if (s.includes('running') || s.includes('ready') || s.includes('active') || s.includes('bound')) return 'healthy'
  if (s.includes('pending') || s.includes('unschedulable') || s.includes('waiting')) return 'warning'
  if (s.includes('crash') || s.includes('fail') || s.includes('error') || s.includes('terminat')) return 'error'
  return 'neutral'
}

function getAge(r: K8sResource): string {
  const ts = r.metadata?.creationTimestamp
  if (!ts) return '-'
  const diff = Date.now() - new Date(ts).getTime()
  if (diff < 0 || isNaN(diff)) return 'now'
  const mins = Math.floor(diff / 60000)
  if (mins < 60) return `${mins}m`
  const hrs = Math.floor(mins / 60)
  return hrs < 24 ? `${hrs}h` : `${Math.floor(hrs / 24)}d`
}

function getReplicas(r: K8sResource): string {
  const spec = r.spec as { replicas?: number } | undefined
  const st = r.status as { readyReplicas?: number; replicas?: number; numberReady?: number; desiredNumberScheduled?: number } | undefined
  if (st?.readyReplicas !== undefined || spec?.replicas !== undefined) return `${st?.readyReplicas || 0}/${spec?.replicas ?? st?.replicas ?? 0}`
  if (st?.numberReady !== undefined && st?.desiredNumberScheduled !== undefined) return `${st.numberReady}/${st.desiredNumberScheduled}`
  return ''
}

const canLog = (r: K8sResource) => props.selectedKind === 'pods' || (r.kind || '').toLowerCase() === 'pod'
const canScale = (r: K8sResource) => /deploy|stateful|horizontal/i.test(r.kind || props.selectedKind || '')
const canRestart = (r: K8sResource) => /deploy|daemon/i.test(r.kind || props.selectedKind || '')
const canDelete = (r: K8sResource) => props.selectedKind !== 'events' && (r.kind || '').toLowerCase() !== 'event'
</script>

<style scoped>
@import '../../assets/styles/views/explorer.css';
</style>