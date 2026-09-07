<template>
  <div class="plugins-mobile-stream">
    <div
      v-for="p in plugins"
      :key="p.id"
      class="plugin-mobile-card glass-panel"
      :class="{ 'plugin-disabled': !p.enabled }"
    >
      <div class="mobile-card-main">
        <div class="mobile-card-icon">{{ p.icon || '🧩' }}</div>
        <div class="mobile-card-details">
          <div class="mobile-name-row">
            <span class="mobile-plugin-name">{{ p.name }}</span>
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
        <!-- Switch Toggle -->
        <label class="switch" :title="p.enabled ? 'Disable Plugin' : 'Enable Plugin'">
          <input
            type="checkbox"
            :checked="p.enabled"
            :disabled="togglingId === p.id"
            @change="$emit('toggle', p)"
          />
          <span class="slider round"></span>
        </label>

        <!-- Configure -->
        <button
          class="btn-icon"
          title="Configure Plugin Settings"
          @click="$emit('configure', p)"
        >
          ⚙️
        </button>

        <!-- Edit / Inspect -->
        <button
          class="btn-icon"
          title="Inspect / Edit"
          @click="$emit('edit', p)"
        >
          ✏️
        </button>

        <!-- Delete / Uninstall -->
        <button
          class="btn-icon btn-danger-icon"
          title="Uninstall Plugin"
          @click="$emit('delete', p)"
        >
          🗑️
        </button>
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
}>()
</script>
