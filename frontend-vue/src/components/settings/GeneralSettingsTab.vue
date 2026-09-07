<script setup lang="ts">
import type { SettingsFormState, CategoryKey } from '../../composables/useSettings'
import { timezoneOptions, languageOptions, environmentOptions } from '../../composables/useSettings'

defineProps<{
  form: SettingsFormState
  saving: boolean
  isDirty: boolean
}>()

const emit = defineEmits<{
  (e: 'save', category: CategoryKey): void
  (e: 'reset', category: CategoryKey): void
}>()
</script>

<template>
  <div class="settings-card glass-panel animate-fade-in">
    <div class="card-header">
      <div class="card-title-group">
        <div>
          <h2 class="card-title">Cluster Profile & Platform Settings</h2>
          <p class="card-subtitle">
            Configure cluster identity, operational environment tier, maintenance windows, and localization.
          </p>
        </div>
        <span v-if="isDirty" class="dirty-indicator-pill">● Unsaved Changes</span>
      </div>
      <button
        type="button"
        class="btn btn-secondary btn-sm"
        @click="emit('reset', 'platform')"
      >
        <span>↺ Reset Defaults</span>
      </button>
    </div>

    <form class="settings-form" @submit.prevent="emit('save', 'platform')">
      <!-- Cluster Name -->
      <div class="form-group">
        <label class="form-label" for="platform-name">
          <span>Cluster / Platform Name</span>
          <span class="required">*</span>
        </label>
        <p class="field-desc">The organizational cluster title displayed across the navigation header and audit reports.</p>
        <input
          id="platform-name"
          v-model="form.name"
          type="text"
          class="input-glass form-input"
          placeholder="e.g. K8s Self-Host Platform"
          required
        />
      </div>

      <!-- Environment Tier Badges -->
      <div class="form-group">
        <label class="form-label">
          <span>Target Environment Tier</span>
        </label>
        <p class="field-desc">Sets system governance strictness, telemetry sampling rate, and warning thresholds.</p>
        <div class="provider-selector-grid">
          <div
            v-for="env in environmentOptions"
            :key="env.value"
            class="provider-card"
            :class="{ active: form.environment === env.value }"
            @click="form.environment = env.value as any"
          >
            <div class="provider-card-head">
              <span class="badge" :class="'badge-' + env.badgeColor">{{ env.value.toUpperCase() }}</span>
            </div>
            <p class="provider-desc">{{ env.label }}</p>
          </div>
        </div>
      </div>

      <!-- Maintenance Mode Toggle -->
      <div class="form-group toggle-group">
        <div class="toggle-info">
          <span class="toggle-label">Cluster Maintenance Mode</span>
          <p class="field-desc">
            Temporarily pauses automated deployments, rollouts, and non-essential background reconciliations.
          </p>
        </div>
        <label class="toggle-switch">
          <input v-model="form.maintenance_mode" type="checkbox" />
          <span class="toggle-slider"></span>
        </label>
      </div>

      <!-- Timezone & Language -->
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

      <!-- Actions -->
      <div class="form-actions">
        <span class="field-desc">Changes take effect globally across all active tenant nodes.</span>
        <button type="submit" class="btn btn-primary" :disabled="saving">
          <span v-if="saving" class="spinner spinner-sm"></span>
          <span>{{ saving ? '💾 Saving Changes...' : '💾 Save General Settings' }}</span>
        </button>
      </div>
    </form>
  </div>
</template>
