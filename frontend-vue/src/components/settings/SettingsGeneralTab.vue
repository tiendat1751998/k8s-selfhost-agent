<script setup lang="ts">
import BaseIcon from '../ui/BaseIcon.vue'
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
      </div>

      <form class="settings-form" @submit.prevent="emit('save', 'platform')">
        <!-- Row 1: Platform Name -->
        <div class="setting-row">
          <div class="setting-meta">
            <label class="setting-title" for="platform-name">
              <span>Platform Name</span>
              <span class="required">*</span>
            </label>
            <p class="setting-desc">The organizational title displayed across the navigation header and reports.</p>
          </div>
          <div class="setting-control-col">
            <input
              id="platform-name"
              v-model="form.name"
              type="text"
              class="input-glass form-input input-compact-lg"
              placeholder="e.g. K8s Self-Host Platform"
              required
            />
          </div>
        </div>

        <!-- Row 2: Display Timezone -->
        <div class="setting-row">
          <div class="setting-meta">
            <label class="setting-title" for="platform-tz">Display Timezone</label>
            <p class="setting-desc">Default timezone used for metric timestamps, log streams, and audit history.</p>
          </div>
          <div class="setting-control-col">
            <select id="platform-tz" v-model="form.timezone" class="input-glass form-select input-compact-md">
              <option v-for="tz in timezoneOptions" :key="tz.value" :value="tz.value">
                {{ tz.label }}
              </option>
            </select>
          </div>
        </div>

        <!-- Row 3: Display Language -->
        <div class="setting-row">
          <div class="setting-meta">
            <label class="setting-title" for="platform-lang">Display Language</label>
            <p class="setting-desc">Interface localization preference for menus, alerts, and notifications.</p>
          </div>
          <div class="setting-control-col">
            <select id="platform-lang" v-model="form.language" class="input-glass form-select input-compact-md">
              <option v-for="lang in languageOptions" :key="lang.value" :value="lang.value">
                {{ lang.label }}
              </option>
            </select>
          </div>
        </div>

        <!-- Card Actions Bar -->
        <div class="card-actions-bar">
          <button type="button" class="btn btn-secondary btn-sm" @click="emit('reset', 'platform')">
            <BaseIcon name="rotate-ccw" size="xs" /> <span>Reset Defaults</span>
          </button>
          <button type="submit" class="btn btn-primary btn-sm" :disabled="saving">
            <BaseIcon name="hard-drive" size="xs" /> <span>{{ saving ? 'Saving Changes...' : 'Save General Settings' }}</span>
          </button>
        </div>
      </form>
    </div>

    <!-- SRE Node Auto-Remediation & Fast-Failover Policy -->
    <NodeRemediationSettings />
  </div>
</template>
