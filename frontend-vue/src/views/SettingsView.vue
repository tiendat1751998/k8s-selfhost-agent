<script setup lang="ts">
import { computed } from 'vue'
import '../assets/styles/views/settings.css'
import '../assets/styles/components/settings-tabs.css'
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
  { id: 'general', label: 'General', icon: 'sliders', category: 'platform' },
  { id: 'security', label: 'Security', icon: 'shield', category: 'security' },
  { id: 'tenancy', label: 'Tenancy', icon: 'server', category: 'tenancy' },
  { id: 'notifications', label: 'Notifications', icon: 'bell', category: 'notifications' },
  { id: 'apikeys', label: 'API Keys', icon: 'key', category: 'apikeys', count: 3 },
  { id: 'about', label: 'About', icon: 'help-circle' },
]

const activeTabLabel = computed(() => {
  const found = tabsList.find(t => t.id === activeTab.value)
  return found ? found.label : 'General'
})

function handleSaveSettings() {
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

function handleResetSettings() {
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
  <div class="view-container settings-view-container animate-fade-in">
    <!-- Elevated Desktop Settings Tabs Header & Compact 38px Toolbar (>=768px, ~65px from Top HUD) -->
    <header class="settings-tabs-header tabs-bar desktop-only" role="region" aria-label="Settings Toolbar">
      <nav class="settings-tabs-nav" aria-label="Settings categories">
        <button
          v-for="tab in tabsList"
          :key="tab.id"
          type="button"
          class="settings-tab-btn tab-btn"
          :class="{ 'active': activeTab === tab.id, 'tab-btn-active': activeTab === tab.id }"
          @click="activeTab = tab.id"
        >
          <BaseIcon :name="tab.icon" size="xs" />
          <span>{{ tab.label }}</span>
          <span v-if="tab.category && isDirtyCategory(tab.category)" class="dirty-dot" title="Unsaved changes"></span>
          <span v-if="tab.count !== undefined" class="tab-badge">{{ tab.count }}</span>
        </button>
      </nav>

      <div class="settings-toolbar-actions">
        <button
          type="button"
          class="btn-settings-sync"
          :disabled="loading || saving"
          @click="loadSettings"
          title="Reload configuration from cluster"
        >
          <BaseIcon name="refresh" size="xs" :class="{ 'animate-spin': loading }" />
          <span>{{ loading ? 'Syncing...' : 'Sync / Refresh' }}</span>
        </button>
        <button
          type="button"
          class="btn-settings-save"
          :disabled="saving || loading"
          @click="handleSaveSettings"
          title="Save active configuration"
        >
          <span v-if="saving" class="spinner spinner-sm" aria-hidden="true"></span>
          <BaseIcon v-else name="save" size="xs" />
          <span>{{ saving ? 'Saving...' : 'Save Settings' }}</span>
        </button>
      </div>
    </header>

    <!-- Mobile 40-44px Command Bar (<768px) with 32x32px Action Buttons -->
    <div class="settings-mobile-command-bar mobile-only">
      <div class="command-bar-left">
        <span class="command-bar-title font-bold"><BaseIcon name="sliders" size="sm" /> Settings</span>
      </div>
      <div class="command-bar-actions">
        <button
          type="button"
          class="btn-icon-cmd"
          title="Save Active Tab Settings"
          aria-label="Save Settings"
          :disabled="saving"
          @click="handleSaveSettings"
        >
          <span v-if="saving" class="spinner spinner-sm"></span>
          <BaseIcon v-else name="save" size="xs" />
        </button>
        <button
          type="button"
          class="btn-icon-cmd"
          title="Reset Active Tab to Defaults"
          aria-label="Reset Defaults"
          :disabled="loading || saving"
          @click="handleResetSettings"
        >
          <BaseIcon name="refresh" size="xs" :class="{ 'animate-spin': loading }" />
        </button>
      </div>
    </div>

    <!-- Mobile 20px Centered Micro-Telemetry Strip (<768px): Active Tab · 2FA · API Keys · Org -->
    <div class="settings-micro-telemetry mobile-only font-mono" role="status" aria-label="Settings Micro Telemetry">
      <span class="tel-item tel-name"><BaseIcon name="sliders" size="xs" /> {{ activeTabLabel }}</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-sec"><BaseIcon name="shield" size="xs" /> {{ totpStatus?.enabled ? '2FA' : 'No 2FA' }}</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-alert"><BaseIcon name="key" size="xs" /> 3 Keys</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-tenancy"><BaseIcon name="server" size="xs" /> {{ form.name ? 'Active' : 'Default' }}</span>
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
      <BaseIcon :name="statusMessage.type === 'success' ? 'check-circle' : 'alert-triangle'" size="sm" class="banner-icon" />
      <span class="banner-text">{{ statusMessage.text }}</span>
      <button class="banner-close" aria-label="Close Banner" @click="statusMessage = null"><BaseIcon name="x" size="xs" /></button>
    </div>

    <!-- Loading State -->
    <div v-if="loading" class="loading-state glass-panel">
      <div class="spinner"></div>
      <span>Loading platform settings from cluster API...</span>
    </div>

    <!-- TAB CONTENTS -->
    <div v-else class="tab-content">
      <div class="settings-content-wrapper">
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
