<script setup lang="ts" generic="T = any">
import { ref, computed } from 'vue'

export interface Column<T> {
  key: keyof T | string
  label: string
  width?: string
  align?: 'left' | 'center' | 'right'
  sortable?: boolean
  render?: (row: T) => unknown
}

const props = defineProps<{
  columns: Column<T>[]
  data: T[]
  loading?: boolean
  error?: string | null
  emptyMessage?: string
  searchable?: boolean
  searchPlaceholder?: string
}>()

const search = ref('')
const sortKey = ref<string>('')
const sortOrder = ref<'asc' | 'desc'>('asc')

function getCellValue(row: T, key: keyof T | string): unknown {
  if (row === null || row === undefined) return undefined
  return (row as Record<string, unknown>)[key as string]
}

function deepSearchMatch(obj: unknown, query: string, depth = 0): boolean {
  if (obj === null || obj === undefined || depth > 2) return false
  if (typeof obj === 'string' || typeof obj === 'number' || typeof obj === 'boolean') {
    return String(obj).toLowerCase().includes(query)
  }
  if (Array.isArray(obj)) {
    return obj.some(item => deepSearchMatch(item, query, depth + 1))
  }
  if (typeof obj === 'object') {
    return Object.values(obj as Record<string, unknown>).some(val => deepSearchMatch(val, query, depth + 1))
  }
  return false
}

function getRowKey(row: T, idx: number): string | number {
  if (row && typeof row === 'object') {
    const r = row as Record<string, unknown>
    if (r.id !== undefined && r.id !== null) return String(r.id)
    if (r.name !== undefined && r.name !== null) return String(r.name)
    if (r.key !== undefined && r.key !== null) return String(r.key)
    if (r.metadata && typeof r.metadata === 'object') {
      const meta = r.metadata as Record<string, unknown>
      if (meta.uid) return String(meta.uid)
      if (meta.name) return String(meta.name)
    }
  }
  return idx
}

const filteredData = computed(() => {
  let result = [...props.data]

  if (props.searchable && search.value.trim()) {
    const q = search.value.toLowerCase().trim()
    result = result.filter(item => deepSearchMatch(item, q))
  }

  if (sortKey.value) {
    result.sort((a, b) => {
      const va = getCellValue(a, sortKey.value)
      const vb = getCellValue(b, sortKey.value)
      if (va === vb) return 0
      if (va === undefined || va === null) return 1
      if (vb === undefined || vb === null) return -1
      const res = String(va).localeCompare(String(vb), undefined, { numeric: true })
      return sortOrder.value === 'asc' ? res : -res
    })
  }

  return result
})

function handleSort(key: string, sortable?: boolean) {
  if (!sortable) return
  if (sortKey.value === key) {
    sortOrder.value = sortOrder.value === 'asc' ? 'desc' : 'asc'
  } else {
    sortKey.value = key
    sortOrder.value = 'asc'
  }
}
</script>

<template>
  <div class="data-table-container glass-panel">
    <!-- Optional Toolbar -->
    <div v-if="searchable || $slots.toolbar" class="table-toolbar">
      <div v-if="searchable" class="table-search">
        <span class="search-icon"><BaseIcon name="search" size="xs" /></span>
        <input 
          v-model="search"
          type="text" 
          :placeholder="searchPlaceholder || 'Filter table records...'"
          class="input-glass search-input"
        />
        <span v-if="search" class="clear-search" @click="search = ''"><BaseIcon name="x" size="xs" /></span>
      </div>
      <div class="table-actions">
        <slot name="toolbar"></slot>
      </div>
    </div>

    <!-- Table Frame -->
    <div class="table-scroll-wrapper">
      <table class="data-table">
        <thead>
          <tr>
            <th 
              v-for="col in columns" 
              :key="String(col.key)"
              :style="{ width: col.width, textAlign: col.align || 'left' }"
              :class="{ 'th-sortable': col.sortable }"
              @click="handleSort(String(col.key), col.sortable)"
            >
              <div class="th-content" :style="{ justifyContent: col.align === 'right' ? 'flex-end' : col.align === 'center' ? 'center' : 'flex-start' }">
                <span>{{ col.label }}</span>
                <span v-if="col.sortable" class="sort-indicator">
                  {{ sortKey === col.key ? (sortOrder === 'asc' ? '▲' : '▼') : '↕' }}
                </span>
              </div>
            </th>
          </tr>
        </thead>

        <tbody>
          <!-- Loading State -->
          <tr v-if="loading" class="row-state row-loading">
            <td :colspan="columns.length || 7" class="cell-loading">
              <div class="cell-loading-content">
                <div class="spinner"></div>
                <span>Streaming telemetry & synchronizing state...</span>
              </div>
            </td>
          </tr>

          <!-- Error State -->
          <tr v-else-if="error" class="row-state row-error">
            <td :colspan="columns.length || 7" class="cell-error">
              <div class="cell-error-content">
                <span class="error-icon"><BaseIcon name="alert-triangle" size="xs" /></span>
                <span>{{ error }}</span>
              </div>
            </td>
          </tr>

          <!-- Empty State -->
          <tr v-else-if="filteredData.length === 0" class="row-state row-empty">
            <td :colspan="columns.length || 7" class="cell-empty">
              <div class="cell-empty-content">
                <div class="empty-icon"><BaseIcon name="package" size="lg" /></div>
                <div class="empty-text">{{ emptyMessage || 'No matching records discovered.' }}</div>
              </div>
            </td>
          </tr>

          <!-- Data Rows -->
          <tr v-else v-for="(row, idx) in filteredData" :key="getRowKey(row, idx)" class="data-row">
            <td 
              v-for="col in columns" 
              :key="String(col.key)"
              :style="{ textAlign: col.align || 'left' }"
            >
              <slot :name="`cell-${String(col.key)}`" :row="row" :value="getCellValue(row, col.key)">
                {{ col.render ? col.render(row) : (getCellValue(row, col.key) ?? '-') }}
              </slot>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- Table Footer / Count -->
    <div class="table-footer">
      <span>Showing {{ filteredData.length }} of {{ data.length }} items</span>
    </div>
  </div>
</template>

<style scoped>
@import '../../assets/styles/components/ui/data-table.css';
</style>
