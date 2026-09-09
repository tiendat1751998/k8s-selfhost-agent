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
@import '../../assets/styles/components/ui/base.css';
</style>
