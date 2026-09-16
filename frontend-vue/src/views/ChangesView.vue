<script setup lang="ts">
import { ref, computed } from 'vue'
import '../assets/styles/views/changes.css'
import '../assets/styles/components/changes-drawers.css'
import { useChangesTimeline } from '../composables/useChangesTimeline'
import ChangesFilterBar from '../components/changes/ChangesFilterBar.vue'
import BaseIcon from '../components/ui/BaseIcon.vue'
import ChangesTimelineStream from '../components/changes/ChangesTimelineStream.vue'
import ChangesMobileCards from '../components/changes/ChangesMobileCards.vue'
import ChangeDiffDrawer from '../components/changes/ChangeDiffDrawer.vue'
import ModalDrawer from '../components/ui/ModalDrawer.vue'

const {
  loading,
  feedbackMessage,
  maintenanceWindows,
  searchQuery,
  selectedCluster,
  selectedTimeWindow,
  selectedStatus,
  selectedDiffEvent,
  isDiffDrawerOpen,
  showCreateModal,
  isSubmitting,
  newChange,
  filteredEvents,
  availableClusters,
  totalChanges24h,
  totalRollbacks,
  configDrifts,
  loadTimelineData,
  handleApprove,
  handleReject,
  handleRollback,
  handleCreateChange,
  inspectDiff
} = useChangesTimeline()

const isFilterOpen = ref(false)

const verifiedCount = computed(() => {
  return filteredEvents.value.filter(e => e.status === 'approved' || e.status === 'deployed').length
})

const rolloutsCount = computed(() => {
  return filteredEvents.value.filter(e => e.eventType === 'rollback' || e.eventType === 'gitops').length || totalRollbacks.value
})
</script>

