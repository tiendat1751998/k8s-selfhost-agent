<script setup lang="ts">
import { reactive } from 'vue'
import BaseIcon from '../ui/BaseIcon.vue'

defineProps<{
  saving: boolean
}>()

const emit = defineEmits<{
  (e: 'save', category: 'tenancy', config: typeof tenancyConfig): void
  (e: 'reset', category: 'tenancy'): void
}>()

const tenancyConfig = reactive({
  isolation_mode: 'namespace',
  default_cpu_limit: '16',
  default_memory_limit: '32',
  default_storage_limit: '200',
  strict_network_isolation: true,
  auto_provision_ingress: false,
})

function handleReset() {
  tenancyConfig.isolation_mode = 'namespace'
  tenancyConfig.default_cpu_limit = '16'
  tenancyConfig.default_memory_limit = '32'
  tenancyConfig.default_storage_limit = '200'
  tenancyConfig.strict_network_isolation = true
  tenancyConfig.auto_provision_ingress = false
  emit('reset', 'tenancy')
}

function handleSave() {
  emit('save', 'tenancy', tenancyConfig)
}
</script>

<template>
  <div class="settings-card glass-panel animate-fade-in">
    <div class="card-header">
      <div>
        <h2 class="card-title">Multi-Tenancy & Resource Governance</h2>
        <p class="card-subtitle">
          Configure default namespace isolation boundaries, tenant resource caps, and cross-boundary network security.
        </p>
      </div>
    </div>

    <form class="settings-form" @submit.prevent="handleSave">
      <!-- Row 1: Isolation Architecture -->
      <div class="setting-row">
        <div class="setting-meta">
          <label class="setting-title" for="tenancy-mode">
            <span>Isolation Architecture</span>
            <span class="required">*</span>
          </label>
          <p class="setting-desc">Determines how tenant boundaries are partitioned on physical infrastructure.</p>
        </div>
        <div class="setting-control-col">
          <select id="tenancy-mode" v-model="tenancyConfig.isolation_mode" class="input-glass form-select input-compact-md">
            <option value="namespace">Namespace Isolation (ResourceQuota)</option>
            <option value="vcluster">Virtual Cluster (vCluster Synced)</option>
            <option value="dedicated_nodes">Dedicated Worker Pools (Taints)</option>
          </select>
        </div>
      </div>

      <!-- Row 2: Default CPU Quota -->
      <div class="setting-row">
        <div class="setting-meta">
          <label class="setting-title" for="tenant-cpu">Default CPU Quota</label>
          <p class="setting-desc">Default aggregate vCPU allocatable per tenant.</p>
        </div>
        <div class="setting-control-col">
          <div class="input-addon-wrap">
            <input
              id="tenant-cpu"
              v-model="tenancyConfig.default_cpu_limit"
              type="number"
              min="1"
              max="256"
              class="input-glass form-input"
              style="width: 120px;"
            />
            <span class="input-addon">Cores</span>
          </div>
        </div>
      </div>

      <!-- Row 3: Default RAM Quota -->
      <div class="setting-row">
        <div class="setting-meta">
          <label class="setting-title" for="tenant-mem">Default RAM Quota</label>
          <p class="setting-desc">Default memory reservation threshold per tenant.</p>
        </div>
        <div class="setting-control-col">
          <div class="input-addon-wrap">
            <input
              id="tenant-mem"
              v-model="tenancyConfig.default_memory_limit"
              type="number"
              min="1"
              max="1024"
              class="input-glass form-input"
              style="width: 120px;"
            />
            <span class="input-addon">GiB</span>
          </div>
        </div>
      </div>

      <!-- Row 4: Default Storage Quota -->
      <div class="setting-row">
        <div class="setting-meta">
          <label class="setting-title" for="tenant-storage">Default Storage Quota</label>
          <p class="setting-desc">Default persistent volume claim quota.</p>
        </div>
        <div class="setting-control-col">
          <div class="input-addon-wrap">
            <input
              id="tenant-storage"
              v-model="tenancyConfig.default_storage_limit"
              type="number"
              min="10"
              max="10000"
              class="input-glass form-input"
              style="width: 120px;"
            />
            <span class="input-addon">GiB</span>
          </div>
        </div>
      </div>

      <!-- Row 5: Strict Network Isolation -->
      <div class="setting-row">
        <div class="setting-meta">
          <label class="setting-title" for="toggle-strict-network">Strict Cross-Tenant Network Isolation</label>
          <p class="setting-desc">
            Automatically inject Calico/Cilium NetworkPolicies blocking inter-namespace east-west traffic unless whitelisted.
          </p>
        </div>
        <div class="setting-control-col">
          <label class="toggle-switch">
            <input id="toggle-strict-network" v-model="tenancyConfig.strict_network_isolation" type="checkbox" aria-label="Strict Cross-Tenant Network Isolation" />
            <span class="toggle-slider"></span>
          </label>
        </div>
      </div>

      <!-- Row 6: Auto-Provision Dedicated Ingress -->
      <div class="setting-row">
        <div class="setting-meta">
          <label class="setting-title" for="toggle-auto-ingress">Auto-Provision Dedicated Ingress Subdomain</label>
          <p class="setting-desc">
            Allocate an isolated Traefik/Ingress endpoint per newly registered tenant (e.g. tenant-name.cluster.local).
          </p>
        </div>
        <div class="setting-control-col">
          <label class="toggle-switch">
            <input id="toggle-auto-ingress" v-model="tenancyConfig.auto_provision_ingress" type="checkbox" aria-label="Auto-Provision Dedicated Ingress Subdomain" />
            <span class="toggle-slider"></span>
          </label>
        </div>
      </div>

      <!-- Card Actions Bar -->
      <div class="card-actions-bar">
        <button type="button" class="btn btn-secondary btn-sm" @click="handleReset">
          <BaseIcon name="rotate-ccw" size="xs" /> <span>Reset Defaults</span>
        </button>
        <button type="submit" class="btn btn-primary btn-sm" :disabled="saving">
          <BaseIcon name="hard-drive" size="xs" /> <span>{{ saving ? 'Saving Changes...' : 'Save Tenancy Settings' }}</span>
        </button>
      </div>
    </form>
  </div>
</template>
