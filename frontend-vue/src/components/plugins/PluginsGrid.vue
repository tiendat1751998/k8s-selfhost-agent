<template>
  <div class="plugins-grid-wrapper">
    <!-- Starter Catalog Presets (Quick Install) if no plugins installed -->
    <section class="templates-section glass-panel" v-if="plugins.length === 0 && !loading">
      <div class="section-title-row">
        <h3>🚀 Starter Plugin Catalog</h3>
        <span class="section-hint">Click any preset to install and activate instantly</span>
      </div>
      <div class="templates-grid">
        <div 
          v-for="tpl in starterPresets" 
          :key="tpl.name" 
          class="template-card"
          @click="$emit('installPreset', tpl)"
        >
          <div class="tpl-icon">{{ tpl.icon }}</div>
          <div class="tpl-info">
            <h4>{{ tpl.name }} <span class="badge" :class="categoryBadgeClass(tpl.category || 'devtools')">{{ tpl.category }}</span></h4>
            <p>{{ tpl.description }}</p>
          </div>
          <button class="btn btn-sm btn-primary" :disabled="installingPreset">
            + Install
          </button>
        </div>
      </div>
    </section>

    <!-- Plugins Grid -->
    <div v-else class="plugins-grid">
      <div
        v-for="p in plugins"
        :key="p.id"
        class="plugin-card glass-panel"
        :class="{ 'plugin-disabled': !p.enabled }"
      >
        <!-- Card Header -->
        <div class="card-header">
          <div class="plugin-identity">
            <div class="plugin-avatar">{{ p.icon || '🧩' }}</div>
            <div>
              <div class="plugin-name-row">
                <h3 class="plugin-name">{{ p.name }}</h3>
                <span class="version-tag">v{{ p.version }}</span>
              </div>
              <div class="plugin-meta-row">
                <span class="badge" :class="categoryBadgeClass(p.category)">
                  {{ p.category }}
                </span>
                <span v-if="p.author" class="author-tag">by {{ p.author }}</span>
              </div>
            </div>
          </div>

          <!-- Switch Toggle -->
          <div class="toggle-wrapper" :title="p.enabled ? 'Click to Disable' : 'Click to Enable'">
            <label class="switch">
              <input
                type="checkbox"
                :checked="p.enabled"
                :disabled="togglingId === p.id"
                @change="$emit('toggle', p)"
              />
              <span class="slider round"></span>
            </label>
          </div>
        </div>

        <!-- Description -->
        <p class="plugin-desc">
          {{ p.description || 'No description provided.' }}
        </p>

        <!-- Entry Point URL -->
        <div class="entrypoint-box" v-if="p.entry_point">
          <span class="entry-icon">🔗</span>
          <span class="entry-url" :title="p.entry_point">{{ p.entry_point }}</span>
        </div>

        <!-- Permissions Chips -->
        <div class="permissions-container" v-if="p.permissions && p.permissions.length > 0">
          <span class="perm-label">Permissions:</span>
          <div class="perm-chips">
            <span
              v-for="perm in p.permissions"
              :key="perm"
              class="perm-chip"
            >
              {{ perm }}
            </span>
          </div>
        </div>

        <!-- Card Footer -->
        <div class="card-footer">
          <div class="footer-left">
            <span class="config-count" @click="$emit('configure', p)">
              ⚙️ {{ Object.keys(p.config || {}).length }} config keys
            </span>
            <span 
              class="runtime-status-pill"
              :class="getRuntimeStatusClass(p.id)"
            >
              {{ getRuntimeStatusLabel(p.id) }}
            </span>
          </div>

          <div class="footer-actions">
            <button 
              class="btn-icon" 
              title="Test bundle load / Hot-reload" 
              @click="$emit('testBundle', p)"
            >
              ⚡
            </button>
            <button 
              class="btn-icon" 
              title="Configure Settings" 
              @click="$emit('configure', p)"
            >
              ⚙️
            </button>
            <button 
              class="btn-icon" 
              title="Edit Plugin" 
              @click="$emit('edit', p)"
            >
              ✏️
            </button>
            <button 
              class="btn-icon btn-danger-icon" 
              title="Delete Plugin" 
              @click="$emit('delete', p)"
            >
              🗑️
            </button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import type { Plugin, CreatePluginDTO } from '../../api/plugins'

defineProps<{
  plugins: Plugin[]
  starterPresets: CreatePluginDTO[]
  togglingId: string | null
  loading: boolean
  installingPreset: boolean
  categoryBadgeClass: (cat?: string) => string
  getRuntimeStatusLabel: (id: string) => string
  getRuntimeStatusClass: (id: string) => string
}>()

defineEmits<{
  (e: 'toggle', plugin: Plugin): void
  (e: 'configure', plugin: Plugin): void
  (e: 'edit', plugin: Plugin): void
  (e: 'delete', plugin: Plugin): void
  (e: 'testBundle', plugin: Plugin): void
  (e: 'installPreset', preset: CreatePluginDTO): void
}>()
</script>
