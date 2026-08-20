<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import MetricCard from '../components/ui/MetricCard.vue'
import DataTable, { type Column } from '../components/ui/DataTable.vue'
import StatusBadge from '../components/ui/StatusBadge.vue'
import ModalDrawer from '../components/ui/ModalDrawer.vue'
import {
  promotionsApi,
  type Promotion,
  type Environment,
  type CreatePromotionPayload
} from '../api/compute'

const loading = ref(false)
const error = ref<string | null>(null)
const actionLoading = ref<string | null>(null)
const toastMessage = ref<{ text: string; type: 'success' | 'error' } | null>(null)

const promotions = ref<Promotion[]>([])

// Create Promotion Modal
const showCreateModal = ref(false)
const newPromotion = ref<CreatePromotionPayload>({
  service: '',
  version: '',
  from_env: 'dev',
  to_env: 'qa',
  requester: 'sre-engineer'
})

async function fetchPromotions() {
  loading.value = true
  error.value = null
  try {
    const res = await promotionsApi.list()
    promotions.value = Array.isArray(res.data) ? res.data : []
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : 'Failed to retrieve promotions'
    error.value = msg
    promotions.value = []
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  fetchPromotions()
})

const pendingCount = computed(() => promotions.value.filter(p => p.status === 'pending').length)
const approvedCount = computed(() => promotions.value.filter(p => p.status === 'approved' || p.status === 'promoting').length)
const completedCount = computed(() => promotions.value.filter(p => p.status === 'completed').length)
const rejectedCount = computed(() => promotions.value.filter(p => p.status === 'rejected' || p.status === 'failed').length)

// Pipeline Board Stages
const environments: Environment[] = ['dev', 'qa', 'staging', 'production']

function getPromotionsForEnv(env: Environment) {
  return promotions.value.filter(p => p.to_env === env)
}

const columns: Column<Promotion>[] = [
  { key: 'service', label: 'Service Name', sortable: true },
  { key: 'version', label: 'Target Version', width: '130px', sortable: true },
  { key: 'from_env', label: 'Source', width: '110px', sortable: true },
  { key: 'to_env', label: 'Destination', width: '110px', sortable: true },
  { key: 'requester', label: 'Requested By', width: '140px', sortable: true },
  { key: 'status', label: 'Status', width: '130px', sortable: true },
  { key: 'created_at', label: 'Requested At', width: '160px', sortable: true },
  { key: 'actions', label: 'Approvals', width: '220px', align: 'right' },
]

function showToast(text: string, type: 'success' | 'error' = 'success') {
  toastMessage.value = { text, type }
  setTimeout(() => {
    if (toastMessage.value?.text === text) {
      toastMessage.value = null
    }
  }, 4000)
}

async function handleCreatePromotion() {
  if (!newPromotion.value.service || !newPromotion.value.version) {
    showToast('Service name and version are required', 'error')
    return
  }
  if (newPromotion.value.from_env === newPromotion.value.to_env) {
    showToast('Source and destination environments must differ', 'error')
    return
  }

  actionLoading.value = 'create'
  try {
    await promotionsApi.create(newPromotion.value)
    showToast(`Promotion request for ${newPromotion.value.service} created!`)
    showCreateModal.value = false
    newPromotion.value.service = ''
    newPromotion.value.version = ''
    await fetchPromotions()
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : 'Promotion request failed'
    showToast(msg, 'error')
  } finally {
    actionLoading.value = null
  }
}

async function handleApprove(p: Promotion) {
  actionLoading.value = p.id
  try {
    await promotionsApi.approve(p.id)
    showToast(`Promotion for ${p.service} approved! Ready for deployment.`)
    await fetchPromotions()
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : 'Approval failed'
    showToast(msg, 'error')
  } finally {
    actionLoading.value = null
  }
}

async function handleReject(p: Promotion) {
  actionLoading.value = p.id
  try {
    await promotionsApi.reject(p.id)
    showToast(`Promotion for ${p.service} rejected.`)
    await fetchPromotions()
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : 'Rejection failed'
    showToast(msg, 'error')
  } finally {
    actionLoading.value = null
  }
}

async function handleComplete(p: Promotion) {
  actionLoading.value = p.id
  try {
    await promotionsApi.complete(p.id)
    showToast(`Promotion for ${p.service} completed successfully in ${p.to_env}!`)
    await fetchPromotions()
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : 'Completion failed'
    showToast(msg, 'error')
  } finally {
    actionLoading.value = null
  }
}

