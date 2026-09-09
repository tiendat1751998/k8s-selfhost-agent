<template>
  <div class="plugins-table-container glass-panel">
    <table class="plugins-table">
      <thead>
        <tr>
          <th>Extension</th>
          <th>Category</th>
          <th>WASM Runtime</th>
          <th>Permissions & Scopes</th>
          <th>Config</th>
          <th style="text-align: right;">Actions</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="p in plugins" :key="p.id" :class="{ 'plugin-disabled': !p.enabled }">
          <!-- Identity -->
          <td>
            <div class="table-identity-cell">
              <span class="table-icon">{{ p.icon || '🧩' }}</span>
              <div>
                <div class="table-name-row">
                  <span class="table-plugin-name">{{ p.name }}</span>
                  <span class="version-tag">v{{ p.version }}</span>
                </div>
                <div class="table-author" v-if="p.author">by {{ p.author }}</div>
              </div>
            </div>
          </td>

          <!-- Category -->
          <td>
            <span class="badge" :class="categoryBadgeClass(p.category)">
              {{ p.category }}
            </span>
          </td>

          <!-- WASM Sandbox Runtime Status -->
          <td>
            <span class="runtime-status-pill" :class="getRuntimeStatusClass(p.id)">
              {{ getRuntimeStatusLabel(p.id) }}
            </span>
          </td>

          <!-- Permissions & Scopes -->
          <td>
            <div class="perm-chips" v-if="p.permissions && p.permissions.length > 0">
              <span v-for="perm in p.permissions.slice(0, 3)" :key="perm" class="perm-chip">
                {{ perm }}
              </span>
              <span v-if="p.permissions.length > 3" class="perm-chip" :title="p.permissions.join(', ')">
                +{{ p.permissions.length - 3 }} more
              </span>
            </div>
            <span v-else class="text-muted" style="font-size: 11.5px;">None</span>
          </td>

          <!-- Config Keys Count -->
          <td>
            <span class="config-count" @click="$emit('configure', p)">
              ⚙️ {{ Object.keys(p.config || {}).length }} keys
            </span>
          </td>

          <!-- Actions -->
          <td>
            <div class="table-actions-cell">
              <button
                class="btn-table-action"
                :title="p.enabled ? 'Disable Plugin' : 'Enable Plugin'"
                :disabled="togglingId === p.id"
                @click="$emit('toggle', p)"
              >
                <span>⚡</span> {{ p.enabled ? 'Disable' : 'Enable' }}
              </button>

              <button
                class="btn-table-action"
                title="Configure Plugin Runtime Variables"
                @click="$emit('configure', p)"
              >
                <span>⚙️</span> Configure
              </button>

              <button
                class="btn-table-action"
                title="Inspect / Edit Plugin Metadata"
                @click="$emit('inspect', p)"
              >
                <span>🔍</span> Inspect
              </button>

              <button
                class="btn-table-action btn-table-uninstall"
                title="Uninstall Plugin"
                @click="$emit('uninstall', p)"
              >
                <span>🗑</span> Uninstall
              </button>
            </div>
          </td>
        </tr>
      </tbody>
    </table>
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
  (e: 'inspect', plugin: Plugin): void
  (e: 'uninstall', plugin: Plugin): void
}>()
</script>
