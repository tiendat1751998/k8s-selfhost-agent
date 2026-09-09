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
}>()

function confirmDelete(org: Organization) {
  if (window.confirm(`Are you sure you want to purge organization container "${org.name}" (${org.id})?`)) {
    emit('deleteOrg', org.id)
  }
}
</script>

<template>
  <div class="mobile-card-stream">
    <div
      v-for="org in organizations"
      :key="org.id"
      class="mobile-stream-card glass-panel"
    >
      <div class="card-left">
        <div class="mobile-avatar">{{ org.name.charAt(0).toUpperCase() }}</div>
        <div class="mobile-meta">
          <div class="title-row">
            <span class="mobile-title">{{ org.name }}</span>
            <span class="mobile-tier">{{ org.tier }}</span>
          </div>
          <div class="stats-row">
            <span class="font-mono text-muted">{{ org.id }}</span>
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
          title="Members"
          @click="emit('openMembers', org.id)"
        >
          👥
        </button>
        <button
          class="btn-icon-mobile rbac"
          title="RBAC"
          @click="emit('openRbac')"
        >
          🔑
        </button>
        <button
          class="btn-icon-mobile quota"
          title="Quota"
          @click="emit('openQuota', org)"
        >
          ⚙️
        </button>
        <button
          class="btn-icon-mobile delete"
          title="Delete"
          @click="confirmDelete(org)"
        >
          🗑
        </button>
      </div>
    </div>
  </div>
</template>
