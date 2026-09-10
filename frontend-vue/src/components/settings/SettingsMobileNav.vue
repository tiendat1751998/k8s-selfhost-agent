<script setup lang="ts">
import type { TabKey, CategoryKey } from '../../composables/useSettings'

defineProps<{
  activeTab: TabKey
  tabs: { id: TabKey; label: string; icon: string; category?: CategoryKey; count?: number }[]
  isDirtyCategory?: (category: CategoryKey) => boolean
}>()

const emit = defineEmits<{
  (e: 'update:activeTab', tab: TabKey): void
}>()
</script>

<template>
  <div class="mobile-nav-wrapper mobile-only">
    <div class="mobile-tabs-container glass-panel">
      <button
        v-for="tab in tabs"
        :key="tab.id"
        type="button"
        class="mobile-tab-item"
        :class="{ active: activeTab === tab.id }"
        @click="emit('update:activeTab', tab.id)"
      >
        <span>{{ tab.icon }} {{ tab.label }}</span>
        <span v-if="tab.category && isDirtyCategory?.(tab.category)" class="dirty-dot"></span>
        <span v-if="tab.count !== undefined" class="tab-badge">{{ tab.count }}</span>
      </button>
    </div>
  </div>
</template>
