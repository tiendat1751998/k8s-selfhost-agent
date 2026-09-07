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
.overview-activity-feed {
  padding: 18px 20px;
  border-radius: 14px;
  background: rgba(15, 23, 42, 0.65);
  border: 1px solid rgba(255, 255, 255, 0.08);
  backdrop-filter: blur(12px);
  -webkit-backdrop-filter: blur(12px);
  display: flex;
  flex-direction: column;
  gap: 14px;
}
.feed-header { display: flex; align-items: center; justify-content: space-between; gap: 12px; flex-wrap: wrap; }
.feed-title-wrap { display: flex; flex-direction: column; gap: 2px; }
.feed-title { display: flex; align-items: center; gap: 8px; font-size: 1.05rem; font-weight: 800; letter-spacing: -0.01em; color: var(--text-primary, #f8fafc); margin: 0; }
.feed-icon { font-size: 16px; }
.feed-subtitle { font-size: 11.5px; color: var(--text-secondary, #94a3b8); }
.btn-alert-center { background: rgba(56, 189, 248, 0.1); border: 1px solid rgba(56, 189, 248, 0.25); color: #38bdf8; padding: 4px 10px; border-radius: 6px; font-size: 11px; font-weight: 600; cursor: pointer; transition: all 0.2s ease; }
.btn-alert-center:hover { background: rgba(56, 189, 248, 0.2); transform: translateY(-1px); }
.empty-feed { display: flex; align-items: center; justify-content: center; gap: 10px; padding: 24px 16px; border-radius: 8px; background: rgba(255, 255, 255, 0.02); color: var(--text-muted, #64748b); font-size: 12.5px; }
.empty-feed-icon { font-size: 20px; }
.feed-list { display: flex; flex-direction: column; gap: 8px; }
.feed-item { display: flex; align-items: center; gap: 12px; padding: 10px 14px; border-radius: 8px; background: rgba(255, 255, 255, 0.03); border: 1px solid rgba(255, 255, 255, 0.06); cursor: pointer; transition: all 0.2s ease; }
.feed-item:hover { background: rgba(255, 255, 255, 0.06); border-color: rgba(56, 189, 248, 0.3); transform: translateX(2px); }
.item-badge-dot { width: 8px; height: 8px; border-radius: 50%; flex-shrink: 0; }
.severity-critical .item-badge-dot { background: #f43f5e; box-shadow: 0 0 8px #f43f5e; }
.severity-warning .item-badge-dot { background: #f59e0b; box-shadow: 0 0 8px #f59e0b; }
.severity-info .item-badge-dot { background: #06b6d4; box-shadow: 0 0 8px #06b6d4; }
.severity-success .item-badge-dot { background: #10b981; box-shadow: 0 0 8px #10b981; }
.item-body { display: flex; flex-direction: column; gap: 2px; flex: 1; min-width: 0; }
.item-header { display: flex; align-items: center; justify-content: space-between; gap: 8px; }
.item-title { font-size: 12.5px; color: var(--text-primary, #f8fafc); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.item-time { font-size: 10.5px; color: var(--text-muted, #64748b); white-space: nowrap; }
.item-desc { font-size: 11.5px; color: var(--text-secondary, #94a3b8); margin: 0; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.item-arrow { color: var(--text-muted, #64748b); font-size: 13px; transition: transform 0.2s ease, color 0.2s ease; }
.feed-item:hover .item-arrow { color: #38bdf8; transform: translateX(2px); }
.font-mono { font-family: var(--font-mono, monospace); }
.font-bold { font-weight: 700; }
</style>
