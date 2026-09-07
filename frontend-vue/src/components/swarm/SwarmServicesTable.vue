<script setup lang="ts">
import StatusBadge from '../ui/StatusBadge.vue'
import type { DockerService } from '../../api/compute'

const props = defineProps<{
  services: DockerService[]
  actionLoading?: string | null
}>()

const emit = defineEmits<{
  (e: 'inspect', service: DockerService): void
  (e: 'scale', service: DockerService, delta: number): void
  (e: 'update', service: DockerService): void
  (e: 'logs', serviceId: string, name: string): void
  (e: 'remove', serviceId: string): void
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
  <div class="services-table-view animate-fade-in">
    <div v-if="props.services.length === 0" class="empty-state glass-panel">
      <span>No Swarm services deployed. Connect Swarm manager socket to discover services.</span>
    </div>

    <div v-else class="services-table-wrap">
      <table class="services-table">
        <thead>
          <tr>
            <th>Service & Image</th>
            <th>Replicas</th>
            <th>Published Ports</th>
            <th>Status</th>
            <th>Last Updated</th>
            <th style="text-align: right;">Actions</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="svc in props.services" :key="svc.id">
            <!-- Service Name & Image -->
            <td>
              <div class="svc-table-name-cell">
                <span class="svc-icon" aria-hidden="true">🐳</span>
                <div>
                  <div class="svc-table-title font-mono" @click="emit('inspect', svc)">
                    {{ svc.name }}
                  </div>
                  <span class="svc-table-image font-mono text-muted">{{ svc.image }}</span>
                </div>
              </div>
            </td>

            <!-- Replica Stepper / Display -->
            <td>
              <div class="stepper-controls" style="display: inline-flex; padding: 2px 6px;">
                <button
                  class="stepper-btn"
                  style="width: 22px; height: 22px; font-size: 14px;"
                  :disabled="props.actionLoading === `scale-${svc.id}` || svc.replicas <= 0"
                  title="Decrease replicas"
                  @click="emit('scale', svc, -1)"
                >
                  <span>−</span>
                </button>
                <span class="stepper-value font-mono" style="font-size: 13px; min-width: 20px;">
                  {{ svc.replicas }}
                </span>
                <button
                  class="stepper-btn"
                  style="width: 22px; height: 22px; font-size: 14px;"
                  :disabled="props.actionLoading === `scale-${svc.id}`"
                  title="Increase replicas"
                  @click="emit('scale', svc, 1)"
                >
                  <span>+</span>
                </button>
              </div>
            </td>

            <!-- Published Ports -->
            <td>
              <div class="ports-list font-mono">
                <span v-for="p in (svc.ports || ['80:80/tcp'])" :key="p" class="port-tag">
                  {{ p }}
                </span>
              </div>
            </td>

            <!-- Status -->
            <td>
              <StatusBadge :status="svc.replicas > 0 ? 'running' : 'standby'" size="sm" />
            </td>

            <!-- Updated Time -->
            <td>
              <span class="font-mono text-muted text-xs">{{ formatDate(svc.updated_at) }}</span>
            </td>

            <!-- Labeled Action Buttons -->
            <td>
              <div class="svc-actions-row" style="justify-content: flex-end;">
                <button
                  class="btn btn-xs btn-scale"
                  :disabled="props.actionLoading === `scale-${svc.id}`"
                  title="Scale service replica count"
                  @click="emit('scale', svc, 1)"
                >
                  <span>⚡ Scale</span>
                </button>

                <button
                  class="btn btn-xs btn-update"
                  :disabled="props.actionLoading === `update-${svc.id}`"
                  title="Rolling update service"
                  @click="emit('update', svc)"
                >
                  <span>🔄 Update</span>
                </button>

                <button
                  class="btn btn-xs btn-logs"
                  :disabled="props.actionLoading === svc.id"
                  title="Inspect service logs"
                  @click="emit('logs', svc.id, svc.name)"
                >
                  <span>📄 Logs</span>
                </button>

                <button
                  class="btn btn-xs btn-remove"
                  :disabled="props.actionLoading === `remove-${svc.id}`"
                  title="Remove Swarm service"
                  @click="emit('remove', svc.id)"
                >
                  <span>🗑 Remove</span>
                </button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
