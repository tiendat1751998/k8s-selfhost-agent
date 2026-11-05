<script setup lang="ts">
import { ref, computed } from 'vue'
import BaseIcon from '../ui/BaseIcon.vue'
import LogTargetItem from './LogTargetItem.vue'
import type { LogTarget, TargetServiceItem } from '../../composables/useLogTargets'

export interface AvailableServiceItem {
  id: string
  name: string
  category: 'app' | 'systemd'
  icon: string
  count: number
}

interface Props {
  availableServices?: AvailableServiceItem[]
  fallbackAppServices?: TargetServiceItem[]
  fallbackSystemdServices?: TargetServiceItem[]
  getServiceCount: (svcId: string) => number
  targetSearch?: string
  activeService?: string
  sourceCategory?: 'all' | 'app' | 'systemd'
  mutedServices?: string[]
  currentNode?: string
}

const props = withDefaults(defineProps<Props>(), {
  availableServices: () => [],
  fallbackAppServices: () => [],
  fallbackSystemdServices: () => [],
  targetSearch: '',
  activeService: '',
  sourceCategory: 'all',
  mutedServices: () => [],
  currentNode: '',
})

const emit = defineEmits<{
  (e: 'update:activeService', serviceId: string): void
  (e: 'update:sourceCategory', cat: 'all' | 'app' | 'systemd'): void
  (e: 'selectService', target: LogTarget): void
  (e: 'toggleMuteService', serviceId: string): void
  (e: 'clearMutedServices'): void
}>()

const servicesExpanded = ref(true)
const showIdleServices = ref(false)

const displayServices = computed<AvailableServiceItem[]>(() => {
  let list: AvailableServiceItem[] = []
  if (props.availableServices && props.availableServices.length > 0) {
    list = [...props.availableServices]
  } else {
    list = [
      ...props.fallbackAppServices.map((s) => ({
        id: s.id,
        name: s.name,
        category: 'app' as const,
        icon: s.icon || 'box',
        count: props.getServiceCount(s.id),
      })),
      ...props.fallbackSystemdServices.map((s) => ({
        id: s.id,
        name: s.name,
        category: 'systemd' as const,
        icon: s.icon || 'cpu',
        count: props.getServiceCount(s.id),
      })),
    ]
  }

  if (props.sourceCategory === 'app') {
    list = list.filter((s) => s.category === 'app')
  } else if (props.sourceCategory === 'systemd') {
    list = list.filter((s) => s.category === 'systemd')
  }

  const q = props.targetSearch.trim().toLowerCase()
  if (q) {
    list = list.filter((s) => s.name.toLowerCase().includes(q) || s.id.toLowerCase().includes(q))
  }

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

function handleAllServicesClick() {
  emit('update:activeService', '')
}

function handleServiceClick(svc: AvailableServiceItem) {
  if (props.activeService?.toLowerCase() === svc.id.toLowerCase()) {
    emit('update:activeService', '')
  } else {
    emit('update:activeService', svc.id)
    if (props.currentNode) {
      const target: LogTarget = {
        type: 'service',
        id: svc.id,
        name: `${props.currentNode} / ${svc.name}`,
        icon: svc.icon,
        node: props.currentNode,
        service: svc.id,
      }
      emit('selectService', target)
    } else {
      const target: LogTarget = {
        type: 'service',
        id: svc.id,
        name: svc.name,
        icon: svc.icon,
        service: svc.id,
      }
      emit('selectService', target)
    }
  }
}

defineExpose({
  totalServiceLogsCount,
  displayServicesCount: computed(() => displayServices.value.length),
})
</script>

<template>
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
</template>
