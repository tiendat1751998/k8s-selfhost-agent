<script setup lang="ts">
import type { Organization } from '../../api/management'
import type { TenantStatSummary } from '../../composables/useTenancyRbac'

interface Props {
  organizations: Organization[]
  stats?: Record<string, TenantStatSummary>
}

withDefaults(defineProps<Props>(), {
  stats: () => ({})
})

const emit = defineEmits<{
  (e: 'openMembers', orgId: string): void
  (e: 'openRbac'): void
  (e: 'openQuota', org: Organization): void
  (e: 'deleteOrg', orgId: string): void
  (e: 'createOrg'): void
}>()

function confirmDelete(org: Organization) {
  if (window.confirm(`Are you sure you want to purge organization container "${org.name}" (${org.id})?`)) {
    emit('deleteOrg', org.id)
  }
}
</script>

<template>
  <div class="mobile-card-stream">
    <!-- Dedicated Empty State when 0 organizations configured -->
    <div
      v-if="organizations.length === 0"
      class="tenancy-empty-mobile glass-panel"
      role="button"
      tabindex="0"
      @click="emit('createOrg')"
      @keydown.enter="emit('createOrg')"
    >
      <p class="empty-mobile-text">🏢 No organizations configured yet. Tap + to onboard a new tenant workspace.</p>
    </div>

    <!-- High-density Tenant Cards (68-75px height) -->
    <div
      v-for="org in organizations"
      v-else
      :key="org.id"
      class="mobile-stream-card glass-panel"
    >
      <div class="card-left">
        <div class="mobile-avatar">{{ org.name.charAt(0).toUpperCase() }}</div>
        <div class="mobile-meta">
          <div class="title-row">
            <span class="mobile-title" :title="org.name">{{ org.name }}</span>
            <span class="mobile-tier font-mono" :title="org.tier">{{ org.tier }}</span>
          </div>
          <div class="stats-row">
            <span class="font-mono text-muted text-truncate" :title="org.id">{{ org.id }}</span>
            <span class="dot-sep">•</span>
            <span class="font-mono text-cyan">{{ stats[org.id]?.projectCount ?? 0 }} namespaces</span>
            <span class="dot-sep">•</span>
            <span class="font-mono text-emerald">{{ stats[org.id]?.workloadCount ?? 0 }} pods</span>
          </div>
        </div>
      </div>

      <div class="card-right">
        <button
          class="btn-icon-mobile members"
          title="Manage Organization Members"
          aria-label="Manage Members"
          @click="emit('openMembers', org.id)"
        >
          👥
        </button>
        <button
          class="btn-icon-mobile rbac"
          title="Configure RBAC Roles"
          aria-label="Configure RBAC"
          @click="emit('openRbac')"
        >
          🛡️
        </button>
        <button
          class="btn-icon-mobile quota"
          title="Configure Resource Quotas"
          aria-label="Configure Quota"
          @click="emit('openQuota', org)"
        >
          ⚙️
        </button>
        <button
          class="btn-icon-mobile delete"
          title="Purge Organization Container"
          aria-label="Purge Organization"
          @click="confirmDelete(org)"
        >
          🗑️
        </button>
      </div>
    </div>
  </div>
</template>
