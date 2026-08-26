<script setup lang="ts" generic="T extends Record<string, unknown> = Record<string, unknown>">
export interface TableColumn {
  key: string
  label: string
}
defineProps<{
  columns: TableColumn[]
  data: T[]
}>()
</script>

<template>
  <div class="base-table-container">
    <table class="base-table">
      <thead>
        <tr>
          <th v-for="col in columns" :key="col.key">{{ col.label }}</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="(row, i) in data" :key="i">
          <td v-for="col in columns" :key="col.key">
            <slot :name="col.key" :row="row" :value="row[col.key]">
              {{ row[col.key] }}
            </slot>
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>

<style scoped>
.base-table-container {
  overflow-x: auto;
}
.base-table {
  width: 100%;
  border-collapse: collapse;
  text-align: left;
}
.base-table th {
  padding: var(--space-sm) var(--space-md);
  font-size: var(--text-caption);
  color: var(--color-muted);
  font-weight: 600;
  border-bottom: 1px solid var(--color-hairline);
}
.base-table td {
  padding: var(--space-sm) var(--space-md);
  font-size: var(--text-body-sm);
  color: var(--color-on-dark);
  border-bottom: 1px solid var(--color-hairline);
}
.base-table tbody tr:last-child td {
  border-bottom: none;
}
.base-table tbody tr:hover {
  background: var(--color-surface-elevated);
}

@media (max-width: 640px) {
  .base-table th,
  .base-table td {
    padding: 8px 10px;
    font-size: 11.5px;
  }

  .base-table {
    min-width: 480px;
  }
}
</style>
