<script setup lang="ts">
import type { Cluster } from '../../api/fleet'
import StatusBadge from '../ui/StatusBadge.vue'

defineProps<{
  clusters: Cluster[]
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
</script>

<template>
  <div class="section-box glass-panel">
    <div class="box-header">
      <div>
        <h2 class="box-title">Kubernetes Fleet Control Planes</h2>
        <p class="box-subtitle">Managed multi-region Kubernetes clusters with live status and telemetry</p>
      </div>
      <button v-if="clusters.length > 0" class="btn btn-secondary btn-xs" @click="emit('import')">
        <span>+ Add Cluster</span>
      </button>
    </div>

    <!-- Empty State for K8s Clusters -->
    <div v-if="clusters.length === 0" class="empty-fleet-card">
      <div class="empty-icon-wrap">
        <span class="empty-icon">⎈</span>
      </div>
      <h3 class="empty-title">No External Kubernetes Clusters Registered</h3>
      <p class="empty-desc">
        Connect your multi-region Kubernetes clusters to establish centralized federation, health monitoring, and unified workload orchestration.
      </p>

      <div class="fleet-features-grid">
        <div class="feature-item glass-panel">
          <div class="feature-header">
            <span class="feature-badge-icon">🌐</span>
            <h4 class="feature-heading">Federated Multi-Cluster Management</h4>
          </div>
          <p class="feature-text">
            Unified topology dashboard aggregating clusters across AWS EKS, GCP GKE, Azure AKS, Bare-Metal, and Edge datacenters.
          </p>
        </div>

        <div class="feature-item glass-panel">
          <div class="feature-header">
            <span class="feature-badge-icon">🗺️</span>
            <h4 class="feature-heading">Multi-Region Fleet Orchestration</h4>
          </div>
          <p class="feature-text">
            Real-time cross-region workload distribution, tier segregation (Production, Staging, Edge), and topology mapping.
          </p>
        </div>

        <div class="feature-item glass-panel">
          <div class="feature-header">
            <span class="feature-badge-icon">🔑</span>
            <h4 class="feature-heading">Kubeconfig Import & Secure Vault</h4>
          </div>
          <p class="feature-text">
            Direct import of kubeconfig manifests encrypted with AES-256 GCM in local vault with SHA-256 integrity verification.
          </p>
        </div>

        <div class="feature-item glass-panel">
          <div class="feature-header">
            <span class="feature-badge-icon">🛡️</span>
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

    <!-- K8s Clusters Cards Grid -->
    <div v-else class="clusters-card-grid">
      <div v-for="cluster in clusters" :key="cluster.id" class="cluster-card glass-panel">
        <div class="card-top">
          <div class="cluster-brand">
            <span class="cluster-icon">⎈</span>
            <div>
              <h3 class="cluster-title">{{ cluster.name }}</h3>
              <span class="cluster-group font-mono">{{ cluster.group || 'default' }} · {{ (cluster.provider || 'generic').toUpperCase() }}</span>
            </div>
          </div>
          <StatusBadge :status="cluster.health_status || cluster.status || 'unknown'" size="sm" />
        </div>

        <div class="card-body-meta">
          <div class="meta-row">
            <span class="meta-lbl">Region / DC:</span>
            <span class="meta-val font-mono">{{ cluster.region || 'local' }}</span>
          </div>
          <div class="meta-row">
            <span class="meta-lbl">Kubernetes Version:</span>
            <span class="meta-val font-mono text-cyan">{{ cluster.version || 'Pending Discovery' }}</span>
          </div>
          <div class="meta-row">
            <span class="meta-lbl">Active Nodes:</span>
            <span class="meta-val font-mono text-emerald">{{ cluster.nodes !== undefined ? `${cluster.nodes} Nodes` : '—' }}</span>
          </div>
          <div v-if="cluster.last_health_check" class="meta-row">
            <span class="meta-lbl">Last Health Audit:</span>
            <span class="meta-val font-mono text-muted">{{ new Date(cluster.last_health_check).toLocaleTimeString() }}</span>
          </div>
        </div>

        <div class="card-actions">
          <button class="btn btn-primary btn-xs" @click="emit('details', cluster)"><span>⚡ Essentials</span></button>
          <button
            class="btn btn-secondary btn-xs"
            :disabled="actionLoading === cluster.id"
            @click="emit('discover', cluster)"
          >
            <span>🔍 Discover</span>
          </button>
          <button
            class="btn btn-secondary btn-xs"
            :disabled="actionLoading === cluster.id"
            @click="emit('upgrade', cluster)"
          >
            <span>⬆️ Upgrade</span>
          </button>
          <button
            class="btn btn-secondary btn-xs btn-remove btn-evict"
            :disabled="actionLoading === cluster.id"
            @click="emit('remove', cluster)"
          >
            <span>🗑️ Evict</span>
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
