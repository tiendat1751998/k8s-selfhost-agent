<script setup lang="ts">
import StatusBadge from '../ui/StatusBadge.vue'
import BaseIcon from '../ui/BaseIcon.vue'
import type { SwarmClusterInfo } from '../../api/fleet'

defineProps<{
  swarmInfo: SwarmClusterInfo
}>()
</script>

<template>
  <!-- Docker Swarm Cluster Banner -->
  <div class="swarm-section glass-panel">
    <!-- Desktop/Tablet View (Compact ~38px) -->
    <div class="swarm-desktop-content desktop-only">
      <div class="swarm-compact-row">
        <div class="swarm-brand">
          <BaseIcon name="layers" size="sm" class="swarm-logo text-cyan" />
          <span class="swarm-title">Docker Swarm</span>
          <span class="swarm-badge font-mono">LOCAL CONTROL PLANE</span>
          <StatusBadge :status="swarmInfo.node_count > 0 ? 'active' : 'ready'" size="sm" />
        </div>

        <div class="swarm-meta-inline font-mono">
          <span class="meta-inline-item"><span class="text-muted">Nodes:</span> <strong class="text-emerald">{{ swarmInfo.node_count }}</strong></span>
          <span class="sep">·</span>
          <span class="meta-inline-item"><span class="text-muted">Topology:</span> <strong class="text-cyan">{{ swarmInfo.manager_count }}M / {{ swarmInfo.worker_count }}W</strong></span>
          <span class="sep">·</span>
          <span class="meta-inline-item"><span class="text-muted">Role:</span> <strong>{{ swarmInfo.is_manager ? 'Manager' : 'Worker' }}</strong></span>
          <span class="sep">·</span>
          <span class="meta-inline-item"><span class="text-muted">ID:</span> <span class="text-muted text-truncate" style="max-width: 120px;" :title="swarmInfo.id">{{ swarmInfo.id || 'swarm-local' }}</span></span>
        </div>

        <div class="swarm-header-actions">
          <router-link to="/infra/hosts" class="btn btn-secondary btn-xs font-mono">
            <BaseIcon name="server" size="xs" /> <span>Compute Hosts</span> <BaseIcon name="chevron-right" size="xs" />
          </router-link>
        </div>
      </div>
    </div>

    <!-- Mobile Compact Card (<640px: ~100-120px) -->
    <div class="swarm-mobile-compact mobile-only">
      <div class="swarm-mobile-top">
        <div class="swarm-mobile-brand">
          <BaseIcon name="layers" size="xs" class="swarm-logo-sm" />
          <span class="swarm-mobile-title font-mono">Docker Swarm</span>
          <StatusBadge :status="swarmInfo.node_count > 0 ? 'active' : 'ready'" size="sm" />
        </div>
        <router-link to="/infra/hosts" class="btn btn-secondary btn-xs swarm-link-btn font-mono" title="View Compute Hosts">
          <span>Hosts</span> <BaseIcon name="chevron-right" size="xs" />
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