function formatDate(d?: string) {
  if (!d) return '-'
  try {
    return new Date(d).toLocaleDateString([], { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' })
  } catch {
    return d
  }
}
</script>

<template>
  <div class="view-container animate-fade-in">
    <!-- Header -->
    <div class="view-header">
      <div>
        <div class="view-tag">
          <span class="pulse-dot pulse-dot-cyan"></span>
          <span>PROGRESSIVE DELIVERY & GOVERNANCE PIPELINE</span>
        </div>
        <h1 class="view-title">Multi-Stage Release Promotions</h1>
        <p class="view-desc">
          Automated gate approvals and progressive environment promotion pipeline across Dev ➔ QA ➔ Staging ➔ Production.
        </p>
      </div>

      <div class="header-actions">
        <button class="btn btn-secondary" :disabled="loading" @click="fetchPromotions">
          <span>{{ loading ? '⏳ Querying...' : '🔄 Refresh' }}</span>
        </button>
        <button class="btn btn-primary" @click="showCreateModal = true">
          <span>+ Request Promotion</span>
        </button>
      </div>
    </div>

    <!-- Notification Toast -->
    <div v-if="toastMessage" class="toast-banner animate-fade-in" :class="`toast-${toastMessage.type}`">
      <span>{{ toastMessage.type === 'success' ? '✅' : '⚠️' }}</span>
      <span>{{ toastMessage.text }}</span>
      <button class="toast-close" @click="toastMessage = null">✕</button>
    </div>

    <!-- Metric HUD -->
    <div class="metrics-grid">
      <MetricCard
        title="Pending Approvals"
        :value="pendingCount"
        subtitle="Promotion requests awaiting review"
        icon="⏳"
        badge="GATE"
        :badge-color="pendingCount > 0 ? 'amber' : 'emerald'"
        :trend="pendingCount > 0 ? 'Review Required' : 'All Clear'"
        :trend-type="pendingCount > 0 ? 'neutral' : 'positive'"
      />
      <MetricCard
        title="Active Promoting"
        :value="approvedCount"
        subtitle="Canary rollout & staging verification"
        icon="🚀"
        badge="ROLLOUT"
        badge-color="cyan"
        trend="In-Flight Verification"
        trend-type="positive"
      />
      <MetricCard
        title="Completed Releases"
        :value="completedCount"
        subtitle="Successfully promoted to destination"
        icon="✅"
        badge="SHIPPED"
        badge-color="emerald"
        trend="Continuous Delivery"
        trend-type="positive"
      />
      <MetricCard
        title="Rejected / Aborted"
        :value="rejectedCount"
        subtitle="Failed quality gates or security review"
        icon="🛑"
        badge="REJECTED"
        :badge-color="rejectedCount > 0 ? 'rose' : 'emerald'"
      />
    </div>

    <!-- Visual Pipeline Board (Dev ➔ QA ➔ Staging ➔ Prod) -->
    <div class="section-box glass-panel">
      <div class="box-header">
        <div>
          <h2 class="box-title">Visual Release Pipeline Board</h2>
          <p class="box-subtitle">Progressive gate stages with active release artifacts</p>
        </div>
      </div>

      <div class="pipeline-board">
        <div v-for="env in environments" :key="env" class="pipeline-column glass-panel">
          <div class="column-header">
            <div class="stage-badge" :class="`stage-${env}`">
              {{ env.toUpperCase() }}
            </div>
            <span class="stage-count font-mono">{{ getPromotionsForEnv(env).length }} Releases</span>
          </div>

          <div class="column-body">
            <div v-if="getPromotionsForEnv(env).length === 0" class="stage-empty">
              <span>No promotion requests found for {{ env }}. Click '+ Request Promotion' to submit a release.</span>
            </div>

            <div 
              v-for="p in getPromotionsForEnv(env)" 
              :key="p.id" 
              class="promotion-card glass-panel"
              :class="`p-status-${p.status}`"
            >
              <div class="p-card-top">
                <span class="p-service">{{ p.service }}</span>
                <StatusBadge :status="p.status" size="sm" />
              </div>

              <div class="p-version font-mono">
                <span>{{ p.from_env }} ➔ </span>
                <span class="text-cyan">{{ p.version }}</span>
              </div>

              <div class="p-meta font-mono text-muted">
                <span>By: {{ p.requester }}</span>
              </div>

              <!-- Quick Actions -->
              <div v-if="p.status === 'pending'" class="p-card-actions">
                <button class="btn btn-secondary btn-xs" @click="handleReject(p)">✕ Reject</button>
                <button class="btn btn-primary btn-xs" @click="handleApprove(p)">✓ Approve</button>
              </div>
              <div v-else-if="p.status === 'approved' || p.status === 'promoting'" class="p-card-actions">
                <button class="btn btn-primary btn-xs" @click="handleComplete(p)">Complete Rollout ➔</button>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Promotions History Data Table -->
    <div class="section-box glass-panel table-box">
      <div class="box-header" style="padding: 18px 22px; border-bottom: 1px solid var(--border-subtle);">
        <div>
          <h2 class="box-title">Promotion Request Audit Table</h2>
          <p class="box-subtitle">Full historical ledger of promotion approvals and audit trail</p>
        </div>
      </div>

      <DataTable
        :columns="columns"
        :data="promotions"
        :loading="loading"
        :error="error"
        empty-message="No promotion requests found. Click '+ Request Promotion' to submit a new release for environment gating."
        searchable
        search-placeholder="Search promotions by service, requester, or version..."
      >
        <template #cell-service="{ row }">
          <span class="font-mono text-cyan" style="font-weight: 700;">{{ row.service }}</span>
        </template>

        <template #cell-version="{ row }">
          <span class="font-mono text-emerald">{{ row.version }}</span>
        </template>

        <template #cell-from_env="{ row }">
          <span class="env-pill-tag font-mono">{{ row.from_env }}</span>
        </template>

        <template #cell-to_env="{ row }">
          <span class="env-pill-tag font-mono text-cyan">{{ row.to_env }}</span>
        </template>

        <template #cell-status="{ row }">
          <StatusBadge :status="row.status" size="sm" />
        </template>

        <template #cell-created_at="{ row }">
          <span class="font-mono text-muted">{{ formatDate(row.created_at) }}</span>
        </template>

        <template #cell-actions="{ row }">
          <div class="table-actions-row">
            <button 
              v-if="row.status === 'pending'"
              class="btn btn-secondary btn-xs" 
              :disabled="actionLoading === row.id"
              @click="handleReject(row)"
            >
              <span>Reject</span>
            </button>
            <button 
              v-if="row.status === 'pending'"
              class="btn btn-primary btn-xs" 
              :disabled="actionLoading === row.id"
              @click="handleApprove(row)"
            >
              <span>Approve</span>
            </button>
            <button 
              v-if="row.status === 'approved' || row.status === 'promoting'"
              class="btn btn-primary btn-xs" 
              :disabled="actionLoading === row.id"
              @click="handleComplete(row)"
            >
              <span>Complete</span>
            </button>
          </div>
        </template>
      </DataTable>
    </div>

    <!-- Request Promotion Modal -->
    <ModalDrawer
      v-model:show="showCreateModal"
      mode="modal"
      title="Request Environment Promotion"
      subtitle="Submit a workload version promotion request for gatekeeper review"
      max-width="560px"
    >
      <div class="modal-form">
        <div class="form-group">
          <label class="form-label">Service Name</label>
          <input v-model="newPromotion.service" type="text" placeholder="e.g. auth-service" class="input-glass" />
        </div>

        <div class="form-group">
          <label class="form-label">Target Release Version / Git SHA</label>
          <input v-model="newPromotion.version" type="text" placeholder="v2.4.1-rc1 or git-sha" class="input-glass font-mono" />
        </div>

        <div class="form-row">
          <div class="form-group flex-1">
            <label class="form-label">Source Stage</label>
            <select v-model="newPromotion.from_env" class="input-glass">
              <option value="dev">DEV (Development)</option>
              <option value="qa">QA (Automated Tests)</option>
              <option value="staging">STAGING (Pre-Prod)</option>
            </select>
          </div>

          <div class="form-group flex-1">
            <label class="form-label">Destination Stage</label>
            <select v-model="newPromotion.to_env" class="input-glass">
              <option value="qa">QA (Automated Tests)</option>
              <option value="staging">STAGING (Pre-Prod)</option>
              <option value="production">PRODUCTION (Live Users)</option>
            </select>
          </div>
        </div>

        <div class="form-group">
          <label class="form-label">Requester ID / Name</label>
          <input v-model="newPromotion.requester" type="text" class="input-glass font-mono" />
        </div>
      </div>

      <template #footer="{ close }">
        <button class="btn btn-secondary" @click="close">Cancel</button>
        <button class="btn btn-primary" :disabled="actionLoading === 'create'" @click="handleCreatePromotion">
          <span>{{ actionLoading === 'create' ? 'Submitting...' : 'Submit Promotion Request ➔' }}</span>
        </button>
      </template>
    </ModalDrawer>
  </div>
</template>

<style scoped>
.view-container {
  display: flex;
  flex-direction: column;
  gap: 24px;
}

.view-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 20px;
}

.view-tag {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  font-size: 11px;
  font-weight: 700;
  color: var(--accent-cyan);
  letter-spacing: 0.08em;
  margin-bottom: 6px;
}

.view-title {
  font-size: 24px;
  font-weight: 800;
  color: #fff;
  letter-spacing: -0.02em;
}

.view-desc {
  font-size: 13px;
  color: var(--text-secondary);
  max-width: 750px;
  margin-top: 4px;
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 10px;
}

.toast-banner {
  padding: 12px 16px;
  border-radius: 12px;
  display: flex;
  align-items: center;
  gap: 12px;
  font-size: 13px;
}

.toast-success {
  background: rgba(16, 185, 129, 0.15);
  border: 1px solid rgba(16, 185, 129, 0.3);
  color: #34d399;
}

.toast-error {
  background: rgba(244, 63, 94, 0.15);
  border: 1px solid rgba(244, 63, 94, 0.3);
  color: #fb7185;
}

.toast-close {
  margin-left: auto;
  background: none;
  border: none;
  color: inherit;
  cursor: pointer;
}

.metrics-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(240px, 1fr));
  gap: 16px;
}

