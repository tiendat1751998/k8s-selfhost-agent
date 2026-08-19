<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import MetricCard from '../components/ui/MetricCard.vue'
import {
  settingsApi,
  type Setting,
  type SettingUpdate,
  type IntegrationTestResult
} from '../api/settings'

// Active Tab State
type TabKey = 'general' | 'security' | 'notifications' | 'integrations' | 'about'
const activeTab = ref<TabKey>('general')

// Loading and Status States
const loading = ref(true)
const saving = ref(false)
const statusMessage = ref<{ text: string; type: 'success' | 'error' } | null>(null)
let messageTimer: ReturnType<typeof setTimeout> | null = null

function showMessage(text: string, type: 'success' | 'error' = 'success') {
  if (messageTimer) clearTimeout(messageTimer)
  statusMessage.value = { text, type }
  messageTimer = setTimeout(() => {
    statusMessage.value = null
  }, 4000)
}

// Reactive Form State
const form = reactive({
  // General (platform)
  name: 'K8s Self-Host Platform',
  timezone: 'UTC',
  language: 'en',

  // Security (security)
  session_timeout_minutes: 60,
  password_min_length: 8,
  require_2fa: false,

  // Notifications (notifications)
  smtp_enabled: false,
  smtp_host: '',
  smtp_port: 587,
  webhook_url: '',

  // Integrations (integrations)
  argocd_url: '',
  trivy_url: '',
  vault_url: '',
  grafana_url: '',
})

// Store defaults received from backend
const defaultSettings = ref<Record<string, Record<string, string>>>({})

// Integration Test State
interface TestState {
  testing: boolean
  result: IntegrationTestResult | null
  error?: string
}

type IntegrationKey = 'argocd_url' | 'trivy_url' | 'vault_url' | 'grafana_url'

const integrationTests = reactive<Record<IntegrationKey, TestState>>({
  argocd_url: { testing: false, result: null },
  trivy_url: { testing: false, result: null },
  vault_url: { testing: false, result: null },
  grafana_url: { testing: false, result: null },
})

// Timezone & Language Options
const timezoneOptions = [
  { value: 'UTC', label: '🌐 UTC — Coordinated Universal Time' },
  { value: 'Asia/Ho_Chi_Minh', label: '🇻🇳 Asia/Ho_Chi_Minh — Indochina Time (UTC+7)' },
  { value: 'Asia/Singapore', label: '🇸🇬 Asia/Singapore — Singapore Time (UTC+8)' },
  { value: 'Asia/Tokyo', label: '🇯🇵 Asia/Tokyo — Japan Standard Time (UTC+9)' },
  { value: 'Europe/London', label: '🇬🇧 Europe/London — Greenwich Mean Time (UTC+0/+1)' },
  { value: 'Europe/Berlin', label: '🇩🇪 Europe/Berlin — Central European Time (UTC+1/+2)' },
  { value: 'America/New_York', label: '🇺🇸 America/New_York — Eastern Time (UTC-5/-4)' },
  { value: 'America/Chicago', label: '🇺🇸 America/Chicago — Central Time (UTC-6/-5)' },
  { value: 'America/Los_Angeles', label: '🇺🇸 America/Los_Angeles — Pacific Time (UTC-8/-7)' },
  { value: 'Australia/Sydney', label: '🇦🇺 Australia/Sydney — Eastern Australia (UTC+10/+11)' },
]

const languageOptions = [
  { value: 'en', label: '🇺🇸 English (United States)' },
  { value: 'vi', label: '🇻🇳 Tiếng Việt (Vietnam)' },
]

// Computed Metrics
const configuredIntegrationsCount = computed(() => {
  let count = 0
  if (form.argocd_url?.trim()) count++
  if (form.trivy_url?.trim()) count++
  if (form.vault_url?.trim()) count++
  if (form.grafana_url?.trim()) count++
  return count
})

// Populate Form from API
function populateFormFromSettings(settingsList: Setting[]) {
  for (const item of settingsList) {
    const val = item.value
    switch (item.key) {
      // Platform
      case 'name':
        form.name = val || 'K8s Self-Host Platform'
        break
      case 'timezone':
        form.timezone = val || 'UTC'
        break
      case 'language':
        form.language = val || 'en'
        break

      // Security
      case 'session_timeout_minutes':
        form.session_timeout_minutes = parseInt(val, 10) || 60
        break
      case 'password_min_length':
        form.password_min_length = parseInt(val, 10) || 8
        break
      case 'require_2fa':
        form.require_2fa = val === 'true'
        break

      // Notifications
      case 'smtp_enabled':
        form.smtp_enabled = val === 'true'
        break
      case 'smtp_host':
        form.smtp_host = val || ''
        break
      case 'smtp_port':
        form.smtp_port = parseInt(val, 10) || 587
        break
      case 'webhook_url':
        form.webhook_url = val || ''
        break

      // Integrations
      case 'argocd_url':
        form.argocd_url = val || ''
        break
      case 'trivy_url':
        form.trivy_url = val || ''
        break
      case 'vault_url':
        form.vault_url = val || ''
        break
      case 'grafana_url':
        form.grafana_url = val || ''
        break
    }
  }
}

