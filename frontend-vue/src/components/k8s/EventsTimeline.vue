<script setup lang="ts">
import { useEventsTimeline, type EventsTimelineProps } from '../../composables/useEventsTimeline'

const props = withDefaults(defineProps<EventsTimelineProps>(), {
  autoRefresh: false,
})

const {
  events,
  loading,
  error,
  selectedTypeFilter,
  searchQuery,
  isAutoRefreshActive,
  filteredEvents,
  warningCount,
  normalCount,
  loadEvents,
  toggleAutoRefresh,
  getEventTime,
} = useEventsTimeline(props)
</script>

<template>
  <div class="events-timeline">
    <!-- Header & Controls Bar -->
    <div class="timeline-toolbar glass-panel">
      <div class="toolbar-left">
        <!-- Type Filter Chips -->
        <div class="filter-chips">
          <button
            type="button"
            class="chip-btn"
            :class="{ 'is-active': selectedTypeFilter === 'all' }"
            @click="selectedTypeFilter = 'all'"
          >
            All ({{ events.length }})
          </button>
          <button
            type="button"
            class="chip-btn chip-warning"
            :class="{ 'is-active': selectedTypeFilter === 'Warning' }"
            @click="selectedTypeFilter = 'Warning'"
          >
            ⚠️ Warnings ({{ warningCount }})
          </button>
          <button
            type="button"
            class="chip-btn chip-normal"
            :class="{ 'is-active': selectedTypeFilter === 'Normal' }"
            @click="selectedTypeFilter = 'Normal'"
          >
            ℹ️ Normal ({{ normalCount }})
          </button>
        </div>

        <!-- Search Input -->
        <div class="search-box">
          <span class="search-icon">🔍</span>
          <input
            v-model="searchQuery"
            type="text"
            class="input-glass font-mono search-input"
            placeholder="Search reason, object, message..."
          />
          <button
            v-if="searchQuery"
            type="button"
            class="clear-btn"
            title="Clear search"
            @click="searchQuery = ''"
          >
            ✕
          </button>
        </div>
      </div>

      <div class="toolbar-right">
        <!-- Auto Refresh Toggle -->
        <button
          type="button"
          class="btn-toggle-auto font-mono"
          :class="{ 'is-active': isAutoRefreshActive }"
          @click="toggleAutoRefresh"
          title="Toggle live 5s polling"
        >
          <span class="pulse-dot" :class="isAutoRefreshActive ? 'pulse-dot-emerald' : 'pulse-dot-muted'"></span>
          <span>Live 5s</span>
        </button>

        <!-- Refresh Button -->
        <button
          type="button"
          class="btn btn-secondary btn-xs btn-refresh"
          :disabled="loading"
          @click="loadEvents()"
        >
          <span :class="{ 'spin-icon': loading }">🔄</span>
          <span>{{ loading ? 'Updating...' : 'Refresh' }}</span>
        </button>
      </div>
    </div>

    <!-- Error Banner -->
    <div v-if="error" class="events-error-banner glass-panel animate-fade-in">
      <span class="err-icon">⚠️</span>
      <div class="err-text">
        <strong>Error loading events:</strong>
        <span class="font-mono font-small">{{ error }}</span>
      </div>
      <button type="button" class="btn btn-secondary btn-xs" @click="loadEvents()">
        Retry
      </button>
    </div>

    <!-- Loading Skeleton -->
    <div v-if="loading && events.length === 0" class="timeline-loading">
      <div v-for="n in 4" :key="n" class="skeleton-event-item glass-panel animate-pulse">
        <div class="sk-line sk-line-short"></div>
        <div class="sk-line sk-line-long"></div>
      </div>
    </div>

    <!-- Empty State -->
    <div v-else-if="filteredEvents.length === 0" class="events-empty glass-panel">
      <div class="empty-icon">📢</div>
      <div class="empty-title">No Kubernetes Events Found</div>
      <div class="empty-desc text-muted">
        {{
          searchQuery
            ? 'No events match the current search filter.'
            : selectedTypeFilter !== 'all'
            ? 'No ' + selectedTypeFilter + ' events recorded in this scope.'
            : 'No Kubernetes events recorded for this scope.'
        }}
      </div>
    </div>

    <!-- Timeline Event List -->
    <div v-else class="timeline-list">
      <div 
        v-for="(ev, idx) in filteredEvents"
        :key="ev.metadata?.uid || ((ev.metadata?.name || 'ev') + '-' + idx)"
        class="event-card glass-panel"
        :class="'event-type-' + (ev.type || 'Normal').toLowerCase()"
      >
        <!-- Timeline Marker Dot -->
        <div class="event-marker">
          <span 
            class="marker-dot"
            :class="ev.type === 'Warning' ? 'marker-warning' : 'marker-normal'"
          ></span>
          <span v-if="idx !== filteredEvents.length - 1" class="marker-line"></span>
        </div>

        <!-- Event Content -->
        <div class="event-body">
          <!-- Top Row: Type Badge, Reason, Count, Age -->
          <div class="event-header-row">
            <div class="header-badges">
              <span 
                class="event-type-badge font-mono"
                :class="ev.type === 'Warning' ? 'type-warning' : 'type-normal'"
              >
                {{ ev.type === 'Warning' ? '⚠️ Warning' : 'ℹ️ Normal' }}
              </span>

              <span class="event-reason font-mono font-bold">
                {{ ev.reason || 'Event' }}
              </span>

              <span 
                v-if="ev.count && ev.count > 1"
                class="event-count-badge font-mono"
                :title="'Occurred ' + ev.count + ' times'"
              >
                x{{ ev.count }}
              </span>
            </div>

            <div class="event-meta-right">
              <span class="event-age font-mono" :title="ev.lastTimestamp || ev.eventTime || ev.firstTimestamp || ev.metadata?.creationTimestamp">
                🕒 {{ getEventTime(ev) }}
              </span>
            </div>
          </div>

          <!-- Involved Object Row -->
          <div v-if="ev.involvedObject" class="event-object-row font-mono">
            <span class="obj-label text-muted">Object:</span>
            <span class="obj-kind text-cyan">{{ ev.involvedObject.kind || 'Resource' }}</span>
            <span class="obj-sep text-muted">/</span>
            <span class="obj-name text-white">{{ ev.involvedObject.name || 'unknown' }}</span>
            <span v-if="ev.involvedObject.namespace" class="obj-ns text-muted">
              (ns: {{ ev.involvedObject.namespace }})
            </span>
          </div>

          <!-- Message Row -->
          <div class="event-message font-mono">
            {{ ev.message || 'No additional message detail.' }}
          </div>

          <!-- Footer details: Source Component / Sub-object -->
          <div v-if="ev.source?.component || ev.source?.host || ev.involvedObject?.fieldPath" class="event-footer-row font-mono text-muted font-small">
            <span v-if="ev.source?.component">
              Source: <span class="text-cyan">{{ ev.source.component }}</span>
            </span>
            <span v-if="ev.source?.host" class="footer-item">
              Host: <span class="text-muted">{{ ev.source.host }}</span>
            </span>
            <span v-if="ev.involvedObject?.fieldPath" class="footer-item">
              Field: <span class="text-amber">+{{ ev.involvedObject.fieldPath }}</span>
            </span>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
@import '../../assets/styles/components/events-timeline.css';
</style>