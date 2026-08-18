<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { fleetApi, incidentsApi, type Cluster, type Incident } from '../api/compute'
import { aiApi, type AIProvider } from '../api/management'
import MetricCard from '../components/ui/MetricCard.vue'
import StatusBadge from '../components/ui/StatusBadge.vue'

const router = useRouter()
const loading = ref(false)
const error = ref<string | null>(null)
const clusters = ref<Cluster[]>([])
const recentIncidents = ref<Incident[]>([])
const aiProviders = ref<AIProvider[]>([])

const quickLinks = [
  { path: '/ai-hub', icon: '🧠', title: 'AI Provider Hub', desc: 'Manage AI LLM models & circuit breakers' },
  { path: '/tenancy', icon: '🏢', title: 'Tenancy & RBAC', desc: 'Multi-org boundaries & permission matrix' },
  { path: '/changes', icon: '📝', title: 'Change Requests', desc: 'ITIL approval workflow & RFC review' },
  { path: '/alerts', icon: '🔥', title: 'Alert Engine', desc: 'Prometheus alerts & Slack/Telegram' },
  { path: '/reports', icon: '📊', title: 'Reports Center', desc: 'SOC2, CIS & FinOps cost reports' },
  { path: '/backup', icon: '📦', title: 'Disaster Recovery', desc: 'Dual-Sync NVMe & S3 replication' },
]

async function loadData() {
  loading.value = true
  error.value = null
  try {
    const [fleetRes, incRes, aiRes] = await Promise.allSettled([
      fleetApi.list(),
      incidentsApi.list({ limit: 5 }),
      aiApi.getProviders()
    ])
    if (fleetRes.status === 'fulfilled') {
      clusters.value = fleetRes.value || []
    }
    if (incRes.status === 'fulfilled') {
      recentIncidents.value = incRes.value.data || []
    }
    if (aiRes.status === 'fulfilled') {
      aiProviders.value = aiRes.value || []
    }
  } catch (err: unknown) {
    error.value = err instanceof Error ? err.message : 'Failed to load overview telemetry'
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  loadData()
})

const onlineClustersCount = computed(() => {
  return clusters.value.filter(c => c.status === 'active' || c.health_status === 'healthy').length
})

const activeAiProvidersCount = computed(() => {
  return aiProviders.value.filter(p => p.status === 'ready' || p.status === 'active' || p.status === 'healthy').length
})

const totalNodes = computed(() => {
  return clusters.value.reduce((acc, c) => acc + (c.nodes || 0), 0)
})

function formatTime(isoStr?: string): string {
  if (!isoStr) return 'Just now'
  try {
    const date = new Date(isoStr)
    const now = new Date()
    const diffMs = now.getTime() - date.getTime()
    const diffMins = Math.floor(diffMs / 60000)
    if (diffMins < 1) return 'Just now'
    if (diffMins < 60) return `${diffMins} mins ago`
    const diffHours = Math.floor(diffMins / 60)
    if (diffHours < 24) return `${diffHours} hours ago`
    return date.toLocaleDateString()
  } catch {
    return isoStr
  }
}
</script>

