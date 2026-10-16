<script setup lang="ts">
import { ref, computed, watch, onMounted } from 'vue'
import BaseIcon from '../ui/BaseIcon.vue'
import {
  hostsApi,
  type NodeK8sHealthReport,
  type RemediationRequest,
  type RemediationResult,
} from '../../api/hosts'

const props = defineProps<{
  hostId: string
  hostStatus?: string
}>()

const loading = ref(false)
const executingKey = ref<string | null>(null)
const health = ref<NodeK8sHealthReport | null>(null)
const history = ref<RemediationResult[]>([])
const feedback = ref<{ type: 'success' | 'error' | 'cooldown'; message: string; output?: string } | null>(null)
const selectedService = ref('kubelet')
const customService = ref('')
const expandedHistory = ref<Record<number, boolean>>({})

const services = [
  'kubelet', 'containerd', 'docker', 'postgresql', 'nginx',
  'redis', 'haproxy', 'keepalived', 'etcd', 'traefik', 'clickhouse-server', 'custom',
]

const effectiveService = computed(() => {
  return selectedService.value === 'custom' ? customService.value.trim() : selectedService.value
})

const isRuntimeHealthy = computed(() => {
  return Boolean(
    health.value?.runtime_healthy ||
    health.value?.containerd_healthy ||
    health.value?.docker_healthy ||
    health.value?.crio_healthy
  )
})

const anomalyList = computed(() => {
  return health.value?.detected_anomalies ?? health.value?.anomalies ?? []
})

async function fetchDiagnostics() {
  if (!props.hostId) return
  loading.value = true
  try {
    const [hRes, histRes] = await Promise.allSettled([
      hostsApi.getK8sHealth(props.hostId),
      hostsApi.getRemediationHistory(props.hostId),
    ])
    if (hRes.status === 'fulfilled') health.value = hRes.value
    if (histRes.status === 'fulfilled') history.value = histRes.value
  } finally {
    loading.value = false
  }
}

async function triggerRemediation(action: RemediationRequest['action'], target?: string) {
  const key = `${action}:${target || ''}`
  executingKey.value = key
  feedback.value = null
  try {
    const res = await hostsApi.remediate(props.hostId, { action, target })
    const isSuccess = res.success !== false && res.status !== 'failed'
    feedback.value = {
      type: isSuccess ? 'success' : 'error',
      message: isSuccess
        ? `Action ${action} completed successfully (${res.duration_ms}ms)`
        : `Action ${action} failed (${res.duration_ms}ms)`,
      output: res.details || res.output || 'Remediation action executed successfully.',
    }
    await fetchDiagnostics()
  } catch (err: unknown) {
    const msg = err instanceof Error ? err.message : String(err)
    const isCooldown = msg.includes('429') || msg.toLowerCase().includes('cooldown')
    feedback.value = {
      type: isCooldown ? 'cooldown' : 'error',
      message: isCooldown ? `Remediation rate limit / cooldown active: ${msg}` : `Remediation failed: ${msg}`,
    }
  } finally {
    executingKey.value = null
  }
}

function toggleHistoryItem(idx: number) {
  expandedHistory.value[idx] = !expandedHistory.value[idx]
}

watch(() => props.hostId, () => {
  feedback.value = null
  fetchDiagnostics()
})

onMounted(() => fetchDiagnostics())
</script>

