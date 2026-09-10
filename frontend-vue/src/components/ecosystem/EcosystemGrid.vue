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
  <section class="tools-grid">
    <div
      v-for="tool in tools"
      :key="tool.id || tool.name"
      class="tool-card glass-panel"
      :class="{
        'border-healthy': tool.health === 'healthy',
        'border-degraded': tool.health === 'degraded' || tool.status === 'unreachable',
        'border-unconfigured': tool.status === 'not_configured'
      }"
    >
      <!-- Card Top Header -->
      <div class="card-header">
        <div class="tool-main-info">
          <div class="tool-icon-wrap">
            <BaseIcon :name="getToolIcon(tool)" size="md" />
          </div>
          <div style="min-width: 0;">
            <h3 class="tool-name" :title="tool.name">{{ tool.name }}</h3>
            <span class="category-badge">{{ tool.category.toUpperCase() }}</span>
          </div>
        </div>

        <div class="status-badge-wrap">
          <span
            v-if="tool.status === 'not_configured'"
            class="status-pill pill-muted"
          >
            <BaseIcon name="clock" size="xs" /> Not Configured
          </span>
          <span
            v-else-if="tool.health === 'healthy'"
            class="status-pill pill-healthy"
          >
            <BaseIcon name="check-circle" size="xs" /> Healthy
          </span>
          <span
            v-else-if="tool.status === 'unreachable'"
            class="status-pill pill-degraded"
          >
            <BaseIcon name="x-circle" size="xs" /> Unreachable
          </span>
          <span
            v-else
            class="status-pill pill-warning"
          >
            <BaseIcon name="alert-triangle" size="xs" /> Degraded
          </span>
        </div>
      </div>

      <!-- Card Body Details -->
      <div class="card-body">
        <div v-if="tool.version" class="detail-row">
          <span class="detail-label">Version:</span>
          <span class="version-badge font-mono">{{ tool.version }}</span>
        </div>

        <div class="detail-row">
          <span class="detail-label">Endpoint:</span>
          <div class="endpoint-val font-mono">
            <a
              v-if="tool.endpoint"
              :href="tool.endpoint"
              target="_blank"
              rel="noopener noreferrer"
              class="endpoint-link"
              :title="tool.endpoint"
            >
              {{ tool.endpoint }} ↗
            </a>
            <span v-else class="endpoint-empty">
              Not configured in Settings
            </span>
          </div>
        </div>

        <div class="detail-row">
          <span class="detail-label">Discovery Source:</span>
          <span class="source-badge" :class="`source-${tool.source}`">
            <template v-if="tool.source === 'settings'"><BaseIcon name="sliders" size="xs" /> Settings</template><template v-else-if="tool.source === 'manual'"><BaseIcon name="edit" size="xs" /> Manual</template><template v-else><BaseIcon name="anchor" size="xs" /> K8s</template>
          </span>
        </div>

        <!-- Metadata Tag Pills -->
        <div v-if="tool.metadata && Object.keys(tool.metadata).length > 0" class="metadata-tags">
          <span
            v-for="(val, key) in tool.metadata"
            :key="key"
            class="meta-pill"
          >
            <strong class="meta-k">{{ key }}:</strong> {{ val }}
          </span>
        </div>
      </div>

      <!-- Card Footer -->
      <div class="card-footer">
        <span class="last-checked">
          <BaseIcon name="clock" size="xs" /> {{ formatRelativeTime(tool.last_checked) }}
        </span>

        <div class="card-actions-quick">
          <button
            class="btn-card-action btn-ping"
            :disabled="syncingId === tool.id"
            title="Ping"
            aria-label="Ping"
            @click="emit('sync', tool)"
          >
            <BaseIcon :name="syncingId === tool.id ? 'refresh' : 'zap'" size="xs" :class="{ 'spin-anim': syncingId === tool.id }" />
            <span>Ping</span>
          </button>
          <button
            class="btn-card-action btn-health"
            title="Health"
            aria-label="Health"
            @click="emit('inspectHealth', tool)"
          >
            <BaseIcon name="activity" size="xs" />
            <span>Health</span>
          </button>
          <button
            class="btn-card-action btn-config"
            title="Config"
            aria-label="Config"
            @click="emit('configure', tool)"
          >
            <BaseIcon name="sliders" size="xs" />
          </button>
          <button
            v-if="tool.source === 'manual' || tool.id"
            class="btn-card-action btn-card-delete"
            :disabled="deletingId === tool.id"
            title="Disconnect"
            aria-label="Disconnect"
            @click="emit('delete', tool)"
          >
            <BaseIcon name="trash" size="xs" />
          </button>
        </div>
      </div>
    </div>
  </section>
</template>
