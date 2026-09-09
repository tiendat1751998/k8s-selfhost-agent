<script setup lang="ts">
export type AlertTabType = 'active' | 'muted' | 'all'

interface Props {
  activeTab: AlertTabType
  activeCount: number
  mutedCount: number
}

defineProps<Props>()

const emit = defineEmits<{
  (e: 'update:activeTab', tab: AlertTabType): void
}>()
</script>

<template>
  <div class="alert-modal-tabs">
    <button
      type="button"
      class="tab-btn"
      :class="{ 'tab-btn-active': activeTab === 'active' }"
      @click="emit('update:activeTab', 'active')"
    >
      <span class="tab-icon">🚨</span>
      <span>Active Alerts</span>
      <span class="tab-count" :class="activeCount > 0 ? 'count-rose' : 'count-muted'">
        {{ activeCount }}
      </span>
    </button>

    <button
      type="button"
      class="tab-btn"
      :class="{ 'tab-btn-active': activeTab === 'muted' }"
      @click="emit('update:activeTab', 'muted')"
    >
      <span class="tab-icon">🔕</span>
      <span>Silenced / Muted</span>
      <span class="tab-count" :class="mutedCount > 0 ? 'count-amber' : 'count-muted'">
        {{ mutedCount }}
      </span>
    </button>

    <button
      type="button"
      class="tab-btn"
      :class="{ 'tab-btn-active': activeTab === 'all' }"
      @click="emit('update:activeTab', 'all')"
    >
      <span class="tab-icon">📋</span>
      <span>All Alerts</span>
      <span class="tab-count count-cyan">
        {{ activeCount + mutedCount }}
      </span>
    </button>
  </div>
</template>

<style scoped>
@import '../../../assets/styles/components/alert-center.css';
</style>
