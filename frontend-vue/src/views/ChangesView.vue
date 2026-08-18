<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import {
  changesApi,
  type ChangeRequest,
  type MaintenanceWindow
} from '../api/management'
import MetricCard from '../components/ui/MetricCard.vue'
import StatusBadge from '../components/ui/StatusBadge.vue'
import DataTable, { type Column } from '../components/ui/DataTable.vue'
import ModalDrawer from '../components/ui/ModalDrawer.vue'

// State
const loading = ref(false)
const error = ref<string | null>(null)
const changes = ref<ChangeRequest[]>([])
const selectedStatus = ref<string>('all')
const feedbackMessage = ref<string | null>(null)

// Maintenance Windows
const maintenanceWindows = ref<MaintenanceWindow[]>([])

// Modal State
const showCreateModal = ref(false)
const isSubmitting = ref(false)
const newChange = ref({
  title: '',
  description: '',
  type: 'standard' as 'standard' | 'emergency',
  cluster: 'prod-us-east-1',
  namespace: 'production-core',
  resource: 'deployment/payment-processor',
  requester: 'sre.lead@enterprise.io'
})

async function loadChanges() {
  loading.value = true
  error.value = null
  try {
    const res = await changesApi.getChanges()
    changes.value = res?.data || []
  } catch (err: any) {
    changes.value = []
    error.value = err?.message || 'Failed to load change requests'
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  loadChanges()
})

const filteredChanges = computed(() => {
  if (selectedStatus.value === 'all') return changes.value
  return changes.value.filter(c => c.status === selectedStatus.value)
})

const pendingCount = computed(() => changes.value.filter(c => c.status === 'pending').length)
const approvedCount = computed(() => changes.value.filter(c => c.status === 'approved').length)
const emergencyCount = computed(() => changes.value.filter(c => c.type === 'emergency').length)

const changeColumns: Column<ChangeRequest>[] = [
  { key: 'id', label: 'RFC ID', sortable: true, width: '110px' },
  { key: 'title', label: 'Change Request & Scope', sortable: true },
  { key: 'type', label: 'Type', sortable: true, width: '120px' },
  { key: 'cluster', label: 'Target Resource', sortable: true },
  { key: 'requester', label: 'Requester / Approver', sortable: true },
  { key: 'status', label: 'Status', sortable: true, width: '130px' },
  { key: 'actions', label: 'Governance Action', align: 'right', width: '200px' }
]

function showFeedback(msg: string) {
  feedbackMessage.value = msg
  setTimeout(() => {
    if (feedbackMessage.value === msg) {
      feedbackMessage.value = null
    }
  }, 4000)
}

async function handleApprove(cr: ChangeRequest) {
  try {
    await changesApi.approveChange(cr.id)
    cr.status = 'approved'
    cr.approver = 'current.user@enterprise.io'
    showFeedback(`Change request ${cr.id} successfully approved.`)
  } catch (e: any) {
    showFeedback(`Failed to approve change request: ${e?.message || 'Unknown error'}`)
  }
}

async function handleReject(cr: ChangeRequest) {
  try {
    await changesApi.rejectChange(cr.id)
    cr.status = 'rejected'
    cr.approver = 'current.user@enterprise.io'
    showFeedback(`Change request ${cr.id} rejected.`)
  } catch (e: any) {
    showFeedback(`Failed to reject change request: ${e?.message || 'Unknown error'}`)
  }
}

async function handleCreateChange() {
  if (!newChange.value.title || !newChange.value.resource) return
  isSubmitting.value = true
  const cr: ChangeRequest = {
    id: `cr-${Math.floor(Math.random() * 9000 + 1000)}`,
    title: newChange.value.title,
    description: newChange.value.description,
    type: newChange.value.type,
    status: 'pending',
    requester: newChange.value.requester,
    cluster: newChange.value.cluster,
    namespace: newChange.value.namespace,
    resource: newChange.value.resource,
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString()
  }

  try {
    const created = await changesApi.createChange(cr)
    changes.value.unshift(created || cr)
    showCreateModal.value = false
    showFeedback(`Change Request ${cr.id} registered for review.`)
    newChange.value.title = ''
    newChange.value.description = ''
  } catch (e: any) {
    showFeedback(`Failed to submit change request: ${e?.message || 'Unknown error'}`)
  } finally {
    isSubmitting.value = false
  }
}
</script>

