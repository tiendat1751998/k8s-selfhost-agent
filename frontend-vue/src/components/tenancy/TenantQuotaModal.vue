<script setup lang="ts">
import { ref, watch } from 'vue'
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
  (e: 'save', payload: { orgId: string; quota: Record<string, unknown>; org: Organization }): void
}>()

const maxPods = ref(50)
const maxProjects = ref(10)
const maxMembers = ref(25)

watch(() => props.selectedOrg, (org) => {
  if (org) {
    if (org.tier?.toLowerCase().includes('enterprise')) {
      maxPods.value = 100
      maxProjects.value = 25
      maxMembers.value = 50
    } else {
      maxPods.value = 50
      maxProjects.value = 10
      maxMembers.value = 25
    }
  }
}, { immediate: true })

function handleSave(close: () => void) {
  if (!props.selectedOrg) return
  emit('save', {
    orgId: props.selectedOrg.id,
    quota: {
      max_pods: Math.max(1, maxPods.value || 50),
      max_projects: Math.max(1, maxProjects.value || 10),
      max_members: Math.max(1, maxMembers.value || 25),
      tier: props.selectedOrg.tier
    },
    org: props.selectedOrg
  })
  close()
}
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
          <span class="quota-metric-values font-mono">{{ stats[selectedOrg.id]?.workloadCount ?? 0 }} / {{ maxPods }} pods</span>
        </div>
        <div class="quota-track">
          <div
            class="quota-fill quota-fill-cyan"
            :style="{ width: Math.min(100, ((stats[selectedOrg.id]?.workloadCount ?? 0) / maxPods) * 100) + '%' }"
          ></div>
        </div>
      </div>

      <div class="quota-bar-group">
        <div class="quota-bar-header">
          <span class="quota-metric-name">Project Namespaces</span>
          <span class="quota-metric-values font-mono">{{ stats[selectedOrg.id]?.projectCount ?? 0 }} / {{ maxProjects }} namespaces</span>
        </div>
        <div class="quota-track">
          <div
            class="quota-fill quota-fill-emerald"
            :style="{ width: Math.min(100, ((stats[selectedOrg.id]?.projectCount ?? 0) / maxProjects) * 100) + '%' }"
          ></div>
        </div>
      </div>

      <div class="quota-bar-group">
        <div class="quota-bar-header">
          <span class="quota-metric-name">SSO Identity Seats</span>
          <span class="quota-metric-values font-mono">{{ stats[selectedOrg.id]?.memberCount ?? 0 }} / {{ maxMembers }} seats</span>
        </div>
        <div class="quota-track">
          <div
            class="quota-fill quota-fill-amber"
            :style="{ width: Math.min(100, ((stats[selectedOrg.id]?.memberCount ?? 0) / maxMembers) * 100) + '%' }"
          ></div>
        </div>
      </div>

      <div class="quota-limits-form" style="margin-top: 16px; border-top: 1px solid var(--color-border-subtle); padding-top: 14px;">
        <label class="font-bold text-xs" style="color: var(--text-primary); display: block; margin-bottom: 8px;">Quota Policy Limits</label>
        <div style="display: grid; grid-template-columns: 1fr 1fr 1fr; gap: 8px;">
          <div class="form-group">
            <label class="text-xs text-muted">Max Pods</label>
            <input v-model.number="maxPods" type="number" min="1" max="1000" class="input-glass" />
          </div>
          <div class="form-group">
            <label class="text-xs text-muted">Max Namespaces</label>
            <input v-model.number="maxProjects" type="number" min="1" max="100" class="input-glass" />
          </div>
          <div class="form-group">
            <label class="text-xs text-muted">Max SSO Seats</label>
            <input v-model.number="maxMembers" type="number" min="1" max="500" class="input-glass" />
          </div>
        </div>
      </div>
    </div>
    <template #footer="{ close }">
      <button class="btn btn-secondary" type="button" @click="close">Close</button>
      <button
        class="btn btn-primary"
        type="button"
        @click="handleSave(close)"
      >
        Save Quota
      </button>
    </template>
  </ModalDrawer>
</template>
