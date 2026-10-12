<script setup lang="ts">
import BaseIcon from '../ui/BaseIcon.vue'

defineProps<{
  rulesCount: number
  activeCount: number
  healingSuccessRate: number
  savedHours: number | string
  executionsCount: number
  loading: boolean
  mobileTab: 'rules' | 'history'
  searchQuery: string
  selectedTrigger: 'all' | 'crashloop' | 'nodepressure' | 'deploymentfailed'
}>()

const emit = defineEmits<{
  (e: 'update:mobileTab', val: 'rules' | 'history'): void
  (e: 'update:searchQuery', val: string): void
  (e: 'update:selectedTrigger', val: 'all' | 'crashloop' | 'nodepressure' | 'deploymentfailed'): void
  (e: 'refresh'): void
  (e: 'create'): void
}>()

const triggerFilterPills = [
  { key: 'all', label: 'All', icon: 'zap' },
  { key: 'crashloop', label: 'CrashLoop', icon: 'refresh' },
  { key: 'nodepressure', label: 'NodePressure', icon: 'shield' },
  { key: 'deploymentfailed', label: 'DeploymentFailed', icon: 'flame' },
] as const
</script>

<template>
  <div class="automation-header-bar-wrapper">
    <!-- 44px Mobile Command Bar (<768px) -->
    <div class="mobile-command-bar automation-mobile-command-bar mobile-only">
      <div class="command-bar-left">
        <span class="command-bar-title font-bold"><BaseIcon name="zap" size="xs" /> Automation ({{ rulesCount }})</span>
      </div>
      <div class="command-bar-actions">
        <button
          class="btn-icon-cmd"
          :disabled="loading"
          title="Refresh Automation Workflows"
          aria-label="Refresh"
          @click="emit('refresh')"
        >
          <BaseIcon :name="loading ? 'clock' : 'refresh'" size="xs" />
        </button>
        <button
          class="btn-icon-cmd"
          title="Create Automation Rule"
          aria-label="Create Rule"
          @click="emit('create')"
        >
          <BaseIcon name="plus" size="xs" />
        </button>
      </div>
    </div>

    <!-- 20px Mobile Micro-Telemetry Strip (<768px) -->
    <div class="mobile-micro-telemetry automation-micro-telemetry mobile-only font-mono" role="status" aria-label="Automation Micro Telemetry">
      <span class="tel-item tel-rules"><BaseIcon name="zap" size="xs" /> {{ rulesCount }} rules</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-active"><BaseIcon name="check-circle" size="xs" /> {{ activeCount }} act</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-healed"><BaseIcon name="shield" size="xs" /> {{ executionsCount > 0 ? `${healingSuccessRate}%` : '--' }}</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-saved"><BaseIcon name="clock" size="xs" /> {{ savedHours }}h saved</span>
    </div>

    <!-- Mobile Segmented Tab Switcher (<768px) -->
    <div class="segmented-control mobile-segmented-control mobile-only font-mono">
      <button
        class="segmented-btn"
        :class="{ active: mobileTab === 'rules' }"
        @click="emit('update:mobileTab', 'rules')"
      >
        <BaseIcon name="zap" size="xs" /> Rules ({{ rulesCount }})
      </button>
      <button
        class="segmented-btn"
        :class="{ active: mobileTab === 'history' }"
        @click="emit('update:mobileTab', 'history')"
      >
        <BaseIcon name="history" size="xs" /> History ({{ executionsCount }})
      </button>
    </div>

    <!-- Sleek Unified 38px Enterprise Toolbar -->
    <div class="automation-toolbar-sleek glass-panel desktop-only">
      <!-- Search input with search icon and clear button -->
      <div class="toolbar-search-wrap">
        <BaseIcon name="search" size="xs" class="search-icon" />
        <input
          :value="searchQuery"
          type="text"
          placeholder="Search rule, trigger, action..."
          class="toolbar-search-input"
          aria-label="Search rules by name, condition, action, or trigger"
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

      <!-- Trigger filter pills / tabs -->
      <div class="toolbar-trigger-pills" role="tablist" aria-label="Trigger filters">
        <button
          v-for="pill in triggerFilterPills"
          :key="pill.key"
          type="button"
          role="tab"
          :aria-selected="selectedTrigger === pill.key"
          class="toolbar-pill-btn"
          :class="{ active: selectedTrigger === pill.key }"
          @click="emit('update:selectedTrigger', pill.key)"
        >
          <BaseIcon v-if="pill.icon" :name="pill.icon" size="xs" />
          <span>{{ pill.label }}</span>
        </button>
      </div>

      <!-- Inline compact execution badge strip font-mono -->
      <div class="toolbar-kpi-strip font-mono desktop-only" role="status" aria-label="Automation execution metrics">
        <span class="kpi-badge font-mono">{{ rulesCount }} Rules ({{ activeCount }} Active · {{ executionsCount > 0 ? `${healingSuccessRate}% Healed` : '0 Healed' }} · {{ savedHours }}h Saved)</span>
      </div>

      <!-- Action buttons -->
      <div class="toolbar-actions-group">
        <button
          type="button"
          class="btn btn-primary toolbar-btn"
          title="Create Automation Rule"
          aria-label="Create Automation Rule"
          @click="emit('create')"
        >
          <BaseIcon name="plus" size="xs" />
          <span>+ Create Automation Rule</span>
        </button>
        <button
          type="button"
          class="btn btn-secondary toolbar-btn"
          :disabled="loading"
          title="Refresh Workflows"
          aria-label="Refresh Workflows"
          @click="emit('refresh')"
        >
          <BaseIcon name="refresh" size="xs" :class="{ 'spin-anim': loading }" />
          <span>{{ loading ? 'Syncing...' : 'Refresh' }}</span>
        </button>
      </div>
    </div>
  </div>
</template>
