<script setup lang="ts">
import StatusBadge from '../ui/StatusBadge.vue'
import type { DockerService, DockerNode, DockerContainer } from '../../api/compute'

const props = defineProps<{
  activeTab: 'services' | 'nodes' | 'containers'
  services: DockerService[]
  nodes: DockerNode[]
  containers: DockerContainer[]
  actionLoading?: string | null
}>()

const emit = defineEmits<{
  (e: 'inspect-service', service: DockerService): void
  (e: 'inspect-node', node: DockerNode): void
  (e: 'scale-service', service: DockerService, delta: number): void
  (e: 'view-logs', id: string, name: string, type: 'service' | 'container'): void
  (e: 'toggle-container', container: DockerContainer): void
}>()
</script>

<template>
  <div class="mobile-cards-stream animate-fade-in">
    <!-- 1. Services Mobile Cards Stream (~68-74px/item) -->
    <template v-if="props.activeTab === 'services'">
      <div v-if="props.services.length === 0" class="empty-state glass-panel">
        <span>No Swarm services match the current filter.</span>
      </div>
      <div
        v-for="svc in props.services"
        :key="svc.id"
        class="mobile-stream-card"
        @click="emit('inspect-service', svc)"
      >
        <div class="mobile-card-left">
          <span class="mobile-card-icon" aria-hidden="true">🐳</span>
          <div class="mobile-card-text">
            <span class="mobile-card-title">{{ svc.name }}</span>
            <span class="mobile-card-sub font-mono text-muted">{{ svc.image }} • {{ svc.replicas }} tasks</span>
          </div>
        </div>

        <div class="mobile-card-right" @click.stop>
          <div class="stepper-controls">
            <button
              class="stepper-btn"
              :disabled="props.actionLoading === `scale-${svc.id}` || svc.replicas <= 0"
              title="Decrease replicas"
              aria-label="Decrease replicas"
              @click="emit('scale-service', svc, -1)"
            >
              <span>−</span>
            </button>
            <span class="stepper-value font-mono">
              {{ svc.replicas }}
            </span>
            <button
              class="stepper-btn"
              :disabled="props.actionLoading === `scale-${svc.id}`"
              title="Increase replicas"
              aria-label="Increase replicas"
              @click="emit('scale-service', svc, 1)"
            >
              <span>+</span>
            </button>
          </div>
          <button
            class="btn btn-secondary btn-xs"
            title="Inspect logs"
            aria-label="Inspect logs"
            @click="emit('view-logs', svc.id, svc.name, 'service')"
          >
            <span>📜</span>
          </button>
        </div>
      </div>
    </template>

    <!-- 2. Nodes Mobile Cards Stream (~68-74px/item) -->
    <template v-else-if="props.activeTab === 'nodes'">
      <div v-if="props.nodes.length === 0" class="empty-state glass-panel">
        <span>No Swarm nodes match the current filter.</span>
      </div>
      <div
        v-for="node in props.nodes"
        :key="node.id"
        class="mobile-stream-card"
        @click="emit('inspect-node', node)"
      >
        <div class="mobile-card-left">
          <span class="mobile-card-icon" aria-hidden="true">🖳</span>
          <div class="mobile-card-text">
            <span class="mobile-card-title font-mono">{{ node.name }}</span>
            <span class="mobile-card-sub font-mono" :class="node.role === 'manager' ? 'text-violet' : 'text-muted'">
              {{ (node.role || 'worker').toUpperCase() }} NODE • Docker v{{ node.engine_version || node.version || '26.1.3' }}
            </span>
          </div>
        </div>

        <div class="mobile-card-right" @click.stop>
          <StatusBadge :status="node.availability" size="sm" />
          <button
            class="btn btn-secondary btn-xs"
            title="Inspect node details"
            aria-label="Inspect node details"
            @click="emit('inspect-node', node)"
          >
            <span>🔍</span>
          </button>
        </div>
      </div>
    </template>

    <!-- 3. Standalone Containers Mobile Cards Stream (~68-74px/item) -->
    <template v-else-if="props.activeTab === 'containers'">
      <div v-if="props.containers.length === 0" class="empty-state glass-panel">
        <span>No standalone containers match the current filter.</span>
      </div>
      <div
        v-for="cont in props.containers"
        :key="cont.id"
        class="mobile-stream-card"
        @click="emit('view-logs', cont.id, cont.name, 'container')"
      >
        <div class="mobile-card-left">
          <span class="mobile-card-icon" aria-hidden="true">📦</span>
          <div class="mobile-card-text">
            <span class="mobile-card-title">{{ cont.name }}</span>
            <span class="mobile-card-sub font-mono text-muted">{{ cont.image }}</span>
          </div>
        </div>

        <div class="mobile-card-right" @click.stop>
          <button
            class="power-switch-btn"
            :class="cont.state === 'running' ? 'power-on' : 'power-off'"
            :disabled="props.actionLoading === `toggle-${cont.id}`"
            title="Toggle Container Power"
            aria-label="Toggle Container Power"
            @click="emit('toggle-container', cont)"
          >
            <span class="power-icon">⏻</span>
            <span class="power-state-text font-mono">{{ cont.state === 'running' ? 'ON' : 'OFF' }}</span>
          </button>
          <button
            class="btn btn-secondary btn-xs"
            title="Live logs"
            aria-label="Live logs"
            @click="emit('view-logs', cont.id, cont.name, 'container')"
          >
            <span>📜</span>
          </button>
        </div>
      </div>
    </template>
  </div>
