<script setup lang="ts">
import BaseIcon from '../ui/BaseIcon.vue'
import ClickHouseEngineBadge from './ClickHouseEngineBadge.vue'
import type { LogTarget } from './LogTargetTree.vue'
import type { TimeRangeConfig } from '../../composables/useHistoricalLogSearch'

interface Props {
  selectedTarget: LogTarget
  isSidebarCollapsed: boolean
  searchKeyword: string
  mode: 'live' | 'historical'
  selectedLevel: string
  selectedTimeRange: string
  selectedHistoricalLimit: number
  timeRanges: TimeRangeConfig[]
  isHistoricalLoading: boolean
  hasMoreHistorical: boolean
  totalHistoricalCount: number
  currentLogsCount: number
  autoScroll: boolean
  isScrollLocked: boolean
  wrapLines: boolean
  showHistogram: boolean
  showMobileTree: boolean
  showErrorRadar?: boolean
  activeService?: string
  hoverPauseEnabled?: boolean
  isHoverPaused?: boolean
}

const props = defineProps<Props>()

const emit = defineEmits<{
  (e: 'update:isSidebarCollapsed', val: boolean): void
  (e: 'update:searchKeyword', val: string): void
  (e: 'update:mode', val: 'live' | 'historical'): void
  (e: 'update:selectedLevel', val: string): void
  (e: 'update:selectedTimeRange', val: string): void
  (e: 'update:selectedHistoricalLimit', val: number): void
  (e: 'update:wrapLines', val: boolean): void
  (e: 'update:showHistogram', val: boolean): void
  (e: 'update:showMobileTree', val: boolean): void
  (e: 'update:showErrorRadar', val: boolean): void
  (e: 'update:activeService', val: string): void
  (e: 'clearActiveService'): void
  (e: 'update:hoverPauseEnabled', val: boolean): void
  (e: 'runHistoricalQuery'): void
  (e: 'loadMoreHistorical'): void
  (e: 'toggleLiveTail'): void
  (e: 'clearBuffer'): void
  (e: 'exportLogs'): void
}>()


const severityFilters = [
  { label: 'ALL', value: '' },
  { label: 'ERR', value: 'ERROR' },
  { label: 'WARN', value: 'WARN' },
  { label: 'INFO', value: 'INFO' },
  { label: 'DEBUG', value: 'DEBUG' },
]

function isCurrentLevel(val: string): boolean {
  if (!val) return !props.selectedLevel || props.selectedLevel === 'ALL'
  if (val === 'ERROR') return props.selectedLevel === 'ERROR' || props.selectedLevel === 'ERR'
  return props.selectedLevel.toUpperCase() === val.toUpperCase()
}

function setLevel(val: string) {
  emit('update:selectedLevel', val)
  if (props.mode === 'historical') {
    emit('runHistoricalQuery')
  }
}
</script>

