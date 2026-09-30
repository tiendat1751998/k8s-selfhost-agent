<template>
  <div class="settings-card glass-panel sre-policy-card">
    <div class="card-header">
      <div>
        <h2 class="card-title"><BaseIcon name="zap" size="sm" /> SRE Auto-Remediation & Fast-Failover Settings</h2>
        <p class="card-subtitle">Autonomous controller policies for rapid node failure detection and pod eviction.</p>
      </div>
    </div>

    <!-- Toast Notification -->
    <transition name="fade">
      <div v-if="toast" class="toast-banner" :class="'banner-' + toast.type">
        <BaseIcon :name="toast.type === 'success' ? 'check-circle' : 'alert-triangle'" size="xs" /> <span>{{ toast.text }}</span>
        <button class="toast-close" aria-label="Close notification" @click="toast = null"><BaseIcon name="x" size="xs" /></button>
      </div>
    </transition>

    <form class="sre-form" @submit.prevent="savePolicy">
      <!-- Row 1: Automated Fast-Failover (< 30s) -->
      <div class="setting-row">
        <div class="setting-meta">
          <label class="setting-title" for="toggle-fast-failover">Automated Fast-Failover (&lt; 30s)</label>
          <p class="setting-desc">Trigger autonomous failover workflows when a node ceases health telemetry.</p>
        </div>
        <div class="setting-control-col">
          <label class="toggle-switch">
            <input id="toggle-fast-failover" v-model="policy.automatedFastFailover" type="checkbox" aria-label="Automated Fast-Failover" />
            <span class="toggle-slider"></span>
          </label>
        </div>
      </div>

      <!-- Row 2: Node Heartbeat Timeout -->
      <div class="setting-row">
        <div class="setting-meta">
          <label class="setting-title" for="slider-heartbeat">Node Heartbeat Timeout</label>
          <p class="setting-desc">Maximum acceptable delay before marking a silent node as NotReady.</p>
        </div>
        <div class="setting-control-col">
          <div class="slider-control-cluster">
            <button
              type="button"
              class="btn-stepper"
              :disabled="policy.heartbeatTimeout <= 5"
              aria-label="Decrease timeout"
              @click="stepTimeout(-1)"
            >&minus;</button>
            <div class="slider-track-wrap">
              <input
                id="slider-heartbeat"
                v-model.number="policy.heartbeatTimeout"
                type="range"
                min="5"
                max="30"
                step="1"
                class="slider-input-bounded"
                aria-label="Node Heartbeat Timeout"
              />
              <div class="slider-ticks">
                <span>5s</span>
                <span>15s</span>
                <span>30s</span>
              </div>
            </div>
            <button
              type="button"
              class="btn-stepper"
              :disabled="policy.heartbeatTimeout >= 30"
              aria-label="Increase timeout"
              @click="stepTimeout(1)"
            >+</button>
            <span class="slider-value-pill font-mono">{{ policy.heartbeatTimeout }}s</span>
          </div>
        </div>
      </div>

      <!-- Row 3: Automatic Pod Eviction -->
      <div class="setting-row">
        <div class="setting-meta">
          <label class="setting-title" for="toggle-pod-eviction">Automatic Pod Eviction</label>
          <p class="setting-desc">Aggressively evict pods from partitioned or failing nodes so replicas can respawn elsewhere.</p>
        </div>
        <div class="setting-control-col">
          <label class="toggle-switch">
            <input id="toggle-pod-eviction" v-model="policy.automaticPodEviction" type="checkbox" aria-label="Automatic Pod Eviction" />
            <span class="toggle-slider"></span>
          </label>
        </div>
      </div>

      <!-- Row 4: Max Concurrent Evictions -->
      <div class="setting-row">
        <div class="setting-meta">
          <label class="setting-title" for="input-max-evictions">Max Concurrent Evictions</label>
          <p class="setting-desc">Throttle parallel pod evictions to prevent cascading reschedule storms across healthy nodes.</p>
        </div>
        <div class="setting-control-col">
          <div class="input-addon-wrap">
            <input
              id="input-max-evictions"
              v-model.number="policy.maxConcurrentEvictions"
              type="number"
              min="1"
              max="50"
              class="input-glass form-input"
              style="width: 80px;"
            />
            <span class="input-addon">pods</span>
          </div>
        </div>
      </div>

      <!-- Card Actions Bar -->
      <div class="card-actions-bar">
        <button type="button" class="btn btn-secondary btn-sm" @click="resetDefaults">
          <BaseIcon name="rotate-ccw" size="xs" /> <span>Reset Defaults</span>
        </button>
        <button type="submit" class="btn btn-primary btn-sm" :disabled="saving">
          <BaseIcon name="hard-drive" size="xs" /> <span>{{ saving ? 'Persisting SRE Policy...' : 'Save SRE Policy' }}</span>
        </button>
      </div>
    </form>
  </div>
</template>

<script setup lang="ts">
import { reactive, ref, onMounted } from 'vue'
import BaseIcon from '../ui/BaseIcon.vue'
import { settingsApi } from '../../api/settings'

interface SREPolicy {
  automatedFastFailover: boolean
  heartbeatTimeout: number
  automaticPodEviction: boolean
  maxConcurrentEvictions: number
  autoCordon: boolean
  forceDeleteStuckPods: boolean
}

const STORAGE_KEY = 'k8s_sre_failover_policy'
const defaultPolicy: SREPolicy = {
  automatedFastFailover: true,
  heartbeatTimeout: 15,
  automaticPodEviction: true,
  maxConcurrentEvictions: 5,
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

function stepTimeout(delta: number) {
  const next = policy.heartbeatTimeout + delta
  if (next >= 5 && next <= 30) {
    policy.heartbeatTimeout = next
  }
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
        if (item.key === 'pod_eviction' || item.key === 'force_delete_stuck') policy.automaticPodEviction = item.value === 'true'
        if (item.key === 'max_evictions') policy.maxConcurrentEvictions = parseInt(item.value, 10) || 5
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
        { category: 'sre', key: 'pod_eviction', value: String(policy.automaticPodEviction) },
        { category: 'sre', key: 'max_evictions', value: String(policy.maxConcurrentEvictions) },
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
