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
.cluster-drawer-content { display: flex; flex-direction: column; gap: 14px; }
.overview-meta-card { padding: 14px; border-radius: 12px; background: rgba(11, 15, 25, 0.7); display: flex; flex-direction: column; gap: 10px; border: 1px solid rgba(255, 255, 255, 0.07); }
.card-title-row { display: flex; justify-content: space-between; align-items: center; }
.cluster-brand { display: flex; align-items: center; gap: 10px; }
.cluster-icon { font-size: 22px; color: var(--accent-cyan); }
.cluster-name { font-size: 15px; font-weight: 700; color: #fff; }
.meta-grid { display: grid; grid-template-columns: repeat(2, 1fr); gap: 8px; background: rgba(0, 0, 0, 0.25); padding: 8px 10px; border-radius: 8px; }
.meta-item { display: flex; flex-direction: column; gap: 2px; }
.meta-lbl { font-size: 10px; color: var(--text-muted); text-transform: uppercase; }
.meta-val { font-size: 12px; font-weight: 600; }
.drawer-action-bar .btn { width: 100%; justify-content: center; }
.discovery-section { padding: 12px; border-radius: 10px; display: flex;
  flex-direction: column; gap: 8px; background: rgba(11, 15, 25, 0.5); border: 1px solid var(--border-subtle); }	
.sec-title { font-size: 11.5px; color: #fff; font-weight: 700; }
.labels-chips { display: flex; flex-wrap: wrap; gap: 5px; }
.one-chip { font-size: 10.5px; background: rgba(6, 182, 212, 0.12); color: #38bdf8; border: 1px solid rgba(6, 182, 212, 0.25); padding: 2px 7px; border-radius: 5px; }
.empty-drawer { padding: 36px; text-align: center; font-size: 13px; }
.font-mono { font-family: var(--font-mono); }
.text-cyan { color: var(--accent-cyan); }
.text-emerald { color: var(--accent-emerald); }
.text-muted { color: var(--text-muted); }
.text-xs { font-size: 11px; }
</style>