<template>
  <div class="changes-page">
    <!-- Header -->
    <div class="page-header">
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

    <!-- Feedback Banner -->
    <div v-if="feedbackMessage" class="feedback-banner animate-fade-in">
      <span class="feedback-icon">✓</span>
      <span>{{ feedbackMessage }}</span>
    </div>

    <!-- Metrics Grid -->
    <div class="metrics-grid">
      <MetricCard 
        title="Pending Reviews" 
        :value="pendingCount" 
        trend="Requires Four-Eyes Approval" 
        trendDirection="neutral" 
      />
      <MetricCard 
        title="Approved & Queued" 
        :value="approvedCount" 
        trend="Ready for Deployment" 
        trendDirection="up" 
      />
      <MetricCard 
        title="Emergency RFCs" 
        :value="emergencyCount" 
        trend="Hotfix Expedited" 
        trendDirection="down" 
      />
      <MetricCard 
        title="Active Windows" 
        :value="`${maintenanceWindows.filter(m => m.active).length} LIVE`" 
        trend="Scheduled Maintenance" 
        trendDirection="neutral" 
      />
    </div>

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
    <div class="filter-bar glass-panel">
      <div class="filter-left">
        <span class="filter-label">Filter by Status:</span>
        <div class="status-pills">
          <button 
            v-for="s in ['all', 'pending', 'approved', 'deployed', 'rejected']" 
            :key="s"
            class="spill"
            :class="{ active: selectedStatus === s }"
            @click="selectedStatus = s"
          >
            {{ s.toUpperCase() }}
          </button>
        </div>
      </div>
    </div>

    <!-- DataTable of Changes -->
    <DataTable
      :columns="changeColumns"
      :data="filteredChanges"
      :loading="loading"
      searchable
      searchPlaceholder="Filter RFCs by title, resource, cluster, or requester..."
    >
      <template #cell-id="{ value }">
        <span class="rfc-id font-mono">{{ value }}</span>
      </template>

      <template #cell-title="{ row }">
        <div class="change-title-cell">
          <div class="ct-main">{{ row.title }}</div>
          <small class="ct-desc">{{ row.description }}</small>
        </div>
      </template>

      <template #cell-type="{ value }">
        <span v-if="value === 'emergency'" class="badge badge-rose">EMERGENCY</span>
        <span v-else class="badge badge-cyan">STANDARD</span>
      </template>

      <template #cell-cluster="{ row }">
        <div class="target-cell">
          <div class="tc-resource font-mono text-cyan">{{ row.resource }}</div>
          <div class="tc-cluster font-mono text-muted">{{ row.cluster }} / {{ row.namespace }}</div>
        </div>
      </template>

      <template #cell-requester="{ row }">
        <div class="requester-cell">
          <span class="req-user">{{ row.requester }}</span>
          <small v-if="row.approver" class="app-user text-emerald font-mono">
            Approved by: {{ row.approver }}
          </small>
        </div>
      </template>

      <template #cell-status="{ value, row }">
        <StatusBadge :status="String(value || row.status)" />
      </template>

      <template #cell-actions="{ row }">
        <div v-if="row.status === 'pending'" class="actions-group">
          <button class="btn btn-primary btn-sm" @click="handleApprove(row)">
            <span>✓ Approve</span>
          </button>
          <button class="btn btn-secondary btn-sm" @click="handleReject(row)">
            <span>✕ Reject</span>
          </button>
        </div>
        <div v-else-if="row.status === 'approved'" class="actions-group">
          <button class="btn btn-secondary btn-sm" @click="showFeedback(`Initiating rollout for ${row.id}`)">
            <span>🚀 Deploy</span>
          </button>
        </div>
        <div v-else class="actions-group">
          <span class="text-muted font-mono text-xs">Archived</span>
        </div>
      </template>
    </DataTable>

    <!-- MODAL: CREATE CHANGE REQUEST -->
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

