<script setup lang="ts">
import { useRouter } from 'vue-router'
import type { Cluster } from '../../api/fleet'
import StatusBadge from '../ui/StatusBadge.vue'
import BaseIcon from '../ui/BaseIcon.vue'

defineProps<{
  clusters: Cluster[]
  totalClustersCount?: number
  actionLoading?: string | null
  loading?: boolean
}>()

const emit = defineEmits<{
  (e: 'discover', cluster: Cluster): void
  (e: 'upgrade', cluster: Cluster): void
  (e: 'remove', cluster: Cluster): void
  (e: 'details', cluster: Cluster): void
  (e: 'import'): void
}>()

const router = useRouter()
</script>

<template>
  <div class="section-box glass-panel">


    <!-- Filtered Empty State -->
    <div v-if="clusters.length === 0 && (totalClustersCount || 0) > 0" class="empty-state font-mono">
      <p>No clusters found matching current search/filter criteria.</p>
    </div>

    <!-- Empty State for Fleet Clusters -->
    <div v-else-if="clusters.length === 0" class="empty-fleet-card">
      <div class="empty-icon-wrap">
        <span class="empty-icon"><BaseIcon name="anchor" size="lg" /></span>
      </div>
      <h3 class="empty-title">No Fleet Clusters Registered</h3>
      <p class="empty-desc">
        Connect your multi-region Kubernetes clusters or Docker Swarm engine to establish centralized federation, health monitoring, and unified workload orchestration.
      </p>

      <div class="fleet-features-grid">
        <div class="feature-item glass-panel">
          <div class="feature-header">
            <span class="feature-badge-icon"><BaseIcon name="globe" size="xs" /></span>
            <h4 class="feature-heading">Federated Multi-Cluster Management</h4>
          </div>
          <p class="feature-text">
            Unified topology dashboard aggregating clusters across AWS EKS, GCP GKE, Azure AKS, Bare-Metal, and Edge datacenters.
          </p>
        </div>

        <div class="feature-item glass-panel">
          <div class="feature-header">
            <span class="feature-badge-icon"><BaseIcon name="globe" size="xs" /></span>
            <h4 class="feature-heading">Multi-Region Fleet Orchestration</h4>
          </div>
          <p class="feature-text">
            Real-time cross-region workload distribution, tier segregation (Production, Staging, Edge), and topology mapping.
          </p>
        </div>

        <div class="feature-item glass-panel">
          <div class="feature-header">
            <span class="feature-badge-icon"><BaseIcon name="lock" size="xs" /></span>
            <h4 class="feature-heading">Kubeconfig Import & Secure Vault</h4>
          </div>
          <p class="feature-text">
            Direct import of kubeconfig manifests encrypted with AES-256 GCM in local vault with SHA-256 integrity verification.
          </p>
        </div>

        <div class="feature-item glass-panel">
          <div class="feature-header">
            <span class="feature-badge-icon"><BaseIcon name="shield" size="xs" /></span>
            <h4 class="feature-heading">Cluster Health Auditing</h4>
          </div>
          <p class="feature-text">
            Continuous background probing of API server latency, node pools, discovered namespaces, and custom resource definitions.
          </p>
        </div>
      </div>

      <div class="empty-actions">
        <button class="btn btn-primary btn-lg" @click="emit('import')">
          <span>+ Import Cluster (Kubeconfig)</span>
        </button>
      </div>
    </div>

    <!-- Unified Clusters Cards Grid -->
    <div v-else class="clusters-card-grid">
      <div v-for="cluster in clusters" :key="cluster.id" class="cluster-card glass-panel">
        <div class="card-top">
          <div class="cluster-brand">
            <span
              class="cluster-icon"
              :class="cluster.orchestrator === 'swarm' ? 'text-blue' : 'text-cyan'"
            >
              <BaseIcon :name="cluster.orchestrator === 'swarm' ? 'layers' : 'anchor'" size="md" />
            </span>
            <div>
              <div class="cluster-title-wrap">
                <h3 class="cluster-title">{{ cluster.name }}</h3>
                <span
                  class="orchestrator-pill font-mono"
                  :class="cluster.orchestrator === 'swarm' ? 'pill-swarm' : 'pill-k8s'"
                >
                  {{ cluster.orchestrator === 'swarm' ? 'Docker Swarm' : 'Kubernetes' }}
                </span>
              </div>
              <span class="cluster-group font-mono">{{ cluster.group || 'default' }} ? {{ (cluster.provider || 'generic').toUpperCase() }}</span>
            </div>
          </div>
          <StatusBadge :status="cluster.health_status || cluster.status || 'unknown'" size="sm" />
        </div>

        <div class="card-body-meta">
          <div class="meta-row">
            <span class="meta-lbl">Region / DC:</span>
            <span class="meta-val font-mono">{{ cluster.region || 'local' }}</span>
          </div>

          <!-- Swarm-specific metadata -->
          <template v-if="cluster.orchestrator === 'swarm'">
            <div class="meta-row">
              <span class="meta-lbl">Engine:</span>
              <span class="meta-val font-mono text-cyan">SwarmKit</span>
            </div>
            <div class="meta-row">
              <span class="meta-lbl">Topology:</span>
              <span class="meta-val font-mono text-cyan">
                {{ cluster.swarm_meta ? `${cluster.swarm_meta.manager_count}M / ${cluster.swarm_meta.worker_count}W` : '1M / 0W' }}
              </span>
            </div>
            <div class="meta-row">
              <span class="meta-lbl">Active Nodes:</span>
              <span class="meta-val font-mono text-emerald">{{ cluster.nodes !== undefined ? `${cluster.nodes} Nodes` : '?' }}</span>
            </div>
          </template>

          <!-- Kubernetes-specific metadata -->
          <template v-else>
            <div class="meta-row">
              <span class="meta-lbl">Kubernetes Version:</span>
              <span class="meta-val font-mono text-cyan">{{ cluster.version || 'Pending Discovery' }}</span>
            </div>
            <div class="meta-row">
              <span class="meta-lbl">Active Nodes:</span>
              <span class="meta-val font-mono text-emerald">{{ cluster.nodes !== undefined ? `${cluster.nodes} Nodes` : '?' }}</span>
            </div>
            <div v-if="cluster.last_health_check" class="meta-row">
              <span class="meta-lbl">Last Health Audit:</span>
              <span class="meta-val font-mono text-muted">{{ new Date(cluster.last_health_check).toLocaleTimeString() }}</span>
            </div>
          </template>
        </div>

        <!-- Contextual Card Actions -->
        <div v-if="cluster.orchestrator === 'swarm'" class="card-actions">
          <button class="btn btn-primary btn-xs" @click="emit('details', cluster)">
            <span><BaseIcon name="zap" size="xs" /> Details</span>
          </button>
          <button class="btn btn-secondary btn-xs font-mono" @click="router.push('/infra/hosts')">
            <span><BaseIcon name="server" size="xs" /> Hosts</span>
          </button>
          <button class="btn btn-secondary btn-xs font-mono" @click="router.push('/compute')">
            <span><BaseIcon name="layers" size="xs" /> Services</span>
          </button>
        </div>

        <div v-else class="card-actions">
          <button class="btn btn-primary btn-xs" @click="emit('details', cluster)"><span><BaseIcon name="zap" size="xs" /> Essentials</span></button>
          <button
            class="btn btn-secondary btn-xs"
            :disabled="actionLoading === cluster.id"
            @click="emit('discover', cluster)"
          >
            <span><BaseIcon name="search" size="xs" /> Discover</span>
          </button>
          <button
            class="btn btn-secondary btn-xs"
            :disabled="actionLoading === cluster.id"
            @click="emit('upgrade', cluster)"
          >
            <span><BaseIcon name="arrow-up" size="xs" /> Upgrade</span>
          </button>
          <button
            class="btn btn-secondary btn-xs btn-remove btn-evict"
            :disabled="actionLoading === cluster.id"
            @click="emit('remove', cluster)"
          >
            <span><BaseIcon name="trash" size="xs" /> Evict</span>
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.cluster-title-wrap {
  display: flex;
  align-items: center;
  gap: 6px;
  flex-wrap: wrap;
}

.orchestrator-pill {
  font-size: 10px;
  font-weight: 600;
  padding: 1px 6px;
  border-radius: 4px;
  letter-spacing: 0.02em;
  text-transform: uppercase;
}

.pill-k8s {
  background: rgba(6, 182, 212, 0.12);
  color: #38bdf8;
  border: 1px solid rgba(6, 182, 212, 0.3);
}

.pill-swarm {
  background: rgba(59, 130, 246, 0.15);
  color: #60a5fa;
  border: 1px solid rgba(59, 130, 246, 0.35);
}

.text-blue {
  color: #60a5fa;
}
</style>