// Fetch all settings & defaults
async function loadSettings() {
  loading.value = true
  try {
    // 1. Fetch defaults
    let defaults: Record<string, Record<string, string>> = {}
    try {
      defaults = await settingsApi.getDefaults()
    } catch {
      defaults = {}
    }
    defaultSettings.value = defaults

    // Apply defaults to form initially
    if (defaults.platform) {
      if (defaults.platform.name) form.name = defaults.platform.name
      if (defaults.platform.timezone) form.timezone = defaults.platform.timezone
      if (defaults.platform.language) form.language = defaults.platform.language
    }
    if (defaults.security) {
      if (defaults.security.session_timeout_minutes)
        form.session_timeout_minutes = parseInt(defaults.security.session_timeout_minutes, 10) || 60
      if (defaults.security.password_min_length)
        form.password_min_length = parseInt(defaults.security.password_min_length, 10) || 8
      if (defaults.security.require_2fa)
        form.require_2fa = defaults.security.require_2fa === 'true'
    }
    if (defaults.notifications) {
      if (defaults.notifications.smtp_enabled)
        form.smtp_enabled = defaults.notifications.smtp_enabled === 'true'
      if (defaults.notifications.smtp_host)
        form.smtp_host = defaults.notifications.smtp_host
      if (defaults.notifications.smtp_port)
        form.smtp_port = parseInt(defaults.notifications.smtp_port, 10) || 587
      if (defaults.notifications.webhook_url)
        form.webhook_url = defaults.notifications.webhook_url
    }
    if (defaults.integrations) {
      if (defaults.integrations.argocd_url) form.argocd_url = defaults.integrations.argocd_url
      if (defaults.integrations.trivy_url) form.trivy_url = defaults.integrations.trivy_url
      if (defaults.integrations.vault_url) form.vault_url = defaults.integrations.vault_url
      if (defaults.integrations.grafana_url) form.grafana_url = defaults.integrations.grafana_url
    }

    // 2. Fetch saved settings
    const settingsList = await settingsApi.getAll()
    if (Array.isArray(settingsList) && settingsList.length > 0) {
      populateFormFromSettings(settingsList)
    }
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : 'Failed to load platform settings'
    showMessage(msg, 'error')
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  loadSettings()
})

// Save Category Settings
async function saveCategory(category: 'platform' | 'security' | 'notifications' | 'integrations') {
  saving.value = true
  try {
    const updates: SettingUpdate[] = []

    if (category === 'platform') {
      updates.push(
        { category: 'platform', key: 'name', value: form.name.trim() },
        { category: 'platform', key: 'timezone', value: form.timezone },
        { category: 'platform', key: 'language', value: form.language }
      )
    } else if (category === 'security') {
      updates.push(
        { category: 'security', key: 'session_timeout_minutes', value: String(form.session_timeout_minutes) },
        { category: 'security', key: 'password_min_length', value: String(form.password_min_length) },
        { category: 'security', key: 'require_2fa', value: form.require_2fa ? 'true' : 'false' }
      )
    } else if (category === 'notifications') {
      updates.push(
        { category: 'notifications', key: 'smtp_enabled', value: form.smtp_enabled ? 'true' : 'false' },
        { category: 'notifications', key: 'smtp_host', value: form.smtp_host.trim() },
        { category: 'notifications', key: 'smtp_port', value: String(form.smtp_port) },
        { category: 'notifications', key: 'webhook_url', value: form.webhook_url.trim() }
      )
    } else if (category === 'integrations') {
      updates.push(
        { category: 'integrations', key: 'argocd_url', value: form.argocd_url.trim() },
        { category: 'integrations', key: 'trivy_url', value: form.trivy_url.trim() },
        { category: 'integrations', key: 'vault_url', value: form.vault_url.trim() },
        { category: 'integrations', key: 'grafana_url', value: form.grafana_url.trim() }
      )
    }

    await settingsApi.update(updates)
    showMessage('Settings saved successfully', 'success')
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : 'Failed to save settings'
    showMessage(msg, 'error')
  } finally {
    saving.value = false
  }
}

// Reset Tab Category to Default Values
function resetCategoryToDefaults(category: 'platform' | 'security' | 'notifications' | 'integrations') {
  const catDefaults = defaultSettings.value[category]
  if (!catDefaults) {
    showMessage('No default values found for this category', 'error')
    return
  }

  if (category === 'platform') {
    form.name = catDefaults.name || 'K8s Self-Host Platform'
    form.timezone = catDefaults.timezone || 'UTC'
    form.language = catDefaults.language || 'en'
  } else if (category === 'security') {
    form.session_timeout_minutes = parseInt(catDefaults.session_timeout_minutes || '60', 10)
    form.password_min_length = parseInt(catDefaults.password_min_length || '8', 10)
    form.require_2fa = catDefaults.require_2fa === 'true'
  } else if (category === 'notifications') {
    form.smtp_enabled = catDefaults.smtp_enabled === 'true'
    form.smtp_host = catDefaults.smtp_host || ''
    form.smtp_port = parseInt(catDefaults.smtp_port || '587', 10)
    form.webhook_url = catDefaults.webhook_url || ''
  } else if (category === 'integrations') {
    form.argocd_url = catDefaults.argocd_url || ''
    form.trivy_url = catDefaults.trivy_url || ''
    form.vault_url = catDefaults.vault_url || ''
    form.grafana_url = catDefaults.grafana_url || ''
  }

  showMessage(`Reset ${category} settings to defaults. Click Save to persist.`, 'success')
}

