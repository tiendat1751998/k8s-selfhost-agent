<script setup lang="ts">
defineProps<{
  type?: 'success' | 'warning' | 'danger' | 'info'
}>()
</script>

<template>
  <span class="base-badge" :class="type ? 'badge-' + type : ''">
    <slot></slot>
  </span>
</template>

<style scoped>
.base-badge {
  padding: 2px 8px;
  font-size: var(--text-caption);
  border-radius: var(--rounded-pill);
  background: var(--color-surface-elevated);
  color: var(--color-muted-strong);
}
.badge-success { background: rgba(14, 203, 129, 0.15); color: var(--color-trading-up); }
.badge-warning { background: rgba(252, 213, 53, 0.15); color: var(--color-primary); }
.badge-danger { background: rgba(246, 70, 93, 0.15); color: var(--color-trading-down); }
.badge-info { background: rgba(59, 130, 246, 0.15); color: var(--color-info); }
</style>
