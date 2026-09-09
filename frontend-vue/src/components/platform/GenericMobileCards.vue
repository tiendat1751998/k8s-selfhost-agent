<script setup lang="ts">
import type { GenericPlatformItem } from '../../composables/useGenericPlatform'
import StatusBadge from '../ui/StatusBadge.vue'

const props = withDefaults(
  defineProps<{
    items: GenericPlatformItem[]
    loading?: boolean
    selectedId?: string
  }>(),
  {
    loading: false,
    selectedId: undefined
  }
)

const emit = defineEmits<{
  (e: 'inspect', item: GenericPlatformItem): void
  (e: 'action', actionId: string, item: GenericPlatformItem): void
}>()
</script>

<template>
  <div class="mobile-stream-container" role="feed" aria-label="Mobile platform resources stream">
    <div v-if="loading" class="mobile-loading-box">
      <span class="spinner-inline"></span>
      <span>Loading stream...</span>
    </div>

    <div v-else-if="items.length === 0" class="mobile-empty-box">
      <p>No resources found.</p>
    </div>

    <div
      v-for="item in items"
      :key="item.id"
      class="mobile-stream-card"
      :class="{ 'is-selected': item.id === selectedId }"
      role="article"
      tabindex="0"
      @click="emit('inspect', item)"
      @keydown.enter="emit('inspect', item)"
      @keydown.space.prevent="emit('inspect', item)"
    >
      <!-- Left indicator & status -->
      <div class="card-status-col">
        <StatusBadge :status="item.status" size="sm" />
      </div>

      <!-- Center content: 2 high-density lines, 0 horizontal scroll -->
      <div class="card-info-col">
        <div class="card-title-line">
          <span class="card-name" :title="item.name">{{ item.name }}</span>
          <span class="badge badge-cyan font-mono card-tag">{{ item.tag }}</span>
        </div>
        <div class="card-sub-line">
          <span class="card-detail" :title="item.detail">{{ item.detail }}</span>
          <span v-if="item.namespace" class="card-ns font-mono">ns/{{ item.namespace }}</span>
        </div>
      </div>

      <!-- Right action chevron -->
      <div class="card-action-col">
        <span class="inspect-chevron" aria-hidden="true">›</span>
      </div>
    </div>
  </div>
</template>