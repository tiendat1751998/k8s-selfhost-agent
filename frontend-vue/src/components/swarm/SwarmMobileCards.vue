<script setup lang="ts">
import StatusBadge from '../ui/StatusBadge.vue'
import BaseIcon from '../ui/BaseIcon.vue'
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
          <span class="mobile-card-icon" aria-hidden="true"><BaseIcon name="box" size="sm" /></span>
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
              <span>-</span>
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
            <BaseIcon name="file-text" size="xs" />
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
          <span class="mobile-card-icon" aria-hidden="true"><BaseIcon name="server" size="sm" /></span>
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
            <BaseIcon name="search" size="xs" />
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
          <span class="mobile-card-icon" aria-hidden="true"><BaseIcon name="box" size="sm" /></span>
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
            <span class="power-icon"><BaseIcon name="zap" size="xs" /></span>
            <span class="power-state-text font-mono">{{ cont.state === 'running' ? 'ON' : 'OFF' }}</span>
          </button>
          <button
            class="btn btn-secondary btn-xs"
            title="Live logs"
            aria-label="Live logs"
            @click="emit('view-logs', cont.id, cont.name, 'container')"
          >
            <BaseIcon name="file-text" size="xs" />
          </button>
        </div>
      </div>
    </template>
  </div>
</template>

<style scoped src="../../assets/styles/components/swarm-mobile.css"></style>