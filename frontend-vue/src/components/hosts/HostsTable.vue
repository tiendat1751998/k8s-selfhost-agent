<script setup lang="ts">
import StatusBadge from '../ui/StatusBadge.vue'
import type { ComputeHost } from '../../api/compute'
import type { HostTestResult, HostTypeDefinition } from '../../types/hosts'

defineProps<{
  hosts: ComputeHost[]
  hostTestResults: Record<string, HostTestResult>
  testingHostId: string | null
  getHostTypeMeta: (type?: string) => HostTypeDefinition
  getLatencyBadgeClass: (latency?: number) => string
  formatDate: (d?: string) => string
}>()

const emit = defineEmits<{
  (e: 'test', host: ComputeHost): void
  (e: 'edit', host: ComputeHost): void
  (e: 'delete', host: ComputeHost): void
  (e: 'select', host: ComputeHost): void
  (e: 'copy', text: string): void
}>()
</script>

<template>
  <div class="hosts-table-container glass-panel animate-fade-in">
    <table class="hosts-table">
      <thead>
        <tr>
          <th>HOST NAME</th>
          <th>TYPE</th>
          <th>ENDPOINT</th>
          <th>STATUS</th>
          <th>LATENCY</th>
          <th>LAST SEEN</th>
          <th>LABELS</th>
          <th class="text-right">ACTIONS</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="host in hosts" :key="host.id" class="host-row">
          <td class="col-name" @click="emit('select', host)">
            <div class="row-name-group">
              <span class="row-type-icon">{{ getHostTypeMeta(host.host_type).icon }}</span>
              <div>
                <div class="row-name font-bold">{{ host.name }}</div>
                <div class="row-id font-mono text-muted">{{ host.id }}</div>
              </div>
            </div>
          </td>

          <td class="col-type">
            <span class="type-badge font-mono" :class="getHostTypeMeta(host.host_type).badgeClass">
              {{ getHostTypeMeta(host.host_type).label }}
            </span>
          </td>

          <td class="col-endpoint font-mono">
            <div class="endpoint-copy-wrap">
              <span class="text-cyan">{{ host.endpoint }}</span>
              <button
                class="btn-copy-mini"
                title="Copy endpoint"
                @click.stop="emit('copy', host.endpoint)"
              >
                📋
              </button>
            </div>
          </td>

          <td class="col-status">
            <StatusBadge :status="host.status || 'connected'" size="sm" />
          </td>

          <td class="col-latency font-mono">
            <span v-if="hostTestResults[host.id] && hostTestResults[host.id].latency_ms > 0" :class="getLatencyBadgeClass(hostTestResults[host.id].latency_ms)">
              ⚡ {{ hostTestResults[host.id].latency_ms }}ms
            </span>
            <span v-else class="text-muted">--</span>
          </td>

          <td class="col-lastseen font-mono text-muted text-sm">
            {{ formatDate(host.last_health_check || host.created_at) }}
          </td>

          <td class="col-labels">
            <div v-if="host.labels && Object.keys(host.labels).length > 0" class="table-labels-wrap">
              <span
                v-for="(val, key) in host.labels"
                :key="key"
                class="host-tag-sm font-mono"
              >
                {{ key }}={{ val }}
              </span>
            </div>
            <span v-else class="text-muted font-mono text-xs">None</span>
          </td>

          <td class="col-actions text-right">
            <div class="table-actions-group">
              <button
                class="btn btn-secondary btn-xs"
                :disabled="testingHostId === host.id"
                title="Test Connectivity"
                @click.stop="emit('test', host)"
              >
                <span>{{ testingHostId === host.id ? '⏳' : '⚡' }}</span>
              </button>
              <button
                class="btn btn-secondary btn-xs"
                title="Edit Configuration"
                @click.stop="emit('edit', host)"
              >
                <span>✏️</span>
              </button>
              <button
                class="btn btn-danger-outline btn-xs"
                title="Delete Host"
                @click.stop="emit('delete', host)"
              >
                <span>🗑️</span>
              </button>
            </div>
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>
