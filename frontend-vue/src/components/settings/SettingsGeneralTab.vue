<script setup lang="ts">
import NodeRemediationSettings from './NodeRemediationSettings.vue'

defineProps<{
  form: {
    name: string
    timezone: string
    language: string
  }
  timezoneOptions: Array<{ value: string; label: string }>
  languageOptions: Array<{ value: string; label: string }>
  saving: boolean
}>()

const emit = defineEmits<{
  (e: 'save', category: 'platform'): void
  (e: 'reset', category: 'platform'): void
}>()
</script>

<template>
  <div class="settings-group animate-fade-in">
    <div class="settings-card glass-panel">
      <div class="card-header">
        <div>
          <h2 class="card-title">General Platform Settings</h2>
          <p class="card-subtitle">
            Manage platform identity, default regional timezone, and display language preferences.
          </p>
        </div>
        <button type="button" class="btn btn-secondary btn-sm" @click="emit('reset', 'platform')">
          <span>↺ Reset Defaults</span>
        </button>
      </div>

      <form class="settings-form" @submit.prevent="emit('save', 'platform')">
        <div class="form-group">
          <label class="form-label" for="platform-name">
            <span>Platform Name</span>
            <span class="required">*</span>
          </label>
          <p class="field-desc">The organizational title displayed across the navigation header and reports.</p>
          <input
            id="platform-name"
            v-model="form.name"
            type="text"
            class="input-glass form-input"
            placeholder="e.g. K8s Self-Host Platform"
            required
          />
        </div>

        <div class="form-row">
          <div class="form-group flex-1">
            <label class="form-label" for="platform-tz">
              <span>Display Timezone</span>
            </label>
            <p class="field-desc">Default timezone used for metric timestamps, log streams, and audit history.</p>
            <select id="platform-tz" v-model="form.timezone" class="input-glass form-select">
              <option v-for="tz in timezoneOptions" :key="tz.value" :value="tz.value">
                {{ tz.label }}
              </option>
            </select>
          </div>

          <div class="form-group flex-1">
            <label class="form-label" for="platform-lang">
              <span>Display Language</span>
            </label>
            <p class="field-desc">Interface localization preference for menus, alerts, and notifications.</p>
            <select id="platform-lang" v-model="form.language" class="input-glass form-select">
              <option v-for="lang in languageOptions" :key="lang.value" :value="lang.value">
                {{ lang.label }}
              </option>
            </select>
          </div>
        </div>

        <div class="form-actions">
          <button type="submit" class="btn btn-primary" :disabled="saving">
            <span>{{ saving ? '💾 Saving Changes...' : '💾 Save General Settings' }}</span>
          </button>
        </div>
      </form>
    </div>

    <!-- SRE Node Auto-Remediation & Fast-Failover Policy -->
    <NodeRemediationSettings />
  </div>
</template>
