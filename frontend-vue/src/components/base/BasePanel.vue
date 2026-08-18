<script setup lang="ts">
defineProps<{ title?: string }>()
</script>

<template>
  <div class="base-panel">
    <div v-if="title || $slots.header" class="panel-header">
      <slot name="header">
        <h3 class="panel-title">{{ title }}</h3>
      </slot>
    </div>
    <div class="panel-body">
      <slot></slot>
    </div>
  </div>
</template>

<style scoped>
.base-panel {
  display: flex;
  flex-direction: column;
  background: var(--color-surface-card);
  border-radius: var(--rounded-xl);
  border: 1px solid var(--color-hairline);
  overflow: hidden;
}

.panel-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: var(--space-md) var(--space-lg);
  border-bottom: 1px solid var(--color-hairline);
  flex-shrink: 0;
}

.panel-title {
  font-size: var(--text-title-sm);
  font-weight: 600;
  color: var(--color-on-dark);
}

.panel-body {
  flex: 1;
  overflow-y: auto;
  padding: var(--space-md);
}
</style>
