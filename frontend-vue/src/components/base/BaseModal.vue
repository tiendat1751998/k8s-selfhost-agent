<script setup lang="ts">
defineProps<{
  show: boolean
  title?: string
}>()
defineEmits<{
  (e: 'close'): void
}>()
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
.modal-overlay {
  position: fixed;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  background: rgba(0, 0, 0, 0.7);
  z-index: 9999;
  display: flex;
  justify-content: center;
  align-items: center;
  backdrop-filter: blur(4px);
}
.modal-content {
  width: 100%;
  max-width: 500px;
  background: var(--color-surface-card);
  border: 1px solid var(--color-hairline);
  border-radius: var(--rounded-xl);
  box-shadow: 0 8px 30px rgba(0, 0, 0, 0.5);
  display: flex;
  flex-direction: column;
}
.panel-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: var(--space-md) var(--space-lg);
  border-bottom: 1px solid var(--color-hairline);
}
.panel-title {
  font-size: var(--text-title-sm);
  color: var(--color-on-dark);
}
.close-btn {
  background: transparent;
  border: none;
  color: var(--color-muted);
  font-size: 20px;
  cursor: pointer;
}
.close-btn:hover {
  color: var(--color-on-dark);
}
.panel-body {
  padding: var(--space-lg);
}
</style>