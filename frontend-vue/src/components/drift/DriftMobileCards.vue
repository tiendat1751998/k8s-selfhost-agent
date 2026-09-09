<script setup lang="ts">
import type { EnrichedDriftRecord } from '../../composables/useDriftDetection'

interface Props {
  drifts: EnrichedDriftRecord[]
  loading: boolean
  resolvingId: string | null
}

defineProps<Props>()

const emit = defineEmits<{
  (e: 'inspect', drift: EnrichedDriftRecord): void
  (e: 'sync', id: string): void
}>()

function formatRelativeTime(d: string): string {
  if (!d) return ''
  try {
    const diffMs = Date.now() - new Date(d).getTime()
    const diffMins = Math.floor(diffMs / 60000)
    if (diffMins < 1) return 'just now'
    if (diffMins < 60) return `${diffMins}m ago`
    const diffHrs = Math.floor(diffMins / 60)
    if (diffHrs < 24) return `${diffHrs}h ago`
    return `${Math.floor(diffHrs / 24)}d ago`
  } catch {
    return ''
  }
}
</script>

<template>
  <div class="drift-mobile-stream">
    <div v-if="loading && drifts.length === 0" class="drift-mobile-empty text-muted font-mono">
      <span>⏳ Scanning workloads...</span>
    </div>

    <div v-else-if="drifts.length === 0" class="drift-mobile-empty text-muted font-mono">
      <span>✅ No configuration drift detected. All cluster workloads in sync.</span>
    </div>

    <div
      v-for="item in drifts"
      :key="item.id"
      class="drift-mobile-card"
      :class="[
        `border-sev-${item.severity}`,
        { 'is-drifted': item.status === 'drifted', 'is-suppressed': item.isSuppressed }
      ]"
    >
      <!-- Left: Compact Resource Identity & State (~65px aligned) -->
      <div class="mobile-card-main">
        <div class="mobile-card-top">
          <span class="mobile-kind-tag font-mono">{{ item.resource_kind }}</span>
          <span class="mobile-resource-name font-mono" :title="item.resource">{{ item.resource }}</span>
          <span 
            v-if="item.status === 'drifted' && !item.isSuppressed" 
            class="mobile-severity-dot"
            :class="`dot-${item.severity}`"
            :title="`Severity: ${item.severity}`"
          ></span>
        </div>
        <div class="mobile-card-meta font-mono">
          <span class="mobile-meta-scope">{{ item.namespace || 'default' }}</span>
          <span class="mobile-meta-divider">·</span>
          <span class="mobile-drift-type">{{ item.driftType }}</span>
          <span v-if="item.detected_at" class="mobile-meta-time">· {{ formatRelativeTime(item.detected_at) }}</span>
        </div>
      </div>

      <!-- Right: Touch-Friendly Quick Action Buttons (0 horizontal scroll) -->
      <div class="mobile-card-actions">
        <button
          class="mobile-action-btn btn-inspect"
          title="Inspect Diff"
          aria-label="Inspect Diff"
          @click="emit('inspect', item)"
        >
          <span>🔍</span>
        </button>
        <button
          v-if="item.status === 'drifted'"
          class="mobile-action-btn btn-sync"
          :class="{ 'btn-loading': resolvingId === item.id }"
          :disabled="resolvingId === item.id"
          title="Sync to Git"
          aria-label="Sync to Git"
          @click="emit('sync', item.id)"
        >
          <span>{{ resolvingId === item.id ? '⏳' : '⚡' }}</span>
        </button>
      </div>
    </div>
  </div>
</template>