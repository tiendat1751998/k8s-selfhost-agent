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
      <span>No standalone containers found matching your filter.</span>
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
              aria-label="Toggle Container Power"
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
          <button class="btn btn-secondary btn-xs" aria-label="View live logs" @click="emit('logs', cont.id, cont.name)">
            <span>📜 Live Output</span>
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.containers-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
  gap: 16px;
}

.container-card {
  padding: 18px;
  border-radius: 14px;
  display: flex;
  flex-direction: column;
  gap: 12px;
  background: rgba(11, 15, 25, 0.65);
}

.cont-top {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
}

.cont-title-wrap {
  display: flex;
  align-items: center;
  gap: 10px;
}

.cont-box-icon { font-size: 22px; }
.cont-name { font-size: 14px; font-weight: 700; color: #fff; }
.cont-img { font-size: 11px; }

.power-switch-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  padding: 6px 12px;
  min-height: 32px;
  height: 32px;
  border-radius: 9999px;
  font-size: 11px;
  font-weight: 700;
  cursor: pointer;
  transition: all 0.15s ease;
  border: 1px solid transparent;
  position: relative;
  touch-action: manipulation;
}

.power-switch-btn::after {
  content: '';
  position: absolute;
  inset: -4px;
}

.power-on {
  background: rgba(16, 185, 129, 0.15);
  border-color: rgba(16, 185, 129, 0.4);
  color: #34d399;
}

.power-on:hover {
  background: rgba(244, 63, 94, 0.2);
  border-color: rgba(244, 63, 94, 0.5);
  color: #fb7185;
}

.power-off {
  background: rgba(255, 255, 255, 0.06);
  border-color: var(--border-subtle);
  color: var(--text-muted);
}

.power-off:hover {
  background: rgba(16, 185, 129, 0.2);
  border-color: rgba(16, 185, 129, 0.5);
  color: #34d399;
}

.power-icon { font-size: 12px; }

.cont-status-line {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 11px;
}

.cont-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  border-top: 1px solid var(--border-subtle);
  padding-top: 10px;
}
</style>