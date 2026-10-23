<script setup lang="ts">
import BaseIcon from '../ui/BaseIcon.vue'

defineProps<{
  id: string
  name: string
  icon?: string
  count?: number
  active?: boolean
  muted?: boolean
  showMute?: boolean
}>()

const emit = defineEmits<{
  (e: 'click'): void
  (e: 'mute'): void
}>()
</script>

<template>
  <div
    role="treeitem"
    tabindex="0"
    :aria-selected="active"
    class="tree-item"
    :class="{ active, muted }"
    @click="emit('click')"
    @keydown.enter="emit('click')"
    @keydown.space.prevent="emit('click')"
  >
    <span class="tree-accent-bar" aria-hidden="true" />
    <span class="item-icon" aria-hidden="true">
      <BaseIcon :name="icon || 'box'" size="sm" />
    </span>
    <span class="item-name font-mono" :title="name">{{ name }}</span>
    <span class="item-count font-mono" :class="{ 'has-logs': (count || 0) > 0 }">{{ count || 0 }}</span>
    <button
      v-if="showMute"
      type="button"
      class="item-mute-btn"
      :class="{ 'is-muted': muted }"
      :title="muted ? `Unmute ${name}` : `Mute ${name}`"
      :aria-label="muted ? `Unmute ${name}` : `Mute ${name}`"
      @click.stop="emit('mute')"
    >
      <span aria-hidden="true">{{ muted ? '🔇' : '×' }}</span>
    </button>
  </div>
</template>