<template>
  <div class="log-stream-toolbar-root">
    <!-- Sleek Unified 38px Enterprise Toolbar -->
    <div class="logs-toolbar-sleek glass-panel desktop-only" role="toolbar" aria-label="Kubernetes Log Stream Controls">
      <!-- Zone 1 (Left - Sidebar & Stream Search) -->
      <div class="toolbar-zone-left">
        <button
          type="button" class="toolbar-btn btn-secondary" :class="{ active: isSidebarCollapsed }"
          title="Toggle Log Targets Sidebar" aria-label="Toggle Log Targets Sidebar"
          @click="emit('update:isSidebarCollapsed', !isSidebarCollapsed)"
        >
          <BaseIcon name="sidebar" size="xs" />
        </button>
        <!-- Active Service Filter Pill -->
        <div v-if="activeService" class="toolbar-service-pill font-mono" title="Filtered by service — click × to clear">
          <span class="pill-icon">📦</span>
          <span class="pill-name">{{ activeService }}</span>
          <button
            type="button"
            class="pill-close-btn"
            aria-label="Clear active service filter"
            @click="emit('clearActiveService'); emit('update:activeService', '')"
          >
            &times;
          </button>
        </div>
        <div class="toolbar-search-wrap">
          <BaseIcon name="search" size="xs" class="search-icon" />
          <input
            :value="searchKeyword" type="text"
            :placeholder="mode === 'historical' ? 'ClickHouse search (query=...)' : 'Filter logs (regex)...'"
            class="toolbar-search-input font-mono" aria-label="Filter logs"
            @input="emit('update:searchKeyword', ($event.target as HTMLInputElement).value)"
            @keyup.enter="mode === 'historical' ? emit('runHistoricalQuery') : null"
          />
          <button v-if="searchKeyword" type="button" class="clear-input-btn" aria-label="Clear filter" @click="emit('update:searchKeyword', '')">
            <BaseIcon name="x" size="xs" />
          </button>
        </div>
      </div>

      <!-- Zone 2 (Center-Left - Mode Tabs & Severity Pills) -->
      <div class="toolbar-zone-mode">
        <div class="toolbar-nav-pills font-mono" role="tablist" aria-label="Stream Mode">
          <button type="button" role="tab" :aria-selected="mode === 'live'" class="toolbar-pill-btn" :class="{ active: mode === 'live' }" @click="emit('update:mode', 'live')">
            <BaseIcon name="zap" size="xs" /> <span>Live</span>
          </button>
          <button type="button" role="tab" :aria-selected="mode === 'historical'" class="toolbar-pill-btn" :class="{ active: mode === 'historical' }" @click="emit('update:mode', 'historical')">
            <BaseIcon name="clock" size="xs" /> <span>History</span>
          </button>
        </div>

        <!-- In Live mode: compact Pause/Resume right beside mode tabs -->
        <button
          v-if="mode === 'live'"
          type="button"
          class="toolbar-btn btn-tail-toggle font-mono"
          :class="{ 'btn-tail-active': autoScroll && !isScrollLocked, 'btn-tail-paused': !autoScroll || isScrollLocked }"
          :title="autoScroll && !isScrollLocked ? 'Pause stream auto-scroll' : 'Resume stream auto-scroll'"
          @click="emit('toggleLiveTail')"
        >
          <span class="tail-dot" /> <span>{{ autoScroll && !isScrollLocked ? 'Pause' : 'Resume' }}</span>
        </button>

        <!-- In Live Tail mode: 28px quick severity filter pills -->
        <div v-if="mode === 'live'" class="toolbar-severity-pills font-mono" role="group" aria-label="Filter by Severity">
          <button
            v-for="lvl in severityFilters" :key="lvl.value" type="button"
            class="severity-pill-btn" :class="['pill-' + lvl.label.toLowerCase(), { active: isCurrentLevel(lvl.value) }]"
            @click="setLevel(lvl.value)"
          >{{ lvl.label }}</button>
        </div>

        <!-- In Historical Search mode: time range pills, limit select + query button -->
        <div v-if="mode === 'historical'" class="toolbar-historical-group font-mono">
          <div class="toolbar-time-pills">
            <button
              v-for="r in timeRanges" :key="r.label" type="button"
              class="time-pill-btn" :class="{ active: selectedTimeRange === r.label }"
              @click="emit('update:selectedTimeRange', r.label); emit('runHistoricalQuery')"
            >{{ r.label }}</button>
          </div>
          <select
            :value="selectedHistoricalLimit" class="historical-limit-select font-mono"
            title="Max Log Entries to Fetch" aria-label="Historical log limit"
            @change="emit('update:selectedHistoricalLimit', Number(($event.target as HTMLSelectElement).value)); emit('runHistoricalQuery')"
          >
            <option :value="100">100</option>
            <option :value="500">500</option>
            <option :value="1000">1k</option>
            <option :value="5000">5k</option>
            <option :value="10000">10k</option>
            <option :value="50000">All (Keyset)</option>
          </select>
          <button
            type="button" class="toolbar-btn btn-secondary" :disabled="isHistoricalLoading"
            title="Run Query" @click="emit('runHistoricalQuery')"
          >
            <BaseIcon name="refresh-cw" size="xs" :class="{ 'spin-icon': isHistoricalLoading }" /> <span>{{ isHistoricalLoading ? 'Searching...' : 'Run' }}</span>
          </button>
          <button
            v-if="hasMoreHistorical || totalHistoricalCount > currentLogsCount"
            type="button" class="toolbar-btn btn-secondary load-more-compact" :disabled="isHistoricalLoading"
            title="Load more historical logs" @click="emit('loadMoreHistorical')"
          >
            <span>+More ({{ currentLogsCount }}/{{ totalHistoricalCount }})</span>
          </button>
        </div>
      </div>

      <!-- ClickHouse Log Engine Architecture Badge -->
      <ClickHouseEngineBadge />

      <!-- Zone 4 (Right - Stream Actions Group: Error Radar, Volume, Wrap, Hover, Clear, Export) -->
      <div class="toolbar-actions-group">
        <button
          type="button" class="toolbar-btn btn-error-radar" :class="{ active: showErrorRadar }"
          title="Blind Error Radar — Auto-discover & group errors across 10GB+ logs"
          @click="emit('update:showErrorRadar', !showErrorRadar)"
        >
          <BaseIcon name="alert-triangle" size="xs" /> <span>Error Radar</span>
        </button>
        <button type="button" class="toolbar-btn btn-secondary" :class="{ active: showHistogram }" title="Toggle Volume Histogram" @click="emit('update:showHistogram', !showHistogram)">
          <BaseIcon name="bar-chart-2" size="xs" /> <span>Volume</span>
        </button>
        <button type="button" class="toolbar-btn btn-secondary" :class="{ active: wrapLines }" title="Toggle Line Wrap" @click="emit('update:wrapLines', !wrapLines)">
          <span>Wrap</span>
        </button>
        <button
          type="button"
          class="toolbar-btn btn-secondary"
          :class="{ active: hoverPauseEnabled, 'btn-tail-paused': isHoverPaused }"
          :title="hoverPauseEnabled ? (isHoverPaused ? 'Stream hover-paused' : 'Hover-pause enabled') : 'Hover-pause disabled'"
          aria-label="Toggle hover pause"
          @click="emit('update:hoverPauseEnabled', !hoverPauseEnabled)"
        >
          <span>⏸ Hover</span>
        </button>
        <button type="button" class="toolbar-btn btn-secondary" title="Clear Buffer" aria-label="Clear Buffer" @click="emit('clearBuffer')">
          <BaseIcon name="trash" size="xs" />
        </button>
        <button type="button" class="toolbar-btn btn-secondary" title="Export Logs" aria-label="Export Logs" @click="emit('exportLogs')">
          <BaseIcon name="download" size="xs" />
        </button>
      </div>

    </div>

    <!-- Mobile Command Bar (<768px) -->
    <div class="logs-mobile-command-bar mobile-only">
      <div class="mobile-bar-left">
        <button
          type="button" class="mobile-tree-toggle-btn font-mono"
          aria-label="Toggle log targets drawer" @click="emit('update:showMobileTree', !showMobileTree)"
        >
          <BaseIcon :name="selectedTarget.type === 'node' ? 'server' : (selectedTarget.icon || 'box')" size="xs" />
          <span>{{ selectedTarget.name }}</span>
          <BaseIcon name="chevron-down" size="xs" />
        </button>
      </div>
      <div class="mobile-bar-actions">
        <button type="button" class="mobile-btn font-mono" :class="{ active: mode === 'live' }" @click="emit('update:mode', mode === 'live' ? 'historical' : 'live')">
          <BaseIcon :name="mode === 'live' ? 'zap' : 'search'" size="xs" /> <span>{{ mode === 'live' ? 'Live' : 'History' }}</span>
        </button>
        <button
          type="button" class="mobile-btn font-mono"
          :class="{ 'btn-tail-active': autoScroll && !isScrollLocked, 'btn-tail-paused': !autoScroll || isScrollLocked }"
          @click="emit('toggleLiveTail')"
        >
          <span class="tail-dot" /> <span>{{ autoScroll && !isScrollLocked ? 'Tail' : 'Pause' }}</span>
        </button>
        <button type="button" class="mobile-btn font-mono" :class="{ active: showHistogram }" aria-label="Toggle Volume Histogram" @click="emit('update:showHistogram', !showHistogram)">
          <BaseIcon name="bar-chart-2" size="xs" />
        </button>
        <button
          type="button" class="mobile-btn font-mono" :class="{ active: showErrorRadar }"
          title="Error Radar" aria-label="Error Radar"
          @click="emit('update:showErrorRadar', !showErrorRadar)"
        >
          <BaseIcon name="alert-triangle" size="xs" />
        </button>
        <button type="button" class="mobile-btn font-mono" title="Clear Buffer" aria-label="Clear Buffer" @click="emit('clearBuffer')">
          <BaseIcon name="trash" size="xs" />
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
@import '../../assets/styles/views/logstream.css';
</style>
