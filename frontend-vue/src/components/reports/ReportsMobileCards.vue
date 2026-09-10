<script setup lang="ts">
import type { PlatformReport } from '../../composables/useReports'
import StatusBadge from '../ui/StatusBadge.vue'

defineProps<{
  reports: PlatformReport[]
  loading?: boolean
}>()

const emit = defineEmits<{
  preview: [report: PlatformReport]
  downloadPdf: [report: PlatformReport]
  viewCsv: [report: PlatformReport]
  delete: [id: string]
}>()

function onDownload(item: PlatformReport) {
  if (item.format === 'csv') {
    emit('viewCsv', item)
  } else {
    emit('downloadPdf', item)
  }
}
</script>

<template>
  <div class="reports-mobile-stream">
    <div v-if="loading && reports.length === 0" class="text-center py-6 text-muted text-xs">
      Loading compiled platform reports...
    </div>

    <!-- Dedicated Empty State -->
    <div v-else-if="reports.length === 0" class="mobile-reports-empty">
      <div class="empty-icon">📊</div>
      <p class="empty-text">📊 No generated reports available. Tap ➕ Schedule to create a report.</p>
    </div>

    <div 
      v-for="item in reports" 
      :key="item.id" 
      class="mobile-card-item"
    >
      <div class="mobile-card-top">
        <div class="mobile-card-title-wrap">
          <span class="mobile-card-title" :title="item.title">{{ item.title }}</span>
        </div>
        <div class="mobile-card-badges">
          <span 
            class="badge font-mono text-xs font-bold"
            :class="item.type === 'compliance' ? 'badge-emerald' : item.type === 'security' ? 'badge-rose' : item.type === 'cost' ? 'badge-amber' : 'badge-cyan'"
          >
            {{ item.type.slice(0, 4).toUpperCase() }}
          </span>
          <span class="format-chip font-mono uppercase">{{ item.format }}</span>
        </div>
      </div>

      <div class="mobile-card-bottom">
        <div class="mobile-card-meta">
          <span class="font-mono text-cyan font-bold">{{ item.id }}</span>
          <span>•</span>
          <span>{{ new Date(item.created_at).toLocaleDateString() }}</span>
          <span>•</span>
          <StatusBadge 
            :status="item.status === 'completed' ? 'healthy' : item.status === 'generating' ? 'polling' : 'standby'" 
            :label="item.status.toUpperCase()" 
          />
        </div>

        <div class="mobile-card-actions">
          <button 
            class="mobile-action-btn" 
            :title="`Download ${item.format.toUpperCase()}`"
            :aria-label="`Download ${item.format.toUpperCase()}`"
            @click="onDownload(item)"
          >
            <span>📥</span>
          </button>
          <button 
            class="mobile-action-btn" 
            title="View Report"
            aria-label="View Report"
            @click="emit('preview', item)"
          >
            <span>👁️</span>
          </button>
          <button 
            class="mobile-action-btn btn-delete-crimson" 
            title="Delete Report"
            aria-label="Delete Report"
            @click="emit('delete', item.id)"
          >
            <span>🗑️</span>
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
