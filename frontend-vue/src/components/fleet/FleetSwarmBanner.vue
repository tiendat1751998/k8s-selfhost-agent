<script setup lang="ts">
import StatusBadge from '../ui/StatusBadge.vue'
import type { SwarmClusterInfo } from '../../api/fleet'

defineProps<{
  swarmInfo: SwarmClusterInfo
}>()
</script>

<template>
  <!-- Docker Swarm Cluster Banner -->
  <div class="swarm-section glass-panel">
    <div class="swarm-header">
      <div class="swarm-brand">
        <span class="swarm-logo">🐳</span>
        <div>
          <div class="swarm-title-row">
            <h3 class="swarm-title">Docker Swarm Cluster</h3>
            <span class="swarm-badge font-mono">LOCAL CONTROL PLANE</span>
          </div>
          <p class="swarm-desc">
            Active native Docker cluster orchestrating container services, ingress routing, and multi-host networks.
          </p>
        </div>
      </div>
      <div class="swarm-status-wrap">
        <StatusBadge :status="swarmInfo.node_count > 0 ? 'active' : 'ready'" size="sm" />
      </div>
    </div>

    <div class="swarm-meta-grid">
      <div class="swarm-meta-card">
        <span class="meta-label">Total Swarm Nodes</span>
        <span class="meta-value font-mono text-emerald">{{ swarmInfo.node_count }} Nodes</span>
      </div>
      <div class="swarm-meta-card">
        <span class="meta-label">Managers / Workers</span>
        <span class="meta-value font-mono text-cyan">{{ swarmInfo.manager_count }} Manager / {{ swarmInfo.worker_count }} Worker</span>
      </div>
      <div class="swarm-meta-card">
        <span class="meta-label">Control Node Role</span>
        <span class="meta-value font-mono">{{ swarmInfo.is_manager ? 'Swarm Manager' : 'Worker Node' }}</span>
      </div>
      <div class="swarm-meta-card">
        <span class="meta-label">Cluster ID</span>
        <span class="meta-value font-mono text-muted text-truncate" :title="swarmInfo.id">{{ swarmInfo.id || 'swarm-local' }}</span>
      </div>
    </div>

    <div class="swarm-actions-row">
      <router-link to="/infra/hosts" class="btn btn-secondary btn-xs">
        <span>🖥️ View Swarm Compute Hosts & Nodes ➔</span>
      </router-link>
    </div>
  </div>
</template>
