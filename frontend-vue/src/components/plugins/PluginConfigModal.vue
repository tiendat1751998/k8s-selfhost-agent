<template>
  <div v-if="show && plugin" class="modal-backdrop" @click.self="$emit('close')">
    <div class="modal-card glass-panel config-modal">
      <div class="modal-header">
        <div class="modal-title-row">
          <span class="modal-icon">⚙️</span>
          <div>
            <h3>Plugin Configuration</h3>
            <p class="modal-subtitle">{{ plugin.name }} (v{{ plugin.version }})</p>
          </div>
        </div>
        <button class="modal-close" @click="$emit('close')">✕</button>
      </div>

      <div class="config-modal-body">
        <div class="config-intro">
          Key-value configuration and runtime environment variables passed into plugin sandbox hooks at boot.
        </div>

        <div class="config-table-container">
          <table class="config-table" v-if="configPairs.length > 0">
            <thead>
              <tr>
                <th>Config Key</th>
                <th>Value</th>
                <th class="col-action"></th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="(pair, idx) in configPairs" :key="idx">
                <td>
                  <input
                    v-model="pair.key"
                    type="text"
                    placeholder="e.g. api_url"
                    class="text-input font-mono"
                  />
                </td>
                <td>
                  <input
                    v-model="pair.value"
                    type="text"
                    placeholder="e.g. https://api.endpoint.internal"
                    class="text-input"
                  />
                </td>
                <td>
                  <button class="btn-icon btn-danger-icon" @click="$emit('removePair', idx)">✕</button>
                </td>
              </tr>
            </tbody>
          </table>
          <div v-else class="config-empty">
            No configuration variables set. Click below to add key-value pairs.
          </div>
        </div>

        <button class="btn btn-sm btn-secondary add-pair-btn" @click="$emit('addPair')">
          + Add Config Variable
        </button>

        <div v-if="error" class="form-error-msg">
          ⚠️ {{ error }}
        </div>
      </div>

      <div class="modal-footer">
        <button type="button" class="btn btn-secondary" @click="$emit('close')">
          Cancel
        </button>
        <button type="button" class="btn btn-primary" :disabled="saving" @click="$emit('save')">
          {{ saving ? 'Saving...' : 'Save Configuration' }}
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import type { Plugin } from '../../api/plugins'

defineProps<{
  show: boolean
  plugin: Plugin | null
  configPairs: { key: string; value: string }[]
  saving: boolean
  error: string | null
}>()

defineEmits<{
  (e: 'close'): void
  (e: 'save'): void
  (e: 'addPair'): void
  (e: 'removePair', idx: number): void
}>()
</script>
