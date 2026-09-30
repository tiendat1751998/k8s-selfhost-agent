<script setup lang="ts">
import type { Organization } from '../../api/management'
import type { TenantStatSummary } from '../../composables/useTenancyRbac'
import ActionDropdown from '../ui/ActionDropdown.vue'

interface Props {
  organizations: Organization[]
  stats?: Record<string, TenantStatSummary>
  loading?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  stats: () => ({}),
  loading: false
})

const emit = defineEmits<{
  (e: 'openMembers', orgId: string): void
  (e: 'openRbac'): void
  (e: 'openQuota', org: Organization): void
  (e: 'deleteOrg', orgId: string): void
  (e: 'createOrg'): void
}>()

function confirmDelete(org: Organization) {
  if (window.confirm(`Are you sure you want to purge ORGANIZATION "${org.name}" (${org.id})?`)) {
    emit('deleteOrg', org.id)
  }
}
</script>

<template>
  <div class="tenant-table-wrapper glass-panel">
    

    <div v-if="loading" class="loading-state">
      <div class="spinner"></div>
      <span>Loading ORGANIZATIONs...</span>
    </div>

    <div v-else-if="organizations.length === 0" class="empty-list">
      <p>No tenant organizations found matching the criteria.</p>
    </div>

    <div v-else class="table-scroll">
      <table class="tenant-table">
        <colgroup>
          <col style="width: 21%;" />
          <col style="width: 14%;" />
          <col style="width: 11%;" />
          <col style="width: 11%;" />
          <col style="width: 9%;" />
          <col style="width: 14%;" />
          <col style="width: 20%;" />
        </colgroup>
        <thead>
          <tr>
            <th class="th-left">ORGANIZATION</th>
            <th class="th-tier">TIER</th>
            <th class="th-namespaces">NAMESPACES</th>
            <th class="th-num">PODS</th>
            <th class="th-num">MEMBERS</th>
            <th class="th-status">STATUS</th>
            <th class="th-actions">ACTIONS</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="org in organizations" :key="org.id" class="tenant-row">
            <td class="td-left">
              <div class="tenant-info-cell">
                <div class="tenant-icon">{{ org.name.charAt(0).toUpperCase() }}</div>
                <div class="tenant-details">
                  <span class="tenant-title" :title="org.name">{{ org.name }}</span>
                  <span class="tenant-slug font-mono" :title="org.id">{{ org.id }}</span>
                </div>
              </div>
            </td>
            <td>
              <span class="tier-pill font-mono" :title="org.tier">{{ org.tier }}</span>
            </td>
            <td class="td-namespaces">
              <span class="stat-num font-mono text-cyan" :title="`${stats[org.id]?.projectCount ?? 0} namespaces`">
                {{ stats[org.id]?.projectCount ?? 0 }}
              </span>
            </td>
            <td>
              <span class="stat-num font-mono muted-pill" :title="`${stats[org.id]?.workloadCount ?? 0} pods`">
                {{ stats[org.id]?.workloadCount ?? 0 }} pods
              </span>
            </td>
            <td>
              <span class="stat-num font-mono" :title="`${stats[org.id]?.memberCount ?? 0} members`">
                {{ stats[org.id]?.memberCount ?? 0 }}
              </span>
            </td>
            <td>
              <span class="status-dot-muted"><span class="dot"></span> ISOLATED</span>
            </td>
            <td class="td-right">
              <div class="tenant-action-buttons" style="gap: 8px;">
                <button class="btn btn-xs btn-secondary" @click="emit('openMembers', org.id)">Members</button>
                <ActionDropdown
                  :items="[
                    { id: 'rbac', label: 'Manage RBAC' },
                    { id: 'quota', label: 'Resource Quotas' },
                    { id: 'delete', label: 'Delete Organization', variant: 'danger' }
                  ]"
                  @select="(id) => {
                    if (id === 'rbac') emit('openRbac');
                    if (id === 'quota') emit('openQuota', org);
                    if (id === 'delete') confirmDelete(org);
                  }"
                />
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