.section-box {
  padding: 20px;
  border-radius: 16px;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.table-box {
  padding: 0;
  overflow: hidden;
}

.box-title {
  font-size: 15px;
  font-weight: 700;
  color: #fff;
}

.box-subtitle {
  font-size: 12px;
  color: var(--text-muted);
  margin-top: 2px;
}

.pipeline-board {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 14px;
}

@media (max-width: 1024px) {
  .pipeline-board {
    grid-template-columns: repeat(2, 1fr);
  }
}

@media (max-width: 640px) {
  .pipeline-board {
    grid-template-columns: 1fr;
  }
}

.pipeline-column {
  background: rgba(11, 15, 25, 0.5);
  border-radius: 14px;
  padding: 14px;
  display: flex;
  flex-direction: column;
  gap: 12px;
  min-height: 240px;
}

.column-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  border-bottom: 1px solid var(--border-subtle);
  padding-bottom: 10px;
}

.stage-badge {
  font-size: 11px;
  font-weight: 800;
  padding: 3px 8px;
  border-radius: 6px;
  letter-spacing: 0.05em;
}

.stage-dev { background: rgba(56, 189, 248, 0.15); color: #38bdf8; }
.stage-qa { background: rgba(139, 92, 246, 0.15); color: #a78bfa; }
.stage-staging { background: rgba(245, 158, 11, 0.15); color: #fbbf24; }
.stage-production { background: rgba(16, 185, 129, 0.15); color: #34d399; }

.stage-count {
  font-size: 11px;
  color: var(--text-muted);
}

.column-body {
  display: flex;
  flex-direction: column;
  gap: 10px;
  flex: 1;
}

.stage-empty {
  padding: 24px 8px;
  text-align: center;
  font-size: 11px;
  color: var(--text-muted);
}

.promotion-card {
  padding: 12px;
  border-radius: 10px;
  display: flex;
  flex-direction: column;
  gap: 8px;
  background: rgba(15, 23, 42, 0.7);
}

.p-card-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.p-service {
  font-size: 13px;
  font-weight: 700;
  color: #fff;
}

.p-version {
  font-size: 11px;
}

.p-meta {
  font-size: 10px;
}

.p-card-actions {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 6px;
  margin-top: 4px;
  border-top: 1px solid var(--border-subtle);
  padding-top: 6px;
}

.env-pill-tag {
  font-size: 11px;
  background: rgba(255, 255, 255, 0.05);
  padding: 2px 6px;
  border-radius: 4px;
}

.table-actions-row {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 6px;
}

.modal-form {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.form-group {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.form-row {
  display: flex;
  gap: 12px;
}

.flex-1 { flex: 1; }

.form-label {
  font-size: 11px;
  font-weight: 700;
  color: var(--text-muted);
  text-transform: uppercase;
}

.font-mono { font-family: var(--font-mono); }
.text-cyan { color: var(--accent-cyan); }
.text-emerald { color: var(--accent-emerald); }
.text-muted { color: var(--text-muted); }
.btn-xs { padding: 4px 8px; font-size: 11px; }
</style>
