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
@import '../../assets/styles/components/helm-drawers.css';
</style>
