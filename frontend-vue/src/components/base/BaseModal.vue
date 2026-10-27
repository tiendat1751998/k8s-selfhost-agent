<script setup lang="ts">
import { onMounted, onUnmounted } from 'vue'

const props = defineProps<{
  show: boolean
  title?: string
}>()
const emit = defineEmits<{
  (e: 'close'): void
}>()

function handleKeyDown(e: KeyboardEvent) {
  if (e.key === 'Escape' && props.show) {
    emit('close')
  }
}

onMounted(() => {
  window.addEventListener('keydown', handleKeyDown)
})

onUnmounted(() => {
  window.removeEventListener('keydown', handleKeyDown)
})
</script>

<template>
  <div v-if="show" class="modal-overlay" @click.self="$emit('close')">
    <div class="modal-content panel">
      <div class="panel-header">
        <h3 class="panel-title">{{ title }}</h3>
        <button class="close-btn" @click="$emit('close')">✕</button>
      </div>
      <div class="panel-body">
        <slot></slot>
      </div>
    </div>
  </div>
</template>

<style scoped>
@import '../../assets/styles/components/ui/base.css';
</style>
