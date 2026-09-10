<script setup lang="ts">
import { ref } from 'vue'

interface ApiKeyItem {
  id: string
  name: string
  prefix: string
  role: string
  created_at: string
  last_used: string
}

defineProps<{
  saving: boolean
}>()

const emit = defineEmits<{
  (e: 'save', category: 'apikeys'): void
}>()

const keys = ref<ApiKeyItem[]>([
  {
    id: 'key-1',
    name: 'CI/CD GitHub Actions Deployer',
    prefix: 'k8s_live_8f3a...',
    role: 'Cluster Deployer (Write)',
    created_at: '2026-08-15',
    last_used: '2 hours ago',
  },
  {
    id: 'key-2',
    name: 'Prometheus Remote Write Ingest',
    prefix: 'k8s_live_bc91...',
    role: 'Telemetry Ingest (Write-Only)',
    created_at: '2026-08-20',
    last_used: '1 minute ago',
  },
  {
    id: 'key-3',
    name: 'ArgoCD Controller Webhook Key',
    prefix: 'k8s_live_411d...',
    role: 'GitOps Sync (Admin)',
    created_at: '2026-08-28',
    last_used: '5 minutes ago',
  },
])

const newKeyName = ref('')
const newKeyRole = ref('developer')
const showCreateForm = ref(false)
const copiedId = ref<string | null>(null)

function handleCreateKey() {
  if (!newKeyName.value.trim()) return
  const id = `key-${Date.now()}`
  const roleMap: Record<string, string> = {
    admin: 'Cluster Admin (Full Access)',
    readonly: 'Auditor (Read-Only)',
    developer: 'Cluster Operator (Read/Write)',
  }
  keys.value.push({
    id,
    name: newKeyName.value.trim(),
    prefix: `k8s_live_${Math.random().toString(36).substring(2, 6)}...`,
    role: roleMap[newKeyRole.value] || 'Cluster Operator (Read/Write)',
    created_at: new Date().toISOString().split('T')[0],
    last_used: 'Just now',
  })
  newKeyName.value = ''
  showCreateForm.value = false
  emit('save', 'apikeys')
}

async function handleCopyKey(prefix: string, id: string) {
  try {
    await navigator.clipboard.writeText(prefix)
    copiedId.value = id
    setTimeout(() => {
      if (copiedId.value === id) copiedId.value = null
    }, 2000)
  } catch {
    copiedId.value = id
    setTimeout(() => {
      if (copiedId.value === id) copiedId.value = null
    }, 2000)
  }
}

function handleRevokeKey(id: string) {
  keys.value = keys.value.filter(k => k.id !== id)
  emit('save', 'apikeys')
}
</script>

<template>
  <div class="settings-card glass-panel animate-fade-in">
    <div class="card-header">
      <div>
        <h2 class="card-title">API Keys & Automation Tokens</h2>
        <p class="card-subtitle">
          Manage bearer tokens and service accounts for automated CI/CD pipelines and external integrations.
        </p>
      </div>
      <button
        type="button"
        class="btn btn-secondary btn-sm"
        @click="showCreateForm = !showCreateForm"
      >
        <span>{{ showCreateForm ? '✕ Cancel' : '+ Generate API Key' }}</span>
      </button>
    </div>

    <!-- Generate API Key Form -->
    <div
      v-if="showCreateForm"
      class="create-key-panel glass-panel animate-fade-in"
      style="padding: 16px; margin-bottom: 20px; border-radius: 12px; background: rgba(0,0,0,0.25);"
    >
      <h3 class="subsection-title" style="margin-bottom: 12px;">Generate New Machine-to-Machine Token</h3>
      <form class="settings-form" @submit.prevent="handleCreateKey">
        <div class="form-row">
          <div class="form-group flex-2">
            <label class="form-label" for="api-key-name">Key Name / Description</label>
            <input
              id="api-key-name"
              v-model="newKeyName"
              type="text"
              required
              class="input-glass form-input"
              placeholder="e.g. Jenkins Staging Runner"
            />
          </div>
          <div class="form-group flex-1">
            <label class="form-label" for="api-key-role">Permission Scope</label>
            <select id="api-key-role" v-model="newKeyRole" class="input-glass form-select">
              <option value="developer">Cluster Operator (Read/Write)</option>
              <option value="admin">Cluster Admin (Full Access)</option>
              <option value="readonly">Auditor (Read-Only)</option>
            </select>
          </div>
        </div>
        <div class="form-actions" style="margin-top: 8px;">
          <button type="submit" class="btn btn-primary btn-sm" :disabled="saving">
            <span>{{ saving ? 'Generating...' : 'Confirm Key Creation' }}</span>
          </button>
        </div>
      </form>
    </div>

    <!-- API Keys Data Table (Fixed 100% Column Widths & Zero Horizontal Overflow) -->
    <div class="keys-list-wrapper">
      <div v-if="keys.length > 0" class="keys-table-container">
        <table class="keys-table">
          <colgroup>
            <col style="width: 32%;">
            <col style="width: 26%;">
            <col style="width: 24%;">
            <col style="width: 18%;">
          </colgroup>
          <thead>
            <tr>
              <th scope="col">Token / Name</th>
              <th scope="col">Scope / Role</th>
              <th scope="col">Activity & Created</th>
              <th scope="col" style="text-align: right;">Actions</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="k in keys" :key="k.id">
              <td>
                <div class="key-cell-primary">
                  <span class="key-name-text">{{ k.name }}</span>
                  <span class="badge badge-cyan font-mono">{{ k.prefix }}</span>
                </div>
              </td>
              <td>
                <span class="badge badge-muted">{{ k.role }}</span>
              </td>
              <td>
                <div style="display: flex; flex-direction: column; gap: 2px; font-size: 11.5px; color: var(--text-muted);">
                  <span>Created: <strong class="text-white">{{ k.created_at }}</strong></span>
                  <span>Used: {{ k.last_used }}</span>
                </div>
              </td>
              <td>
                <div class="key-actions-cell">
                  <button
                    type="button"
                    class="btn-action-compact"
                    :title="copiedId === k.id ? 'Copied to clipboard!' : 'Copy Key Prefix'"
                    :aria-label="'Copy ' + k.name"
                    @click="handleCopyKey(k.prefix, k.id)"
                  >
                    <span>{{ copiedId === k.id ? '✅ Copied' : '📋 Copy' }}</span>
                  </button>
                  <button
                    type="button"
                    class="btn-action-compact btn-action-danger"
                    title="Revoke API Key"
                    :aria-label="'Revoke ' + k.name"
                    @click="handleRevokeKey(k.id)"
                  >
                    <span>🗑️ Revoke</span>
                  </button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <div v-else class="empty-state-box glass-panel" style="padding: 30px; text-align: center;">
        <p class="empty-desc">No active API keys found. Generate a key for external automation.</p>
      </div>
    </div>
  </div>
</template>