// Test Integration Service Reachability
async function testService(key: IntegrationKey) {
  const url = form[key]
  if (!url || !url.trim()) {
    integrationTests[key] = {
      testing: false,
      result: null,
      error: 'Please enter a valid URL before testing connectivity'
    }
    return
  }

  integrationTests[key] = { testing: true, result: null }

  try {
    const res = await settingsApi.testIntegration(url.trim())
    integrationTests[key] = {
      testing: false,
      result: res,
    }
  } catch (err: unknown) {
    integrationTests[key] = {
      testing: false,
      result: null,
      error: err instanceof Error ? err.message : 'Reachability check failed'
    }
  }
}
</script>

<template>
  <div class="view-container animate-fade-in">
    <!-- View Header -->
    <div class="view-header">
      <div>
        <div class="view-tag">
          <span class="pulse-dot pulse-dot-cyan"></span>
          <span>GLOBAL PLATFORM CONFIGURATION</span>
        </div>
        <h1 class="view-title">Settings & System Preferences</h1>
        <p class="view-desc">
          Manage tenant-level policies, authentication parameters, outbound alert routing, and third-party DevOps integrations.
        </p>
      </div>

      <div class="header-actions">
        <button class="btn btn-secondary" :disabled="loading" @click="loadSettings">
          <span>{{ loading ? '⏳ Syncing...' : '🔄 Refresh Settings' }}</span>
        </button>
      </div>
    </div>

    <!-- Notification Banner -->
    <div
      v-if="statusMessage"
      class="status-banner animate-fade-in"
      :class="'banner-' + statusMessage.type"
    >
      <span class="banner-icon">{{ statusMessage.type === 'success' ? '✅' : '⚠️' }}</span>
      <span class="banner-text">{{ statusMessage.text }}</span>
      <button class="banner-close" @click="statusMessage = null">✕</button>
    </div>

    <!-- Key Metrics Summary HUD -->
    <div class="metrics-grid">
      <MetricCard
        title="Platform Console"
        :value="form.name || 'Self-Host K8s'"
        badge="ONLINE"
        badge-color="cyan"
        :subtitle="`Zone: ${form.timezone} | Lang: ${form.language.toUpperCase()}`"
        icon="🌐"
      />
      <MetricCard
        title="Security Policy"
        :value="form.require_2fa ? '2FA Enforced' : 'Standard 2FA'"
        :badge="form.require_2fa ? 'STRICT' : 'FLEXIBLE'"
        :badge-color="form.require_2fa ? 'emerald' : 'amber'"
        :subtitle="`Timeout: ${form.session_timeout_minutes}m | Min Pass: ${form.password_min_length}`"
        icon="🛡️"
      />
      <MetricCard
        title="Alert Transports"
        :value="form.smtp_enabled ? 'SMTP Enabled' : 'Webhook Only'"
        :badge="form.smtp_enabled || form.webhook_url ? 'ACTIVE' : 'IDLE'"
        :badge-color="form.smtp_enabled || form.webhook_url ? 'emerald' : 'muted'"
        :subtitle="form.webhook_url ? 'Webhook URL Configured' : 'No Webhook Set'"
        icon="🔔"
      />
      <MetricCard
        title="Active Integrations"
        :value="`${configuredIntegrationsCount} / 4`"
        :badge="configuredIntegrationsCount > 0 ? 'CONNECTED' : 'DISCONNECTED'"
        :badge-color="configuredIntegrationsCount > 0 ? 'cyan' : 'muted'"
        subtitle="ArgoCD, Trivy, Vault, Grafana"
        icon="🔌"
      />
    </div>

    <!-- Tabs Navigation Bar -->
    <div class="tabs-bar glass-panel">
      <button
        class="tab-btn"
        :class="{ 'tab-btn-active': activeTab === 'general' }"
        @click="activeTab = 'general'"
      >
        <span>⚙️ General</span>
      </button>
      <button
        class="tab-btn"
        :class="{ 'tab-btn-active': activeTab === 'security' }"
        @click="activeTab = 'security'"
      >
        <span>🛡️ Security</span>
      </button>
      <button
        class="tab-btn"
        :class="{ 'tab-btn-active': activeTab === 'notifications' }"
        @click="activeTab = 'notifications'"
      >
        <span>🔔 Notifications</span>
      </button>
      <button
        class="tab-btn"
        :class="{ 'tab-btn-active': activeTab === 'integrations' }"
        @click="activeTab = 'integrations'"
      >
        <span>🔌 Integrations</span>
        <span class="tab-badge">{{ configuredIntegrationsCount }}</span>
      </button>
      <button
        class="tab-btn"
        :class="{ 'tab-btn-active': activeTab === 'about' }"
        @click="activeTab = 'about'"
      >
        <span>ℹ️ About</span>
      </button>
    </div>

    <!-- Loading State -->
    <div v-if="loading" class="loading-state glass-panel">
      <div class="spinner"></div>
      <span>Loading platform settings from cluster API...</span>
    </div>

    <!-- TAB CONTENTS -->
    <div v-else class="tab-content">
      <!-- TAB 1: GENERAL (platform) -->
      <div v-if="activeTab === 'general'" class="settings-card glass-panel animate-fade-in">
        <div class="card-header">
          <div>
            <h2 class="card-title">General Platform Settings</h2>
            <p class="card-subtitle">
              Manage platform identity, default regional timezone, and display language preferences.
            </p>
          </div>
          <button class="btn btn-secondary btn-sm" @click="resetCategoryToDefaults('platform')">
            <span>↺ Reset Defaults</span>
          </button>
        </div>

        <form class="settings-form" @submit.prevent="saveCategory('platform')">
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

      <!-- TAB 2: SECURITY (security) -->
      <div v-if="activeTab === 'security'" class="settings-card glass-panel animate-fade-in">
        <div class="card-header">
          <div>
            <h2 class="card-title">Security & Access Policy</h2>
            <p class="card-subtitle">
              Configure session lifetime thresholds, credential complexity minimums, and MFA mandates.
            </p>
          </div>
          <button class="btn btn-secondary btn-sm" @click="resetCategoryToDefaults('security')">
            <span>↺ Reset Defaults</span>
          </button>
        </div>

        <form class="settings-form" @submit.prevent="saveCategory('security')">
          <div class="form-row">
            <div class="form-group flex-1">
              <label class="form-label" for="sec-timeout">
                <span>Session Timeout (Minutes)</span>
                <span class="required">*</span>
              </label>
              <p class="field-desc">Inactivity duration before automatic token invalidation (5 to 1440 minutes).</p>
              <input
                id="sec-timeout"
                v-model.number="form.session_timeout_minutes"
                type="number"
                min="5"
                max="1440"
                class="input-glass form-input"
                required
              />
            </div>

            <div class="form-group flex-1">
              <label class="form-label" for="sec-pass-len">
                <span>Minimum Password Length</span>
                <span class="required">*</span>
              </label>
              <p class="field-desc">Minimum character count required for tenant user credentials.</p>
              <input
                id="sec-pass-len"
                v-model.number="form.password_min_length"
                type="number"
                min="6"
                max="64"
                class="input-glass form-input"
                required
              />
            </div>
          </div>

          <div class="form-group toggle-group">
            <div class="toggle-info">
              <span class="toggle-label">Require Multi-Factor Authentication (2FA)</span>
              <p class="field-desc">
                Mandate TOTP authenticator verification or WebAuthn hardware tokens for all cluster operators.
              </p>
            </div>
            <label class="toggle-switch">
              <input v-model="form.require_2fa" type="checkbox" />
              <span class="toggle-slider"></span>
            </label>
          </div>

          <div class="security-info-box">
            <div class="info-box-header">
              <span>🔒 Zero-Trust Enforcement Notice</span>
            </div>
            <p>
              Modifications to security policies are applied across all active tenant sessions immediately upon saving.
              Token invalidation triggers seamless re-authentication without loss of background telemetry.
            </p>
          </div>

          <div class="form-actions">
            <button type="submit" class="btn btn-primary" :disabled="saving">
              <span>{{ saving ? '💾 Saving Changes...' : '💾 Save Security Settings' }}</span>
            </button>
          </div>
        </form>
      </div>

      <!-- TAB 3: NOTIFICATIONS (notifications) -->
      <div v-if="activeTab === 'notifications'" class="settings-card glass-panel animate-fade-in">
        <div class="card-header">
          <div>
            <h2 class="card-title">Notifications & Alert Routing</h2>
            <p class="card-subtitle">
              Set up outbound SMTP mail relay and incoming webhook endpoints for real-time cluster incident dispatch.
            </p>
          </div>
          <button class="btn btn-secondary btn-sm" @click="resetCategoryToDefaults('notifications')">
            <span>↺ Reset Defaults</span>
          </button>
        </div>

        <form class="settings-form" @submit.prevent="saveCategory('notifications')">
          <div class="form-group toggle-group">
            <div class="toggle-info">
              <span class="toggle-label">Enable SMTP Outbound Email Relay</span>
              <p class="field-desc">
                Allows the platform to send alert digests, scheduled PDF reports, and critical event notifications.
              </p>
            </div>
            <label class="toggle-switch">
              <input v-model="form.smtp_enabled" type="checkbox" />
              <span class="toggle-slider"></span>
            </label>
          </div>

          <div class="form-row">
            <div class="form-group flex-2">
              <label class="form-label" for="smtp-host" :class="{ 'label-disabled': !form.smtp_enabled }">
                <span>SMTP Host</span>
              </label>
              <p class="field-desc">FQDN or IP address of your mail transport agent (e.g. smtp.sendgrid.net).</p>
              <input
                id="smtp-host"
                v-model="form.smtp_host"
                type="text"
                class="input-glass form-input"
                :disabled="!form.smtp_enabled"
                placeholder="smtp.mailgun.org or smtp.office365.com"
              />
            </div>

            <div class="form-group flex-1">
              <label class="form-label" for="smtp-port" :class="{ 'label-disabled': !form.smtp_enabled }">
                <span>SMTP Port</span>
              </label>
              <p class="field-desc">Port for TLS/STARTTLS dispatch (typically 587 or 465).</p>
              <input
                id="smtp-port"
                v-model.number="form.smtp_port"
                type="number"
                class="input-glass form-input"
                :disabled="!form.smtp_enabled"
                placeholder="587"
              />
            </div>
          </div>

          <div class="form-group">
            <label class="form-label" for="webhook-url">
              <span>Incident Webhook URL</span>
            </label>
            <p class="field-desc">
              HTTP POST endpoint for Slack, Discord, PagerDuty, or Microsoft Teams webhook integrations.
            </p>
            <input
              id="webhook-url"
              v-model="form.webhook_url"
              type="url"
              class="input-glass form-input"
              placeholder="https://hooks.slack.com/services/T00/B00/XXXXX"
            />
          </div>

          <div class="form-actions">
            <button type="submit" class="btn btn-primary" :disabled="saving">
              <span>{{ saving ? '💾 Saving Changes...' : '💾 Save Notification Settings' }}</span>
            </button>
          </div>
        </form>
      </div>

      <!-- TAB 4: INTEGRATIONS (integrations) -->
      <div v-if="activeTab === 'integrations'" class="settings-card glass-panel animate-fade-in">
        <div class="card-header">
          <div>
            <h2 class="card-title">DevOps & Toolchain Integrations</h2>
            <p class="card-subtitle">
              Connect external continuous delivery pipelines, vulnerability scanners, secrets vaults, and observability engines.
            </p>
          </div>
          <button class="btn btn-secondary btn-sm" @click="resetCategoryToDefaults('integrations')">
            <span>↺ Reset Defaults</span>
          </button>
        </div>

        <form class="settings-form" @submit.prevent="saveCategory('integrations')">
          <div class="integrations-grid">
            <!-- ArgoCD Integration -->
            <div class="integration-item glass-panel">
              <div class="integration-header">
                <div class="integration-title-group">
                  <div class="integration-icon">🐙</div>
                  <div>
                    <h3 class="integration-name">ArgoCD Continuous Delivery</h3>
                    <p class="integration-desc">GitOps synchronization and application lifecycle management.</p>
                  </div>
                </div>
                <div class="integration-status">
                  <span
                    v-if="integrationTests.argocd_url.testing"
                    class="badge badge-amber"
                  >
                    ⏳ Testing...
                  </span>
                  <span
                    v-else-if="integrationTests.argocd_url.result?.reachable"
                    class="badge badge-emerald"
                  >
                    ✓ HTTP {{ integrationTests.argocd_url.result.status_code }} ({{ integrationTests.argocd_url.result.latency_ms }}ms)
                  </span>
                  <span
                    v-else-if="integrationTests.argocd_url.result && !integrationTests.argocd_url.result.reachable"
                    class="badge badge-rose"
                  >
                    ✗ Unreachable ({{ integrationTests.argocd_url.result.latency_ms }}ms)
                  </span>
                  <span
                    v-else-if="integrationTests.argocd_url.error"
                    class="badge badge-rose"
                  >
                    ✗ {{ integrationTests.argocd_url.error }}
                  </span>
                  <span
                    v-else-if="form.argocd_url"
                    class="badge badge-cyan"
                  >
                    CONFIGURED
                  </span>
                  <span v-else class="badge badge-muted">NOT CONFIGURED</span>
                </div>
              </div>

              <div class="integration-input-row">
                <input
                  v-model="form.argocd_url"
                  type="url"
                  class="input-glass form-input flex-1"
                  placeholder="https://argocd.internal.company.com"
                />
                <button
                  type="button"
                  class="btn btn-secondary btn-sm"
                  :disabled="integrationTests.argocd_url.testing || !form.argocd_url"
                  @click="testService('argocd_url')"
                >
                  <span>{{ integrationTests.argocd_url.testing ? '⏳ Testing...' : '⚡ Test Reachability' }}</span>
                </button>
              </div>
            </div>

            <!-- Trivy Integration -->
            <div class="integration-item glass-panel">
              <div class="integration-header">
                <div class="integration-title-group">
                  <div class="integration-icon">🛡️</div>
                  <div>
                    <h3 class="integration-name">Trivy Security Scanner</h3>
                    <p class="integration-desc">Container image vulnerability & IaC misconfiguration scanner.</p>
                  </div>
                </div>
                <div class="integration-status">
                  <span
                    v-if="integrationTests.trivy_url.testing"
                    class="badge badge-amber"
                  >
                    ⏳ Testing...
                  </span>
                  <span
                    v-else-if="integrationTests.trivy_url.result?.reachable"
                    class="badge badge-emerald"
                  >
                    ✓ HTTP {{ integrationTests.trivy_url.result.status_code }} ({{ integrationTests.trivy_url.result.latency_ms }}ms)
                  </span>
                  <span
                    v-else-if="integrationTests.trivy_url.result && !integrationTests.trivy_url.result.reachable"
                    class="badge badge-rose"
                  >
                    ✗ Unreachable ({{ integrationTests.trivy_url.result.latency_ms }}ms)
                  </span>
                  <span
                    v-else-if="integrationTests.trivy_url.error"
                    class="badge badge-rose"
                  >
                    ✗ {{ integrationTests.trivy_url.error }}
                  </span>
                  <span
                    v-else-if="form.trivy_url"
                    class="badge badge-cyan"
                  >
                    CONFIGURED
                  </span>
                  <span v-else class="badge badge-muted">NOT CONFIGURED</span>
                </div>
              </div>

              <div class="integration-input-row">
                <input
                  v-model="form.trivy_url"
                  type="url"
                  class="input-glass form-input flex-1"
                  placeholder="http://trivy.security.svc:4954"
                />
                <button
                  type="button"
                  class="btn btn-secondary btn-sm"
                  :disabled="integrationTests.trivy_url.testing || !form.trivy_url"
                  @click="testService('trivy_url')"
                >
                  <span>{{ integrationTests.trivy_url.testing ? '⏳ Testing...' : '⚡ Test Reachability' }}</span>
                </button>
              </div>
            </div>

            <!-- HashiCorp Vault Integration -->
            <div class="integration-item glass-panel">
              <div class="integration-header">
                <div class="integration-title-group">
                  <div class="integration-icon">🔐</div>
                  <div>
                    <h3 class="integration-name">HashiCorp Vault</h3>
                    <p class="integration-desc">Centralized secret storage, PKI encryption, and dynamic lease manager.</p>
                  </div>
                </div>
                <div class="integration-status">
                  <span
                    v-if="integrationTests.vault_url.testing"
                    class="badge badge-amber"
                  >
                    ⏳ Testing...
                  </span>
                  <span
                    v-else-if="integrationTests.vault_url.result?.reachable"
                    class="badge badge-emerald"
                  >
                    ✓ HTTP {{ integrationTests.vault_url.result.status_code }} ({{ integrationTests.vault_url.result.latency_ms }}ms)
                  </span>
                  <span
                    v-else-if="integrationTests.vault_url.result && !integrationTests.vault_url.result.reachable"
                    class="badge badge-rose"
                  >
                    ✗ Unreachable ({{ integrationTests.vault_url.result.latency_ms }}ms)
                  </span>
                  <span
                    v-else-if="integrationTests.vault_url.error"
                    class="badge badge-rose"
                  >
                    ✗ {{ integrationTests.vault_url.error }}
                  </span>
                  <span
                    v-else-if="form.vault_url"
                    class="badge badge-cyan"
                  >
                    CONFIGURED
                  </span>
                  <span v-else class="badge badge-muted">NOT CONFIGURED</span>
                </div>
              </div>

              <div class="integration-input-row">
                <input
                  v-model="form.vault_url"
                  type="url"
                  class="input-glass form-input flex-1"
                  placeholder="https://vault.internal.company.com:8200"
                />
                <button
                  type="button"
                  class="btn btn-secondary btn-sm"
                  :disabled="integrationTests.vault_url.testing || !form.vault_url"
                  @click="testService('vault_url')"
                >
                  <span>{{ integrationTests.vault_url.testing ? '⏳ Testing...' : '⚡ Test Reachability' }}</span>
                </button>
              </div>
            </div>

            <!-- Grafana Integration -->
            <div class="integration-item glass-panel">
              <div class="integration-header">
                <div class="integration-title-group">
                  <div class="integration-icon">📊</div>
                  <div>
                    <h3 class="integration-name">Grafana Observability</h3>
                    <p class="integration-desc">Time-series visual metrics, dashboards, and Loki log exploration.</p>
                  </div>
                </div>
                <div class="integration-status">
                  <span
                    v-if="integrationTests.grafana_url.testing"
                    class="badge badge-amber"
                  >
                    ⏳ Testing...
                  </span>
                  <span
                    v-else-if="integrationTests.grafana_url.result?.reachable"
                    class="badge badge-emerald"
                  >
                    ✓ HTTP {{ integrationTests.grafana_url.result.status_code }} ({{ integrationTests.grafana_url.result.latency_ms }}ms)
                  </span>
                  <span
                    v-else-if="integrationTests.grafana_url.result && !integrationTests.grafana_url.result.reachable"
                    class="badge badge-rose"
                  >
                    ✗ Unreachable ({{ integrationTests.grafana_url.result.latency_ms }}ms)
                  </span>
                  <span
                    v-else-if="integrationTests.grafana_url.error"
                    class="badge badge-rose"
                  >
                    ✗ {{ integrationTests.grafana_url.error }}
                  </span>
                  <span
                    v-else-if="form.grafana_url"
                    class="badge badge-cyan"
                  >
                    CONFIGURED
                  </span>
                  <span v-else class="badge badge-muted">NOT CONFIGURED</span>
                </div>
              </div>

              <div class="integration-input-row">
                <input
                  v-model="form.grafana_url"
                  type="url"
                  class="input-glass form-input flex-1"
                  placeholder="https://grafana.internal.company.com"
                />
                <button
                  type="button"
                  class="btn btn-secondary btn-sm"
                  :disabled="integrationTests.grafana_url.testing || !form.grafana_url"
                  @click="testService('grafana_url')"
                >
                  <span>{{ integrationTests.grafana_url.testing ? '⏳ Testing...' : '⚡ Test Reachability' }}</span>
                </button>
              </div>
            </div>
          </div>

          <div class="form-actions">
            <button type="submit" class="btn btn-primary" :disabled="saving">
              <span>{{ saving ? '💾 Saving Changes...' : '💾 Save Integration Settings' }}</span>
            </button>
          </div>
        </form>
      </div>

      <!-- TAB 5: ABOUT -->
      <div v-if="activeTab === 'about'" class="about-container animate-fade-in">
        <div class="settings-card glass-panel">
          <div class="about-hero">
            <div class="about-logo">⎈</div>
            <div class="about-hero-text">
              <div class="about-version-tag">
                <span class="badge badge-emerald">v0.1.0 STABLE</span>
                <span class="badge badge-cyan">ENTERPRISE EDITION</span>
              </div>
              <h2 class="about-title">Kubernetes Self-Host Management Platform</h2>
              <p class="about-desc">
                High-performance unified Kubernetes control plane, AI-driven root cause diagnosis, GitOps orchestration,
                and zero-trust multi-tenancy suite.
              </p>
            </div>
          </div>

          <div class="spec-grid">
            <div class="spec-card glass-panel">
              <div class="spec-icon">⚡</div>
              <div class="spec-meta">
                <span class="spec-label">Backend Control Plane</span>
                <span class="spec-value">Go 1.22+ (Chi Router / Hexagonal Arch)</span>
              </div>
            </div>

            <div class="spec-card glass-panel">
              <div class="spec-icon">🎨</div>
              <div class="spec-meta">
                <span class="spec-label">Frontend Framework</span>
                <span class="spec-value">Vue 3.4+ / Vite 5 / TypeScript</span>
              </div>
            </div>

            <div class="spec-card glass-panel">
              <div class="spec-icon">🗄️</div>
              <div class="spec-meta">
                <span class="spec-label">Primary Database</span>
                <span class="spec-value">PostgreSQL 16 Multi-Tenant</span>
              </div>
            </div>

            <div class="spec-card glass-panel">
              <div class="spec-icon">🚀</div>
              <div class="spec-meta">
                <span class="spec-label">In-Memory Cache & PubSub</span>
                <span class="spec-value">Redis 7 Alpine</span>
              </div>
            </div>

            <div class="spec-card glass-panel">
              <div class="spec-icon">📜</div>
              <div class="spec-meta">
                <span class="spec-label">Software License</span>
                <span class="spec-value">MIT Open Source License</span>
              </div>
            </div>

            <div class="spec-card glass-panel">
              <div class="spec-icon">🔒</div>
              <div class="spec-meta">
                <span class="spec-label">Authentication Scheme</span>
                <span class="spec-value">JWT Bearer Auth & Tenant Isolation</span>
              </div>
            </div>
          </div>

          <div class="about-footer">
            <div class="about-copyright">
              <span>© 2026 K8s Self-Host Platform. Built for autonomous cloud-native operations.</span>
            </div>
            <div class="about-links">
              <span class="about-link">Documentation</span>
              <span class="dot-sep">•</span>
              <span class="about-link">REST API Docs</span>
              <span class="dot-sep">•</span>
              <span class="about-link">GitHub Repository</span>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.view-container {
  display: flex;
  flex-direction: column;
  gap: 22px;
}

