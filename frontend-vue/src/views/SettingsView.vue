<script setup lang="ts">
import BasePanel from '../components/base/BasePanel.vue'
import BaseStatCard from '../components/base/BaseStatCard.vue'
</script>

<template>
  <div class="settings-view">
    <div class="header">
      <h2>Settings</h2>
    </div>
    <div class="content">
      <BaseStatCard label="Theme" value="Dark" icon="??" />
      <BasePanel title="Global Settings">
        <p>Settings content here...</p>
      </BasePanel>
    </div>
  </div>
</template>

<style scoped>
.settings-view {
  padding: var(--space-lg);
  display: flex;
  flex-direction: column;
  gap: var(--space-lg);
}
.header h2 {
  color: var(--color-on-dark);
}
.content {
  display: flex;
  flex-direction: column;
  gap: var(--space-md);
}
</style>
