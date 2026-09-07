<script setup lang="ts">
import ModalDrawer from '../ui/ModalDrawer.vue'
import type { Cluster, ClusterDiscoveryData } from '../../api/fleet'

defineProps<{
  show: boolean
  cluster: Cluster | null
  discoveredData: ClusterDiscoveryData | null
}>()

const emit = defineEmits<{
  (e: 'update:show', val: boolean): void
}>()
</script>

<template>
  <!-- Resource Discovery Drawer -->
  <ModalDrawer
    :show="show"
    mode="drawer"
    :title="`Cluster Discovery: ${cluster?.name || 'Cluster'}`"
    :subtitle="`Provider: ${(cluster?.provider || '').toUpperCase()} · Region: ${cluster?.region || 'unknown'}`"
    max-width="580px"
    @update:show="emit('update:show', $event)"
  >
    <div v-if="discoveredData" class="drawer-content">
      <div class="discovery-header-card">
        <div class="meta-row">
          <span class="text-muted">API Server Latency:</span>
          <span class="text-emerald font-mono">{{ discoveredData.api_server_latency || 'Live (< 25ms)' }}</span>
        </div>
        <div class="meta-row">
          <span class="text-muted">Custom Resource Definitions:</span>
          <span class="font-mono text-cyan">
            {{ discoveredData.crd_count !== undefined ? `${discoveredData.crd_count} CRDs` : 'Discovered in cluster' }}
          </span>
        </div>
        <div v-if="discoveredData.last_sync" class="meta-row">
          <span class="text-muted">Last Synchronized:</span>
          <span class="font-mono text-muted">{{ new Date(discoveredData.last_sync).toLocaleString() }}</span>
        </div>
      </div>

      <div class="discovery-section">
        <h4 class="section-subheading">📦 Discovered Namespaces</h4>
        <div v-if="discoveredData.namespaces && discoveredData.namespaces.length > 0" class="chips-wrap">
          <span v-for="ns in discoveredData.namespaces" :key="ns" class="ns-chip font-mono">
            📁 {{ ns }}
          </span>
        </div>
        <div v-else class="empty-subtext">
          No specific namespace inventory discovered yet.
        </div>
      </div>

      <div class="discovery-section">
        <h4 class="section-subheading">🖥️ Discovered Node Pools</h4>
        <div v-if="discoveredData.node_pools && discoveredData.node_pools.length > 0" class="pools-list">
          <div v-for="(pool, idx) in discoveredData.node_pools" :key="idx" class="pool-item">
            <div class="pool-left">
              <span class="pool-name">{{ pool.name }}</span>
              <span class="pool-inst font-mono">{{ pool.instance || 'Generic Node' }} {{ pool.zone ? `· Zone: ${pool.zone}` : '' }}</span>
            </div>
            <span class="pool-count font-mono">{{ pool.count ?? 1 }} Nodes</span>
          </div>
        </div>
        <div v-else class="empty-subtext">
          {{ cluster?.nodes ? `${cluster.nodes} Worker Nodes active` : 'Node topology details not yet collected.' }}
        </div>
      </div>

      <div class="discovery-section">
        <h4 class="section-subheading">📋 Topology Metadata Payload</h4>
        <pre class="json-preview font-mono">{{ JSON.stringify(discoveredData, null, 2) }}</pre>
      </div>
    </div>

    <div v-else class="empty-state">
      <span>No discovery metadata recorded for this cluster yet. Trigger a discovery scan or re-import kubeconfig.</span>
    </div>

    <template #footer="{ close }">
      <button class="btn btn-secondary" @click="close">Close Drawer</button>
    </template>
  </ModalDrawer>
</template>
