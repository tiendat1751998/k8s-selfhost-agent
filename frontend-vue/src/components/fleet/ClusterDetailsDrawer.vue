<script setup lang="ts">
import { ref } from 'vue'
import ModalDrawer from '../ui/ModalDrawer.vue'
import StatusBadge from '../ui/StatusBadge.vue'
import ClusterEssentialsMatrix from './ClusterEssentialsMatrix.vue'
import BootstrapClusterModal from './BootstrapClusterModal.vue'
import type { Cluster, ClusterDiscoveryData } from '../../api/fleet'

const props = defineProps<{
  show: boolean
  cluster: Cluster | null
  discoveryData?: ClusterDiscoveryData | null
}>()

const emit = defineEmits<{
  (e: 'update:show', val: boolean): void
  (e: 'updated'): void
}>()

const showBootstrapModal = ref(false)
const bootstrapInitialItem = ref<string>('')
const matrixRef = ref<InstanceType<typeof ClusterEssentialsMatrix> | null>(null)

function handleTriggerBootstrap(itemId?: string) {
  bootstrapInitialItem.value = itemId || ''
  showBootstrapModal.value = true
}

function handleBootstrapped() {
  if (matrixRef.value?.refresh) {
    matrixRef.value.refresh()
  }
  emit('updated')
}
</script>

<template>
  <div>
    <ModalDrawer
      :show="show"
      mode="drawer"
      :title="`Cluster Details: ${cluster?.name || 'Cluster'}`"
      :subtitle="`${(cluster?.provider || 'GENERIC').toUpperCase()} · Region: ${cluster?.region || 'local'} · ${cluster?.group || 'default'}`"
      max-width="580px"
      @update:show="(val) => emit('update:show', val)"
    >
      <div v-if="cluster" class="cluster-drawer-content">
        <div class="overview-meta-card glass-panel">
          <div class="card-title-row">
            <div class="cluster-brand">
              <span class="cluster-icon">⧈</span>
              <div>
                <h4 class="cluster-name font-mono">{{ cluster.name }}</h4>
                <span class="cluster-id font-mono text-muted text-xs">{{ cluster.id }}</span>
              </div>
            </div>
            <StatusBadge :status="cluster.health_status || cluster.status || 'unknown'" size="sm" />
          </div>

          <div class="meta-grid font-mono">
            <div class="meta-item">
              <span class="meta-lbl">K8s Version:</span>
              <span class="meta-val text-cyan">{{ cluster.version || 'Unknown' }}</span>
            </div>
            <div class="meta-item">
              <span class="meta-lbl">Active Nodes:</span>
              <span class="meta-val text-emerald">{{ cluster.nodes ?? 1 }} Nodes</span>
            </div>
            <div class="meta-item">
              <span class="meta-lbl">Tier / Group:</span>
              <span class="meta-val text-white">{{ cluster.group || 'default' }}</span>
            </div>
            <div class="meta-item">
              <span class="meta-lbl">Last Audited:</span>
              <span class="meta-val text-muted">
                {{ cluster.last_health_check ? new Date(cluster.last_health_check).toLocaleTimeString() : 'Recently' }}
              </span>
            </div>
          </div>
        </div>

        <div class="drawer-action-bar">
          <button class="btn btn-primary" @click="handleTriggerBootstrap()">
            <span>⚡ Bootstrap Essentials Wizard</span>
          </button>
        </div>

        <ClusterEssentialsMatrix
          ref="matrixRef"
          :cluster-id="cluster.id"
          :cluster-name="cluster.name"
          @bootstrap="handleTriggerBootstrap"
        />

        <div v-if="discoveryData?.namespaces?.length" class="discovery-section glass-panel">
          <h5 class="sec-title font-mono">💘 Discovered Namespaces ({{ discoveryData.namespaces.length }})</h5>
          <div class="chips-wrap">
            <span v-for="ns in discoveryData.namespaces" :key="ns" class="ns-chip font-mono">
              👀 {{ ns }}
            </span>
          </div>
        </div>
      </div>

      <div v-else class="empty-drawer text-muted font-mono">
        No cluster selected.
      </div>

      <template #footer="{close}">
        <button class="btn btn-secondary" @click="close">Close Drawer</button>
      </template>
    </ModalDrawer>


    <BootstrapClusterModal
      v-if="cluster"
      v-model:show="showBootstrapModal"
      :cluster-id="cluster.id"
      :cluster-name="cluster.name"
      :initial-item="bootstrapInitialItem"
      @bootstrapped="handleBootstrapped"
    />
  </div>
</template>

<style scoped>
@import '../../assets/styles/views/fleet.css';
</style>
