<script setup lang="ts">
import BaseIcon from '../ui/BaseIcon.vue'
import type { ComputeHost } from '../../api/compute'
import type { HostTestResult, HostTypeDefinition } from '../../types/hosts'

defineProps<{
  hosts: ComputeHost[]
  hostTestResults: Record<string, HostTestResult>
  testingHostId: string | null
  getHostTypeMeta: (type?: string) => HostTypeDefinition
  getLatencyBadgeClass: (latency?: number) => string
}>()

const emit = defineEmits<{
  (e: 'test', host: ComputeHost): void
  (e: 'edit', host: ComputeHost): void
  (e: 'delete', host: ComputeHost): void
  (e: 'select', host: ComputeHost): void
}>()
</script>

<template>
  <div class="mobile-hosts-stream animate-fade-in">
    <div
      v-for="host in hosts"
      :key="host.id"
      class="mobile-host-card glass-panel"
      @click="emit('select', host)"
    >
      <!-- Left: Host Icon & Status -->
      <div class="m-card-icon-wrap">
        <span class="m-card-icon"><BaseIcon :name="getHostTypeMeta(host.host_type).icon" size="sm" /></span>
        <span
          class="m-status-dot"
          :class="`dot-${host.status === 'connected' || host.status === 'ok' ? 'ok' : host.status === 'error' || host.status === 'unhealthy' ? 'err' : 'down'}`"
        ></span>
      </div>

      <!-- Center: Host Name, Endpoint, Latency -->
      <div class="m-card-info">
        <div class="m-card-title-row">
          <span class="m-host-name font-bold">{{ host.name }}</span>
          <span
            v-if="hostTestResults[host.id]"
            class="m-latency font-mono"
            :class="getLatencyBadgeClass(hostTestResults[host.id].latency_ms)"
          >
            <template v-if="hostTestResults[host.id].latency_ms > 0"><BaseIcon name="zap" size="xs" /> {{ hostTestResults[host.id].latency_ms }}ms</template><template v-else>--</template>
          </span>
          <span v-else class="m-type-tag font-mono">
            {{ getHostTypeMeta(host.host_type).label }}
          </span>
        </div>
        <div class="m-card-sub-row font-mono text-muted">
          <span class="m-endpoint text-cyan">{{ host.endpoint }}</span>
        </div>
      </div>

      <!-- Right: Compact Action Buttons -->
      <div class="m-card-actions" @click.stop>
        <button
          class="btn-m-action btn-m-test"
          :disabled="testingHostId === host.id"
          title="Test Connection"
          @click.stop="emit('test', host)"
        >
          <BaseIcon :name="testingHostId === host.id ? 'refresh' : 'zap'" size="xs" :class="{ 'animate-spin': testingHostId === host.id }" />
        </button>
        <button
          class="btn-m-action btn-m-edit"
          title="Edit Host"
          @click.stop="emit('edit', host)"
        >
          <BaseIcon name="edit" size="xs" />
        </button>
        <button
          class="btn-m-action btn-m-delete"
          title="Delete Host"
          @click.stop="emit('delete', host)"
        >
          <BaseIcon name="trash" size="xs" />
        </button>
      </div>
    </div>
  </div>
</template>
