<script setup lang="ts">
import type { Cluster } from '../../api/fleet'
import StatusBadge from '../ui/StatusBadge.vue'

defineProps<{
  clusters: Cluster[]
  actionLoading?: string | null
}>()

const emit = defineEmits<{
  (e: 'discover', cluster: Cluster): void
  (e: 'upgrade', cluster: Cluster): void
  (e: 'remove', cluster: Cluster): void
  (e: 'details', cluster: Cluster): void
}>()
</script>

<template>
  <div class="mobile-stream-container">
    <div v-if="clusters.length === 0" class="mobile-empty-state glass-panel font-mono">
      <span>No clusters found matching current filters.</span>
    </div>

    <div
      v-for="cluster in clusters"
      :key="cluster.id"
      class="mobile-stream-item glass-panel"
    >
      <div class="mobile-stream-main" title="View Cluster Essentials" @click="emit('details', cluster)">
        <div class="mobile-stream-header">
          <span class="cluster-icon-mini">⎈</span>
          <span class="mobile-stream-name font-mono">{{ cluster.name }}</span>
          <span class="tier-pill">{{ cluster.group || 'prod' }}</span>
          <StatusBadge :status="cluster.health_status || cluster.status || 'unknown'" size="sm" />
        </div>
        <div class="mobile-stream-sub font-mono">
          <span class="text-muted">{{ (cluster.provider || 'k8s').toUpperCase() }} · {{ cluster.region || 'local' }}</span>
          <span class="sub-sep">·</span>
          <span class="text-emerald">{{ cluster.nodes ?? 0 }} Nodes</span>
          <template v-if="cluster.version">
            <span class="sub-sep">·</span>
            <span class="text-cyan">{{ cluster.version }}</span>
          </template>
        </div>
      </div>

      <div class="mobile-stream-actions">
        <button
          class="btn btn-primary btn-xs"
          :disabled="actionLoading === cluster.id"
          title="Cluster Essentials"
          @click.stop="emit('details', cluster)"
        >
          ⚡
        </button>
        <button
          class="btn btn-secondary btn-xs"
          :disabled="actionLoading === cluster.id"
          title="Discover Resources"
          @click.stop="emit('discover', cluster)"
        >
          🔍
        </button>
        <button
          class="btn btn-secondary btn-xs"
          :disabled="actionLoading === cluster.id"
          title="Upgrade Cluster"
          @click.stop="emit('upgrade', cluster)"
        >
          ⬆️
        </button>
        <button
          class="btn btn-secondary btn-xs btn-evict"
          :disabled="actionLoading === cluster.id"
          title="Evict Cluster"
          @click.stop="emit('remove', cluster)"
        >
          🗑️
        </button>
      </div>
    </div>
  </div>
</template>
