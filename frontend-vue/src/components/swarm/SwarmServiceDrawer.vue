<script setup lang="ts">
import StatusBadge from '../ui/StatusBadge.vue'
import ModalDrawer from '../ui/ModalDrawer.vue'
import type { DockerService } from '../../api/compute'
import type { SwarmServiceTask } from '../../composables/useDockerSwarm'

const props = defineProps<{
  show: boolean
  service: DockerService | null
  tasks?: SwarmServiceTask[]
  loading?: boolean
  actionLoading?: string | null
}>()

const emit = defineEmits<{
  (e: 'update:show', value: boolean): void
  (e: 'close'): void
  (e: 'scale', service: DockerService, delta: number): void
  (e: 'update', service: DockerService): void
  (e: 'logs', serviceId: string, name: string): void
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
  <ModalDrawer
    :show="props.show"
    mode="drawer"
    :title="`Service: ${props.service?.name || 'Swarm Service'}`"
    subtitle="Overlay VIP mesh endpoints, task containers, and replica placement telemetry"
    max-width="720px"
    @update:show="emit('update:show', $event)"
    @close="emit('close')"
  >
    <div v-if="props.loading" class="drawer-loading">
      <div class="spinner"></div>
      <span>Retrieving service tasks & VIP mesh telemetry...</span>
    </div>

    <div v-else-if="props.service" class="service-details-body">
      <!-- Header Hero Card -->
      <div class="service-detail-header-card glass-panel">
        <div class="detail-hero-top">
          <div class="detail-id-wrap">
            <span class="detail-icon" aria-hidden="true">🐳</span>
            <div>
              <h3 class="detail-hostname font-mono">{{ props.service.name }}</h3>
              <span class="detail-node-id font-mono text-muted">ID: {{ props.service.id }}</span>
            </div>
          </div>

          <div class="detail-badges">
            <span class="badge badge-cyan">REPLICATED</span>
            <StatusBadge :status="props.service.replicas > 0 ? 'running' : 'standby'" size="sm" />
          </div>
        </div>

        <div class="detail-quick-stats">
          <div class="quick-stat-item">
            <span class="stat-lbl">REPLICAS</span>
            <span class="stat-val font-mono text-cyan">{{ props.service.replicas }} Active Tasks</span>
          </div>
          <div class="quick-stat-item">
            <span class="stat-lbl">IMAGE TAG</span>
            <span class="stat-val font-mono text-muted" style="overflow: hidden; text-overflow: ellipsis; white-space: nowrap;">
              {{ props.service.image }}
            </span>
          </div>
          <div class="quick-stat-item">
            <span class="stat-lbl">LAST UPDATED</span>
            <span class="stat-val font-mono text-emerald">{{ formatDate(props.service.updated_at) }}</span>
          </div>
        </div>
      </div>

      <!-- VIP Endpoints & Overlay Mesh Section -->
      <div class="detail-section">
        <h4 class="detail-section-title">VIP Endpoints & Ingress Mesh Telemetry</h4>
        <div class="specs-grid">
          <div class="spec-card glass-panel">
            <div class="spec-icon" aria-hidden="true">⚡</div>
            <div class="spec-info">
              <span class="spec-label">VIRTUAL IP (VIP)</span>
              <span class="spec-value font-mono text-cyan">{{ (props.service as any).vip || '--' }}</span>
            </div>
          </div>

          <div class="spec-card glass-panel">
            <div class="spec-icon" aria-hidden="true">🌐</div>
            <div class="spec-info">
              <span class="spec-label">PUBLISHED PORTS</span>
              <span class="spec-value font-mono">{{ (props.service.ports && props.service.ports.length > 0) ? props.service.ports.join(', ') : '--' }}</span>
            </div>
          </div>

          <div class="spec-card glass-panel">
            <div class="spec-icon" aria-hidden="true">🔀</div>
            <div class="spec-info">
              <span class="spec-label">ROUTING MODE</span>
              <span class="spec-value font-mono">{{ (props.service as any).mode || 'VIP Round-Robin' }}</span>
            </div>
          </div>

          <div class="spec-card glass-panel">
            <div class="spec-icon" aria-hidden="true">🛡️</div>
            <div class="spec-info">
              <span class="spec-label">OVERLAY NETWORK</span>
              <span class="spec-value font-mono">{{ (props.service as any).network || '--' }}</span>
            </div>
          </div>
        </div>
      </div>

      <!-- Replica Placement & Task Containers Section -->
      <div class="detail-section">
        <div class="labels-header-row">
          <h4 class="detail-section-title">Replica Placement & Task Containers</h4>
          <span class="count-pill">{{ props.tasks?.length || 0 }} Tasks</span>
        </div>

        <div v-if="!props.tasks || props.tasks.length === 0" class="empty-labels glass-panel text-muted font-mono">
          No task containers currently allocated.
        </div>

        <div v-else class="tasks-table-wrap glass-panel">
          <table class="tasks-table font-mono">
            <thead>
              <tr>
                <th>SLOT</th>
                <th>TASK ID</th>
                <th>PLACED NODE</th>
                <th>CONTAINER IP</th>
                <th>STATE</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="task in props.tasks" :key="task.id">
                <td class="text-cyan">#{{ task.slot }}</td>
                <td class="text-muted">{{ task.id }}</td>
                <td class="text-violet">{{ task.node_name }}</td>
                <td class="text-emerald">{{ task.ip || '--' }}</td>
                <td>
                  <StatusBadge :status="task.current_state" size="sm" />
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </div>

    <template #footer="{ close }">
      <div class="drawer-footer-actions">
        <button
          v-if="props.service"
          class="btn btn-secondary"
          @click="emit('logs', props.service.id, props.service.name)"
        >
          <span>📜 Service Logs</span>
        </button>
        <button
          v-if="props.service"
          class="btn btn-primary"
          :disabled="props.actionLoading === `update-${props.service.id}`"
          @click="emit('update', props.service)"
        >
          <span>🔄 Rolling Update</span>
        </button>
        <button class="btn btn-secondary" @click="close">Close</button>
      </div>
    </template>
  </ModalDrawer>
</template>