<template>
  <div class="changes-page">
    <!-- Mobile 40-44px Command Bar (<768px) -->
    <div class="changes-mobile-command-bar mobile-only">
      <div class="command-bar-left">
        <span class="command-bar-title font-bold"><BaseIcon name="git-commit" size="xs" /> Changes ({{ filteredEvents.length }})</span>
      </div>
      <div class="command-bar-actions">
        <button
          class="btn-cmd-filter"
          :class="{ active: isFilterOpen }"
          title="Filter stream"
          aria-label="Filter stream"
          @click="isFilterOpen = !isFilterOpen"
        >
          <BaseIcon name="search" size="xs" />
          <span>Filter</span>
        </button>
        <button
          class="btn-icon-cmd"
          title="Refresh stream"
          aria-label="Refresh stream"
          :disabled="loading"
          @click="loadTimelineData"
        >
          <BaseIcon name="refresh" size="xs" :class="{ 'spin-animation': loading }" />
        </button>
        <button
          class="btn-icon-cmd"
          title="Submit RFC"
          aria-label="Submit RFC"
          @click="showCreateModal = true"
        >
          <BaseIcon name="plus" size="xs" />
        </button>
      </div>
    </div>

    <!-- Mobile 20px Centered Micro-Telemetry Strip (<768px) -->
    <div class="changes-micro-telemetry mobile-only font-mono" role="status" aria-label="Changes Micro Telemetry">
      <span class="tel-item tel-changes"><BaseIcon name="file-text" size="xs" /> {{ totalChanges24h }} Changes</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-rollouts"><BaseIcon name="zap" size="xs" /> {{ rolloutsCount }} Rollouts</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-verified"><BaseIcon name="shield" size="xs" /> {{ verifiedCount }} Verified</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-drift"><BaseIcon name="alert-triangle" size="xs" /> {{ configDrifts }} Drift</span>
    </div>

    <!-- Feedback Banner -->
    <div v-if="feedbackMessage" class="feedback-banner animate-fade-in">
      <BaseIcon name="check-circle" size="xs" class="feedback-icon" />
      <span>{{ feedbackMessage }}</span>
    </div>

    <!-- Active Maintenance Windows Banner -->
    <div v-if="maintenanceWindows.length > 0" class="maintenance-bar glass-panel">
      <div class="mw-header">
        <div class="mw-title-wrap">
          <span class="pulse-dot pulse-dot-emerald"></span>
          <span class="mw-heading">Active Maintenance Windows:</span>
        </div>
        <span class="mw-badge">Air-Gapped Sync</span>
      </div>

      <div class="mw-items">
        <div 
          v-for="mw in maintenanceWindows" 
          :key="mw.id" 
          class="mw-item"
          :class="{ 'mw-active': mw.active }"
        >
          <div class="mw-status-indicator">
            <span v-if="mw.active" class="badge badge-emerald">ACTIVE NOW</span>
            <span v-else class="badge badge-amber">SCHEDULED</span>
          </div>
          <div class="mw-info">
            <div class="mw-title">{{ mw.title }}</div>
            <div class="mw-meta font-mono">
              <span>Cluster: {{ mw.cluster }}</span>
              <span>•</span>
              <span>Window: {{ new Date(mw.start_at).toLocaleTimeString() }} - {{ new Date(mw.end_at).toLocaleTimeString() }}</span>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Filter Bar (Collapsible on mobile via isFilterOpen) -->
    <ChangesFilterBar
      :class="{ 'mobile-filter-active': isFilterOpen }"
      v-model:searchQuery="searchQuery"
      v-model:selectedCluster="selectedCluster"
      v-model:selectedTimeWindow="selectedTimeWindow"
      v-model:selectedStatus="selectedStatus"
      :clusters="availableClusters"
      @create="showCreateModal = true"
    />

    <!-- Desktop: Interactive Timeline Stream (Zero horizontal overflow) -->
    <ChangesTimelineStream
      :events="filteredEvents"
      :loading="loading"
      @diff="inspectDiff"
      @rollback="handleRollback"
      @approve="handleApprove"
      @reject="handleReject"
    />

    <!-- Mobile: Touch-Friendly Card Stream (~68-75px high density, 0 horizontal scroll) -->
    <ChangesMobileCards
      :events="filteredEvents"
      :loading="loading"
      @diff="inspectDiff"
      @rollback="handleRollback"
      @approve="handleApprove"
      @reject="handleReject"
      @refresh="loadTimelineData"
    />

    <!-- Side Drawer: Visual Unified Diff -->
    <ChangeDiffDrawer
      :show="isDiffDrawerOpen"
      :event="selectedDiffEvent"
      @update:show="isDiffDrawerOpen = $event"
      @rollback="handleRollback"
    />

    <!-- Modal Drawer: Submit Change Request (RFC) -->
    <ModalDrawer
      v-model:show="showCreateModal"
      title="Submit Change Request (RFC)"
      subtitle="Register an operational or infrastructure change for peer review and governance."
    >
      <form @submit.prevent="handleCreateChange" class="form-layout">
        <div class="form-group">
          <label>RFC Title</label>
          <input 
            v-model="newChange.title" 
            type="text" 
            placeholder="e.g. Scale ingress gateway replicas and tune keepalive" 
            class="input-glass" 
            required 
          />
        </div>

        <div class="form-group">
          <label>Detailed Description & Rollback Plan</label>
          <textarea 
            v-model="newChange.description" 
            rows="3" 
            placeholder="Outline change rationale, affected pods, and rollback triggers..." 
            class="input-glass"
          ></textarea>
        </div>

        <div class="form-row">
          <div class="form-group">
            <label>Change Classification</label>
            <select v-model="newChange.type" class="input-glass">
              <option value="standard">Standard RFC (Scheduled Window)</option>
              <option value="emergency">Emergency Hotfix (Immediate Review)</option>
            </select>
          </div>
          <div class="form-group">
            <label>Target Cluster</label>
            <select v-model="newChange.cluster" class="input-glass">
              <option v-for="c in availableClusters" :key="c" :value="c">{{ c }}</option>
            </select>
          </div>
        </div>

        <div class="form-row">
          <div class="form-group">
            <label>Namespace</label>
            <input 
              v-model="newChange.namespace" 
              type="text" 
              placeholder="e.g. production-core" 
              class="input-glass" 
              required 
            />
          </div>
          <div class="form-group">
            <label>Target Resource (Kind/Name)</label>
            <input 
              v-model="newChange.resource" 
              type="text" 
              placeholder="e.g. deployment/auth-service" 
              class="input-glass" 
              required 
            />
          </div>
        </div>
      </form>

      <template #footer="{ close }">
        <button class="btn btn-secondary" type="button" @click="close">Cancel</button>
        <button class="btn btn-primary" :disabled="isSubmitting" @click="handleCreateChange">
          {{ isSubmitting ? 'Registering RFC...' : 'Submit for Review' }}
        </button>
      </template>
    </ModalDrawer>
  </div>
</template>