.view-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  flex-wrap: wrap;
  gap: 16px;
}

.view-tag {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  font-size: 11px;
  font-weight: 700;
  color: var(--accent-cyan);
  letter-spacing: 0.05em;
  margin-bottom: 6px;
}

.view-title {
  font-size: 24px;
  font-weight: 800;
  color: #fff;
  letter-spacing: -0.02em;
}

.view-desc {
  font-size: 13px;
  color: var(--text-secondary);
  max-width: 820px;
  margin-top: 4px;
}

.header-actions {
  display: flex;
  gap: 12px;
}

/* Status Banner */
.status-banner {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 18px;
  border-radius: 12px;
  font-size: 13px;
}

.banner-success {
  background: rgba(16, 185, 129, 0.12);
  border: 1px solid rgba(16, 185, 129, 0.3);
  color: #34d399;
}

.banner-error {
  background: rgba(244, 63, 94, 0.12);
  border: 1px solid rgba(244, 63, 94, 0.3);
  color: #fda4af;
}

.banner-icon {
  font-size: 16px;
}

.banner-text {
  flex: 1;
  font-weight: 500;
}

.banner-close {
  background: transparent;
  border: none;
  color: currentColor;
  cursor: pointer;
  opacity: 0.7;
  font-size: 14px;
}

