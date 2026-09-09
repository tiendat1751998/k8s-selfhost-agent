<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import type { Organization, Member } from '../../api/management'
import ModalDrawer from '../ui/ModalDrawer.vue'
import StatusBadge from '../ui/StatusBadge.vue'

interface Props {
  show: boolean
  organizations: Organization[]
  members: Member[]
  selectedOrgId?: string | null
}

const props = withDefaults(defineProps<Props>(), {
  selectedOrgId: null
})

const emit = defineEmits<{
  (e: 'update:show', val: boolean): void
  (e: 'invite', member: Partial<Member>): void
  (e: 'revoke', memberId: string): void
}>()

const formModel = ref({
  user: '',
  orgId: props.selectedOrgId || '',
  role: 'developer',
  scope: 'project-wide'
})

watch(() => props.selectedOrgId, (newId) => {
  if (newId) formModel.value.orgId = newId
})

const currentOrg = computed(() => {
  if (!props.selectedOrgId || props.selectedOrgId === 'all') return null
  return props.organizations.find(o => o.id === props.selectedOrgId) || null
})

const scopedMembers = computed(() => {
  if (!props.selectedOrgId || props.selectedOrgId === 'all') return props.members
  return props.members.filter(m => m.orgId === props.selectedOrgId)
})

function handleInviteSubmit() {
  if (!formModel.value.user) return
  emit('invite', {
    user: formModel.value.user,
    orgId: formModel.value.orgId || props.selectedOrgId || props.organizations[0]?.id || 'org-default',
    role: formModel.value.role,
    scope: formModel.value.scope
  })
  formModel.value.user = ''
}
</script>

<template>
  <ModalDrawer
    :show="show"
    mode="drawer"
    title="Organization Members & Access Control"
    :subtitle="currentOrg ? `Managing identities for ${currentOrg.name}` : 'Manage member identities and RBAC bindings.'"
    @update:show="emit('update:show', $event)"
  >
    <div class="member-drawer-layout">
      <!-- Member Invitation Form -->
      <form class="form-layout invite-section" @submit.prevent="handleInviteSubmit">
        <h4 class="section-title">+ Invite New Team Member</h4>
        <div class="form-group">
          <label>Member Email / SSO Identity</label>
          <input
            v-model="formModel.user"
            type="email"
            placeholder="e.g. engineer@enterprise.io"
            class="input-glass"
            required
          />
        </div>

        <div class="form-group">
          <label>Target Organization Container</label>
          <select v-model="formModel.orgId" class="input-glass">
            <option v-for="org in organizations" :key="org.id" :value="org.id">
              {{ org.name }} ({{ org.id }})
            </option>
          </select>
        </div>

        <div class="form-grid-2">
          <div class="form-group">
            <label>RBAC Role Assignment</label>
            <select v-model="formModel.role" class="input-glass">
              <option value="admin">Admin (Full Plane)</option>
              <option value="operator">Operator (Deploy/Logs)</option>
              <option value="developer">Developer (Pods/Dev)</option>
              <option value="auditor">Auditor (Compliance)</option>
              <option value="viewer">Viewer (Read-Only)</option>
            </select>
          </div>

          <div class="form-group">
            <label>Access Scope</label>
            <select v-model="formModel.scope" class="input-glass">
              <option value="org-wide">Organization-Wide</option>
              <option value="project-wide">Project-Wide</option>
              <option value="sandbox-only">Developer Sandbox</option>
            </select>
          </div>
        </div>

        <button class="btn btn-primary btn-sm" type="submit">
          <span>Dispatch Invite & Bind Role</span>
        </button>
      </form>

      <!-- Active Scoped Members List -->
      <div class="members-list-section">
        <h4 class="section-title">Bound Members ({{ scopedMembers.length }})</h4>

        <div v-if="scopedMembers.length === 0" class="empty-list">
          No members assigned to this organization container.
        </div>

        <div v-else class="member-items">
          <div v-for="mem in scopedMembers" :key="mem.id" class="member-item glass-panel">
            <div class="member-item-info">
              <div class="member-avatar">{{ mem.user.charAt(0).toUpperCase() }}</div>
              <div class="member-details">
                <span class="member-email">{{ mem.user }}</span>
                <div class="member-tags">
                  <StatusBadge
                    :status="mem.role === 'admin' ? 'danger' : mem.role === 'operator' ? 'warning' : mem.role === 'auditor' ? 'violet' : 'active'"
                    :label="mem.role.toUpperCase()"
                    size="sm"
                  />
                  <span class="scope-tag font-mono">{{ mem.scope }}</span>
                </div>
              </div>
            </div>

            <button
              class="btn btn-secondary btn-sm revoke-btn"
              title="Revoke access"
              @click="emit('revoke', mem.id)"
            >
              <span>Revoke</span>
            </button>
          </div>
        </div>
      </div>
    </div>

    <template #footer="{ close }">
      <button class="btn btn-secondary" type="button" @click="close">Close</button>
    </template>
  </ModalDrawer>
</template>

<style scoped>
@import '../../assets/styles/views/tenancy.css';
</style>
