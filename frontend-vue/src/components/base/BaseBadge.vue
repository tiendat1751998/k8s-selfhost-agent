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
@import '../../assets/styles/components/ui/base.css';
</style>
