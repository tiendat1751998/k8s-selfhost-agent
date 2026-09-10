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
    <!-- Dedicated Empty State -->
    <div v-if="tools.length === 0" class="mobile-empty-state glass-panel">
      <p class="mobile-empty-text">
        <BaseIcon name="globe" size="sm" /> No ecosystem integrations connected. Tap + Connect to link an integration.
      </p>
    </div>

    <!-- Mobile Cards List -->
    <div
      v-else
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
          <span class="mobile-tool-name" :title="tool.name">{{ tool.name }}</span>
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
        ><BaseIcon name="check-circle" size="xs" /></span>
        <span
          v-else-if="tool.status === 'unreachable'"
          class="status-pill pill-degraded"
        ><BaseIcon name="x-circle" size="xs" /></span>
        <span
          v-else
          class="status-pill pill-warning"
        ><BaseIcon name="alert-triangle" size="xs" /></span>

        <button
          class="mobile-action-btn btn-ping"
          :disabled="syncingId === tool.id"
          title="Ping"
          aria-label="Ping"
          @click="emit('sync', tool)"
        >
          <BaseIcon :name="syncingId === tool.id ? 'refresh' : 'zap'" size="xs" :class="{ 'spin-anim': syncingId === tool.id }" />
        </button>
        <button
          class="mobile-action-btn btn-health"
          title="Health"
          aria-label="Health"
          @click="emit('inspectHealth', tool)"
        ><BaseIcon name="activity" size="xs" /></button>
        <button
          class="mobile-action-btn btn-config"
          title="Config"
          aria-label="Config"
          @click="emit('configure', tool)"
        ><BaseIcon name="sliders" size="xs" /></button>
        <button
          class="mobile-action-btn btn-card-delete"
          :disabled="deletingId === tool.id"
          title="Disconnect"
          aria-label="Disconnect"
          @click="emit('delete', tool)"
        ><BaseIcon name="trash" size="xs" /></button>
      </div>
    </div>
  </div>
</template>
