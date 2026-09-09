<script setup lang="ts">
import ModalDrawer from '../ui/ModalDrawer.vue'
import type { RenderResponse, Template } from '../../api/scaffold'
import type { ScaffolderLogEntry } from '../../composables/useScaffolder'

defineProps<{
  show: boolean
  activeTemplate: Template | null
  renderResult: RenderResponse | null
  activeOutputTab: 'yaml' | 'compose' | 'helm' | 'logs'
  copySuccess: boolean
  logs: ScaffolderLogEntry[]
  isDryRun: boolean
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'update:activeOutputTab', tab: 'yaml' | 'compose' | 'helm' | 'logs'): void
  (e: 'copy'): void
  (e: 'download'): void
}>()
</script>

<template>
  <ModalDrawer
    v-if="show && activeTemplate"
    :show="show"
    :title="isDryRun ? `Dry-Run Manifest Preview: ${activeTemplate.name}` : `Scaffold Artifacts & Logs: ${activeTemplate.name}`"
    @close="emit('close')"
  >
    <div class="logs-drawer-content">
      <!-- Header Toolbar -->
      <div class="drawer-header-bar">
        <div class="drawer-tabs">
          <button
            :class="['drawer-tab-btn', { active: activeOutputTab === 'yaml' }]"
            @click="emit('update:activeOutputTab', 'yaml')"
          >
            ☸️ K8s Manifest
          </button>
          <button
            :class="['drawer-tab-btn', { active: activeOutputTab === 'compose' }]"
            @click="emit('update:activeOutputTab', 'compose')"
          >
            🐳 Docker Compose
          </button>
          <button
            :class="['drawer-tab-btn', { active: activeOutputTab === 'helm' }]"
            @click="emit('update:activeOutputTab', 'helm')"
          >
            ⛵ Helm Values
          </button>
          <button
            :class="['drawer-tab-btn', { active: activeOutputTab === 'logs' }]"
            @click="emit('update:activeOutputTab', 'logs')"
          >
            📜 Stream Logs ({{ logs.length }})
          </button>
        </div>

        <div class="drawer-actions">
          <button
            class="btn-icon-action"
            :title="copySuccess ? 'Copied!' : 'Copy Code'"
            :disabled="!renderResult && logs.length === 0"
            @click="emit('copy')"
          >
            {{ copySuccess ? '✅' : '📋' }}
          </button>
          <button
            class="btn-icon-action"
            title="Download File"
            :disabled="!renderResult && logs.length === 0"
            @click="emit('download')"
          >
            💾
          </button>
        </div>
      </div>

      <!-- Real-Time Log Terminal Window -->
      <div v-if="activeOutputTab === 'logs'" class="terminal-logs-window">
        <div v-if="logs.length === 0" class="code-placeholder">
          <span class="placeholder-icon">📜</span>
          <p>No execution logs yet.</p>
        </div>
        <div
          v-for="log in logs"
          :key="log.id"
          class="terminal-log-line"
        >
          <span class="log-time">[{{ log.timestamp }}]</span>
          <span :class="['log-badge', log.level]">{{ log.level.toUpperCase() }}</span>
          <span class="log-text">{{ log.message }}</span>
        </div>
      </div>

      <!-- Rendered Code Viewer (YAML, Compose, Helm) -->
      <div v-else class="code-editor-viewer glass-panel">
        <template v-if="renderResult">
          <pre v-if="activeOutputTab === 'yaml'" class="code-content"><code>{{ renderResult.rendered_yaml }}</code></pre>
          <pre v-else-if="activeOutputTab === 'compose'" class="code-content"><code>{{ renderResult.rendered_compose }}</code></pre>
          <pre v-else-if="activeOutputTab === 'helm'" class="code-content"><code>{{ renderResult.rendered_helm }}</code></pre>
        </template>
        <div v-else class="code-placeholder">
          <span class="placeholder-icon">📄</span>
          <p>Click <strong>"Generate Manifests"</strong> in the wizard to render template files.</p>
        </div>
      </div>

      <!-- Service Catalog Success Alert -->
      <div v-if="renderResult?.catalog_entry_id" class="catalog-success-alert glass-panel">
        <span>📚 Registered in Service Catalog (Catalog UUID: <code>{{ renderResult.catalog_entry_id }}</code>)</span>
      </div>
    </div>
  </ModalDrawer>
</template>
