<template>
  <div class="settings-card glass-panel sre-policy-card">
    <div class="card-header">
      <div>
        <h2 class="card-title">⚡ SRE Auto-Remediation & Fast-Failover Settings</h2>
        <p class="card-subtitle">Autonomous controller policies for rapid node failure detection and pod eviction.</p>
      </div>
      <button type="button" class="btn btn-secondary btn-sm" @click="resetDefaults"><span>↺ Reset Defaults</span></button>
    </div>

    <!-- Toast Notification -->
    <transition name="fade">
      <div v-if="toast" class="toast-banner" :class="'banner-' + toast.type">
        <span>{{ toast.type === 'success' ? '✅' : '⚠️' }} {{ toast.text }}</span>
        <button class="toast-close" @click="toast = null">✕</button>
      </div>
    </transition>

    <form class="sre-form" @submit.prevent="savePolicy">
      <!-- 1. Automated Fast-Failover (< 30s) -->
      <div class="toggle-group">
        <label class="toggle-label" for="toggle-fast-failover">
          <input id="toggle-fast-failover" v-model="policy.automatedFastFailover" type="checkbox" class="checkbox-custom" />
          <div class="toggle-info">
            <span class="toggle-title">Automated Fast-Failover (&lt; 30s)</span>
            <span class="toggle-desc">Trigger autonomous failover workflows when a node ceases health telemetry.</span>
          </div>
        </label>
      </div>

      <!-- 2. Heartbeat Timeout Slider (5s - 30s, default 15s) -->
      <div class="form-group">
        <div class="slider-header">
          <label class="form-label" for="slider-heartbeat"><span>Node Heartbeat Timeout</span></label>
          <span class="slider-val font-mono">{{ policy.heartbeatTimeout }}s</span>
        </div>
        <p class="field-desc">Maximum acceptable delay before marking a silent node as NotReady.</p>
        <input id="slider-heartbeat" v-model.number="policy.heartbeatTimeout" type="range" min="5" max="30" step="1" class="slider-input" />
        <div class="slider-labels font-mono"><span>5s (Aggressive)</span><span>15s (Recommended)</span><span>30s (Conservative)</span></div>
      </div>

      <!-- 3. Auto-Cordon Unhealthy Nodes -->
      <div class="toggle-group">
        <label class="toggle-label" for="toggle-auto-cordon">
          <input id="toggle-auto-cordon" v-model="policy.autoCordon" type="checkbox" class="checkbox-custom" />
          <div class="toggle-info">
            <span class="toggle-title">Auto-Cordon Unhealthy Nodes</span>
            <span class="toggle-desc">Mark failing nodes as Unschedulable to immediately block new workloads.</span>
          </div>
        </label>
      </div>

      <!-- 4. Force-Delete Stuck Terminating Pods (GracePeriod=0) -->
      <div class="toggle-group">
        <label class="toggle-label" for="toggle-force-delete">
          <input id="toggle-force-delete" v-model="policy.forceDeleteStuckPods" type="checkbox" class="checkbox-custom" />
          <div class="toggle-info">
            <span class="toggle-title">Force-Delete Stuck Terminating Pods (GracePeriod=0)</span>
            <span class="toggle-desc">Aggressively purge dead pods on partitioned nodes so replicas can respawn.</span>
          </div>
        </label>
      </div>

      <div class="form-actions">
        <button type="submit" class="btn btn-primary" :disabled="saving">
          <span>{{ saving ? '💾 Persisting SRE Policy...' : '💾 Save SRE Policy' }}</span>
        </button>
      </div>
    </form>
  </div>
</template>

<script setup lang="ts">
import { reactive, ref, onMounted } from 'vue'
import { settingsApi } from '../../api/settings'

interface SREPolicy {
  automatedFastFailover: boolean
  heartbeatTimeout: number
  autoCordon: boolean
  forceDeleteStuckPods: boolean
}

const STORAGE_KEY = 'k8s_sre_failover_policy'
const defaultPolicy: SREPolicy = {
  automatedFastFailover: true,
  heartbeatTimeout: 15,
  autoCordon: true,
  forceDeleteStuckPods: true,
}

const policy = reactive<SREPolicy>({ ...defaultPolicy })
const saving = ref(false)
const toast = ref<{ type: 'success' | 'error'; text: string } | null>(null)
let toastTimer: ReturnType<typeof setTimeout> | null = null

function showToast(text: string, type: 'success' | 'error' = 'success') {
  if (toastTimer) clearTimeout(toastTimer)
  toast.value = { text, type }
  toastTimer = setTimeout(() => { toast.value = null }, 3500)
}

onMounted(async () => {
  const cached = localStorage.getItem(STORAGE_KEY)
  if (cached) {
    try { Object.assign(policy, JSON.parse(cached)) } catch { /* ignore */ }
  }
  try {
    const list = await settingsApi.getByCategory('sre')
    if (Array.isArray(list)) {
      for (const item of list) {
        if (item.key === 'fast_failover') policy.automatedFastFailover = item.value === 'true'
        if (item.key === 'heartbeat_timeout') policy.heartbeatTimeout = parseInt(item.value, 10) || 15
        if (item.key === 'auto_cordon') policy.autoCordon = item.value === 'true'
        if (item.key === 'force_delete_stuck') policy.forceDeleteStuckPods = item.value === 'true'
      }
    }
  } catch { /* use cached */ }
})

async function savePolicy() {
  saving.value = true
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(policy))
    try {
      await settingsApi.update([
        { category: 'sre', key: 'fast_failover', value: String(policy.automatedFastFailover) },
        { category: 'sre', key: 'heartbeat_timeout', value: String(policy.heartbeatTimeout) },
        { category: 'sre', key: 'auto_cordon', value: String(policy.autoCordon) },
        { category: 'sre', key: 'force_delete_stuck', value: String(policy.forceDeleteStuckPods) },
      ])
    } catch { /* local storage persisted */ }
    showToast('SRE auto-remediation policy saved successfully.')
  } catch (err: unknown) {
    showToast(err instanceof Error ? err.message : 'Failed to save policy', 'error')
  } finally {
    saving.value = false
  }
}

function resetDefaults() {
  Object.assign(policy, defaultPolicy)
  showToast('SRE policy values reset to defaults.')
}
</script>
