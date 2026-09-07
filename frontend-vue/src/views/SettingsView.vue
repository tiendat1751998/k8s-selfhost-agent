<script setup lang="ts">
import '../assets/styles/views/settings.css'
import MetricCard from '../components/ui/MetricCard.vue'
import SettingsGeneralTab from '../components/settings/SettingsGeneralTab.vue'
import SettingsSecurityTab from '../components/settings/SettingsSecurityTab.vue'
import SettingsTenancyTab from '../components/settings/SettingsTenancyTab.vue'
import SettingsNotificationsTab from '../components/settings/SettingsNotificationsTab.vue'
import SettingsApiKeysTab from '../components/settings/SettingsApiKeysTab.vue'
import AboutSettingsTab from '../components/settings/AboutSettingsTab.vue'
import Disable2FAModal from '../components/settings/Disable2FAModal.vue'
import {
  useSettings,
  timezoneOptions,
  languageOptions,
} from '../composables/useSettings'

const {
  activeTab,
  form,
  loading,
  saving,
  statusMessage,
  saveCategory,
  resetCategoryToDefaults,
  loadSettings,
  totpStatus,
  loadingTotpStatus,
  showDisable2FAModal,
  disablePassword,
  disableTotpCode,
  disabling2FA,
  disableError,
  handleDisable2FA,
} = useSettings()
</script>

<template>
  <div class="view-container animate-fade-in">
    <!-- View Header -->
    <div class="view-header desktop-header desktop-only">
      <div>
        <div class="view-tag">
          <span class="pulse-dot pulse-dot-cyan"></span>
          <span>GLOBAL PLATFORM CONFIGURATION</span>
        </div>
        <h1 class="view-title">Settings & System Preferences</h1>
        <p class="view-desc">
          Manage tenant-level policies, authentication parameters, outbound alert routing, and security policies.
        </p>
      </div>

      <div class="header-actions">
        <button class="btn btn-secondary" :disabled="loading" @click="loadSettings">
          <span>{{ loading ? '⏳ Syncing...' : '🔄 Refresh Settings' }}</span>
        </button>
      </div>
    </div>

    <!-- Mobile 40px Command Bar (<640px) -->
    <div class="settings-mobile-command-bar mobile-only">
      <div class="command-bar-left">
        <span class="command-bar-title font-bold">⚙️ Settings</span>
      </div>
      <div class="command-bar-actions">
        <button
          type="button"
          class="btn-icon-cmd"
          title="Refresh Settings"
          aria-label="Refresh Settings"
          :disabled="loading"
          @click="loadSettings"
        >
          <svg class="cmd-icon" :class="{ 'spin-animate': loading }" viewBox="0 0 24 24" fill="none" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" /></svg>
        </button>
      </div>
    </div>

    <!-- Mobile 20px Centered Micro-Telemetry Strip (<640px) -->
    <div class="settings-micro-telemetry mobile-only font-mono" role="status" aria-label="Settings Micro Telemetry">
      <span class="tel-item tel-name">⚙️ 6 tabs</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-sec">🔐 {{ totpStatus?.enabled ? 'TOTP Active' : 'TOTP Off' }}</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-alert">👤 admin</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-tenancy">🏢 default</span>
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
    <div class="metrics-grid desktop-metrics desktop-only">
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
        title="Multi-Tenancy"
        value="Isolation Active"
        badge="SECURE"
        badge-color="emerald"
        subtitle="Network Policies & RBAC Enforced"
        icon="🏢"
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
        :class="{ 'tab-btn-active': activeTab === 'tenancy' }"
        @click="activeTab = 'tenancy'"
      >
        <span>🏢 Tenancy</span>
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
        :class="{ 'tab-btn-active': activeTab === 'apikeys' }"
        @click="activeTab = 'apikeys'"
      >
        <span>🔑 API Keys</span>
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
      <SettingsGeneralTab
        v-if="activeTab === 'general'"
        :form="form"
        :timezone-options="timezoneOptions"
        :language-options="languageOptions"
        :saving="saving"
        @save="saveCategory"
        @reset="resetCategoryToDefaults"
      />

      <SettingsSecurityTab
        v-if="activeTab === 'security'"
        :form="form"
        :saving="saving"
        :totp-status="totpStatus"
        :loading-totp-status="loadingTotpStatus"
        @save="saveCategory"
        @reset="resetCategoryToDefaults"
        @open-disable2fa="showDisable2FAModal = true"
        @open-disable2-f-a="showDisable2FAModal = true"
      />

      <SettingsTenancyTab
        v-if="activeTab === 'tenancy'"
        :saving="saving"
        @save="saveCategory"
        @reset="resetCategoryToDefaults"
      />

      <SettingsNotificationsTab
        v-if="activeTab === 'notifications'"
        :form="form"
        :saving="saving"
        @save="saveCategory"
        @reset="resetCategoryToDefaults"
      />

      <SettingsApiKeysTab
        v-if="activeTab === 'apikeys'"
        :saving="saving"
        @save="saveCategory"
      />

      <AboutSettingsTab
        v-if="activeTab === 'about'"
      />
    </div>

    <!-- Disable 2FA Modal -->
    <Disable2FAModal
      v-model:show="showDisable2FAModal"
      v-model:password="disablePassword"
      v-model:totp-code="disableTotpCode"
      :disabling="disabling2FA"
      :error="disableError"
      @confirm="handleDisable2FA"
    />
  </div>
</template>
