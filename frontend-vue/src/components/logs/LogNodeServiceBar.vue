<script setup lang="ts">
import { computed } from 'vue'
import type { LogTarget } from '../../composables/useLogTargets'
import BaseIcon from '../ui/BaseIcon.vue'

export interface AvailableServiceItem {
  id: string
  name: string
  category: 'app' | 'systemd'
  icon: string
  count: number
}

const props = withDefaults(
  defineProps<{
    selectedTarget: LogTarget
    availableServices: AvailableServiceItem[]
    activeService?: string
    sourceCategory?: 'all' | 'app' | 'systemd'
    mutedServices?: string[]
    hoverPauseEnabled?: boolean
    isHoverPaused?: boolean
  }>(),
  {
    activeService: '',
    sourceCategory: 'all',
    mutedServices: () => [],
    hoverPauseEnabled: true,
    isHoverPaused: false,
  }
)

const emit = defineEmits<{
  (e: 'update:activeService', serviceId: string): void
  (e: 'update:sourceCategory', cat: 'all' | 'app' | 'systemd'): void
  (e: 'toggleMuteService', serviceId: string): void
  (e: 'clearMutedServices'): void
  (e: 'update:hoverPauseEnabled', val: boolean): void
}>()

const totalCount = computed(() => props.availableServices.reduce((sum, s) => sum + s.count, 0))

const filteredServices = computed(() => {
  let list = props.availableServices
  if (props.sourceCategory === 'app') {
    list = list.filter((s) => s.category === 'app')
  } else if (props.sourceCategory === 'systemd') {
    list = list.filter((s) => s.category === 'systemd')
  }
  return list.slice(0, 25)
})

function handleChipClick(svcId: string) {
  emit('update:activeService', props.activeService === svcId ? '' : svcId)
}
</script>

<template>
  <div class="log-node-service-bar font-mono" role="toolbar" aria-label="Node and Service Log Isolation Toolbar">
    <!-- Left Category Pills -->
    <div class="category-pills" role="radiogroup" aria-label="Service Category Filter">
      <button
        type="button"
        class="cat-pill-btn"
        :class="{ active: sourceCategory === 'all' }"
        :aria-pressed="sourceCategory === 'all'"
        @click="emit('update:sourceCategory', 'all')"
      >
        All Clean
      </button>
      <button
        type="button"
        class="cat-pill-btn"
        :class="{ active: sourceCategory === 'app' }"
        :aria-pressed="sourceCategory === 'app'"
        @click="emit('update:sourceCategory', 'app')"
      >
        📦 Apps Only
      </button>
      <button
        type="button"
        class="cat-pill-btn"
        :class="{ active: sourceCategory === 'systemd' }"
        :aria-pressed="sourceCategory === 'systemd'"
        @click="emit('update:sourceCategory', 'systemd')"
      >
        ⚙️ SystemD
      </button>
    </div>

    <div class="divider-v" aria-hidden="true" />

    <!-- Center Scrollable Service Quick-Filter Chips -->
    <div class="service-chips-stream" role="group" aria-label="Service Quick Filter Chips">
      <button
        type="button"
        class="service-chip"
        :class="{ active: !activeService }"
        @click="emit('update:activeService', '')"
      >
        <span>All</span>
        <span class="chip-count">({{ totalCount }})</span>
      </button>

      <div
        v-for="svc in filteredServices"
        :key="svc.id"
        class="service-chip-wrapper"
        :class="{
          'chip-muted': mutedServices.includes(svc.id.toLowerCase()),
          'chip-active': activeService.toLowerCase() === svc.id.toLowerCase(),
        }"
      >
        <button
          type="button"
          class="service-chip"
          :class="{ active: activeService.toLowerCase() === svc.id.toLowerCase() }"
          :title="`Isolate ${svc.name} (${svc.count} logs)`"
          @click="handleChipClick(svc.id)"
        >
          <span class="chip-cat-ico">{{ svc.category === 'app' ? '📦' : '⚙️' }}</span>
          <span class="chip-label">{{ svc.name }}</span>
          <span class="chip-count">{{ svc.count }}</span>
        </button>
        <button
          type="button"
          class="chip-mute-btn"
          :title="mutedServices.includes(svc.id.toLowerCase()) ? 'Unmute service' : 'Mute/Exclude this service'"
          :aria-label="`Mute ${svc.name}`"
          @click.stop="emit('toggleMuteService', svc.id)"
        >
          <BaseIcon :name="mutedServices.includes(svc.id.toLowerCase()) ? 'volume-x' : 'x'" size="xs" />
        </button>
      </div>
    </div>

    <!-- Right Controls: Muted Reset & Hover-Pause Toggle -->
    <div class="service-bar-actions">
      <button
        v-if="mutedServices.length > 0"
        type="button"
        class="btn-muted-reset font-mono"
        title="Reset all muted services"
        @click="emit('clearMutedServices')"
      >
        <span>🔇 Muted ({{ mutedServices.length }})</span>
        <span class="reset-txt">[Reset]</span>
      </button>

      <button
        type="button"
        class="btn-hover-pause-toggle font-mono"
        :class="{ active: hoverPauseEnabled, 'is-paused': isHoverPaused }"
        :title="hoverPauseEnabled ? (isHoverPaused ? 'Viewport currently hover-paused' : 'Hover-pause enabled') : 'Hover-pause disabled'"
        @click="emit('update:hoverPauseEnabled', !hoverPauseEnabled)"
      >
        <span>{{ isHoverPaused ? '⏸ Hover Paused' : (hoverPauseEnabled ? '⏸ Pause on Hover' : '▶ Auto Scroll') }}</span>
      </button>
    </div>
  </div>
</template>

<style scoped>
@import '../../assets/styles/views/logstream-terminal.css';
</style>
