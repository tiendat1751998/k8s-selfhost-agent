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
      <thead>
        <tr>
          <th>Integration Tool</th>
          <th>Endpoint & Version</th>
          <th>Health Status</th>
          <th>Discovery</th>
          <th>Last Checked</th>
          <th style="text-align: right;">Actions</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="tool in tools" :key="tool.id || tool.name">
          <td>
            <div class="table-tool-cell">
              <span class="table-tool-icon">{{ getToolIcon(tool) }}</span>
              <div class="table-tool-meta">
                <span class="table-tool-name">{{ tool.name }}</span>
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
              ⚪ Not Configured
            </span>
            <span
              v-else-if="tool.health === 'healthy'"
              class="status-pill pill-healthy"
            >
              🟢 Healthy
            </span>
            <span
              v-else-if="tool.status === 'unreachable'"
              class="status-pill pill-degraded"
            >
              🔴 Unreachable
            </span>
            <span
              v-else
              class="status-pill pill-warning"
            >
              🟡 Degraded
            </span>
          </td>
          <td>
            <span class="source-badge">
              {{ tool.source === 'settings' ? '⚙️ Settings' : tool.source === 'manual' ? '✍️ Manual' : '☸️ K8s' }}
            </span>
          </td>
          <td class="font-mono text-muted" style="font-size: 12px;">
            {{ formatRelativeTime(tool.last_checked) }}
          </td>
          <td>
            <div class="table-actions-cell">
              <button
                class="table-btn btn-sync"
                :disabled="syncingId === tool.id"
                title="Sync Webhook Probe"
                @click="emit('sync', tool)"
              >
                <span>{{ syncingId === tool.id ? '⏳' : '🔄' }}</span>
                <span>Sync</span>
              </button>
              <button
                class="table-btn btn-config"
                title="Configure Integration"
                @click="emit('configure', tool)"
              >
                <span>⚙️</span>
                <span>Configure</span>
              </button>
              <button
                class="table-btn btn-health"
                title="Inspect Health Latency & Logs"
                @click="emit('inspectHealth', tool)"
              >
                <span>🔍</span>
                <span>Health</span>
              </button>
              <button
                class="table-btn btn-disconnect"
                :disabled="deletingId === tool.id"
                title="Disconnect Integration"
                @click="emit('delete', tool)"
              >
                <span>🗑</span>
                <span>Disconnect</span>
              </button>
            </div>
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>
