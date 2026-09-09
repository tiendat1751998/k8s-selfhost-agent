<script setup lang="ts">
import { ref, computed } from 'vue'
import type { Organization } from '../../api/management'
import type { TenantStatSummary } from '../../composables/useTenancyRbac'
import StatusBadge from '../ui/StatusBadge.vue'

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

const searchQuery = ref('')

const filteredOrgs = computed(() => {
  if (!searchQuery.value.trim()) return props.organizations
  const q = searchQuery.value.toLowerCase().trim()
  return props.organizations.filter(
    org => org.name.toLowerCase().includes(q) || org.id.toLowerCase().includes(q) || org.tier.toLowerCase().includes(q)
  )
})

function confirmDelete(org: Organization) {
  if (window.confirm(`Are you sure you want to purge organization container "${org.name}" (${org.id})?`)) {
    emit('deleteOrg', org.id)
  }
}
</script>

<template>
  <div class="tenant-table-wrapper glass-panel">
    <div class="table-toolbar">
      <div class="toolbar-search">
        <span class="search-icon">🔍</span>
        <input
          v-model="searchQuery"
          type="text"
          placeholder="Filter organizations by name, slug, or tier..."
          class="input-glass search-input"
        />
        <button v-if="searchQuery" class="clear-btn" @click="searchQuery = ''">✕</button>
      </div>

      <div class="toolbar-actions">
        <span class="tenant-count-badge">{{ filteredOrgs.length }} Organizations</span>
        <button class="btn btn-primary btn-sm" @click="emit('createOrg')">
          <span>+ New Organization</span>
        </button>
      </div>
    </div>

    <div v-if="loading" class="loading-state">
      <div class="spinner"></div>
      <span>Loading organization containers...</span>
    </div>

    <div v-else-if="filteredOrgs.length === 0" class="empty-list">
      <p>No tenant organizations found matching the criteria.</p>
    </div>

    <div v-else class="table-scroll">
      <table class="tenant-table">
        <thead>
          <tr>
            <th class="th-left">Organization Container</th>
            <th>Service Tier</th>
            <th>Namespaces</th>
            <th>Workload Pods</th>
            <th>SSO Members</th>
            <th>Boundary Status</th>
            <th class="th-right">Cluster Actions</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="org in filteredOrgs" :key="org.id" class="tenant-row">
            <td class="td-left">
              <div class="tenant-info-cell">
                <div class="tenant-icon">{{ org.name.charAt(0).toUpperCase() }}</div>
                <div class="tenant-details">
                  <span class="tenant-title">{{ org.name }}</span>
                  <span class="tenant-slug font-mono">{{ org.id }}</span>
                </div>
              </div>
            </td>
            <td>
              <span class="tenant-tier-chip font-mono">{{ org.tier }}</span>
            </td>
            <td>
              <span class="stat-num font-mono text-cyan">{{ stats[org.id]?.projectCount ?? 0 }}</span>
            </td>
            <td>
              <span class="stat-num font-mono text-emerald">{{ stats[org.id]?.workloadCount ?? 0 }} pods</span>
            </td>
            <td>
              <span class="stat-num font-mono">{{ stats[org.id]?.memberCount ?? 0 }}</span>
            </td>
            <td>
              <StatusBadge status="healthy" label="ISOLATED" size="sm" />
            </td>
            <td class="td-right">
              <div class="tenant-action-buttons">
                <button
                  class="action-btn action-btn-members"
                  title="Manage Organization Members"
                  @click="emit('openMembers', org.id)"
                >
                  <span>👥 Members</span>
                </button>
                <button
                  class="action-btn action-btn-rbac"
                  title="Configure RBAC Roles"
                  @click="emit('openRbac')"
                >
                  <span>🔑 RBAC</span>
                </button>
                <button
                  class="action-btn action-btn-quota"
                  title="Configure Resource Quotas"
                  @click="emit('openQuota', org)"
                >
                  <span>⚙️ Quota</span>
                </button>
                <button
                  class="action-btn action-btn-delete"
                  title="Purge Organization Container"
                  @click="confirmDelete(org)"
                >
                  <span>🗑 Delete</span>
                </button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