<template>
  <div class="overview-page animate-fade-in">
    <!-- Header -->
    <div class="page-header">
      <div class="header-titles">
        <div class="header-badge">
          <span class="badge badge-emerald">Cluster Mesh Active</span>
          <span class="badge badge-cyan">v2.4 Enterprise Control</span>
        </div>
        <h1 class="page-title">Enterprise Platform Overview</h1>
        <p class="page-desc">
          Unified control plane telemetry across multi-region Kubernetes clusters, autonomous AI SRE agents, zero-trust governance, and disaster recovery pipelines.
        </p>
      </div>

      <div class="header-actions">
        <button class="btn btn-secondary" @click="router.push('/logs')">
          <span>📜 Stream Logs</span>
        </button>
        <button class="btn btn-primary" @click="router.push('/ai-hub')">
          <span>🧠 Open AI Hub</span>
        </button>
      </div>
    </div>

    <!-- Core Metrics -->
    <div class="metrics-grid">
      <MetricCard 
        title="Global Fleet Health" 
        :value="clusters.length > 0 ? `${Math.round((onlineClustersCount / clusters.length) * 100)}%` : '—'" 
        :trend="clusters.length > 0 ? `${onlineClustersCount}/${clusters.length} Clusters Online` : 'No clusters connected'" 
        :trendDirection="clusters.length > 0 ? 'up' : 'neutral'" 
      />
      <MetricCard 
        title="Total Cluster Nodes" 
        :value="String(totalNodes)" 
        trend="Managed Fleet Nodes" 
        trendDirection="neutral" 
      />
      <MetricCard 
        title="Active AI Models" 
        :value="aiProviders.length > 0 ? `${aiProviders.length} ${aiProviders.length === 1 ? 'Model' : 'Models'}` : '0 Models'" 
        :trend="aiProviders.length > 0 ? `${activeAiProvidersCount} Providers Active` : 'No providers registered'" 
        :trendDirection="aiProviders.length > 0 ? 'neutral' : 'neutral'" 
      />
      <MetricCard 
        title="Active Incidents" 
        :value="String(recentIncidents.filter(i => i.status !== 'resolved').length)" 
        trend="Autonomous SRE Tracking" 
        trendDirection="down" 
      />
    </div>

    <!-- Quick Navigation Hub -->
    <div class="hub-section">
      <h3 class="section-title">Enterprise Feature Hub</h3>
      <div class="hub-grid">
        <div 
          v-for="link in quickLinks" 
          :key="link.path" 
          class="hub-card glass-panel"
          @click="router.push(link.path)"
        >
          <div class="hub-card-top">
            <span class="hub-icon">{{ link.icon }}</span>
            <span class="hub-arrow">→</span>
          </div>
          <h4 class="hub-title">{{ link.title }}</h4>
          <p class="hub-desc">{{ link.desc }}</p>
        </div>
      </div>
    </div>

    <!-- Bottom Split: Cluster Status & Recent Events -->
    <div class="split-section">
      <!-- Clusters Telemetry -->
      <div class="cluster-panel glass-panel">
        <div class="panel-head">
          <h3>Fleet Clusters Telemetry</h3>
          <span class="badge badge-emerald">{{ onlineClustersCount }}/{{ clusters.length }} ONLINE</span>
        </div>
        <div class="cluster-list">
          <div v-if="clusters.length === 0" class="empty-list">
            No clusters connected yet
          </div>
          <div v-for="c in clusters" :key="c.id || c.name" class="cluster-item">
            <div class="c-left">
              <span class="c-name font-mono">{{ c.name }}</span>
              <small class="c-meta">{{ c.nodes || 0 }} Nodes • {{ c.region || c.provider || 'default' }}</small>
            </div>
            <div class="c-metrics font-mono">
              <span>Provider: <strong class="text-cyan">{{ c.provider || 'k8s' }}</strong></span>
              <span>Version: <strong class="text-emerald">{{ c.version || 'v1.31' }}</strong></span>
            </div>
            <StatusBadge :status="c.health_status || c.status || 'healthy'" size="sm" />
          </div>
        </div>
      </div>

      <!-- Recent Incidents & Approvals -->
      <div class="activity-panel glass-panel">
        <div class="panel-head">
          <h3>Recent Operations & Incidents</h3>
          <button class="btn btn-secondary btn-sm" @click="router.push('/changes')">
            <span>View All</span>
          </button>
        </div>
        <div class="incident-list">
          <div v-if="recentIncidents.length === 0" class="empty-list">
            No recent incidents recorded
          </div>
          <div v-for="inc in recentIncidents" :key="inc.id" class="incident-item">
            <div class="inc-left">
              <div class="inc-title">{{ inc.message || inc.type }}</div>
              <small class="inc-meta font-mono">{{ inc.cluster_name || 'k8s' }} • {{ formatTime(inc.created_at) }}</small>
            </div>
            <StatusBadge :status="inc.status" size="sm" />
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.overview-page {
  display: flex;
  flex-direction: column;
  gap: 24px;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 20px;
}

.header-badge {
  display: flex;
  gap: 8px;
  margin-bottom: 8px;
}

.page-title {
  font-size: 26px;
  font-weight: 800;
  letter-spacing: -0.03em;
  color: #fff;
  margin-bottom: 6px;
}

.page-desc {
  font-size: 13px;
  color: var(--text-secondary);
  max-width: 840px;
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 12px;
}

.metrics-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 16px;
}

/* Hub Section */
.hub-section {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.section-title {
  font-size: 15px;
  font-weight: 700;
  color: #fff;
}

.hub-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(260px, 1fr));
  gap: 16px;
}

.hub-card {
  padding: 18px;
  display: flex;
  flex-direction: column;
  gap: 8px;
  cursor: pointer;
  transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1);
}

.hub-card:hover {
  transform: translateY(-2px);
  border-color: rgba(6, 182, 212, 0.4);
  box-shadow: 0 10px 25px -5px rgba(6, 182, 212, 0.15);
}

.hub-card-top {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.hub-icon {
  font-size: 24px;
}

.hub-arrow {
  color: var(--text-muted);
  font-size: 14px;
  transition: transform 0.15s ease;
}

.hub-card:hover .hub-arrow {
  color: var(--accent-sky);
  transform: translateX(3px);
}

.hub-title {
  font-size: 14px;
  font-weight: 700;
  color: #fff;
}

.hub-desc {
  font-size: 11px;
  color: var(--text-muted);
  line-height: 1.4;
}

/* Split Section */
.split-section {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 20px;
}

@media (max-width: 1024px) {
  .split-section {
    grid-template-columns: 1fr;
  }
}

.panel-head {
  padding: 16px 20px;
  border-bottom: 1px solid var(--border-subtle);
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.panel-head h3 {
  font-size: 14px;
  font-weight: 700;
  color: #fff;
}

.cluster-list, .incident-list {
  padding: 12px 16px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.cluster-item, .incident-item {
  padding: 12px 14px;
  background: rgba(11, 15, 25, 0.5);
  border: 1px solid var(--border-subtle);
  border-radius: 10px;
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
}

.c-left, .inc-left {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.c-name {
  font-size: 13px;
  font-weight: 700;
  color: #fff;
}

.c-meta, .inc-meta {
  font-size: 11px;
  color: var(--text-muted);
}

.c-metrics {
  display: flex;
  gap: 12px;
  font-size: 11px;
}

.inc-title {
  font-size: 13px;
  font-weight: 600;
  color: var(--text-primary);
}

.btn-sm {
  padding: 4px 10px;
  font-size: 11px;
}

.empty-list {
  padding: 24px 16px;
  text-align: center;
  color: var(--text-muted);
  font-size: 12px;
}
</style>
