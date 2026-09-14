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
  <div class="ecosystem-table-container glass-panel">
    <table class="ecosystem-table">
      <colgroup>
        <col style="width: 22%;" />
        <col style="width: 24%;" />
        <col style="width: 16%;" />
        <col style="width: 12%;" />
        <col style="width: 10%;" />
        <col style="width: 16%;" />
      </colgroup>
      <thead>
        <tr>
          <th style="width: 22%;">Integration Tool</th>
          <th style="width: 24%;">Endpoint & Version</th>
          <th style="width: 16%;">Health Status</th>
          <th style="width: 12%;">Discovery</th>
          <th style="width: 10%;">Last Checked</th>
          <th style="width: 16%; text-align: right;">Actions</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="tool in tools" :key="tool.id || tool.name">
          <td>
            <div class="table-tool-cell">
              <span class="table-tool-icon"><BaseIcon :name="getToolIcon(tool)" size="sm" /></span>
              <div class="table-tool-meta">
                <span class="table-tool-name" :title="tool.name">{{ tool.name }}</span>
                <span class="category-badge">{{ tool.category.toUpperCase() }}</span>
              </div>
            </div>
          </td>
          <td>
            <div class="endpoint-val font-mono">
              <a
                v-if="tool.endpoint"
                :href="tool.endpoint"
                target="_blank"
                rel="noopener noreferrer"
                class="endpoint-link"
                :title="tool.endpoint"
              >
                {{ tool.endpoint }}
              </a>
              <span v-else class="endpoint-empty">Unset</span>
            </div>
            <div v-if="tool.version" class="version-badge font-mono" style="margin-top: 4px; display: inline-block;">
              {{ tool.version }}
            </div>
          </td>
          <td>
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
          </td>
          <td>
            <span class="source-badge">
              <template v-if="tool.source === 'settings'"><BaseIcon name="sliders" size="xs" /> Settings</template><template v-else-if="tool.source === 'manual'"><BaseIcon name="edit" size="xs" /> Manual</template><template v-else><BaseIcon name="anchor" size="xs" /> K8s</template>
            </span>
          </td>
          <td class="font-mono text-muted" style="font-size: 12px;">
            {{ formatRelativeTime(tool.last_checked) }}
          </td>
          <td>
            <div class="table-actions-cell">
              <button
                class="table-btn btn-ping"
                :disabled="syncingId === tool.id"
                title="Ping"
                aria-label="Ping"
                @click="emit('sync', tool)"
              >
                <BaseIcon :name="syncingId === tool.id ? 'refresh' : 'zap'" size="xs" :class="{ 'spin-anim': syncingId === tool.id }" />
              </button>
              <button
                class="table-btn btn-health"
                title="Health"
                aria-label="Health"
                @click="emit('inspectHealth', tool)"
              >
                <BaseIcon name="activity" size="xs" />
              </button>
              <button
                class="table-btn btn-config"
                title="Config"
                aria-label="Config"
                @click="emit('configure', tool)"
              >
                <BaseIcon name="sliders" size="xs" />
              </button>
              <button
                class="table-btn btn-disconnect"
                :disabled="deletingId === tool.id"
                title="Disconnect"
                aria-label="Disconnect"
                @click="emit('delete', tool)"
              >
                <BaseIcon name="trash" size="xs" />
              </button>
            </div>
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>