.banner-close:hover {
  opacity: 1;
}

/* Metrics HUD */
.metrics-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(230px, 1fr));
  gap: 16px;
}

/* Tabs Bar */
.tabs-bar {
  padding: 6px 10px;
  display: flex;
  gap: 8px;
  overflow-x: auto;
  border-radius: 14px;
}

.tab-btn {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 8px 16px;
  border-radius: 10px;
  font-size: 13px;
  font-weight: 600;
  color: var(--text-secondary);
  background: transparent;
  border: 1px solid transparent;
  cursor: pointer;
  transition: all 0.15s ease;
  white-space: nowrap;
}

.tab-btn:hover {
  color: var(--text-primary);
  background: rgba(255, 255, 255, 0.04);
}

.tab-btn-active {
  color: #fff;
  background: rgba(6, 182, 212, 0.15);
  border-color: rgba(6, 182, 212, 0.35);
}

.tab-badge {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 1px 7px;
  border-radius: 9999px;
  font-size: 11px;
  font-weight: 700;
  background: rgba(6, 182, 212, 0.25);
  color: #38bdf8;
}

/* Loading State */
.loading-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 48px;
  gap: 16px;
  color: var(--text-secondary);
  font-size: 14px;
}

.spinner {
  width: 32px;
  height: 32px;
  border: 3px solid rgba(255, 255, 255, 0.1);
  border-top-color: var(--accent-cyan);
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}

