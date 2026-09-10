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
    <!-- Desktop/Tablet View (Compact ~100px) -->
    <div class="swarm-desktop-content desktop-only">
      <div class="swarm-header">
        <div class="swarm-brand">
          <span class="swarm-logo">🐳</span>
          <div>
            <div class="swarm-title-row">
              <h3 class="swarm-title">Docker Swarm Cluster</h3>
              <span class="swarm-badge">LOCAL CONTROL PLANE</span>
            </div>
            <p class="swarm-desc">
              Active native Docker cluster orchestrating container services, ingress routing, and multi-host networks.
            </p>
          </div>
        </div>
        <div class="swarm-header-actions">
          <StatusBadge :status="swarmInfo.node_count > 0 ? 'active' : 'ready'" size="sm" />
          <router-link to="/infra/hosts" class="btn btn-secondary btn-xs font-mono">
            <span>🖥️ Compute Hosts ➔</span>
          </router-link>
        </div>
      </div>

      <div class="swarm-meta-grid">
        <div class="swarm-meta-card">
          <span class="meta-label">Total Swarm Nodes</span>
          <span class="meta-value font-mono text-emerald">{{ swarmInfo.node_count }} Nodes</span>
        </div>
        <div class="swarm-meta-card">
          <span class="meta-label">Managers / Workers</span>
          <span class="meta-value font-mono text-cyan">{{ swarmInfo.manager_count }}M / {{ swarmInfo.worker_count }}W</span>
        </div>
        <div class="swarm-meta-card">
          <span class="meta-label">Control Node Role</span>
          <span class="meta-value">{{ swarmInfo.is_manager ? 'Swarm Manager' : 'Worker Node' }}</span>
        </div>
        <div class="swarm-meta-card">
          <span class="meta-label">Cluster ID</span>
          <span class="meta-value font-mono text-muted text-truncate" :title="swarmInfo.id">{{ swarmInfo.id || 'swarm-local' }}</span>
        </div>
      </div>
    </div>

    <!-- Mobile Compact Card (<640px: ~100-120px) -->
    <div class="swarm-mobile-compact mobile-only">
      <div class="swarm-mobile-top">
        <div class="swarm-mobile-brand">
          <span class="swarm-logo-sm">🐳</span>
          <span class="swarm-mobile-title font-mono">Docker Swarm</span>
          <StatusBadge :status="swarmInfo.node_count > 0 ? 'active' : 'ready'" size="sm" />
        </div>
        <router-link to="/infra/hosts" class="btn btn-secondary btn-xs swarm-link-btn font-mono" title="View Compute Hosts">
          <span>Hosts ➔</span>
        </router-link>
      </div>

      <div class="swarm-mobile-stats font-mono">
        <div class="swarm-stat-badge">
          <span class="stat-lbl">Nodes:</span>
          <span class="stat-val text-emerald">{{ swarmInfo.node_count }}</span>
        </div>
        <div class="swarm-stat-badge">
          <span class="stat-lbl">Role:</span>
          <span class="stat-val">{{ swarmInfo.is_manager ? 'Manager' : 'Worker' }}</span>
        </div>
        <div class="swarm-stat-badge">
          <span class="stat-lbl">Topology:</span>
          <span class="stat-val text-cyan">{{ swarmInfo.manager_count }}M / {{ swarmInfo.worker_count }}W</span>
        </div>
      </div>
    </div>
  </div>
</template>
