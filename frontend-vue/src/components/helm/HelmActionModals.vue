<script setup lang="ts">
import ModalDrawer from '../ui/ModalDrawer.vue'
import type { HelmRelease, HelmRepo, HelmRevisionHistory, UpgradeReleaseRequest } from '../../api/helm'

defineProps<{
  showUpgradeModal: boolean
  upgradeTarget: HelmRelease | null
  upgradeForm: UpgradeReleaseRequest
  upgrading: boolean
  showRollbackModal: boolean
  rollbackTarget: HelmRelease | null
  rollbackRevision: number | null
  rollbackHistoryList: HelmRevisionHistory[]
  loadingRollbackHistory: boolean
  rollingBack: boolean
  showUninstallModal: boolean
  uninstallTarget: HelmRelease | null
  uninstalling: boolean
  showRemoveRepoModal: boolean
  repoToRemove: HelmRepo | null
  removingRepo: boolean
}>()

const emit = defineEmits<{
  (e: 'update:showUpgradeModal', val: boolean): void
  (e: 'update:showRollbackModal', val: boolean): void
  (e: 'update:showUninstallModal', val: boolean): void
  (e: 'update:showRemoveRepoModal', val: boolean): void
  (e: 'update:rollbackRevision', val: number): void
  (e: 'upgrade'): void
  (e: 'rollback'): void
  (e: 'uninstall'): void
  (e: 'removeRepo'): void
}>()

function formatReleaseDate(dateStr?: string): string {
  if (!dateStr) return '—'
  try {
    const d = new Date(dateStr)
    return isNaN(d.getTime()) ? dateStr : d.toLocaleDateString()
  } catch {
    return dateStr
  }
}
</script>

<template>
  <div class="helm-action-modals">
    <!-- Upgrade Modal -->
    <ModalDrawer
      :show="showUpgradeModal"
      mode="modal"
      max-width="680px"
      :title="upgradeTarget ? `Upgrade Release: ${upgradeTarget.name}` : 'Upgrade Release'"
      :subtitle="upgradeTarget ? `Namespace: ${upgradeTarget.namespace} • Current: ${upgradeTarget.chart}` : ''"
      @close="emit('update:showUpgradeModal', false)"
    >
      <div v-if="upgradeTarget" class="modal-form-body">
        <div class="form-grid">
          <div class="form-group">
            <label class="form-label">Target Version</label>
            <input v-model="upgradeForm.version" type="text" class="input-glass" placeholder="e.g. 1.3.0" />
          </div>
          <div class="form-group checkbox-group">
            <label class="cyber-checkbox-label">
              <input v-model="upgradeForm.reuseValues" type="checkbox" class="cyber-checkbox" />
              <span>Reuse existing release values</span>
            </label>
          </div>
          <div class="form-group full-width">
            <label class="form-label">Custom Values Override (YAML)</label>
            <textarea v-model="upgradeForm.values" class="cyber-textarea" rows="8" placeholder="# Optional YAML values..."></textarea>
          </div>
        </div>
        <div class="modal-footer-actions">
          <button type="button" class="btn-cyber btn-secondary" @click="emit('update:showUpgradeModal', false)">Cancel</button>
          <button type="button" class="btn-cyber btn-primary" :disabled="upgrading" @click="emit('upgrade')">
            <span :class="{ 'spin-anim': upgrading }">🔄</span>
            <span>{{ upgrading ? 'Upgrading Release...' : 'Deploy Upgrade' }}</span>
          </button>
        </div>
      </div>
    </ModalDrawer>

    <!-- Rollback Modal -->
    <ModalDrawer
      :show="showRollbackModal"
      mode="modal"
      max-width="580px"
      :title="rollbackTarget ? `Rollback Release: ${rollbackTarget.name}` : 'Rollback Release'"
      :subtitle="rollbackTarget ? `Current Revision: #${rollbackTarget.revision || 1}` : ''"
      @close="emit('update:showRollbackModal', false)"
    >
      <div v-if="rollbackTarget" class="modal-form-body">
        <div v-if="loadingRollbackHistory" class="loading-state">
          <div class="cyber-spinner"></div>
          <p class="font-mono text-muted">Fetching revision history...</p>
        </div>
        <div v-else class="form-grid">
          <div class="form-group full-width">
            <label class="form-label">Select Revision to Restore <span class="text-rose">*</span></label>
            <select :value="rollbackRevision" class="input-glass" @change="emit('update:rollbackRevision', Number(($event.target as HTMLSelectElement).value))">
              <option v-for="h in rollbackHistoryList" :key="h.revision" :value="h.revision">
                Revision #{{ h.revision }} — {{ h.chart }} ({{ h.status }}) [{{ formatReleaseDate(h.updated) }}]
              </option>
            </select>
          </div>
          <div class="alert-box alert-warning full-width">
            <span>⚠️ Rollback will revert cluster resources to Revision #{{ rollbackRevision || '—' }}.</span>
          </div>
        </div>
        <div class="modal-footer-actions">
          <button type="button" class="btn-cyber btn-secondary" @click="emit('update:showRollbackModal', false)">Cancel</button>
          <button type="button" class="btn-cyber btn-warning" :disabled="rollingBack || rollbackRevision === null" @click="emit('rollback')">
            <span :class="{ 'spin-anim': rollingBack }">↩️</span>
            <span>{{ rollingBack ? 'Rolling back...' : 'Confirm Rollback' }}</span>
          </button>
        </div>
      </div>
    </ModalDrawer>

    <!-- Uninstall Modal -->
    <ModalDrawer
      :show="showUninstallModal"
      mode="modal"
      max-width="520px"
      title="Confirm Release Uninstall"
      subtitle="Danger Zone: Resource Removal"
      @close="emit('update:showUninstallModal', false)"
    >
      <div v-if="uninstallTarget" class="modal-form-body">
        <div class="danger-warning-box">
          <span class="warning-icon">⚠️</span>
          <p>
            You are about to uninstall <strong class="text-rose font-mono">{{ uninstallTarget.name }}</strong> in namespace <strong class="text-cyan font-mono">{{ uninstallTarget.namespace }}</strong>. All Kubernetes workloads and resources managed by this Helm release will be deleted.
          </p>
        </div>
        <div class="modal-footer-actions">
          <button type="button" class="btn-cyber btn-secondary" @click="emit('update:showUninstallModal', false)">Cancel</button>
          <button type="button" class="btn-cyber btn-danger" :disabled="uninstalling" @click="emit('uninstall')">
            <span>{{ uninstalling ? 'Uninstalling...' : 'Uninstall Release' }}</span>
          </button>
        </div>
      </div>
    </ModalDrawer>

    <!-- Remove Repo Modal -->
    <ModalDrawer
      :show="showRemoveRepoModal"
      mode="modal"
      max-width="480px"
      title="Remove Helm Repository"
      subtitle="Unregister chart repository"
      @close="emit('update:showRemoveRepoModal', false)"
    >
      <div v-if="repoToRemove" class="modal-form-body">
        <p class="text-body font-mono">
          Are you sure you want to remove repository <strong class="text-rose">{{ repoToRemove.name }}</strong> ({{ repoToRemove.url }})?
        </p>
        <div class="modal-footer-actions">
          <button type="button" class="btn-cyber btn-secondary" @click="emit('update:showRemoveRepoModal', false)">Cancel</button>
          <button type="button" class="btn-cyber btn-danger" :disabled="removingRepo" @click="emit('removeRepo')">
            <span>{{ removingRepo ? 'Removing...' : 'Remove Repository' }}</span>
          </button>
        </div>
      </div>
    </ModalDrawer>
  </div>
