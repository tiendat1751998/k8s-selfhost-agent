<script setup lang="ts">
import { ref, computed } from 'vue'
import type { LogEntry } from '../../stores/logStore'
import BaseIcon from '../ui/BaseIcon.vue'
import LogTargetItem from './LogTargetItem.vue'
import LogTargetServicesSection, { type AvailableServiceItem } from './LogTargetServicesSection.vue'
import {
  useLogTargets,
  type LogTarget,
  type TargetNodeItem,
  type TargetServiceItem,
  type TargetWorkloadItem,
} from '../../composables/useLogTargets'

export type { AvailableServiceItem, LogTarget, TargetNodeItem, TargetServiceItem, TargetWorkloadItem }

const props = withDefaults(
  defineProps<{
    modelValue: LogTarget
    logs: LogEntry[]
    cluster?: string
    availableServices?: AvailableServiceItem[]
    activeService?: string
    sourceCategory?: 'all' | 'app' | 'systemd'
    mutedServices?: string[]
  }>(),
  {
    cluster: '',
    availableServices: () => [],
    activeService: '',
    sourceCategory: 'all',
    mutedServices: () => [],
  }
)

const emit = defineEmits<{
  (e: 'update:modelValue', target: LogTarget): void
  (e: 'select', target: LogTarget): void
  (e: 'update:activeService', serviceId: string): void
  (e: 'update:sourceCategory', cat: 'all' | 'app' | 'systemd'): void
  (e: 'toggleMuteService', serviceId: string): void
  (e: 'clearMutedServices'): void
}>()

const nodesExpanded = ref(true)
const showIdleNodes = ref(false)

const {
  targetSearch,
  filteredHostNodes,
  filteredAppServices,
  filteredSystemdServices,
  getNodeCount,
  getServiceCount,
} = useLogTargets(props, emit)

const activeHostNodes = computed(() => {
  return filteredHostNodes.value.filter((node) => {
    const count = getNodeCount(node.id)
    const isSelected =
      (props.modelValue.type === 'node' && props.modelValue.id === node.id) ||
      props.modelValue.node === node.id
    return !!node.online || count > 0 || isSelected
  })
})

const idleHostNodes = computed(() => {
  return filteredHostNodes.value.filter((node) => {
    const count = getNodeCount(node.id)
    const isSelected =
      (props.modelValue.type === 'node' && props.modelValue.id === node.id) ||
      props.modelValue.node === node.id
    return !node.online && count === 0 && !isSelected
  })
})

const visibleHostNodes = computed(() => {
  if (showIdleNodes.value || activeHostNodes.value.length === 0) {
    return filteredHostNodes.value
  }
  return activeHostNodes.value
})

const currentNode = computed(() =>
  props.modelValue.node || (props.modelValue.type === 'node' ? props.modelValue.id : '')
)

const totalServiceLogs = computed(() => {
  if (props.availableServices && props.availableServices.length > 0) {
    return props.availableServices.reduce((acc, s) => acc + s.count, 0)
  }
  return (
    filteredAppServices.value.reduce((acc, s) => acc + getServiceCount(s.id), 0) +
    filteredSystemdServices.value.reduce((acc, s) => acc + getServiceCount(s.id), 0)
  )
})

function handleNodeClick(node: TargetNodeItem) {
  emit('update:activeService', '')
  const isOnline = Boolean(node.online)
  const target: LogTarget = {
    type: 'node',
    id: node.id,
    name: node.name,
    icon: 'server',
    node: node.id,
    online: isOnline,
    status: isOnline ? 'online' : 'offline',
  }
  emit('select', target)
  emit('update:modelValue', target)
}

function handleSelectService(target: LogTarget) {
  emit('select', target)
  emit('update:modelValue', target)
}
</script>

<template>
  <aside class="log-target-tree glass-panel" aria-label="Log Stream Target Selector">
    <div class="tree-header">
      <div class="tree-title">
        <span class="tree-icon"><BaseIcon name="layers" size="sm" /></span>
        <span>Log Targets</span>
      </div>
      <span class="tree-badge font-mono">{{ totalServiceLogs }} logs</span>
    </div>

    <!-- Quick Filter Search Input -->
    <div class="tree-search-bar">
      <span class="tree-search-ico" aria-hidden="true"><BaseIcon name="search" size="sm" /></span>
      <input
        v-model="targetSearch"
        type="text"
        placeholder="Filter nodes or apps..."
        class="tree-search-input font-mono"
        aria-label="Filter nodes or apps"
      />
      <button
        v-if="targetSearch"
        type="button"
        class="tree-search-clear"
        aria-label="Clear target filter"
        @click="targetSearch = ''"
      >
        <BaseIcon name="x" size="xs" />
      </button>
    </div>

    <div class="tree-content">
      <div
        v-if="visibleHostNodes.length === 0 && !availableServices?.length && !filteredAppServices.length && !filteredSystemdServices.length"
        class="tree-empty-state font-mono"
        style="padding: 24px 16px; text-align: center; color: var(--color-muted); font-size: 12px; display: flex; flex-direction: column; align-items: center; gap: 8px;"
      >
        <BaseIcon name="layers" size="md" />
        <span>No log targets discovered</span>
      </div>

      <!-- Section 1: Nodes (Compact Node Selector at Top) -->
      <div v-if="filteredHostNodes.length > 0" class="tree-section">
        <div class="section-toggle" @click="nodesExpanded = !nodesExpanded">
          <span class="section-caret" :class="{ 'caret-down': nodesExpanded }">&#9656;</span>
          <BaseIcon name="server" size="xs" />
          <span class="section-label">Nodes</span>
          <span class="section-count font-mono">{{ visibleHostNodes.length }}</span>
        </div>
        <div v-show="nodesExpanded" class="section-items">
          <LogTargetItem
            v-for="node in visibleHostNodes"
            :key="node.id"
            :id="node.id"
            :name="node.name"
            :icon="node.icon || 'server'"
            :count="getNodeCount(node.id)"
            :online="node.online"
            :hide-zero-count="true"
            :active="modelValue.type === 'node' && (modelValue.id === node.id || modelValue.node === node.id) && !activeService"
            @click="handleNodeClick(node)"
          />

          <!-- Collapse idle nodes under a clean toggle button -->
          <button
            v-if="activeHostNodes.length > 0 && idleHostNodes.length > 0"
            type="button"
            class="idle-nodes-toggle-btn font-mono"
            :title="showIdleNodes ? 'Hide offline nodes' : 'Show offline nodes'"
            @click="showIdleNodes = !showIdleNodes"
          >
            <span>{{ showIdleNodes ? '▾ Hide offline' : `+ ${idleHostNodes.length} offline` }}</span>
          </button>
        </div>
      </div>

      <!-- Section 2: Services Section -->
      <LogTargetServicesSection
        :available-services="availableServices"
        :fallback-app-services="filteredAppServices"
        :fallback-systemd-services="filteredSystemdServices"
        :get-service-count="getServiceCount"
        :target-search="targetSearch"
        :active-service="activeService"
        :source-category="sourceCategory"
        :muted-services="mutedServices"
        :current-node="currentNode"
        @update:active-service="emit('update:activeService', $event)"
        @update:source-category="emit('update:sourceCategory', $event)"
        @select-service="handleSelectService"
        @toggle-mute-service="emit('toggleMuteService', $event)"
        @clear-muted-services="emit('clearMutedServices')"
      />
    </div>
  </aside>
</template>

<style scoped>
@import '../../assets/styles/views/logstream.css';
</style>
