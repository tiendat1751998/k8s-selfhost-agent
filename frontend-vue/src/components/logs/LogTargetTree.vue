<script setup lang="ts">
import { ref, computed } from 'vue'
import type { LogEntry } from '../../stores/logStore'
import BaseIcon from '../ui/BaseIcon.vue'
import LogTargetItem from './LogTargetItem.vue'
import {
  useLogTargets,
  type LogTarget,
  type TargetNodeItem,
  type TargetServiceItem,
  type TargetWorkloadItem,
} from '../../composables/useLogTargets'

export interface AvailableServiceItem {
  id: string
  name: string
  category: 'app' | 'systemd'
  icon: string
  count: number
}

export type { LogTarget, TargetNodeItem, TargetServiceItem, TargetWorkloadItem }

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
const servicesExpanded = ref(true)
const showIdleNodes = ref(false)
const showIdleServices = ref(false)

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

const displayServices = computed<AvailableServiceItem[]>(() => {
  let list: AvailableServiceItem[] = []
  if (props.availableServices && props.availableServices.length > 0) {
    list = [...props.availableServices]
  } else {
    // Fallback if availableServices prop wasn't passed: derive from composable
    list = [
      ...filteredAppServices.value.map((s) => ({
        id: s.id,
        name: s.name,
        category: 'app' as const,
        icon: s.icon || 'box',
        count: getServiceCount(s.id),
      })),
      ...filteredSystemdServices.value.map((s) => ({
        id: s.id,
        name: s.name,
        category: 'systemd' as const,
        icon: s.icon || 'cpu',
        count: getServiceCount(s.id),
      })),
    ]
  }

  // Filter by segmented category tab
  if (props.sourceCategory === 'app') {
    list = list.filter((s) => s.category === 'app')
  } else if (props.sourceCategory === 'systemd') {
    list = list.filter((s) => s.category === 'systemd')
  }

  // Filter by search query
  const q = targetSearch.value.trim().toLowerCase()
  if (q) {
    list = list.filter((s) => s.name.toLowerCase().includes(q) || s.id.toLowerCase().includes(q))
  }

  // Sort: Apps first, then count descending
  list.sort((a, b) => {
    if (a.category !== b.category) {
      return a.category === 'app' ? -1 : 1
    }
    return b.count - a.count
  })

  return list
})

const activeServices = computed(() => {
  return displayServices.value.filter((s) => {
    const isSelected = props.activeService?.toLowerCase() === s.id.toLowerCase()
    const isMuted = props.mutedServices?.some((m) => m.toLowerCase() === s.id.toLowerCase())
    return s.count > 0 || isSelected || isMuted
  })
})

const idleServices = computed(() => {
  return displayServices.value.filter((s) => {
    const isSelected = props.activeService?.toLowerCase() === s.id.toLowerCase()
    const isMuted = props.mutedServices?.some((m) => m.toLowerCase() === s.id.toLowerCase())
    return s.count === 0 && !isSelected && !isMuted
  })
})

const visibleServices = computed(() => {
  if (showIdleServices.value || activeServices.value.length === 0) {
    return displayServices.value
  }
  return activeServices.value
})

const totalServiceLogsCount = computed(() => {
  return displayServices.value.reduce((acc, s) => acc + s.count, 0)
})

function isServiceMuted(svcId: string): boolean {
  const lower = svcId.toLowerCase()
  return (props.mutedServices || []).some((m) => m.toLowerCase() === lower)
}

function handleNodeClick(node: TargetNodeItem) {
  emit('update:activeService', '')
  const target: LogTarget = {
    type: 'node',
    id: node.id,
    name: node.name,
    icon: 'server',
  }
  emit('select', target)
  emit('update:modelValue', target)
}

function handleAllServicesClick() {
  emit('update:activeService', '')
}

function handleServiceClick(svc: AvailableServiceItem) {
  if (props.activeService?.toLowerCase() === svc.id.toLowerCase()) {
    emit('update:activeService', '')
  } else {
    emit('update:activeService', svc.id)
    const currentNode =
      props.modelValue.node || (props.modelValue.type === 'node' ? props.modelValue.id : '')
    if (currentNode) {
      const target: LogTarget = {
        type: 'service',
        id: svc.id,
        name: `${currentNode} / ${svc.name}`,
        icon: svc.icon,
        node: currentNode,
        service: svc.id,
      }
      emit('select', target)
      emit('update:modelValue', target)
    } else {
      const target: LogTarget = {
        type: 'service',
        id: svc.id,
        name: svc.name,
        icon: svc.icon,
        service: svc.id,
      }
      emit('select', target)
      emit('update:modelValue', target)
    }
  }
}
</script>