</template>

<style scoped>
.modal-form-body { display: flex; flex-direction: column; gap: 16px; }
.form-grid { display: grid; grid-template-columns: repeat(2, 1fr); gap: 14px; }
.form-group { display: flex; flex-direction: column; gap: 6px; }
.form-group.full-width { grid-column: span 2; }
.form-label { font-size: 12px; font-weight: 600; color: var(--text-secondary); }
.cyber-textarea { width: 100%; padding: 10px 12px; background: #090c10; border: 1px solid rgba(255, 255, 255, 0.1); border-radius: 8px; color: var(--text-primary); font-family: var(--font-mono); font-size: 12px; line-height: 1.45; resize: vertical; }
.checkbox-group { grid-column: span 2; }
.cyber-checkbox-label { display: flex; align-items: center; gap: 8px; font-size: 12px; color: var(--text-secondary); cursor: pointer; }
.cyber-checkbox { accent-color: var(--color-primary); }
.danger-warning-box { padding: 14px; border-radius: 8px; background: rgba(244, 63, 94, 0.1); border: 1px solid rgba(244, 63, 94, 0.3); color: #fb7185; font-size: 13px; line-height: 1.5; }
.alert-box { padding: 10px 14px; border-radius: 8px; font-size: 12px; }
.alert-warning { background: rgba(245, 158, 11, 0.1); border: 1px solid rgba(245, 158, 11, 0.3); color: #fbbf24; }
.modal-footer-actions { display: flex; justify-content: flex-end; align-items: center; gap: 10px; margin-top: 10px; padding-top: 14px; border-top: 1px solid rgba(255, 255, 255, 0.08); }
.loading-state { display: flex; flex-direction: column; align-items: center; justify-content: center; padding: 32px 16px; text-align: center; gap: 10px; }
.cyber-spinner { width: 28px; height: 28px; border: 3px solid rgba(252, 213, 53, 0.15); border-top-color: var(--color-primary); border-radius: 50%; animation: spin 0.8s linear infinite; }
.spin-anim { display: inline-block; animation: spin 1s linear infinite; }
@keyframes spin { from { transform: rotate(0deg); } to { transform: rotate(360deg); } }
@media (max-width: 640px) { .form-grid { grid-template-columns: 1fr; } .form-group.full-width { grid-column: span 1; } }
</style>