<style scoped>
.changes-page {
  display: flex;
  flex-direction: column;
  gap: 24px;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 20px;
}

.header-badge {
  display: flex;
  gap: 8px;
  margin-bottom: 8px;
}

.page-title {
  font-size: 26px;
  font-weight: 800;
  letter-spacing: -0.03em;
  color: #fff;
  margin-bottom: 6px;
}

.page-desc {
  font-size: 13px;
  color: var(--text-secondary);
  max-width: 840px;
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 12px;
}

.feedback-banner {
  background: rgba(16, 185, 129, 0.12);
  border: 1px solid rgba(16, 185, 129, 0.3);
  color: #34d399;
  padding: 12px 18px;
  border-radius: 12px;
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 13px;
  font-weight: 600;
}

.metrics-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 16px;
}

/* Maintenance Bar */
.maintenance-bar {
  padding: 16px 20px;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.mw-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.mw-title-wrap {
  display: flex;
  align-items: center;
  gap: 8px;
}

.mw-heading {
  font-size: 13px;
  font-weight: 700;
  color: #fff;
  letter-spacing: 0.02em;
}

.mw-badge {
  font-size: 10px;
  font-weight: 700;
  background: rgba(255, 255, 255, 0.08);
  padding: 2px 8px;
  border-radius: 6px;
  color: var(--text-muted);
}

.mw-items {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(320px, 1fr));
  gap: 12px;
}

.mw-item {
  background: rgba(11, 15, 25, 0.6);
  border: 1px solid var(--border-subtle);
  border-radius: 10px;
  padding: 12px 16px;
  display: flex;
  align-items: center;
  gap: 14px;
}

.mw-item.mw-active {
  border-color: rgba(16, 185, 129, 0.4);
  background: rgba(16, 185, 129, 0.05);
}

.mw-title {
  font-size: 13px;
  font-weight: 600;
  color: var(--text-primary);
}

.mw-meta {
  font-size: 11px;
  color: var(--text-muted);
  display: flex;
  gap: 6px;
  margin-top: 2px;
}

/* Filter Bar */
.filter-bar {
  padding: 12px 20px;
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.filter-left {
  display: flex;
  align-items: center;
  gap: 14px;
}

.filter-label {
  font-size: 12px;
  font-weight: 600;
  color: var(--text-muted);
}

.status-pills {
  display: flex;
  gap: 6px;
}

.spill {
  padding: 5px 12px;
  border-radius: 6px;
  background: rgba(255, 255, 255, 0.04);
  border: 1px solid var(--border-subtle);
  color: var(--text-secondary);
  font-size: 11px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.15s ease;
}

.spill:hover {
  background: rgba(255, 255, 255, 0.08);
  color: var(--text-primary);
}

.spill.active {
  background: rgba(6, 182, 212, 0.15);
  border-color: rgba(6, 182, 212, 0.4);
  color: #38bdf8;
}

/* Cells */
.rfc-id {
  font-size: 12px;
  font-weight: 700;
  color: var(--accent-sky);
}

.change-title-cell {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.ct-main {
  font-weight: 600;
  color: var(--text-primary);
}

.ct-desc {
  font-size: 11px;
  color: var(--text-muted);
}

.target-cell {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.tc-resource {
  font-size: 12px;
}

.tc-cluster {
  font-size: 11px;
}

.requester-cell {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.req-user {
  font-size: 12px;
  color: var(--text-primary);
}

.app-user {
  font-size: 10px;
}

.actions-group {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 8px;
}

.text-xs {
  font-size: 11px;
}

/* Form */
.form-layout {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.form-row {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 14px;
}

.form-group {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.form-group label {
  font-size: 12px;
  font-weight: 600;
  color: var(--text-secondary);
}

.btn-sm {
  padding: 6px 12px;
  font-size: 12px;
}

.empty-list {
  padding: 24px 16px;
  text-align: center;
  color: var(--text-muted);
  font-size: 12px;
}
</style>