<template>
  <aside class="log-target-tree glass-panel" aria-label="Log Stream Target Selector">
    <div class="tree-header">
      <div class="tree-title">
        <span class="tree-icon"><BaseIcon name="layers" size="sm" /></span>
        <span>Log Targets</span>
      </div>
      <span class="tree-badge font-mono">{{ totalServiceLogsCount }} logs</span>
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
        v-if="visibleHostNodes.length === 0 && displayServices.length === 0"
        class="tree-empty-state font-mono"
        style="padding: 24px 16px; text-align: center; color: var(--text-muted); font-size: 12px; display: flex; flex-direction: column; align-items: center; gap: 8px;"
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

      <!-- Section 2: Services (Scoped to Selected Node with Segmented Tabs) -->
      <div class="tree-section tree-section-services">
        <div class="section-toggle" @click="servicesExpanded = !servicesExpanded">
          <span class="section-caret" :class="{ 'caret-down': servicesExpanded }">&#9656;</span>
          <BaseIcon name="box" size="xs" />
          <span class="section-label">Services</span>
          <span class="section-count font-mono">{{ displayServices.length }}</span>
        </div>

        <div v-show="servicesExpanded" class="section-items">
          <!-- 3-Tab Segmented Control -->
          <div class="tree-segmented-tabs font-mono" role="tablist" aria-label="Service Category">
            <button
              type="button"
              role="tab"
              :aria-selected="sourceCategory === 'all'"
              class="tree-seg-tab"
              :class="{ active: sourceCategory === 'all' }"
              @click="emit('update:sourceCategory', 'all')"
            >
              All
            </button>
            <button
              type="button"
              role="tab"
              :aria-selected="sourceCategory === 'app'"
              class="tree-seg-tab"
              :class="{ active: sourceCategory === 'app' }"
              @click="emit('update:sourceCategory', 'app')"
            >
              <BaseIcon name="box" size="xs" /> Apps
            </button>
            <button
              type="button"
              role="tab"
              :aria-selected="sourceCategory === 'systemd'"
              class="tree-seg-tab"
              :class="{ active: sourceCategory === 'systemd' }"
              @click="emit('update:sourceCategory', 'systemd')"
            >
              <BaseIcon name="cpu" size="xs" /> System
            </button>
          </div>

          <!-- Muted Services Reset Strip -->
          <div v-if="mutedServices && mutedServices.length > 0" class="tree-muted-strip font-mono">
            <span class="muted-info"><BaseIcon name="bell-off" size="xs" /> {{ mutedServices.length }} muted</span>
            <button
              type="button"
              class="muted-reset-btn"
              title="Reset muted services"
              @click="emit('clearMutedServices')"
            >
              [Reset]
            </button>
          </div>

          <!-- All Services Row -->
          <LogTargetItem
            id="all-services"
            name="All Services"
            icon="layers"
            :count="totalServiceLogsCount"
            :active="!activeService"
            @click="handleAllServicesClick"
          />

          <!-- Active Services List -->
          <LogTargetItem
            v-for="svc in visibleServices"
            :key="svc.id"
            :id="svc.id"
            :name="svc.name"
            :icon="svc.icon"
            :count="svc.count"
            :active="activeService.toLowerCase() === svc.id.toLowerCase()"
            :muted="isServiceMuted(svc.id)"
            :show-mute="true"
            @click="handleServiceClick(svc)"
            @mute="emit('toggleMuteService', svc.id)"
          />

          <!-- Subtle bottom toggle for 0-log idle services -->
          <button
            v-if="activeServices.length > 0 && idleServices.length > 0"
            type="button"
            class="idle-nodes-toggle-btn font-mono"
            :title="showIdleServices ? 'Hide idle services' : 'Show idle services'"
            @click="showIdleServices = !showIdleServices"
          >
            <span>{{ showIdleServices ? '▾ Hide idle services' : `+ ${idleServices.length} idle services (0 logs)` }}</span>
          </button>
        </div>
      </div>
    </div>
  </aside>
</template>

<style scoped>
@import '../../assets/styles/views/logstream.css';
</style>

