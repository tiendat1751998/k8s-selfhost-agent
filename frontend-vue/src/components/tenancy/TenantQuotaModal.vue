<script setup lang="ts">
import ModalDrawer from '../ui/ModalDrawer.vue'
import type { Organization } from '../../api/management'
import type { TenantStatSummary } from '../../composables/useTenancyRbac'

const props = defineProps<{
  show: boolean
  selectedOrg: Organization | null
  stats: Record<string, TenantStatSummary>
}>()

const emit = defineEmits<{
  (e: 'update:show', val: boolean): void
  (e: 'save', org: Organization): void
}>()
</script>

<template>
  <ModalDrawer
    :show="show"
    :title="selectedOrg ? `Resource Quota: ${selectedOrg.name}` : 'Tenant Resource Quota'"
    subtitle="Cluster compute boundaries, pod allocation limits, and namespace quotas."
    @update:show="emit('update:show', $event)"
  >
    <div v-if="selectedOrg" class="quota-modal-body">
      <div class="quota-info-banner">
        <span class="font-mono text-cyan">{{ selectedOrg.id }}</span>
        <span class="quota-tier-tag font-mono">{{ selectedOrg.tier }}</span>
      </div>

      <div class="quota-bar-group">
        <div class="quota-bar-header">
          <span class="quota-metric-name">Workload Pods</span>
          <span class="quota-metric-values font-mono">{{ stats[selectedOrg.id]?.workloadCount ?? 0 }} / 50 pods</span>
        </div>
        <div class="quota-track">
          <div
            class="quota-fill quota-fill-cyan"
            :style="{ width: Math.min(100, ((stats[selectedOrg.id]?.workloadCount ?? 0) / 50) * 100) + '%' }"
          ></div>
        </div>
      </div>

      <div class="quota-bar-group">
        <div class="quota-bar-header">
          <span class="quota-metric-name">Project Namespaces</span>
          <span class="quota-metric-values font-mono">{{ stats[selectedOrg.id]?.projectCount ?? 0 }} / 10 namespaces</span>
        </div>
        <div class="quota-track">
          <div
            class="quota-fill quota-fill-emerald"
            :style="{ width: Math.min(100, ((stats[selectedOrg.id]?.projectCount ?? 0) / 10) * 100) + '%' }"
          ></div>
        </div>
      </div>

      <div class="quota-bar-group">
        <div class="quota-bar-header">
          <span class="quota-metric-name">SSO Identity Seats</span>
          <span class="quota-metric-values font-mono">{{ stats[selectedOrg.id]?.memberCount ?? 0 }} / 25 seats</span>
        </div>
        <div class="quota-track">
          <div
            class="quota-fill quota-fill-amber"
            :style="{ width: Math.min(100, ((stats[selectedOrg.id]?.memberCount ?? 0) / 25) * 100) + '%' }"
          ></div>
        </div>
      </div>
    </div>
    <template #footer="{ close }">
      <button class="btn btn-secondary" type="button" @click="close">Close</button>
      <button
        class="btn btn-primary"
        type="button"
        @click="selectedOrg && emit('save', selectedOrg); close()"
      >
        Save Quota
      </button>
    </template>
  </ModalDrawer>
</template>
