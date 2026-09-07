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
.overview-alert-banner {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 14px 20px;
  border-radius: 12px;
  backdrop-filter: blur(12px);
  -webkit-backdrop-filter: blur(12px);
  box-shadow: 0 4px 20px rgba(0, 0, 0, 0.25);
  animation: slideDown 0.3s cubic-bezier(0.16, 1, 0.3, 1);
  flex-wrap: wrap;
}

@keyframes slideDown {
  from { opacity: 0; transform: translateY(-8px); }
  to { opacity: 1; transform: translateY(0); }
}

.severity-critical {
  background: rgba(244, 63, 94, 0.12);
  border: 1px solid rgba(244, 63, 94, 0.45);
  box-shadow: 0 0 20px rgba(244, 63, 94, 0.15);
}

.severity-warning {
  background: rgba(245, 158, 11, 0.12);
  border: 1px solid rgba(245, 158, 11, 0.4);
  box-shadow: 0 0 20px rgba(245, 158, 11, 0.15);
}

.severity-info {
  background: rgba(6, 182, 212, 0.1);
  border: 1px solid rgba(6, 182, 212, 0.3);
}

.banner-left {
  display: flex;
  align-items: flex-start;
  gap: 14px;
  flex: 1;
  min-width: 260px;
}

.banner-icon-wrap {
  font-size: 24px;
  line-height: 1;
  padding-top: 2px;
}

.banner-content {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.banner-title-row {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
}

.banner-title {
  font-size: 14px;
  color: var(--text-primary, #f8fafc);
}

.severity-critical .banner-title { color: #fb7185; }
.severity-warning .banner-title { color: #fbbf24; }

.banner-desc {
  font-size: 12.5px;
  color: var(--text-secondary, #94a3b8);
  margin: 0;
  line-height: 1.4;
}

.banner-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.btn-banner {
  padding: 6px 14px;
  border-radius: 8px;
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.2s ease;
  white-space: nowrap;
}

.btn-banner-primary {
  background: rgba(244, 63, 94, 0.2);
  border: 1px solid rgba(244, 63, 94, 0.4);
  color: #fda4af;
}

.severity-warning .btn-banner-primary {
  background: rgba(245, 158, 11, 0.2);
  border: 1px solid rgba(245, 158, 11, 0.4);
  color: #fcd34d;
}

.btn-banner-primary:hover {
  background: rgba(244, 63, 94, 0.3);
  transform: translateY(-1px);
}

.btn-banner-secondary {
  background: rgba(255, 255, 255, 0.06);
  border: 1px solid rgba(255, 255, 255, 0.12);
  color: var(--text-primary, #f8fafc);
}

.btn-banner-secondary:hover {
  background: rgba(255, 255, 255, 0.12);
}

.btn-banner-close {
  background: transparent;
  border: none;
  color: var(--text-muted, #64748b);
  cursor: pointer;
  padding: 6px 8px;
  font-size: 14px;
  line-height: 1;
  border-radius: 6px;
}

.btn-banner-close:hover {
  color: #fff;
  background: rgba(255, 255, 255, 0.08);
}

.badge-rose {
  background: rgba(244, 63, 94, 0.2);
  color: #fb7185;
  border: 1px solid rgba(244, 63, 94, 0.4);
}

.badge-amber {
  background: rgba(245, 158, 11, 0.2);
  color: #fbbf24;
  border: 1px solid rgba(245, 158, 11, 0.4);
}

@media (max-width: 640px) {
  .overview-alert-banner { padding: 12px; gap: 10px; }
  .banner-actions { width: 100%; justify-content: flex-end; }
  .banner-desc { font-size: 11.5px; }
}
</style>
