<script setup lang="ts">
import StatusBadge from '../ui/StatusBadge.vue'
import type { DockerContainer } from '../../api/compute'

const props = defineProps<{
  containers: DockerContainer[]
  actionLoading?: string | null
}>()

const emit = defineEmits<{
  (e: 'toggle', container: DockerContainer): void
  (e: 'logs', id: string, name: string): void
}>()

function formatDate(d?: string): string {
  if (!d) return '-'
  try {
    return new Date(d).toLocaleDateString([], { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' })
  } catch {
    return d
  }
}
</script>

<template>
  <div class="containers-rack-view animate-fade-in">
    <div v-if="props.containers.length === 0" class="empty-state glass-panel">
      <span>No standalone containers found.</span>
    </div>

    <div v-else class="containers-grid">
      <div v-for="cont in props.containers" :key="cont.id" class="container-card glass-panel">
        <div class="cont-top">
          <div class="cont-title-wrap">
            <span class="cont-box-icon" aria-hidden="true">📦</span>
            <div>
              <h3 class="cont-name">{{ cont.name }}</h3>
              <span class="cont-img font-mono text-muted">{{ cont.image }}</span>
            </div>
          </div>

          <!-- Visual Power Toggle Switch -->
          <div class="power-switch-wrap">
            <button
              class="power-switch-btn"
              :class="cont.state === 'running' ? 'power-on' : 'power-off'"
              :disabled="props.actionLoading === `toggle-${cont.id}`"
              title="Toggle Container Power"
              @click="emit('toggle', cont)"
            >
              <span class="power-icon">⏻</span>
              <span class="power-state-text font-mono">{{ cont.state === 'running' ? 'ON' : 'OFF' }}</span>
            </button>
          </div>
        </div>

        <div class="cont-status-line font-mono">
          <StatusBadge :status="cont.state" size="sm" />
          <span class="cont-status-msg text-muted">{{ cont.status }}</span>
        </div>

        <div class="cont-footer">
          <span class="cont-date font-mono text-muted">Started: {{ formatDate(cont.created) }}</span>
          <button class="btn btn-secondary btn-xs" @click="emit('logs', cont.id, cont.name)">
            <span>📜 Live Output</span>
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
