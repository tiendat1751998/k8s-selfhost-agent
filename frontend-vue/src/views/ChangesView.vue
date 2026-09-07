<script setup lang="ts">
import '../assets/styles/views/changes.css'
import { useChangesTimeline } from '../composables/useChangesTimeline'
import ChangesHudCards from '../components/changes/ChangesHudCards.vue'
import ChangesFilterBar from '../components/changes/ChangesFilterBar.vue'
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
  highRiskMutations,
  handleApprove,
  handleReject,
  handleRollback,
  handleCreateChange,
  inspectDiff
} = useChangesTimeline()
</script>

<template>
  <div class="changes-page">
    <!-- Header -->
    <div class="page-header desktop-header desktop-only">
      <div class="header-titles">
        <div class="header-badge">
          <span class="badge badge-cyan">ITIL Change Governance</span>
          <span class="badge badge-emerald">Audit Trail Enforced</span>
        </div>
        <h1 class="page-title">Enterprise Change Management (RFC)</h1>
        <p class="page-desc">
          Review, approve, and execute production cluster configuration modifications, emergency hotfixes, and maintenance windows with four-eyes verification.
        </p>
      </div>

      <div class="header-actions">
        <button class="btn btn-primary" @click="showCreateModal = true">
          <span>+ Submit Change Request</span>
        </button>
      </div>
    </div>

    <!-- Mobile 40px Command Bar (<640px) -->
    <div class="changes-mobile-command-bar mobile-only">
      <div class="command-bar-left">
        <span class="command-bar-title font-bold">🔄 RFC Changes ({{ filteredEvents.length }})</span>
      </div>
      <div class="command-bar-actions">
        <button
          class="btn-icon-cmd"
          title="Submit RFC"
          aria-label="Submit RFC"
          @click="showCreateModal = true"
        >
          <span>➕</span>
        </button>
      </div>
    </div>

    <!-- Mobile 20px Centered Micro-Telemetry Strip (<640px) -->
    <div class="changes-micro-telemetry mobile-only font-mono" role="status" aria-label="Changes Micro Telemetry">
      <span class="tel-item tel-changes">🔄 {{ totalChanges24h }} chgs</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-rollbacks">⏪ {{ totalRollbacks }} rolls</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-drifts">🎯 {{ configDrifts }} drift</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-risk">⚠️ {{ highRiskMutations }} risk</span>
    </div>

    <!-- Feedback Banner -->
    <div v-if="feedbackMessage" class="feedback-banner animate-fade-in">
      <span class="feedback-icon">✓</span>
      <span>{{ feedbackMessage }}</span>
    </div>

    <!-- HUD KPI Cards -->
    <ChangesHudCards
      class="desktop-only"
      :totalChanges="totalChanges24h"
      :totalRollbacks="totalRollbacks"
      :configDrifts="configDrifts"
      :highRiskMutations="highRiskMutations"
    />

    <!-- Active Maintenance Windows Banner -->
    <div class="maintenance-bar glass-panel">
      <div class="mw-header">
        <div class="mw-title-wrap">
          <span class="pulse-dot pulse-dot-emerald"></span>
          <span class="mw-heading">Active Maintenance Windows:</span>
        </div>
        <span class="mw-badge">Air-Gapped Sync</span>
      </div>

      <div v-if="maintenanceWindows.length === 0" class="empty-list">
        No active maintenance windows scheduled
      </div>
      <div v-else class="mw-items">
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

    <!-- Filter Bar -->
    <ChangesFilterBar
      v-model:searchQuery="searchQuery"
      v-model:selectedCluster="selectedCluster"
      v-model:selectedTimeWindow="selectedTimeWindow"
      v-model:selectedStatus="selectedStatus"
      :clusters="availableClusters"
    />

    <!-- Desktop: Interactive Timeline Stream -->
    <ChangesTimelineStream
      :events="filteredEvents"
      :loading="loading"
      @diff="inspectDiff"
      @rollback="handleRollback"
      @approve="handleApprove"
      @reject="handleReject"
    />

    <!-- Mobile: Touch-Friendly Card Stream (~65px/item, 0 horizontal scroll) -->
    <ChangesMobileCards
      :events="filteredEvents"
      :loading="loading"
      @diff="inspectDiff"
      @rollback="handleRollback"
      @approve="handleApprove"
      @reject="handleReject"
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
              <option value="prod-us-east-1">prod-us-east-1 (Primary)</option>
              <option value="prod-eu-west-1">prod-eu-west-1 (Secondary)</option>
              <option value="staging-us-east">staging-us-east</option>
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
