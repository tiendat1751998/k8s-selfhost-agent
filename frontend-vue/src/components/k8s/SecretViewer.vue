<script setup lang="ts">
import { ref, computed } from 'vue'
import type { K8sResource } from '../../api/k8s'

const props = defineProps<{
  secret?: K8sResource | null
  data?: Record<string, string>
}>()

const revealedKeys = ref<Record<string, boolean>>({})
const copiedKey = ref<string | null>(null)
const revealAll = ref(false)

const secretData = computed<Record<string, string>>(() => {
  if (props.data) return props.data
  if (props.secret?.stringData) return props.secret.stringData
  if (props.secret?.data) return props.secret.data
  return {}
})

const entries = computed(() => Object.entries(secretData.value))

function decodeVal(val: string): string {
  if (!val) return ''
  if (props.secret?.stringData) return val
  try {
    return atob(val)
  } catch {
    return val
  }
}

function toggleReveal(key: string) {
  revealedKeys.value[key] = !revealedKeys.value[key]
}

function toggleRevealAll() {
  revealAll.value = !revealAll.value
  for (const [k] of entries.value) {
    revealedKeys.value[k] = revealAll.value
  }
}

async function copyValue(key: string, rawVal: string) {
  const decoded = decodeVal(rawVal)
  try {
    await navigator.clipboard.writeText(decoded)
    copiedKey.value = key
    setTimeout(() => {
      if (copiedKey.value === key) copiedKey.value = null
    }, 2000)
  } catch {
    // fallback
  }
}
</script>

<template>
  <div class="secret-viewer">
    <div class="secret-warning-banner">
      <span class="warning-icon">??</span>
      <div class="warning-text">
        <strong>Sensitive data ? do not share</strong>
        <span>Credentials, tokens, and private keys are masked by default to prevent unauthorized viewing.</span>
      </div>
      <button 
        v-if="entries.length > 0"
        type="button" 
        class="btn btn-secondary btn-xs reveal-all-btn"
        @click="toggleRevealAll"
      >
        <span>{{ revealAll ? '?? Hide All' : '??? Reveal All' }}</span>
      </button>
    </div>

    <div v-if="entries.length === 0" class="empty-secret">
      <span>No data keys found in this Secret.</span>
    </div>

    <div v-else class="secret-table-wrapper glass-panel">
      <table class="secret-table">
        <thead>
          <tr>
            <th style="width: 35%;">Key Name</th>
            <th>Value</th>
            <th style="width: 140px; text-align: right;">Actions</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="[key, rawVal] in entries" :key="key" class="secret-row">
            <td class="key-cell font-mono">
              <span class="key-badge">{{ key }}</span>
            </td>
            <td class="value-cell font-mono">
              <div class="value-display">
                <span v-if="revealedKeys[key] || revealAll" class="revealed-text select-all">
                  {{ decodeVal(rawVal) }}
                </span>
                <span v-else class="masked-text">
                  ????????????????????
                </span>
              </div>
            </td>
            <td class="actions-cell">
              <div class="actions-group">
                <button 
                  type="button"
                  class="btn-action"
                  :title="revealedKeys[key] || revealAll ? 'Hide value' : 'Reveal value'"
                  @click="toggleReveal(key)"
                >
                  <span>{{ revealedKeys[key] || revealAll ? '??' : '???' }}</span>
                </button>
                <button 
                  type="button"
                  class="btn-action"
                  :class="{ 'btn-copied': copiedKey === key }"
                  :title="copiedKey === key ? 'Copied to clipboard!' : 'Copy decoded value'"
                  @click="copyValue(key, rawVal)"
                >
                  <span>{{ copiedKey === key ? '?' : '??' }}</span>
                </button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<style scoped>
.secret-viewer {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.secret-warning-banner {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 16px;
  background: rgba(245, 158, 11, 0.12);
  border: 1px solid rgba(245, 158, 11, 0.3);
  border-radius: 12px;
  color: #fbbf24;
  font-size: 12px;
}

.warning-icon {
  font-size: 18px;
  flex-shrink: 0;
}

.warning-text {
  display: flex;
  flex-direction: column;
  gap: 2px;
  flex: 1;
}

.warning-text strong {
  color: #fde68a;
}

.reveal-all-btn {
  white-space: nowrap;
  flex-shrink: 0;
}

.empty-secret {
  padding: 24px;
  text-align: center;
  color: var(--text-muted);
  font-size: 13px;
  border: 1px dashed var(--border-subtle);
  border-radius: 12px;
}

.secret-table-wrapper {
  overflow: hidden;
  border: 1px solid var(--border-subtle);
  border-radius: 12px;
}

.secret-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 13px;
}

.secret-table th {
  background: rgba(15, 23, 42, 0.8);
  padding: 10px 16px;
  color: var(--text-secondary);
  font-size: 11px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  border-bottom: 1px solid var(--border-subtle);
  text-align: left;
}

.secret-row td {
  padding: 12px 16px;
  border-bottom: 1px solid var(--border-subtle);
  vertical-align: middle;
}

.secret-row:last-child td {
  border-bottom: none;
}

.key-cell {
  color: var(--accent-cyan);
  font-weight: 600;
}

.key-badge {
  display: inline-block;
  padding: 2px 8px;
  background: rgba(6, 182, 212, 0.1);
  border: 1px solid rgba(6, 182, 212, 0.2);
  border-radius: 6px;
  word-break: break-all;
}

.value-cell {
  word-break: break-all;
}

.value-display {
  display: flex;
  align-items: center;
  min-height: 24px;
}

.masked-text {
  color: var(--text-muted);
  letter-spacing: 0.15em;
  user-select: none;
}

.revealed-text {
  color: #38bdf8;
  background: rgba(15, 23, 42, 0.7);
  padding: 4px 8px;
  border-radius: 6px;
  border: 1px solid var(--border-subtle);
  font-size: 12px;
  max-height: 120px;
  overflow-y: auto;
  white-space: pre-wrap;
}

.actions-cell {
  text-align: right;
}

.actions-group {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.btn-action {
  width: 28px;
  height: 28px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  background: rgba(255, 255, 255, 0.06);
  border: 1px solid var(--border-subtle);
  border-radius: 6px;
  color: var(--text-primary);
  cursor: pointer;
  font-size: 13px;
  transition: all 0.15s ease;
}

.btn-action:hover {
  background: rgba(56, 189, 248, 0.15);
  border-color: var(--accent-sky);
}

.btn-copied {
  background: rgba(16, 185, 129, 0.2);
  border-color: rgba(16, 185, 129, 0.4);
}

.font-mono {
  font-family: var(--font-mono);
}

.btn-xs {
  padding: 4px 10px;
  font-size: 11px;
}
</style>
