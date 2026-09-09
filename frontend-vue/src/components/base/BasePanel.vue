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
@import '../../assets/styles/components/ui/base.css';
</style>
