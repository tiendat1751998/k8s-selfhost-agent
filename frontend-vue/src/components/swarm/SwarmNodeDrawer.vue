<script setup lang="ts">
import StatusBadge from '../ui/StatusBadge.vue'
import ModalDrawer from '../ui/ModalDrawer.vue'
import type { DockerNode, NodeDetails } from '../../api/compute'

const props = defineProps<{
  show: boolean
  node: DockerNode | null
  details: NodeDetails | null
  loading?: boolean
}>()

const emit = defineEmits<{
  (e: 'update:show', value: boolean): void
  (e: 'close'): void
}>()

function formatDate(d?: string): string {
  if (!d) return '-'
  try {
    return new Date(d).toLocaleDateString([], { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' })
  } catch {
    return d
  }
}

function formatMemory(mem?: number): string {
  if (!mem || mem <= 0) return '--'
  if (mem >= 1024 * 1024 * 1024) {
    return `${(mem / (1024 * 1024 * 1024)).toFixed(1)} GB`
  }
  if (mem >= 1024 * 1024) {
    return `${(mem / (1024 * 1024)).toFixed(0)} MB`
  }
  return `${mem} B`
}
</script>

<template>
  <ModalDrawer
    :show="props.show"
    mode="drawer"
    :title="`Node Details: ${props.node?.hostname || props.node?.name || 'Compute Node'}`"
    subtitle="Complete hardware specifications, network interfaces, and placement labels"
    max-width="680px"
    @update:show="emit('update:show', $event)"
    @close="emit('close')"
  >
    <div v-if="props.loading" class="drawer-loading">
      <div class="spinner"></div>
      <span>Retrieving node specifications & labels...</span>
    </div>

    <div v-else-if="props.details" class="node-details-body">
      <!-- Header Status Card -->
      <div class="node-detail-header-card glass-panel">
        <div class="detail-hero-top">
          <div class="detail-id-wrap">
            <span class="detail-icon" aria-hidden="true">🖥️</span>
            <div>
              <h3 class="detail-hostname font-mono">{{ props.details.hostname }}</h3>
              <span class="detail-node-id font-mono text-muted">ID: {{ props.details.id }}</span>
            </div>
          </div>

          <div class="detail-badges">
            <span class="badge" :class="props.details.role === 'manager' ? 'badge-violet' : 'badge-cyan'">
              {{ (props.details.role || 'worker').toUpperCase() }}
            </span>
            <StatusBadge :status="props.details.availability" size="sm" />
          </div>
        </div>

        <div class="detail-quick-stats">
          <div class="quick-stat-item">
            <span class="stat-lbl">STATUS</span>
            <span class="stat-val font-mono" :class="props.details.status === 'ready' ? 'text-emerald' : 'text-rose'">
              {{ (props.details.status || 'unknown').toUpperCase() }}
            </span>
          </div>
          <div class="quick-stat-item">
            <span class="stat-lbl">JOINED CLUSTER</span>
            <span class="stat-val font-mono text-muted">{{ formatDate(props.details.joined_at) }}</span>
          </div>
          <div class="quick-stat-item">
            <span class="stat-lbl">PRIMARY IP</span>
            <span class="stat-val font-mono text-cyan">{{ props.details.ip || '--' }}</span>
          </div>
        </div>
      </div>

      <!-- Hardware Specs Section -->
      <div class="detail-section">
        <h4 class="detail-section-title">Hardware & Compute Architecture</h4>
        <div class="specs-grid">
          <div class="spec-card glass-panel">
            <div class="spec-icon" aria-hidden="true">⚡</div>
            <div class="spec-info">
              <span class="spec-label">CPU CORES</span>
              <span class="spec-value font-mono">{{ props.details.cpus ? `${props.details.cpus} vCPU Cores` : '--' }}</span>
            </div>
          </div>

          <div class="spec-card glass-panel">
            <div class="spec-icon" aria-hidden="true">🧠</div>
            <div class="spec-info">
              <span class="spec-label">SYSTEM MEMORY</span>
              <span class="spec-value font-mono">{{ formatMemory(props.details.memory) }}</span>
            </div>
          </div>

          <div class="spec-card glass-panel">
            <div class="spec-icon" aria-hidden="true">🐧</div>
            <div class="spec-info">
              <span class="spec-label">OPERATING SYSTEM</span>
              <span class="spec-value font-mono">{{ props.details.os && props.details.os !== '--' ? `${props.details.os} (${props.details.architecture || '--'})` : '--' }}</span>
            </div>
          </div>

          <div class="spec-card glass-panel">
            <div class="spec-icon" aria-hidden="true">🐳</div>
            <div class="spec-info">
              <span class="spec-label">DOCKER ENGINE</span>
              <span class="spec-value font-mono">{{ props.details.engine_version && props.details.engine_version !== '--' ? `v${props.details.engine_version}` : '--' }}</span>
            </div>
          </div>
        </div>
      </div>

      <!-- Node Labels Section -->
      <div class="detail-section">
        <div class="labels-header-row">
          <h4 class="detail-section-title">Node Metadata & Placement Labels</h4>
          <span class="count-pill">{{ Object.keys(props.details.labels || {}).length }}</span>
        </div>

        <div v-if="!props.details.labels || Object.keys(props.details.labels).length === 0" class="empty-labels glass-panel text-muted font-mono">
          No custom placement labels assigned to this node.
        </div>

        <div v-else class="labels-table-wrap glass-panel">
          <table class="labels-table font-mono">
            <thead>
              <tr>
                <th>LABEL KEY</th>
                <th>ASSIGNED VALUE</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="(val, key) in props.details.labels" :key="key">
                <td class="text-cyan">{{ key }}</td>
                <td class="text-emerald">{{ val }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </div>

    <template #footer="{ close }">
      <div class="drawer-footer-actions">
        <button class="btn btn-secondary" @click="close">Close</button>
      </div>
    </template>
  </ModalDrawer>
</template>