/* Card & Forms */
.settings-card {
  padding: 24px;
  display: flex;
  flex-direction: column;
  gap: 22px;
}

.card-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 16px;
  border-bottom: 1px solid var(--border-subtle);
  padding-bottom: 16px;
}

.card-title {
  font-size: 18px;
  font-weight: 700;
  color: #fff;
}

.card-subtitle {
  font-size: 13px;
  color: var(--text-secondary);
  margin-top: 4px;
}

.btn-sm {
  padding: 6px 12px;
  font-size: 12px;
}

.settings-form {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.form-row {
  display: flex;
  gap: 20px;
  flex-wrap: wrap;
}

.flex-1 {
  flex: 1;
  min-width: 260px;
}

.flex-2 {
  flex: 2;
  min-width: 280px;
}

.form-group {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.form-label {
  font-size: 13px;
  font-weight: 600;
  color: var(--text-primary);
  display: flex;
  align-items: center;
  gap: 4px;
}

.label-disabled {
  opacity: 0.5;
}

.required {
  color: var(--accent-rose);
}

.field-desc {
  font-size: 12px;
  color: var(--text-muted);
  line-height: 1.4;
}

.form-input,
.form-select {
  width: 100%;
  padding: 10px 14px;
  font-size: 13px;
}

.form-input:disabled,
.form-select:disabled {
  opacity: 0.4;
  cursor: not-allowed;
  background: rgba(0, 0, 0, 0.2);
}

/* Custom Toggle Switch */
.toggle-group {
  display: flex;
  flex-direction: row;
  align-items: center;
  justify-content: space-between;
  gap: 20px;
  padding: 16px;
  background: rgba(255, 255, 255, 0.02);
  border: 1px solid var(--border-subtle);
  border-radius: 12px;
}

.toggle-info {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.toggle-label {
  font-size: 13px;
  font-weight: 600;
  color: var(--text-primary);
}

.toggle-switch {
  position: relative;
  display: inline-block;
  width: 48px;
  height: 26px;
  flex-shrink: 0;
}

.toggle-switch input {
  opacity: 0;
  width: 0;
  height: 0;
}

.toggle-slider {
  position: absolute;
  cursor: pointer;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background-color: rgba(255, 255, 255, 0.15);
  transition: 0.25s;
  border-radius: 9999px;
  border: 1px solid var(--border-medium);
}

.toggle-slider:before {
  position: absolute;
  content: "";
  height: 18px;
  width: 18px;
  left: 3px;
  bottom: 3px;
  background-color: #fff;
  transition: 0.25s;
  border-radius: 50%;
  box-shadow: 0 2px 4px rgba(0, 0, 0, 0.4);
}

.toggle-switch input:checked + .toggle-slider {
  background: var(--grad-cyan);
  border-color: var(--accent-cyan);
}

.toggle-switch input:checked + .toggle-slider:before {
  transform: translateX(22px);
}

/* Info Box */
.security-info-box {
  padding: 14px 18px;
  border-radius: 12px;
  background: rgba(6, 182, 212, 0.06);
  border: 1px solid rgba(6, 182, 212, 0.2);
  font-size: 12px;
  color: var(--text-secondary);
  line-height: 1.5;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.info-box-header {
  font-weight: 700;
  color: var(--accent-cyan);
}

/* Integrations Grid */
.integrations-grid {
  display: grid;
  grid-template-columns: 1fr;
  gap: 16px;
}

.integration-item {
  padding: 18px 20px;
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.integration-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  flex-wrap: wrap;
  gap: 12px;
}

.integration-title-group {
  display: flex;
  align-items: center;
  gap: 12px;
}

.integration-icon {
  font-size: 24px;
  width: 44px;
  height: 44px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(255, 255, 255, 0.04);
  border: 1px solid var(--border-subtle);
  border-radius: 10px;
}

.integration-name {
  font-size: 14px;
  font-weight: 700;
  color: #fff;
}

.integration-desc {
  font-size: 12px;
  color: var(--text-muted);
}

.integration-input-row {
  display: flex;
  gap: 12px;
  align-items: center;
}

/* Form Actions */
.form-actions {
  display: flex;
  justify-content: flex-end;
  border-top: 1px solid var(--border-subtle);
  padding-top: 18px;
  margin-top: 6px;
}

/* About Tab */
.about-hero {
  display: flex;
  gap: 20px;
  align-items: center;
  padding: 16px;
  background: rgba(255, 255, 255, 0.02);
  border-radius: 14px;
  border: 1px solid var(--border-subtle);
}

.about-logo {
  font-size: 40px;
  color: var(--accent-cyan);
  width: 72px;
  height: 72px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(6, 182, 212, 0.1);
  border: 1px solid rgba(6, 182, 212, 0.25);
  border-radius: 16px;
  flex-shrink: 0;
}

.about-hero-text {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.about-version-tag {
  display: flex;
  gap: 8px;
}

.about-title {
  font-size: 20px;
  font-weight: 800;
  color: #fff;
  letter-spacing: -0.01em;
}

.about-desc {
  font-size: 13px;
  color: var(--text-secondary);
  line-height: 1.5;
}

.spec-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
  gap: 14px;
}

.spec-card {
  padding: 14px 18px;
  display: flex;
  align-items: center;
  gap: 14px;
}

.spec-icon {
  font-size: 20px;
}

.spec-meta {
  display: flex;
  flex-direction: column;
}

.spec-label {
  font-size: 11px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  color: var(--text-muted);
}

.spec-value {
  font-size: 13px;
  font-weight: 600;
  color: #fff;
}

.about-footer {
  display: flex;
  justify-content: space-between;
  align-items: center;
  flex-wrap: wrap;
  gap: 12px;
  padding-top: 16px;
  border-top: 1px solid var(--border-subtle);
  font-size: 12px;
  color: var(--text-muted);
}

.about-links {
  display: flex;
  align-items: center;
  gap: 8px;
}

.about-link {
  color: var(--accent-cyan);
  cursor: pointer;
  text-decoration: none;
}

.about-link:hover {
  text-decoration: underline;
}

.dot-sep {
  opacity: 0.4;
}

@media (max-width: 768px) {
  .form-row {
    flex-direction: column;
  }
  .integration-input-row {
    flex-direction: column;
    align-items: stretch;
  }
  .about-hero {
    flex-direction: column;
    text-align: center;
  }
  .about-version-tag {
    justify-content: center;
  }
  .about-footer {
    flex-direction: column;
    text-align: center;
  }
}
</style>

