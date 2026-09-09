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
@import '../../assets/styles/components/secret-viewer.css';
</style>
