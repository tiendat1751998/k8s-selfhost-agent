<script setup lang="ts">
interface Props {
  placement?: 'header' | 'footer'
  activeCount: number
  mutedCount: number
}

withDefaults(defineProps<Props>(), {
  placement: 'header',
  activeCount: 0,
  mutedCount: 0,
})

const emit = defineEmits<{
  (e: 'mute-all', mode: 'restart' | '1h' | '24h'): void
  (e: 'unmute-all'): void
}>()
</script>

<template>
  <!-- Header Placement -->
  <div v-if="placement === 'header'" class="alert-header-batch-actions">
    <button
      v-if="activeCount > 0"
      type="button"
      class="btn-mute-all-header"
      @click="emit('mute-all', 'restart')"
      title="Silence all active node alerts until server restart"
    >
      <span>🔕 Mute All</span>
    </button>
    <button
      v-if="mutedCount > 0"
      type="button"
      class="btn-unmute-all-header"
      @click="emit('unmute-all')"
      title="Restore all silenced alert rules"
    >
      <span>🔔 Unmute All</span>
    </button>
  </div>

  <!-- Footer Placement -->
  <div v-else class="alert-footer-batch-actions">
    <button
      v-if="activeCount > 0"
      type="button"
      class="btn btn-secondary btn-footer-mute"
      @click="emit('mute-all', 'restart')"
    >
      <span>🔕 Mute All (Until Restart)</span>
    </button>
    <button
      v-if="mutedCount > 0"
      type="button"
      class="btn btn-secondary btn-footer-unmute"
      @click="emit('unmute-all')"
    >
      <span>🔔 Unmute All</span>
    </button>
  </div>
</template>

<style scoped>
@import '../../../assets/styles/components/alert-center.css';

.alert-header-batch-actions,
.alert-footer-batch-actions {
  display: inline-flex;
  align-items: center;
  gap: 8px;
}
</style>
