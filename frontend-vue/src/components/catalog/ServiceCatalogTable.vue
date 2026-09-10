<script setup lang="ts">
import DataTable, { type Column } from '../ui/DataTable.vue'
import type { ServiceEntry } from '../../api/catalog'

defineProps<{
  services: ServiceEntry[]
  columns: Column<ServiceEntry>[]
  loading: boolean
  error: string | null
  getTypeIcon: (type: string) => string
  getTypeBadgeClass: (type: string) => string
  getLifecycleBadgeClass: (lifecycle: string) => string
  getLifecycleDotClass: (lifecycle: string) => string
}>()

function handleApiClick(service: ServiceEntry) {
  if (service.docs_url) {
    window.open(service.docs_url, '_blank', 'noopener,noreferrer')
  } else {
    emit('open-detail', service)
  }
}

const emit = defineEmits<{
  (e: 'open-detail', service: ServiceEntry): void
  (e: 'deploy', service: ServiceEntry): void
  (e: 'config', service: ServiceEntry): void
  (e: 'delete', service: ServiceEntry): void
}>()
</script>

<template>
  <div class="section-box glass-panel table-box">
    <div class="box-header">
      <div class="box-header-title">
        <h2 class="box-title">Registered Services & Component Directory</h2>
        <p class="box-subtitle">
          Backstage-compliant software inventory with metadata links and live Kubernetes annotations.
        </p>
      </div>
      <div class="box-header-meta">
        <span class="badge badge-cyan font-mono">{{ services.length }} Services</span>
      </div>
    </div>

    <DataTable
      :columns="columns"
      :data="services"
      :loading="loading"
      :error="error"
      empty-message="No services registered matching current filter criteria."
      searchable
      search-placeholder="Filter loaded rows by name, owner, repo..."
    >
      <!-- Cell: Name -->
      <template #cell-name="{ row }">
        <div class="service-name-cell">
          <div class="type-mini-icon">{{ getTypeIcon(row.type) }}</div>
          <div class="name-meta">
            <a
              href="javascript:void(0)"
              class="service-title-link font-mono"
              @click="emit('open-detail', row)"
            >
              {{ row.name }}
            </a>
            <span v-if="row.description" class="service-desc-snippet">
              {{ row.description }}
            </span>
          </div>
        </div>
      </template>

      <!-- Cell: Type -->
      <template #cell-type="{ row }">
        <span class="type-badge" :class="getTypeBadgeClass(row.type)">
          <span class="type-icon-dot">{{ getTypeIcon(row.type) }}</span>
          <span>{{ row.type }}</span>
        </span>
      </template>

      <!-- Cell: Lifecycle -->
      <template #cell-lifecycle="{ row }">
        <span class="lifecycle-badge" :class="getLifecycleBadgeClass(row.lifecycle)">
          <span class="lifecycle-dot" :class="getLifecycleDotClass(row.lifecycle)"></span>
          <span>{{ row.lifecycle }}</span>
        </span>
      </template>

      <!-- Cell: Owner -->
      <template #cell-owner_team="{ row }">
        <div class="owner-cell">
          <span class="owner-team font-mono">{{ row.owner_team || 'Unassigned' }}</span>
          <span v-if="row.owner_email" class="owner-email text-muted font-mono">
            {{ row.owner_email }}
          </span>
        </div>
      </template>

      <!-- Cell: Endpoint -->
      <template #cell-endpoint="{ row }">
        <div v-if="row.annotations && row.annotations['api.endpoint']" class="endpoint-cell font-mono">
          <span class="endpoint-badge" :title="row.annotations['api.endpoint']">
            ⚡ {{ row.annotations['api.endpoint'] }}
          </span>
        </div>
        <div v-else-if="row.docs_url" class="endpoint-cell font-mono">
          <a :href="row.docs_url" target="_blank" rel="noopener noreferrer" class="endpoint-link">
            📖 Docs Spec ↗
          </a>
        </div>
        <span v-else class="text-muted font-mono">-</span>
      </template>

      <!-- Cell: Repo -->
      <template #cell-repo_url="{ row }">
        <div v-if="row.repo_url" class="repo-cell">
          <a
            :href="row.repo_url"
            target="_blank"
            rel="noopener noreferrer"
            class="repo-link font-mono"
            title="Open Git Repository"
          >
            <span>🔗 Repo</span>
            <span class="external-icon">↗</span>
          </a>
        </div>
        <span v-else class="text-muted font-mono">-</span>
      </template>

      <!-- Cell: Tags -->
      <template #cell-tags="{ row }">
        <div v-if="row.tags && row.tags.length > 0" class="tags-pill-wrap">
          <span
            v-for="(tag, idx) in row.tags.slice(0, 3)"
            :key="idx"
            class="tag-pill font-mono"
          >
            #{{ tag }}
          </span>
          <span v-if="row.tags.length > 3" class="tag-overflow font-mono">
            +{{ row.tags.length - 3 }}
          </span>
        </div>
        <span v-else class="text-muted font-mono">-</span>
      </template>

      <!-- Cell: Actions with labeled buttons -->
      <template #cell-actions="{ row }">
        <div class="table-actions-row">
          <button
            type="button"
            class="btn btn-secondary btn-xs btn-action-labeled"
            title="View full service details"
            @click="emit('open-detail', row)"
          >
            <span>🔍 Details</span>
          </button>
          <button
            type="button"
            class="btn btn-secondary btn-xs btn-action-labeled"
            title="API specifications & documentation"
            @click="handleApiClick(row)"
          >
            <span>⚡ APIs</span>
          </button>
          <button
            type="button"
            class="btn btn-secondary btn-xs btn-action-labeled"
            title="Configure service registration"
            @click="emit('config', row)"
          >
            <span>✏️ Edit</span>
          </button>
        </div>
      </template>
    </DataTable>
  </div>
</template>
