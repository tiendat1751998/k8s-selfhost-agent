<script setup lang="ts">
import BaseIcon from '../../ui/BaseIcon.vue'

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
      <BaseIcon name="bell-off" size="xs" />
      <span>Mute All</span>
    </button>
    <button
      v-if="mutedCount > 0"
      type="button"
      class="btn-unmute-all-header"
      @click="emit('unmute-all')"
      title="Restore all silenced alert rules"
    >
      <BaseIcon name="bell" size="xs" />
      <span>Unmute All</span>
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
      <BaseIcon name="bell-off" size="xs" />
      <span>Mute All (Until Restart)</span>
    </button>
    <button
      v-if="mutedCount > 0"
      type="button"
      class="btn btn-secondary btn-footer-unmute"
      @click="emit('unmute-all')"
    >
      <BaseIcon name="bell" size="xs" />
      <span>Unmute All</span>
    </button>
  </div>
</template>

<style scoped>
@import '../../../assets/styles/components/alert-center.css';
</style>