</template>

<style scoped>
.mobile-cards-stream {
  display: flex;
  flex-direction: column;
  gap: 8px;
  width: 100%;
  max-width: 100%;
  box-sizing: border-box;
}

.mobile-stream-card {
  min-height: 68px;
  padding: 10px 14px;
  border-radius: 12px;
  background: rgba(11, 15, 25, 0.7);
  border: 1px solid var(--border-subtle);
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  transition: all 0.15s ease;
  cursor: pointer;
  touch-action: manipulation;
  width: 100%;
  max-width: 100%;
  box-sizing: border-box;
}

.mobile-stream-card:active {
  background: rgba(255, 255, 255, 0.05);
}

.mobile-card-left {
  display: flex;
  align-items: center;
  gap: 10px;
  min-width: 0;
  flex: 1;
  overflow: hidden;
}

.mobile-card-icon {
  font-size: 20px;
  flex-shrink: 0;
}

.mobile-card-text {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 2px;
  overflow: hidden;
}

.mobile-card-title {
  font-size: 13px;
  font-weight: 700;
  color: #fff;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.mobile-card-sub {
  font-size: 10.5px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.mobile-card-right {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
}

.stepper-controls {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  background: rgba(0, 0, 0, 0.4);
  padding: 3px 6px;
  border-radius: 8px;
  border: 1px solid var(--border-subtle);
}

.stepper-btn {
  width: 32px;
  min-width: 32px;
  height: 32px;
  min-height: 32px;
  border-radius: 6px;
  border: 1px solid var(--border-medium);
  background: rgba(255, 255, 255, 0.08);
  color: #fff;
  font-size: 15px;
  font-weight: bold;
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  position: relative;
  touch-action: manipulation;
  transition: all 0.15s ease;
}

.stepper-btn::after {
  content: '';
  position: absolute;
  inset: -4px;
}

.stepper-btn:hover:not(:disabled) {
  background: var(--accent-cyan);
  border-color: var(--accent-cyan);
}

.stepper-btn:disabled {
  opacity: 0.3;
  cursor: not-allowed;
}

.stepper-value {
  font-size: 14px;
  font-weight: 800;
  color: #fff;
  min-width: 20px;
  text-align: center;
}

.mobile-stream-card .btn-xs {
  height: 32px !important;
  min-height: 32px !important;
  width: 32px !important;
  min-width: 32px !important;
  padding: 0 !important;
  display: inline-flex !important;
  align-items: center !important;
  justify-content: center !important;
  position: relative;
  touch-action: manipulation;
}

.mobile-stream-card .btn-xs::after {
  content: '';
  position: absolute;
  inset: -4px;
}

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

.power-icon {
  font-size: 12px;
}
</style>