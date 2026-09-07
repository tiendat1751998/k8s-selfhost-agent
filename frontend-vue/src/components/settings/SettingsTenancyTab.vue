<script setup lang="ts">
import { reactive } from 'vue'

const props = defineProps<{
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
      <button type="button" class="btn btn-secondary btn-sm" @click="handleReset">
        <span>↺ Reset Defaults</span>
      </button>
    </div>

    <form class="settings-form" @submit.prevent="handleSave">
      <div class="form-group">
        <label class="form-label" for="tenancy-mode">
          <span>Isolation Architecture</span>
          <span class="required">*</span>
        </label>
        <p class="field-desc">Determines how tenant boundaries are partitioned on physical infrastructure.</p>
        <select id="tenancy-mode" v-model="tenancyConfig.isolation_mode" class="input-glass form-select">
          <option value="namespace">📦 Namespace Isolation (ResourceQuota + LimitRange + NetworkPolicy)</option>
          <option value="vcluster">🌐 Virtual Cluster (vCluster Synced Control Plane)</option>
          <option value="dedicated_nodes">⚡ Dedicated Worker Node Pools (Taints & Tolerations)</option>
        </select>
      </div>

      <div class="form-row">
        <div class="form-group flex-1">
          <label class="form-label" for="tenant-cpu">
            <span>Default CPU Quota (Cores)</span>
          </label>
          <p class="field-desc">Default aggregate vCPU allocatable per tenant.</p>
          <input
            id="tenant-cpu"
            v-model="tenancyConfig.default_cpu_limit"
            type="number"
            min="1"
            max="256"
            class="input-glass form-input"
          />
        </div>

        <div class="form-group flex-1">
          <label class="form-label" for="tenant-mem">
            <span>Default RAM Quota (GiB)</span>
          </label>
          <p class="field-desc">Default memory reservation threshold per tenant.</p>
          <input
            id="tenant-mem"
            v-model="tenancyConfig.default_memory_limit"
            type="number"
            min="1"
            max="1024"
            class="input-glass form-input"
          />
        </div>

        <div class="form-group flex-1">
          <label class="form-label" for="tenant-storage">
            <span>Default Storage Quota (GiB)</span>
          </label>
          <p class="field-desc">Default persistent volume claim quota.</p>
          <input
            id="tenant-storage"
            v-model="tenancyConfig.default_storage_limit"
            type="number"
            min="10"
            max="10000"
            class="input-glass form-input"
          />
        </div>
      </div>

      <div class="form-group toggle-group">
        <div class="toggle-info">
          <span id="lbl-strict-network" class="toggle-label">Strict Cross-Tenant Network Isolation (Default-Deny)</span>
          <p class="field-desc">
            Automatically inject Calico/Cilium NetworkPolicies blocking inter-namespace east-west traffic unless whitelisted.
          </p>
        </div>
        <label class="toggle-switch">
          <input id="toggle-strict-network" v-model="tenancyConfig.strict_network_isolation" type="checkbox" aria-labelledby="lbl-strict-network" />
          <span class="toggle-slider"></span>
        </label>
      </div>

      <div class="form-group toggle-group">
        <div class="toggle-info">
          <span id="lbl-auto-ingress" class="toggle-label">Auto-Provision Dedicated Ingress Subdomain</span>
          <p class="field-desc">
            Allocate an isolated Traefik/Ingress endpoint per newly registered tenant (e.g. tenant-name.cluster.local).
          </p>
        </div>
        <label class="toggle-switch">
          <input id="toggle-auto-ingress" v-model="tenancyConfig.auto_provision_ingress" type="checkbox" aria-labelledby="lbl-auto-ingress" />
          <span class="toggle-slider"></span>
        </label>
      </div>

      <div class="form-actions">
        <button type="submit" class="btn btn-primary" :disabled="saving">
          <span>{{ saving ? '💾 Saving Changes...' : '💾 Save Tenancy Settings' }}</span>
        </button>
      </div>
    </form>
  </div>
</template>
