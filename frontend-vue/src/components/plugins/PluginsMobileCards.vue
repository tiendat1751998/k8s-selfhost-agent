<template>
  <div class="plugins-mobile-stream">
    <!-- Dedicated Mobile Empty State -->
    <div v-if="plugins.length === 0" class="plugins-mobile-empty glass-panel">
      <div class="mobile-empty-icon"><BaseIcon name="plug" size="lg" /></div>
      <p class="mobile-empty-title">No plugins installed matching criteria. Tap Install to browse repository.</p>
      <button class="btn btn-sm btn-primary mobile-empty-btn" @click="$emit('install')">
        <BaseIcon name="plus" size="xs" /> Install
      </button>
    </div>

    <!-- High-Density Mobile Cards (68-75px) -->
    <div
      v-else
      v-for="p in plugins"
      :key="p.id"
      class="plugin-mobile-card glass-panel"
      :class="{ 'plugin-disabled': !p.enabled }"
    >
      <div class="mobile-card-main">
        <div class="mobile-card-icon"><BaseIcon :name="p.icon || 'plug'" size="md" /></div>
        <div class="mobile-card-details">
          <div class="mobile-name-row">
            <span class="mobile-plugin-name" :title="p.name">{{ p.name }}</span>
            <span class="version-tag">v{{ p.version }}</span>
          </div>
          <div class="mobile-meta-row">
            <span class="badge" :class="categoryBadgeClass(p.category)">
              {{ p.category }}
            </span>
            <span class="runtime-status-pill" :class="getRuntimeStatusClass(p.id)">
              {{ getRuntimeStatusLabel(p.id) }}
            </span>
          </div>
        </div>
      </div>

      <div class="mobile-card-actions">
        <!-- Toggle min 32px Touch Target -->
        <button
          class="btn-icon mobile-action-btn"
          :class="{ 'btn-active-toggle': p.enabled }"
          :title="p.enabled ? 'Disable Plugin' : 'Enable Plugin'"
          :disabled="togglingId === p.id"
          aria-label="Toggle Plugin"
          @click="$emit('toggle', p)"
        ><BaseIcon name="zap" size="xs" /></button>

        <!-- Configure min 32px Touch Target -->
        <button
          class="btn-icon mobile-action-btn"
          title="Configure Plugin Settings"
          aria-label="Configure Plugin"
          @click="$emit('configure', p)"
        ><BaseIcon name="sliders" size="xs" /></button>

        <!-- Edit / Inspect min 32px Touch Target -->
        <button
          class="btn-icon mobile-action-btn"
          title="Inspect / Edit"
          aria-label="Inspect Plugin"
          @click="$emit('edit', p)"
        ><BaseIcon name="edit" size="xs" /></button>

        <!-- Delete / Uninstall min 32px Touch Target -->
        <button
          class="btn-icon btn-danger-icon mobile-action-btn"
          title="Uninstall Plugin"
          aria-label="Uninstall Plugin"
          @click="$emit('delete', p)"
        ><BaseIcon name="trash" size="xs" /></button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import type { Plugin } from '../../api/plugins'

defineProps<{
  plugins: Plugin[]
  togglingId: string | null
  categoryBadgeClass: (cat?: string) => string
  getRuntimeStatusLabel: (id: string) => string
  getRuntimeStatusClass: (id: string) => string
}>()

defineEmits<{
  (e: 'toggle', plugin: Plugin): void
  (e: 'configure', plugin: Plugin): void
  (e: 'edit', plugin: Plugin): void
  (e: 'delete', plugin: Plugin): void
  (e: 'install'): void
}>()
</script>