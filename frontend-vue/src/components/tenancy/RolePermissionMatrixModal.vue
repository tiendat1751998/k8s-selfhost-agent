<script setup lang="ts">
import type { RBACMatrix } from '../../api/management'
import type { RbacResourceItem } from '../../composables/useTenancyRbac'
import ModalDrawer from '../ui/ModalDrawer.vue'

interface Props {
  show: boolean
  matrix: RBACMatrix
  roles: string[]
  resources: RbacResourceItem[]
  selectedRole?: string | null
}

withDefaults(defineProps<Props>(), {
  selectedRole: null
})

const emit = defineEmits<{
  (e: 'update:show', val: boolean): void
  (e: 'toggle', role: string, resourceKey: string): void
  (e: 'sync'): void
}>()
</script>

<template>
  <ModalDrawer
    :show="show"
    title="Granular RBAC Privilege Matrix"
    subtitle="Configure and enforce runtime cluster access policies across role bindings."
    @update:show="emit('update:show', $event)"
  >
    <div class="matrix-modal-content">
      <div class="matrix-top-bar">
        <span class="matrix-info">
          Click individual cells to toggle runtime access policies.
        </span>
        <button class="btn btn-secondary btn-sm" @click="emit('sync')">
          <span>💾 Sync to APIServer</span>
        </button>
      </div>

      <div class="matrix-scroll-wrap">
        <table class="rbac-table">
          <thead>
            <tr>
              <th class="th-resource">Protected Resource & Action</th>
              <th v-for="role in roles" :key="role" class="th-role">
                <span
                  class="role-header-badge"
                  :class="{ highlighted: selectedRole === role }"
                >
                  {{ role.toUpperCase() }}
                </span>
              </th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="res in resources" :key="res.key" class="rbac-row">
              <td class="td-resource">
                <div class="resource-name">{{ res.label }}</div>
                <small class="resource-key font-mono">{{ res.key }}</small>
              </td>
              <td
                v-for="role in roles"
                :key="role"
                class="td-perm"
                @click="emit('toggle', role, res.key)"
              >
                <div
                  class="perm-badge"
                  :class="matrix[role]?.[res.key] ? 'perm-allowed' : 'perm-denied'"
                >
                  <span v-if="matrix[role]?.[res.key]">✓ ALLOWED</span>
                  <span v-else>✕ DENIED</span>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <template #footer="{ close }">
      <button class="btn btn-secondary" type="button" @click="close">Done</button>
      <button class="btn btn-primary" type="button" @click="emit('sync'); close()">
        Save & Sync Policies
      </button>
    </template>
  </ModalDrawer>
</template>

<style scoped>
.matrix-modal-content {
  display: flex;
  flex-direction: column;
  gap: 16px;
}
.matrix-top-bar {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 12px;
}
.matrix-info {
  font-size: 12px;
  color: var(--text-secondary);
}
.matrix-scroll-wrap {
  overflow-x: auto;
  scrollbar-width: thin;
  border-radius: 8px;
  border: 1px solid var(--border-subtle);
}
.role-header-badge.highlighted {
  background: rgba(168, 85, 247, 0.25);
  border-color: #c084fc;
  color: #fff;
}
</style>
