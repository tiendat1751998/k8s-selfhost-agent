<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { apiKeysApi, type ApiKeyItem } from '../../api/settings'
import BaseIcon from '../ui/BaseIcon.vue'

const props = withDefaults(defineProps<{
  saving?: boolean
  keys?: ApiKeyItem[]
}>(), {
  saving: false,
})

const emit = defineEmits<{
  (e: 'save', category: 'apikeys'): void
  (e: 'update:keys', keys: ApiKeyItem[]): void
  (e: 'refresh'): void
}>()

const localKeys = ref<ApiKeyItem[]>([])
const activeKeys = computed<ApiKeyItem[]>({
  get() {
    return props.keys !== undefined ? props.keys : localKeys.value
  },
  set(val: ApiKeyItem[]) {
    if (props.keys !== undefined) {
      emit('update:keys', val)
    } else {
      localKeys.value = val
    }
  }
})

const newKeyName = ref('')
const newKeyRole = ref('developer')
const showCreateForm = ref(false)
const copiedId = ref<string | null>(null)
const loadingKeys = ref(false)
const isCreating = ref(false)
const createError = ref<string | null>(null)
const actionError = ref<string | null>(null)
const revokingId = ref<string | null>(null)

async function loadKeys() {
  loadingKeys.value = true
  actionError.value = null
  try {
    const res = await apiKeysApi.list()
    activeKeys.value = Array.isArray(res) ? res : []
  } catch (err: unknown) {
    activeKeys.value = []
    actionError.value = err instanceof Error ? err.message : 'Failed to retrieve API keys'
  } finally {
    loadingKeys.value = false
  }
}

onMounted(() => {
  if (props.keys === undefined || props.keys.length === 0) {
    loadKeys()
  }
})

const newlyCreatedKey = ref<{ name: string; token: string } | null>(null)
const copiedNewKey = ref(false)

async function copyNewKey() {
  if (!newlyCreatedKey.value) return
  await navigator.clipboard.writeText(newlyCreatedKey.value.token)
  copiedNewKey.value = true
  setTimeout(() => { copiedNewKey.value = false }, 2000)
}

