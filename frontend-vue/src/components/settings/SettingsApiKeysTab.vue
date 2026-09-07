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

function handleCreateKey() {
  if (!newKeyName.value.trim()) return
  const id = `key-${Date.now()}`
  const rand = Math.random().toString(36).substring(2, 6)
  keys.value.unshift({
    id,
    name: newKeyName.value.trim(),
    prefix: `k8s_live_${rand}...`,
    role: newKeyRole.value === 'admin' ? 'Cluster Admin' : 'Cluster Operator',
    created_at: new Date().toISOString().split('T')[0],
    last_used: 'Just now',
  })
  newKeyName.value = ''
  showCreateForm.value = false
}

function handleRevokeKey(id: string) {
  keys.value = keys.value.filter(k => k.id !== id)
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
        class="btn btn-primary btn-sm"
        @click="showCreateForm = !showCreateForm"
      >
        <span>{{ showCreateForm ? '✕ Close Form' : '+ Generate API Key' }}</span>
      </button>
    </div>

    <!-- Generate API Key Form -->
    <div v-if="showCreateForm" class="create-key-panel glass-panel animate-fade-in" style="padding: 16px; margin-bottom: 20px; border-radius: 12px; background: rgba(0,0,0,0.25);">
      <h3 class="subsection-title" style="margin-bottom: 12px;">Generate New Machine-to-Machine Token</h3>
      <form class="settings-form" @submit.prevent="handleCreateKey">
        <div class="form-row">
          <div class="form-group flex-2">
            <label class="form-label" for="api-key-name">Key Name / Description</label>
            <input
              id="api-key-name" v-model="newKeyName"
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
          <button type="submit" class="btn btn-primary btn-sm" :disabled="!newKeyName.trim()">
            <span>🔑 Generate Token</span>
          </button>
        </div>
      </form>
    </div>

    <!-- API Keys List -->
    <div class="keys-list-wrapper">
      <div v-if="keys.length > 0" class="keys-table-container">
        <div
          v-for="k in keys"
          :key="k.id"
          class="key-item-row glass-panel"
          style="display: flex; justify-content: space-between; align-items: center; padding: 14px 18px; margin-bottom: 10px; border-radius: 10px;"
        >
          <div class="key-info" style="display: flex; flex-direction: column; gap: 4px;">
            <div style="display: flex; align-items: center; gap: 10px;">
              <span class="font-bold text-white">{{ k.name }}</span>
              <span class="badge badge-cyan font-mono" style="font-size: 11px;">{{ k.prefix }}</span>
            </div>
            <div style="font-size: 12px; color: var(--text-muted); display: flex; gap: 12px;">
              <span>Role: <strong class="text-white">{{ k.role }}</strong></span>
              <span>•</span>
              <span>Created: {{ k.created_at }}</span>
              <span>•</span>
              <span>Last Used: {{ k.last_used }}</span>
            </div>
          </div>
          <button
            type="button"
            class="btn btn-secondary btn-sm btn-danger-outline"
            @click="handleRevokeKey(k.id)"
          >
            <span>Revoke</span>
          </button>
        </div>
      </div>
      <div v-else class="empty-state-box glass-panel" style="padding: 30px; text-align: center;">
        <p class="empty-desc">No active API keys found. Generate a key for external automation.</p>
      </div>
    </div>
  </div>
</template>
