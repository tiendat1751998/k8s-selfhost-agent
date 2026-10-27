<script setup lang="ts">
import BaseIcon from '../ui/BaseIcon.vue'

defineProps<{
  activeTab: 'firing' | 'rules' | 'channels' | 'history'
  firingCount: number
  rulesCount: number
  channelsCount: number
  historyCount: number
  criticalP1Count: number
  warningCount: number
  silencedRulesCount: number
  searchQuery: string
  loading: boolean
}>()

const emit = defineEmits<{
  (e: 'update:activeTab', tab: 'firing' | 'rules' | 'channels' | 'history'): void
  (e: 'update:searchQuery', query: string): void
  (e: 'openCreateRule'): void
  (e: 'openCreateChannel'): void
  (e: 'refresh'): void
}>()
</script>

<template>
  <div class="alerts-header-bar-wrapper">
    <!-- Mobile 40-44px Command Bar (<768px) -->
    <div class="alerts-mobile-command-bar mobile-only">
      <div class="command-bar-left">
        <span class="command-bar-title font-bold"><BaseIcon name="bell" size="sm" /> Alerts ({{ firingCount }})</span>
      </div>
      <div class="command-bar-actions">
        <button
          class="btn-icon-cmd"
          title="New Alert Rule"
          aria-label="New Alert Rule"
          @click="emit('openCreateRule')"
        >
          <BaseIcon name="plus" size="xs" />
        </button>
        <button
          class="btn-icon-cmd"
          :disabled="loading"
          title="Sync / Refresh Telemetry"
          aria-label="Sync / Refresh Telemetry"
          @click="emit('refresh')"
        >
          <BaseIcon name="refresh" size="xs" :class="{ 'animate-spin': loading }" />
        </button>
      </div>
    </div>

    <!-- Mobile 20px Centered Micro-Telemetry Strip (<768px) -->
    <div class="alerts-micro-telemetry mobile-only font-mono" role="status" aria-label="Alerts Micro Telemetry">
      <span class="tel-item tel-crit"><BaseIcon name="alert-triangle" size="xs" /> {{ criticalP1Count }} Critical</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-warn"><BaseIcon name="alert-triangle" size="xs" /> {{ warningCount }} Warning</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-silent"><BaseIcon name="bell-off" size="xs" /> {{ silencedRulesCount }} Silenced</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-chan"><BaseIcon name="radio" size="xs" /> {{ channelsCount }} Channels</span>
    </div>

    <!-- Mobile Tab Navigation Strip (<768px) -->
    <div class="alerts-mobile-tabs mobile-only" role="tablist" aria-label="Alerts Navigation">
      <button
        type="button"
        role="tab"
        :aria-selected="activeTab === 'firing'"
        class="mobile-tab-btn"
        :class="{ active: activeTab === 'firing' }"
        @click="emit('update:activeTab', 'firing')"
      >
        <BaseIcon name="flame" size="xs" />
        <span>Firing ({{ firingCount }})</span>
      </button>
      <button
        type="button"
        role="tab"
        :aria-selected="activeTab === 'rules'"
        class="mobile-tab-btn"
        :class="{ active: activeTab === 'rules' }"
        @click="emit('update:activeTab', 'rules')"
      >
        <BaseIcon name="sliders" size="xs" />
        <span>Rules ({{ rulesCount }})</span>
      </button>
      <button
        type="button"
        role="tab"
        :aria-selected="activeTab === 'channels'"
        class="mobile-tab-btn"
        :class="{ active: activeTab === 'channels' }"
        @click="emit('update:activeTab', 'channels')"
      >
        <BaseIcon name="radio" size="xs" />
        <span>Channels ({{ channelsCount }})</span>
      </button>
      <button
        type="button"
        role="tab"
        :aria-selected="activeTab === 'history'"
        class="mobile-tab-btn"
        :class="{ active: activeTab === 'history' }"
        @click="emit('update:activeTab', 'history')"
      >
        <BaseIcon name="clock" size="xs" />
        <span>History ({{ historyCount }})</span>
      </button>
    </div>

    <!-- Sleek Unified 42px Enterprise Toolbar (.alerts-toolbar-sleek) -->
    <div class="alerts-toolbar-sleek glass-panel desktop-only" role="toolbar" aria-label="Alerts Management Toolbar">
      <!-- Search input with search icon and clear button -->
      <div class="toolbar-search-wrap">
        <BaseIcon name="search" size="xs" class="search-icon" />
        <input
          :value="searchQuery"
          type="text"
          placeholder="Filter message, rule, channel..."
          class="toolbar-search-input"
          aria-label="Filter alert message, rule ID, channel name"
          @input="emit('update:searchQuery', ($event.target as HTMLInputElement).value)"
        />
        <button
          v-if="searchQuery"
          type="button"
          class="clear-input-btn"
          aria-label="Clear search"
          @click="emit('update:searchQuery', '')"
        >
          <BaseIcon name="x" size="xs" />
        </button>
      </div>

      <!-- Navigation tabs / pills -->
      <div class="toolbar-nav-pills" role="tablist" aria-label="Alerts Navigation Tabs">
        <button
          type="button"
          role="tab"
          :aria-selected="activeTab === 'firing'"
          class="toolbar-pill-btn"
          :class="{ active: activeTab === 'firing' }"
          @click="emit('update:activeTab', 'firing')"
        >
          <BaseIcon name="flame" size="xs" />
          <span>Active Firing</span>
          <span class="pill-badge">{{ firingCount }}</span>
        </button>
        <button
          type="button"
          role="tab"
          :aria-selected="activeTab === 'rules'"
          class="toolbar-pill-btn"
          :class="{ active: activeTab === 'rules' }"
          @click="emit('update:activeTab', 'rules')"
        >
          <BaseIcon name="sliders" size="xs" />
          <span>Alert Rules</span>
          <span class="pill-badge">{{ rulesCount }}</span>
        </button>
        <button
          type="button"
          role="tab"
          :aria-selected="activeTab === 'channels'"
          class="toolbar-pill-btn"
          :class="{ active: activeTab === 'channels' }"
          @click="emit('update:activeTab', 'channels')"
        >
          <BaseIcon name="radio" size="xs" />
          <span>Notification Channels</span>
          <span class="pill-badge">{{ channelsCount }}</span>
        </button>
        <button
          type="button"
          role="tab"
          :aria-selected="activeTab === 'history'"
          class="toolbar-pill-btn"
          :class="{ active: activeTab === 'history' }"
          @click="emit('update:activeTab', 'history')"
        >
          <BaseIcon name="clock" size="xs" />
          <span>Alert History</span>
          <span class="pill-badge">{{ historyCount }}</span>
        </button>
      </div>

      <!-- Clean pipeline summary strip font-mono -->
      <div class="toolbar-kpi-strip font-mono" role="status" aria-label="Alerts Telemetry KPI summary">
        <span class="kpi-pipeline">
          <span class="pipeline-badge pipeline-firing" :class="{ 'is-active': firingCount > 0 }">
            <span class="pipeline-indicator" />
            {{ firingCount }} Firing
          </span>
          <span class="pipeline-sep">·</span>
          <span class="pipeline-badge pipeline-armed">{{ rulesCount }} Armed</span>
          <span class="pipeline-sep">·</span>
          <span class="pipeline-badge pipeline-silenced">{{ silencedRulesCount }} Silenced</span>
        </span>
      </div>

      <!-- Context-Aware Action Buttons -->
      <div class="toolbar-actions-group">
        <button
          v-if="activeTab === 'rules'"
          type="button"
          class="btn btn-primary toolbar-btn"
          title="Create New Alert Rule"
          aria-label="New Alert Rule"
          @click="emit('openCreateRule')"
        >
          <BaseIcon name="plus" size="xs" />
          <span>New Rule</span>
        </button>
        <button
          v-else-if="activeTab === 'channels'"
          type="button"
          class="btn btn-primary toolbar-btn"
          title="Add Notification Channel"
          aria-label="Add Channel"
          @click="emit('openCreateChannel')"
        >
          <BaseIcon name="plus" size="xs" />
          <span>Add Channel</span>
        </button>
        <button
          type="button"
          class="btn btn-secondary toolbar-btn"
          :disabled="loading"
          title="Refresh Alerts & Telemetry"
          aria-label="Refresh"
          @click="emit('refresh')"
        >
          <BaseIcon name="refresh" size="xs" :class="{ 'spin-icon': loading }" />
          <span>{{ loading ? 'Syncing...' : 'Refresh' }}</span>
        </button>
      </div>
    </div>
  </div>
</template>