async function handleCreateKey() {
  if (!newKeyName.value.trim()) return
  isCreating.value = true
  createError.value = null
  actionError.value = null
  try {
    const created = await apiKeysApi.create({
      name: newKeyName.value.trim(),
      role: newKeyRole.value,
    })
    if (created && created.id) {
      activeKeys.value = [...activeKeys.value, created]
      if (created.token) {
        newlyCreatedKey.value = { name: created.name, token: created.token }
      }
      newKeyName.value = ''
      showCreateForm.value = false
      emit('save', 'apikeys')
    } else {
      throw new Error('Key generation failed: server returned invalid response')
    }
  } catch (err: unknown) {
    createError.value = err instanceof Error ? err.message : 'Failed to generate API key. Server error.'
  } finally {
    isCreating.value = false
  }
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

async function handleRevokeKey(id: string) {
  revokingId.value = id
  actionError.value = null
  try {
    await apiKeysApi.revoke(id)
    activeKeys.value = activeKeys.value.filter(k => k.id !== id)
    emit('save', 'apikeys')
  } catch (err: unknown) {
    actionError.value = err instanceof Error ? err.message : 'Failed to revoke API key. Request failed.'
  } finally {
    revokingId.value = null
  }
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
        <span>{{ showCreateForm ? 'Cancel' : '+ Generate API Key' }}</span>
      </button>
    </div>

    <!-- Top Action Error Banner -->
    <div
      v-if="actionError"
      class="alert-banner alert-banner-danger animate-fade-in"
      style="margin-bottom: 16px; padding: 10px 14px; border-radius: 8px; display: flex; align-items: center; justify-content: space-between; background: rgba(225, 29, 72, 0.15); border: 1px solid rgba(225, 29, 72, 0.3); color: #f43f5e;"
    >
      <div style="display: flex; align-items: center; gap: 8px; font-size: 13px;">
        <BaseIcon name="alert-triangle" size="xs" />
        <span>{{ actionError }}</span>
      </div>
      <button type="button" class="btn-icon-cmd" style="background: none; border: none; color: inherit; cursor: pointer;" aria-label="Dismiss error" @click="actionError = null">
        <BaseIcon name="x" size="xs" />
      </button>
    </div>

    <!-- Generate API Key Form -->
    <div v-if="showCreateForm" class="create-key-panel glass-panel animate-fade-in" style="padding: 16px; margin-bottom: 20px; border-radius: 12px; background: var(--color-surface-inset); border: 1px solid var(--color-border-subtle);">
      <h3 class="subsection-title" style="margin-bottom: 12px;">Generate New Machine-to-Machine Token</h3>
      <div v-if="createError" class="alert-banner alert-banner-danger" style="margin-bottom: 12px; padding: 8px 12px; border-radius: 6px; font-size: 13px; background: rgba(225, 29, 72, 0.15); border: 1px solid rgba(225, 29, 72, 0.3); color: #f43f5e;">
        {{ createError }}
      </div>
      <form class="settings-form" @submit.prevent="handleCreateKey">
        <div class="form-row" style="align-items: flex-end; gap: 16px;">
          <div class="form-group" style="max-width: 360px; flex: 1;">
            <label class="form-label" for="api-key-name">Key Name / Description</label>
            <input id="api-key-name" v-model="newKeyName" type="text" required class="input-glass form-input" style="max-width: 360px;" placeholder="e.g. Jenkins Staging Runner" :disabled="isCreating || saving" />
          </div>
          <div class="form-group" style="max-width: 200px; flex: 1;">
            <label class="form-label" for="api-key-role">Permission Scope</label>
            <select id="api-key-role" v-model="newKeyRole" class="input-glass form-select" style="max-width: 200px;" :disabled="isCreating || saving">
              <option value="developer">Operator (Read/Write)</option>
              <option value="admin">Admin (Full Access)</option>
              <option value="readonly">Auditor (Read-Only)</option>
            </select>
          </div>
          <div class="form-group" style="flex: 0 0 auto;">
            <button type="submit" class="btn btn-primary btn-sm" style="min-height: 40px;" :disabled="saving || isCreating">
              <span>{{ isCreating ? 'Generating...' : (saving ? 'Saving...' : 'Confirm Key Creation') }}</span>
            </button>
          </div>
        </div>
      </form>
    </div>

    <!-- Prominent New API Key Callout (Token Only Visible Once) -->
    <div
      v-if="newlyCreatedKey"
      class="new-key-callout glass-panel animate-fade-in"
      style="margin-bottom: 16px; padding: 12px 16px; border-radius: 8px; background: rgba(16, 185, 129, 0.1); border: 1px solid rgba(16, 185, 129, 0.35);"
    >
      <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 6px;">
        <span style="font-weight: 700; font-size: 13px; color: var(--color-status-online);">API Key Generated: {{ newlyCreatedKey.name }}</span>
        <button type="button" class="btn-action-compact" @click="newlyCreatedKey = null">
          <BaseIcon name="x" size="xs" /> <span>Dismiss</span>
        </button>
      </div>
      <p style="margin: 0 0 8px 0; font-size: 11.5px; color: var(--text-muted);">
        Copy this API key now — only its SHA-256 hash is stored and it cannot be viewed again.
      </p>
      <div style="display: flex; align-items: center; gap: 8px;">
        <code class="font-mono" style="flex: 1; padding: 6px 10px; border-radius: 6px; background: var(--color-surface-inset); border: 1px solid var(--color-border-subtle); color: #38bdf8; font-size: 12px; word-break: break-all;">
          {{ newlyCreatedKey.token }}
        </code>
        <button type="button" class="btn btn-primary btn-xs" @click="copyNewKey">
          <BaseIcon :name="copiedNewKey ? 'check' : 'copy'" size="xs" />
          <span>{{ copiedNewKey ? 'Copied!' : 'Copy Key' }}</span>
        </button>
      </div>
    </div>

    <!-- API Keys Data Table (Fixed 100% Column Widths & Zero Horizontal Overflow) -->
    <div class="keys-list-wrapper">
      <div v-if="activeKeys.length > 0" class="keys-table-container">
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
            <tr v-for="k in activeKeys" :key="k.id">
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
                  <span>Created: <strong style="color: var(--text-primary);">{{ k.created_at }}</strong></span>
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
                    <BaseIcon :name="copiedId === k.id ? 'check' : 'copy'" size="xs" /> <span>{{ copiedId === k.id ? 'Copied' : 'Copy' }}</span>
                  </button>
                  <button
                    type="button"
                    class="btn-action-compact btn-action-danger"
                    :disabled="revokingId === k.id"
                    title="Revoke API Key"
                    :aria-label="'Revoke ' + k.name"
                    @click="handleRevokeKey(k.id)"
                  >
                    <BaseIcon name="trash" size="xs" /> <span>{{ revokingId === k.id ? 'Revoking...' : 'Revoke' }}</span>
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