<template>
  <div class="remediation-panel font-mono">
    <!-- Section 1: Runtime & K8s Health Badges -->
    <div class="remediation-card">
      <div class="rem-header-flex">
        <h4 class="rem-section-title"><BaseIcon name="shield" size="xs" /><span>K8S & RUNTIME HEALTH</span></h4>
        <button class="btn btn-secondary btn-xs" :disabled="loading" title="Refresh" @click="fetchDiagnostics">
          <BaseIcon name="refresh" size="xs" :class="{ 'animate-spin': loading }" />
          <span>{{ loading ? 'Checking...' : 'Refresh' }}</span>
        </button>
      </div>

      <div class="rem-stats-grid">
        <div class="rem-stat-cell">
          <span class="rem-stat-lbl">Kubelet</span>
          <div class="rem-stat-val">
            <span class="rem-badge" :class="health?.kubelet_healthy ? 'badge-healthy' : 'badge-unhealthy'">
              {{ health?.kubelet_healthy ? 'HEALTHY' : 'UNHEALTHY' }}
            </span>
          </div>
        </div>
        <div class="rem-stat-cell">
          <span class="rem-stat-lbl">Runtime</span>
          <div class="rem-stat-val">
            <span class="rem-badge" :class="isRuntimeHealthy ? 'badge-healthy' : 'badge-unhealthy'">
              {{ health?.container_runtime || 'runtime' }}: {{ isRuntimeHealthy ? 'OK' : 'ERR' }}
            </span>
          </div>
        </div>
        <div class="rem-stat-cell">
          <span class="rem-stat-lbl">D-State</span>
          <div class="rem-stat-val" :class="{ 'text-rose': (health?.d_state_processes ?? health?.d_state_count ?? 0) > 0 }">{{ health?.d_state_processes ?? health?.d_state_count ?? 0 }}</div>
        </div>
        <div class="rem-stat-cell">
          <span class="rem-stat-lbl">Zombies</span>
          <div class="rem-stat-val" :class="{ 'text-amber': (health?.zombie_count ?? 0) > 0 }">{{ health?.zombie_count ?? 0 }}</div>
        </div>
        <div class="rem-stat-cell">
          <span class="rem-stat-lbl">Stuck Shims</span>
          <div class="rem-stat-val" :class="{ 'text-rose': (health?.stuck_shim_count ?? 0) > 0 }">{{ health?.stuck_shim_count ?? 0 }}</div>
        </div>
      </div>

      <!-- Anomalies List -->
      <div v-if="anomalyList.length > 0" class="rem-anomalies-list">
        <div v-for="(anomaly, aIdx) in anomalyList" :key="aIdx" class="rem-anomaly-item">
          <BaseIcon name="alert-triangle" size="xs" /><span>{{ anomaly }}</span>
        </div>
      </div>
    </div>

    <!-- Section 2: Unhealthy App Containers -->
    <div v-if="health?.unhealthy_containers && health.unhealthy_containers.length > 0" class="remediation-card">
      <div class="rem-header-flex">
        <h4 class="rem-section-title"><BaseIcon name="box" size="xs" /><span>CRASHED CONTAINERS</span></h4>
        <span class="rem-badge badge-unhealthy">{{ health.unhealthy_containers.length }} ALERTS</span>
      </div>
      <div class="rem-container-list">
        <div v-for="c in health.unhealthy_containers" :key="c.id || c.name" class="rem-container-item">
          <div class="rem-container-info">
            <span class="rem-container-name">{{ c.name || c.id }}</span>
            <div class="rem-container-meta">
              <span>IMG: {{ c.image }}</span>
              <span>STATE: {{ c.state || c.status }}</span>
              <span v-if="c.restart_count !== undefined">RESTARTS: {{ c.restart_count }}</span>
            </div>
          </div>
          <button
            class="btn btn-danger-outline btn-xs"
            :disabled="executingKey === `restart_container:${c.name || c.id}`"
            @click="triggerRemediation('restart_container', c.name || c.id)"
          >
            <BaseIcon :name="executingKey === `restart_container:${c.name || c.id}` ? 'refresh' : 'play'" size="xs" :class="{ 'animate-spin': executingKey === `restart_container:${c.name || c.id}` }" />
            <span>Restart</span>
          </button>
        </div>
      </div>
    </div>

    <!-- Section 3: 1-Click Node & App Remediation Controls -->
    <div class="remediation-card">
      <div class="rem-header-flex">
        <h4 class="rem-section-title"><BaseIcon name="zap" size="xs" /><span>1-CLICK NODE & SERVICE RESCUE</span></h4>
      </div>
      <div class="rem-quick-actions">
        <button class="btn btn-secondary btn-xs" :disabled="!!executingKey" @click="triggerRemediation('disk_pressure_cleanup')">
          <BaseIcon :name="executingKey === 'disk_pressure_cleanup:' ? 'refresh' : 'trash'" size="xs" :class="{ 'animate-spin': executingKey === 'disk_pressure_cleanup:' }" />
          <span>Clean Disk Pressure</span>
        </button>
        <button class="btn btn-secondary btn-xs" :disabled="!!executingKey" @click="triggerRemediation('cni_ipam_cleanup')">
          <BaseIcon :name="executingKey === 'cni_ipam_cleanup:' ? 'refresh' : 'activity'" size="xs" :class="{ 'animate-spin': executingKey === 'cni_ipam_cleanup:' }" />
          <span>Clean CNI IP Locks</span>
        </button>
        <button class="btn btn-secondary btn-xs" :disabled="!!executingKey" @click="triggerRemediation('reap_zombies')">
          <BaseIcon :name="executingKey === 'reap_zombies:' ? 'refresh' : 'shield'" size="xs" :class="{ 'animate-spin': executingKey === 'reap_zombies:' }" />
          <span>Reap Zombies</span>
        </button>
      </div>

      <!-- App / Systemd Service Rescue -->
      <div class="rem-service-form">
        <div class="rem-service-row">
          <select v-model="selectedService" class="rem-select" aria-label="Select service to restart">
            <option v-for="svc in services" :key="svc" :value="svc">{{ svc }}</option>
          </select>
          <input
            v-if="selectedService === 'custom'"
            v-model="customService"
            type="text"
            placeholder="custom-service-name"
            class="rem-input"
            aria-label="Custom service name"
          />
          <button
            class="btn btn-primary btn-xs"
            :disabled="!effectiveService || !!executingKey"
            @click="triggerRemediation('restart_service', effectiveService)"
          >
            <BaseIcon :name="executingKey === `restart_service:${effectiveService}` ? 'refresh' : 'play'" size="xs" :class="{ 'animate-spin': executingKey === `restart_service:${effectiveService}` }" />
            <span>Restart Service</span>
          </button>
        </div>
      </div>

      <!-- Feedback / Cooldown Banner -->
      <div v-if="feedback" class="rem-feedback" :class="`feedback-${feedback.type}`">
        <div class="rem-feedback-header">
          <BaseIcon :name="feedback.type === 'success' ? 'check-circle' : 'alert-triangle'" size="xs" />
          <span>{{ feedback.message }}</span>
        </div>
        <pre v-if="feedback.output" class="rem-output-pre">{{ feedback.output }}</pre>
      </div>
    </div>

    <!-- Section 4: Self-Healing & Remediation History -->
    <div class="remediation-card">
      <div class="rem-header-flex">
        <h4 class="rem-section-title"><BaseIcon name="clock" size="xs" /><span>REMEDIATION AUDIT TRAIL</span></h4>
        <span class="rem-badge badge-neutral">{{ history.length }} RUNS</span>
      </div>
      <div v-if="history.length > 0" class="rem-history-list">
        <div v-for="(item, idx) in history" :key="idx" class="rem-history-item">
          <div class="rem-history-header">
            <div class="rem-history-action">
              <span class="text-cyan">{{ item.action }}</span>
              <span v-if="item.target" class="text-muted">({{ item.target }})</span>
            </div>
            <div class="rem-history-meta">
              <span class="rem-badge" :class="item.success || item.status === 'success' || item.status === 'ok' ? 'badge-healthy' : 'badge-unhealthy'">
                {{ item.status || (item.success ? 'SUCCESS' : 'FAILED') }}
              </span>
              <span>{{ item.duration_ms }}ms</span>
            </div>
          </div>
          <div v-if="item.details || item.output">
            <button class="rem-history-output-toggle" @click="toggleHistoryItem(idx)">
              {{ expandedHistory[idx] ? 'Hide Output' : 'View Output' }}
            </button>
            <pre v-if="expandedHistory[idx]" class="rem-output-pre">{{ item.details || item.output }}</pre>
          </div>
        </div>
      </div>
      <div v-else class="text-muted text-xs">No self-healing remediation actions executed on this host yet.</div>
    </div>
  </div>
</template>

<style scoped>
@import '../../assets/css/host-remediation.css';
</style>
