<script setup lang="ts">
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import { useAlertStore } from '../../stores/alertStore'

interface Props {
  isClusterDegraded: boolean
  clusterHealthScore: number
  healthyNodes: number
  totalNodes: number
  httpErrorRate?: number
}

const props = withDefaults(defineProps<Props>(), {
  httpErrorRate: 0,
})

const emit = defineEmits<{
  (e: 'dismiss'): void
}>()

const router = useRouter()
const alertStore = useAlertStore()

const downNodesCount = computed(() => Math.max(0, props.totalNodes - props.healthyNodes))

const activeCriticalCount = computed(() => {
  return alertStore.activeAlerts.filter(a => a.value >= 90 || a.type === 'node_down').length
})

const bannerSeverity = computed<'critical' | 'warning' | 'info'>(() => {
  if (downNodesCount.value > 0 || activeCriticalCount.value > 0 || props.httpErrorRate >= 5) {
    return 'critical'
  }
  if (props.isClusterDegraded || props.clusterHealthScore < 80 || props.httpErrorRate > 0) {
    return 'warning'
  }
  return 'info'
})

const bannerTitle = computed(() => {
  if (bannerSeverity.value === 'critical') {
    if (downNodesCount.value > 0) {
      return `Critical: ${downNodesCount.value} Node${downNodesCount.value > 1 ? 's' : ''} Offline in Cluster Mesh`
    }
    if (props.httpErrorRate >= 5) {
      return `Critical: High Ingress Error Rate (${props.httpErrorRate.toFixed(1)}%) Detected`
    }
    return `Critical: ${activeCriticalCount.value} Severe Resource Alert${activeCriticalCount.value > 1 ? 's' : ''} Triggered`
  }
  return `Warning: Cluster Performance Degraded (Health Score: ${props.clusterHealthScore}/100)`
})

const bannerDescription = computed(() => {
  if (downNodesCount.value > 0) {
    return `Immediate action required: ${props.healthyNodes} of ${props.totalNodes} nodes are currently operational.`
  }
  if (props.httpErrorRate >= 5) {
    return `Ingress gateway reports elevated error rates exceeding the 5% threshold.`
  }
  return `One or more hosts are operating under heavy resource saturation.`
})
</script>

<template>
  <transition name="alert-slide">
    <div
      v-if="isClusterDegraded || alertStore.hasCriticalAlerts || alertStore.hasNodeDown"
      class="overview-alert-banner glass-panel"
      :class="`severity-${bannerSeverity}`"
    >
      <div class="banner-left">
        <div class="banner-icon-wrap">
          <span class="banner-icon">{{ bannerSeverity === 'critical' ? '🚨' : '⚠️' }}</span>
        </div>
        <div class="banner-content">
          <div class="banner-title-row">
            <span class="banner-title font-bold">{{ bannerTitle }}</span>
            <span class="badge" :class="bannerSeverity === 'critical' ? 'badge-rose' : 'badge-amber'">
              Score: {{ clusterHealthScore }}%
            </span>
          </div>
          <p class="banner-desc">{{ bannerDescription }}</p>
        </div>
      </div>

      <div class="banner-actions">
        <button
          type="button"
          class="btn-banner btn-banner-primary font-mono"
          @click="alertStore.showAlertCenterModal = true"
        >
          <span>🔔 View Alerts ({{ alertStore.activeAlerts.length }})</span>
        </button>
        <button
          type="button"
          class="btn-banner btn-banner-secondary font-mono"
          @click="router.push('/hosts')"
        >
          <span>🖥️ Hosts</span>
        </button>
        <button
          type="button"
          class="btn-banner-close"
          @click="emit('dismiss')"
          title="Dismiss banner"
        >✕</button>
      </div>
    </div>
  </transition>
</template>

<style scoped>
@import '../../assets/styles/views/overview.css';
</style>
