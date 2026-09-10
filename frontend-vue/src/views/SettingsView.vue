<script setup lang="ts">
import { computed } from 'vue'
import '../assets/styles/views/settings.css'
import '../assets/styles/components/settings-tabs.css'
import MetricCard from '../components/ui/MetricCard.vue'
import SettingsGeneralTab from '../components/settings/SettingsGeneralTab.vue'
import SettingsSecurityTab from '../components/settings/SettingsSecurityTab.vue'
import SettingsTenancyTab from '../components/settings/SettingsTenancyTab.vue'
import SettingsNotificationsTab from '../components/settings/SettingsNotificationsTab.vue'
import SettingsApiKeysTab from '../components/settings/SettingsApiKeysTab.vue'
import AboutSettingsTab from '../components/settings/AboutSettingsTab.vue'
import Disable2FAModal from '../components/settings/Disable2FAModal.vue'
import SettingsMobileNav from '../components/settings/SettingsMobileNav.vue'
import {
  useSettings,
  timezoneOptions,
  languageOptions,
  type TabKey,
  type CategoryKey,
} from '../composables/useSettings'

const {
  activeTab,
  form,
  loading,
  saving,
  statusMessage,
  saveCategory,
  saveAllSettings,
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
  isDirtyCategory,
} = useSettings()

const tabsList: { id: TabKey; label: string; icon: string; category?: CategoryKey; count?: number }[] = [
  { id: 'general', label: 'General', icon: '⚙️', category: 'platform' },
  { id: 'security', label: 'Security', icon: '🛡️', category: 'security' },
  { id: 'tenancy', label: 'Tenancy', icon: '🏢', category: 'tenancy' },
  { id: 'notifications', label: 'Notifications', icon: '🔔', category: 'notifications' },
  { id: 'apikeys', label: 'API Keys', icon: '🔑', category: 'apikeys', count: 3 },
  { id: 'about', label: 'About', icon: 'ℹ️' },
]

const activeTabLabel = computed(() => {
  const found = tabsList.find(t => t.id === activeTab.value)
  return found ? found.label : 'General'
})

function handleMobileSave() {
  if (activeTab.value === 'general') {
    saveCategory('platform')
  } else if (activeTab.value === 'security') {
    saveCategory('security')
  } else if (activeTab.value === 'tenancy') {
    saveCategory('tenancy')
  } else if (activeTab.value === 'notifications') {
    saveCategory('notifications')
  } else {
    saveAllSettings()
  }
}

function handleMobileReset() {
  if (activeTab.value === 'general') {
    resetCategoryToDefaults('platform')
  } else if (activeTab.value === 'security') {
    resetCategoryToDefaults('security')
  } else if (activeTab.value === 'tenancy') {
    resetCategoryToDefaults('tenancy')
  } else if (activeTab.value === 'notifications') {
    resetCategoryToDefaults('notifications')
  } else {
    loadSettings()
  }
}
</script>

<template>
  <div class="view-container animate-fade-in">
    <!-- View Header (Desktop/Tablet >=768px) -->
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

    <!-- Mobile 40-44px Command Bar (<768px) with 32x32px Action Buttons -->
    <div class="settings-mobile-command-bar mobile-only">
      <div class="command-bar-left">
        <span class="command-bar-title font-bold">⚙️ Settings</span>
      </div>
      <div class="command-bar-actions">
        <button
          type="button"
          class="btn-icon-cmd"
          title="Save Active Tab Settings"
          aria-label="Save Settings"
          :disabled="saving"
          @click="handleMobileSave"
        >
          <span v-if="!saving">💾</span>
          <span v-else class="spinner spinner-sm"></span>
        </button>
        <button
          type="button"
          class="btn-icon-cmd"
          title="Reset Active Tab to Defaults"
          aria-label="Reset Defaults"
          :disabled="loading || saving"
          @click="handleMobileReset"
        >
          <span>🔄</span>
        </button>
      </div>
    </div>

    <!-- Mobile 20px Centered Micro-Telemetry Strip (<768px): ⚙️ Active Tab · 🛡️ 2FA · 🔑 API Keys · 🏢 Org -->
    <div class="settings-micro-telemetry mobile-only font-mono" role="status" aria-label="Settings Micro Telemetry">
      <span class="tel-item tel-name">⚙️ {{ activeTabLabel }}</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-sec">🛡️ {{ totpStatus?.enabled ? '2FA' : 'No 2FA' }}</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-alert">🔑 3 Keys</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-tenancy">🏢 {{ form.name ? 'Active' : 'Default' }}</span>
    </div>

    <!-- Mobile Tabs Navigation (Pills with snap, <768px) -->
    <SettingsMobileNav
      v-model:active-tab="activeTab"
      :tabs="tabsList"
      :is-dirty-category="isDirtyCategory"
    />

    <!-- Notification Banner -->
    <div
      v-if="statusMessage"
      class="status-banner animate-fade-in"
      :class="'banner-' + statusMessage.type"
    >
      <span class="banner-icon">{{ statusMessage.type === 'success' ? '✅' : '⚠️' }}</span>
      <span class="banner-text">{{ statusMessage.text }}</span>
      <button class="banner-close" aria-label="Close Banner" @click="statusMessage = null">✕</button>
    </div>

    <!-- Key Metrics Summary HUD (Desktop/Tablet >=768px) -->
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
        badge-color="emerald"
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

    <!-- Desktop Tabs Navigation Bar (>=768px) -->
    <div class="tabs-bar glass-panel desktop-only">
      <button
        v-for="tab in tabsList"
        :key="tab.id"
        class="tab-btn"
        :class="{ 'tab-btn-active': activeTab === tab.id }"
        @click="activeTab = tab.id"
      >
        <span>{{ tab.icon }} {{ tab.label }}</span>
        <span v-if="tab.category && isDirtyCategory(tab.category)" class="dirty-dot"></span>
        <span v-if="tab.count !== undefined" class="tab-badge">{{ tab.count }}</span>
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
