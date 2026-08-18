<script setup lang="ts" generic="T extends Record<string, unknown>">
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

const filteredData = computed(() => {
  let result = [...props.data]

  if (props.searchable && search.value.trim()) {
    const q = search.value.toLowerCase().trim()
    result = result.filter(item => {
      return Object.values(item).some(val => 
        String(val).toLowerCase().includes(q)
      )
    })
  }

  if (sortKey.value) {
    result.sort((a, b) => {
      const va = a[sortKey.value]
      const vb = b[sortKey.value]
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
        <span class="search-icon">🔍</span>
        <input 
          v-model="search"
          type="text" 
          :placeholder="searchPlaceholder || 'Filter table records...'"
          class="input-glass search-input"
        />
        <span v-if="search" class="clear-search" @click="search = ''">✕</span>
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
          <tr v-if="loading" class="row-state">
            <td :colspan="columns.length" class="cell-loading">
              <div class="spinner"></div>
              <span>Streaming telemetry & synchronizing state...</span>
            </td>
          </tr>

          <!-- Error State -->
          <tr v-else-if="error" class="row-state">
            <td :colspan="columns.length" class="cell-error">
              <span class="error-icon">⚠️</span>
              <span>{{ error }}</span>
            </td>
          </tr>

          <!-- Empty State -->
          <tr v-else-if="filteredData.length === 0" class="row-state">
            <td :colspan="columns.length" class="cell-empty">
              <div class="empty-icon">📦</div>
              <div class="empty-text">{{ emptyMessage || 'No matching records discovered.' }}</div>
            </td>
          </tr>

          <!-- Data Rows -->
          <tr v-else v-for="(row, idx) in filteredData" :key="idx" class="data-row">
            <td 
              v-for="col in columns" 
              :key="String(col.key)"
              :style="{ textAlign: col.align || 'left' }"
            >
              <slot :name="`cell-${String(col.key)}`" :row="row" :value="row[col.key]">
                {{ col.render ? col.render(row) : row[col.key] ?? '-' }}
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
.data-table-container {
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

.table-toolbar {
  padding: 14px 18px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  border-bottom: 1px solid var(--border-subtle);
  background: rgba(11, 15, 25, 0.4);
  gap: 16px;
}

.table-search {
  position: relative;
  display: flex;
  align-items: center;
  flex: 1;
  max-width: 380px;
}

.search-icon {
  position: absolute;
  left: 12px;
  font-size: 13px;
  color: var(--text-muted);
}

.search-input {
  width: 100%;
  padding-left: 34px;
  padding-right: 28px;
}

.clear-search {
  position: absolute;
  right: 10px;
  cursor: pointer;
  color: var(--text-muted);
  font-size: 11px;
}

.table-actions {
  display: flex;
  align-items: center;
  gap: 10px;
}

.table-scroll-wrapper {
  overflow-x: auto;
  max-height: 600px;
}

.data-table {
  width: 100%;
  border-collapse: collapse;
  text-align: left;
  font-size: 13px;
}

th {
  padding: 12px 18px;
  background: rgba(15, 23, 42, 0.85);
  color: var(--text-secondary);
  font-size: 11px;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  border-bottom: 1px solid var(--border-medium);
  white-space: nowrap;
  user-select: none;
}

.th-sortable {
  cursor: pointer;
}
.th-sortable:hover {
  color: var(--text-primary);
}

.th-content {
  display: flex;
  align-items: center;
  gap: 6px;
}

.sort-indicator {
  font-size: 9px;
  color: var(--accent-sky);
}

td {
  padding: 14px 18px;
  border-bottom: 1px solid var(--border-subtle);
  color: var(--text-primary);
}

.data-row {
  transition: background-color 0.15s ease;
}

.data-row:hover {
  background: rgba(56, 189, 248, 0.04);
}

.row-state td {
  padding: 48px 24px;
  text-align: center;
}

.cell-loading {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
  color: var(--text-muted);
}

.spinner {
  width: 24px;
  height: 24px;
  border: 2px solid rgba(56, 189, 248, 0.2);
  border-top-color: var(--accent-sky);
  border-radius: 50%;
  animation: spin 0.8s linear infinite;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}

.cell-error {
  color: #fb7185;
  font-weight: 500;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
}

.cell-empty {
  color: var(--text-muted);
}

.empty-icon {
  font-size: 32px;
  margin-bottom: 8px;
}

.empty-text {
  font-size: 13px;
}

.table-footer {
  padding: 10px 18px;
  border-top: 1px solid var(--border-subtle);
  background: rgba(11, 15, 25, 0.4);
  font-size: 11px;
  color: var(--text-muted);
  font-family: var(--font-mono);
}
</style>
