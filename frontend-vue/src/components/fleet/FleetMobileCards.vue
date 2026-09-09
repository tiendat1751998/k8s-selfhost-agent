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
}>()
</script>

<template>
  <div v-if="clusters.length > 0" class="mobile-stream-container">
    <div
      v-for="cluster in clusters"
      :key="cluster.id"
      class="mobile-stream-item glass-panel"
    >
      <div class="mobile-stream-main">
        <div class="mobile-stream-header">
          <span class="cluster-icon" style="font-size: 16px;">⎈</span>
          <span class="mobile-stream-name">{{ cluster.name }}</span>
          <span class="tier-pill font-mono">{{ cluster.group || 'prod' }}</span>
        </div>
        <div class="mobile-stream-sub">
          <span class="font-mono text-muted">{{ (cluster.provider || 'k8s').toUpperCase() }} · {{ cluster.region || 'local' }}</span>
          <span>·</span>
          <span class="font-mono text-emerald">{{ cluster.nodes ?? 0 }} Nodes</span>
          <StatusBadge :status="cluster.health_status || cluster.status || 'unknown'" size="sm" />
        </div>
      </div>

      <div class="mobile-stream-actions">
        <button
          class="btn btn-secondary btn-xs"
          :disabled="actionLoading === cluster.id"
          title="Discover Resources"
          @click="emit('discover', cluster)"
        >
          🔍
        </button>
        <button
          class="btn btn-secondary btn-xs"
          :disabled="actionLoading === cluster.id"
          title="Upgrade Cluster"
          @click="emit('upgrade', cluster)"
        >
          ⬆️
        </button>
        <button
          class="btn btn-secondary btn-xs btn-evict"
          :disabled="actionLoading === cluster.id"
          title="Evict Cluster"
          @click="emit('remove', cluster)"
        >
          🗑️
        </button>
      </div>
    </div>
  </div>
</template>
