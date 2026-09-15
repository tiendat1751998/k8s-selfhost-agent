<script setup lang="ts">
import { ref, computed } from 'vue'
import BaseIcon from '../ui/BaseIcon.vue'
import StatusBadge from '../ui/StatusBadge.vue'
import ActionDropdown, { type ActionItem } from '../ui/ActionDropdown.vue'
import type { ComputeHost } from '../../api/compute'
import type { HostTestResult, HostTypeDefinition } from '../../types/hosts'

const props = defineProps<{
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

type HostSortField = 'name' | 'status' | 'latency'
const sortField = ref<HostSortField>('status')
const sortDirection = ref<'asc' | 'desc'>('asc')

function handleSort(field: HostSortField) {
  if (sortField.value === field) {
    sortDirection.value = sortDirection.value === 'asc' ? 'desc' : 'asc'
  } else {
    sortField.value = field
    if (field === 'latency') {
      sortDirection.value = 'desc'
    } else {
      sortDirection.value = 'asc'
    }
  }
}

function getHostStatusRank(status?: string): number {
  const s = (status || '').toLowerCase()
  if (s === 'connected' || s === 'ok' || s === 'ready') return 0
  if (s === 'degraded' || s === 'warning') return 1
  return 2
}

function getHostLatency(host: ComputeHost): number {
  return props.hostTestResults[host.id]?.latency_ms || 0
}

const sortedHosts = computed<ComputeHost[]>(() => {
  const list = [...props.hosts]
  const field = sortField.value
  const dir = sortDirection.value === 'asc' ? 1 : -1

  return list.sort((a, b) => {
    let diff = 0

    if (field === 'status') {
      diff = getHostStatusRank(a.status) - getHostStatusRank(b.status)
    } else if (field === 'name') {
      diff = (a.name || '').localeCompare(b.name || '')
    } else if (field === 'latency') {
      diff = getHostLatency(a) - getHostLatency(b)
    }

    if (diff !== 0) {
      return diff * dir
    }

    // Deterministic tie-breakers: Status (Connected first) -> Name -> ID
    const statusTie = getHostStatusRank(a.status) - getHostStatusRank(b.status)
    if (statusTie !== 0) return statusTie

    const nameTie = (a.name || '').localeCompare(b.name || '')
    if (nameTie !== 0) return nameTie

    return (a.id || '').localeCompare(b.id || '')
  })
})

function getRowActions(): ActionItem[] {
  return [
    { id: 'select', label: 'View Host Details', icon: 'search' },
    { id: 'copy', label: 'Copy Endpoint', icon: 'copy' },
    { id: 'sep', label: '', separator: true },
    { id: 'delete', label: 'Delete Host', icon: 'trash', variant: 'danger' },
  ]
}

function handleActionSelect(actionId: string, host: ComputeHost) {
  if (actionId === 'select') emit('select', host)
  else if (actionId === 'copy') emit('copy', host.endpoint)
  else if (actionId === 'delete') emit('delete', host)
}
</script>

<template>
  <div class="hosts-table-container glass-panel animate-fade-in">
    <table class="hosts-table">
      <thead>
        <tr>
          <th class="th-name sortable-th" @click="handleSort('name')">
            <div class="th-sort-wrap">
              <span>HOST NAME</span>
              <span class="sort-icon" :class="{ active: sortField === 'name' }">{{ sortField === 'name' ? (sortDirection === 'asc' ? '▲' : '▼') : '↕' }}</span>
            </div>
          </th>
          <th class="th-type">TYPE</th>
          <th class="th-endpoint">ENDPOINT</th>
          <th class="th-status sortable-th" @click="handleSort('status')">
            <div class="th-sort-wrap">
              <span>STATUS</span>
              <span class="sort-icon" :class="{ active: sortField === 'status' }">{{ sortField === 'status' ? (sortDirection === 'asc' ? '▲' : '▼') : '↕' }}</span>
            </div>
          </th>
          <th class="th-latency sortable-th" @click="handleSort('latency')">
            <div class="th-sort-wrap">
              <span>LATENCY</span>
              <span class="sort-icon" :class="{ active: sortField === 'latency' }">{{ sortField === 'latency' ? (sortDirection === 'asc' ? '▲' : '▼') : '↕' }}</span>
            </div>
          </th>
          <th class="th-lastseen">LAST SEEN</th>
          <th class="th-labels">LABELS</th>
          <th class="th-actions text-right">ACTIONS</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="host in sortedHosts" :key="host.id" class="host-row">
          <td class="col-name" @click="emit('select', host)">
            <div class="row-name-group">
              <span class="row-type-icon"><BaseIcon :name="getHostTypeMeta(host.host_type).icon" size="sm" /></span>
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
                type="button"
                class="btn-copy-mini"
                title="Copy endpoint"
                @click.stop="emit('copy', host.endpoint)"
              >
                <BaseIcon name="copy" size="xs" />
              </button>
            </div>
          </td>

          <td class="col-status">
            <StatusBadge :status="host.status || 'connected'" size="sm" />
          </td>

          <td class="col-latency font-mono">
            <span v-if="hostTestResults[host.id] && hostTestResults[host.id].latency_ms > 0" :class="getLatencyBadgeClass(hostTestResults[host.id].latency_ms)">
              <BaseIcon name="zap" size="xs" /> {{ hostTestResults[host.id].latency_ms }}ms
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
                type="button"
                class="btn btn-secondary btn-xs"
                :disabled="testingHostId === host.id"
                title="Test Connectivity"
                @click.stop="emit('test', host)"
              >
                <BaseIcon :name="testingHostId === host.id ? 'refresh' : 'zap'" size="xs" :class="{ 'animate-spin': testingHostId === host.id }" />
              </button>
              <button
                type="button"
                class="btn btn-secondary btn-xs"
                title="Edit Configuration"
                @click.stop="emit('edit', host)"
              >
                <BaseIcon name="edit" size="xs" />
              </button>
              <ActionDropdown
                :items="getRowActions()"
                size="xs"
                trigger-title="More actions"
                @select="handleActionSelect($event, host)"
              />
            </div>
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>

<style scoped>
@import '../../assets/styles/views/infra-hosts.css';

.sortable-th {
  cursor: pointer;
  user-select: none;
  transition: color 0.15s ease;
}

.sortable-th:hover {
  color: var(--text-primary, #fff);
}

.th-sort-wrap {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.sort-icon {
  font-size: 10px;
  opacity: 0.4;
}

.sort-icon.active {
  opacity: 1;
  color: var(--color-cyan, #06b6d4);
  font-weight: bold;
}
</style>
