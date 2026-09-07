<script setup lang="ts">
import type { DetectedTool } from '../../api/ecosystem'

interface Props {
  tools: DetectedTool[]
  deletingId?: string | null
  syncingId?: string | null
  getToolIcon: (tool: DetectedTool) => string
  formatRelativeTime: (dateStr: string) => string
}

defineProps<Props>()

const emit = defineEmits<{
  (e: 'inspectHealth', tool: DetectedTool): void
  (e: 'configure', tool: DetectedTool): void
  (e: 'sync', tool: DetectedTool): void
  (e: 'delete', tool: DetectedTool): void
}>()
</script>

<template>
  <div class="mobile-cards-stream">
    <div
      v-for="tool in tools"
      :key="tool.id || tool.name"
      class="mobile-tool-item glass-panel"
      :class="{
        'border-healthy': tool.health === 'healthy',
        'border-degraded': tool.health === 'degraded' || tool.status === 'unreachable',
        'border-unconfigured': tool.status === 'not_configured'
      }"
    >
      <div class="mobile-tool-left">
        <span class="mobile-tool-icon">{{ getToolIcon(tool) }}</span>
        <div class="mobile-tool-details">
          <span class="mobile-tool-name">{{ tool.name }}</span>
          <div class="mobile-tool-sub">
            <span class="category-badge">{{ tool.category.toUpperCase() }}</span>
            <span>•</span>
            <span class="font-mono text-muted">{{ formatRelativeTime(tool.last_checked) }}</span>
          </div>
        </div>
      </div>

      <div class="mobile-tool-right">
        <span
          v-if="tool.health === 'healthy'"
          class="status-pill pill-healthy"
        >
          🟢 OK
        </span>
        <span
          v-else-if="tool.status === 'unreachable'"
          class="status-pill pill-degraded"
        >
          🔴 Down
        </span>
        <span
          v-else
          class="status-pill pill-warning"
        >
          🟡 Degraded
        </span>

        <button
          class="mobile-action-btn"
          title="Inspect Health"
          @click="emit('inspectHealth', tool)"
        >
          🔍
        </button>
        <button
          class="mobile-action-btn"
          :disabled="syncingId === tool.id"
          title="Sync"
          @click="emit('sync', tool)"
        >
          {{ syncingId === tool.id ? '⏳' : '🔄' }}
        </button>
        <button
          class="mobile-action-btn btn-card-delete"
          :disabled="deletingId === tool.id"
          title="Disconnect"
          @click="emit('delete', tool)"
        >
          🗑
        </button>
      </div>
    </div>
  </div>
</template>
