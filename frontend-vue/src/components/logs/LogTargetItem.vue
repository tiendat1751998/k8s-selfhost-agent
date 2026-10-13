<script setup lang="ts">
import BaseIcon from '../ui/BaseIcon.vue'

defineProps<{
  id: string
  name: string
  sub?: string
  icon?: string
  count?: number
  active?: boolean
}>()

const emit = defineEmits<{
  (e: 'click'): void
}>()
</script>

<template>
  <button
    type="button"
    role="treeitem"
    :aria-selected="active"
    class="tree-item"
    :class="{ active }"
    @click="emit('click')"
  >
    <span class="tree-accent-bar"></span>
    <span class="item-icon">
      <BaseIcon :name="icon || 'box'" size="sm" />
    </span>
    <div class="item-meta">
      <span class="item-name font-mono">{{ name }}</span>
      <span v-if="sub" class="item-sub font-mono">{{ sub }}</span>
    </div>
    <span class="item-count font-mono" :class="{ 'has-logs': (count || 0) > 0 }">{{ count || 0 }}</span>
  </button>
</template>
