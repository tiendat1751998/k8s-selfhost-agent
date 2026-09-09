<script setup lang="ts">
defineProps<{
  label: string
  value: string | number
  icon?: string
  iconClass?: string
}>()
</script>

<template>
  <div class="stat-card">
    <div v-if="icon" class="stat-icon" :class="iconClass">{{ icon }}</div>
    <div class="stat-content">
      <div class="stat-value">{{ value }}</div>
      <div class="stat-label">{{ label }}</div>
    </div>
  </div>
</template>

<style scoped>
@import '../../assets/styles/components/ui/base.css';
</style>
