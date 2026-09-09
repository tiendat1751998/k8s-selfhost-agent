<script setup lang="ts">
import { computed } from 'vue'
import { useRouter } from 'vue-router'
import { useAlertStore } from '../../stores/alertStore'
import { useAppStore } from '../../stores/app'
import type { SystemOverview } from '../../api/overview'

interface Props {
  overview?: SystemOverview | null
}

const props = defineProps<Props>()

const router = useRouter()
const alertStore = useAlertStore()
const appStore = useAppStore()

interface ActivityItem {
  id: string
  title: string
  description: string
  type: 'alert' | 'node' | 'deployment' | 'incident'
  severity: 'critical' | 'warning' | 'info' | 'success'
  time: string
  timestamp: number
  link?: string
}

const activities = computed<ActivityItem[]>(() => {
  const list: ActivityItem[] = []
  const now = Date.now()

  // 1. Alerts from alertStore
  alertStore.activeAlerts.forEach((a, idx) => {
    const isCrit = a.value >= 90 || a.type === 'node_down'
    list.push({
      id: `alert-${a.node_id}-${a.type}-${idx}`,
      title: `${a.node_name || a.node_id}: ${a.type.replace('_', ' ').toUpperCase()}`,
      description: a.message || `Metric value reached ${a.value} (threshold: ${a.threshold})`,
      type: 'alert',
      severity: isCrit ? 'critical' : 'warning',
      time: 'Active now',
      timestamp: now - idx * 60000,
      link: '/hosts',
    })
  })

  // 2. Node Statuses from overview
  props.overview?.nodes.forEach(n => {
    if (n.status === 'down' || n.status === 'offline') {
      list.push({
        id: `node-${n.node_id}-down`,
        title: `Node Offline: ${n.node_name}`,
        description: `Agent telemetry lost or node offline.`,
        type: 'node',
        severity: 'critical',
        time: 'Just now',
        timestamp: now - 30000,
        link: '/hosts',
      })
    } else if (n.cpu_percent >= 80) {
      list.push({
        id: `node-${n.node_id}-cpu`,
        title: `High CPU Load: ${n.node_name}`,
        description: `CPU saturation currently at ${Math.round(n.cpu_percent)}%.`,
        type: 'node',
        severity: 'warning',
        time: '2m ago',
        timestamp: now - 120000,
        link: '/hosts',
      })
    }
  })

  // 3. Incidents from appStore
  appStore.incidents.slice(0, 4).forEach((inc) => {
    list.push({
      id: `inc-${inc.id}`,
      title: `${inc.type} in ${inc.pod_name || inc.namespace}`,
      description: inc.message || `Cluster event logged for ${inc.cluster_name}`,
      type: 'incident',
      severity: inc.severity === 'critical' ? 'critical' : inc.severity === 'high' ? 'warning' : 'info',
      time: inc.created_at ? new Date(inc.created_at).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }) : 'Recent',
      timestamp: inc.created_at ? new Date(inc.created_at).getTime() : now - 300000,
      link: '/deployments',
    })
  })

  return list.sort((a, b) => b.timestamp - a.timestamp).slice(0, 6)
})

function navigateTo(link?: string) {
  if (link) router.push(link)
}
</script>

<template>
  <div class="overview-activity-feed glass-panel">
    <div class="feed-header">
      <div class="feed-title-wrap">
        <h3 class="feed-title">
          <span class="feed-icon">⚡</span>
          <span>Live Cluster Activity Feed</span>
        </h3>
        <span class="feed-subtitle">Real-time alerts, incidents, and node status events</span>
      </div>
      <button
        type="button"
        class="btn-alert-center font-mono"
        @click="alertStore.showAlertCenterModal = true"
      >
        <span>Alert Center ({{ alertStore.activeAlerts.length }})</span>
      </button>
    </div>

    <div v-if="activities.length === 0" class="empty-feed">
      <span class="empty-feed-icon">🛡️</span>
      <span class="empty-feed-text">All cluster systems nominal. No active incidents or threshold breaches.</span>
    </div>

    <div v-else class="feed-list">
      <div
        v-for="item in activities"
        :key="item.id"
        class="feed-item"
        :class="`severity-${item.severity}`"
        @click="navigateTo(item.link)"
      >
        <div class="item-badge-dot"></div>
        <div class="item-body">
          <div class="item-header">
            <span class="item-title font-bold">{{ item.title }}</span>
            <span class="item-time font-mono">{{ item.time }}</span>
          </div>
          <p class="item-desc">{{ item.description }}</p>
        </div>
        <span class="item-arrow">→</span>
      </div>
    </div>
  </div>
</template>

<style scoped>
@import '../../assets/styles/views/overview.css';
</style>
