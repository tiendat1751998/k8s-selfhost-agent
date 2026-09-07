<script setup lang="ts">
import type { GenericPlatformItem, DynamicColumn } from '../../composables/useGenericPlatform'
import StatusBadge from '../ui/StatusBadge.vue'

const props = withDefaults(
  defineProps<{
    items: GenericPlatformItem[]
    columns: DynamicColumn[]
    sortKey?: string
    sortOrder?: 'asc' | 'desc'
    loading?: boolean
    selectedId?: string
  }>(),
  {
    sortKey: '',
    sortOrder: 'asc',
    loading: false,
    selectedId: undefined
  }
)

const emit = defineEmits<{
  (e: 'sort', key: string): void
  (e: 'inspect', item: GenericPlatformItem): void
  (e: 'action', actionId: string, item: GenericPlatformItem): void
}>()

function handleSort(col: DynamicColumn) {
  if (col.sortable) {
    emit('sort', col.key)
  }
}
</script>

<template>
  <div class="generic-table-wrapper glass-panel">
    <div class="table-scroll-container">
      <table class="generic-table" role="grid" aria-label="Dynamic platform resources table">
        <thead>
          <tr>
            <th
              v-for="col in columns"
              :key="col.key"
              :style="{ width: col.width || 'auto' }"
              :class="{ 'is-sortable': col.sortable, 'is-active-sort': sortKey === col.key }"
              :aria-sort="sortKey === col.key ? (sortOrder === 'asc' ? 'ascending' : 'descending') : undefined"
              @click="handleSort(col)"
            >
              <div class="th-content">
                <span>{{ col.label }}</span>
                <span v-if="col.sortable" class="sort-icon">
                  {{ sortKey === col.key ? (sortOrder === 'asc' ? '▲' : '▼') : '↕' }}
                </span>
              </div>
            </th>
            <th class="actions-header" style="width: 110px">Actions</th>
          </tr>
        </thead>
        <tbody>
          <tr v-if="loading" class="loading-row">
            <td :colspan="columns.length + 1" class="loading-cell">
              <div class="loading-spinner-wrap">
                <span class="spinner-inline"></span>
                <span>Streaming live telemetry...</span>
              </div>
            </td>
          </tr>
          <tr v-else-if="items.length === 0" class="empty-row">
            <td :colspan="columns.length + 1" class="empty-cell">
              <div class="empty-state-box">
                <span class="empty-icon">📂</span>
                <p>No platform resources match current query filters.</p>
              </div>
            </td>
          </tr>
          <tr
            v-for="item in items"
            :key="item.id"
            class="resource-row"
            :class="{ 'is-selected': item.id === selectedId }"
            @click="emit('inspect', item)"
          >
            <td v-for="col in columns" :key="col.key" class="resource-cell">
              <!-- Resource / Name column -->
              <div v-if="col.key === 'name'" class="name-col-cell">
                <span class="resource-name">{{ item.name }}</span>
                <span v-if="item.namespace" class="resource-ns font-mono">ns/{{ item.namespace }}</span>
                <span v-else class="resource-id font-mono">{{ item.id }}</span>
              </div>

              <!-- Status column -->
              <div v-else-if="col.key === 'status'" class="status-col-cell">
                <StatusBadge :status="item.status" size="sm" />
              </div>

              <!-- Tag / Classification column -->
              <div v-else-if="col.key === 'tag'" class="tag-col-cell">
                <span class="badge badge-cyan font-mono">{{ item.tag }}</span>
              </div>

              <!-- Telemetry Detail column -->
              <div v-else-if="col.key === 'detail'" class="detail-col-cell">
                <span class="detail-text" :title="item.detail">{{ item.detail }}</span>
              </div>

              <!-- Fallback unstructured column renderer -->
              <div v-else class="generic-col-cell">
                <span>{{ String(item[col.key] ?? '—') }}</span>
              </div>
            </td>

            <!-- Actions Column -->
            <td class="actions-cell" @click.stop>
              <button
                type="button"
                class="btn-inspect"
                title="Inspect manifest & telemetry"
                @click="emit('inspect', item)"
              >
                <span>🔍 Inspect</span>
              </button>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>