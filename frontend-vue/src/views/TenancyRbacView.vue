<script setup lang="ts">
import { useTenancyRbac } from '../composables/useTenancyRbac'
import StatusBadge from '../components/ui/StatusBadge.vue'
import ModalDrawer from '../components/ui/ModalDrawer.vue'
import BaseIcon from '../components/ui/BaseIcon.vue'
import TenantListTable from '../components/tenancy/TenantListTable.vue'
import TenancyMobileCards from '../components/tenancy/TenancyMobileCards.vue'
import RolePermissionMatrixModal from '../components/tenancy/RolePermissionMatrixModal.vue'
import TenantMemberDrawer from '../components/tenancy/TenantMemberDrawer.vue'
import CreateTenantModal from '../components/tenancy/CreateTenantModal.vue'
import '../assets/styles/views/tenancy.css'
import '../assets/styles/components/tenancy-drawers.css'

const {
  loading,
  organizations,
  members,
  rbacMatrix,
  activeTab,
  showOrgModal,
  showProjectModal,
  showMemberDrawer,
  showRbacMatrixModal,
  selectedOrgForDrawer,
  selectedRoleForMatrix,
  showQuotaModal,
  selectedOrgForQuota,
  newProj,
  isSubmitting,
  filteredProjects,
  filteredOrganizations,
  tenancySearchQuery,
  clearSearch,
  loadData,
  filteredMembers,
  organizationStats,
  rbacResources,
  rbacRoles,
  showFeedback,
  toggleRbacPermission,
  syncRbacToApi,
  handleCreateOrg,
  handleDeleteOrg,
  handleCreateProject,
  handleInviteMember,
  removeMember,
  openMemberDrawer,
  openRbacModal,
  openQuotaModal
} = useTenancyRbac()
</script>

<template>
  <div class="tenancy-page">
    <!-- Sleek 42px Toolbar -->
    <div class="tenancy-toolbar-sleek glass-panel desktop-only">
      <div class="toolbar-left">
        <div class="tab-pills">
          <button class="tab-btn" :class="{ active: activeTab === 'tenants' }" @click="activeTab = 'tenants'">
            <span>Orgs</span> <span class="tab-count">{{ organizations.length }}</span>
          </button>
          <button class="tab-btn" :class="{ active: activeTab === 'members' }" @click="activeTab = 'members'">
            <span>Members</span> <span class="tab-count">{{ filteredMembers.length }}</span>
          </button>
          <button class="tab-btn" :class="{ active: activeTab === 'projects' }" @click="activeTab = 'projects'">
            <span>Namespaces</span> <span class="tab-count">{{ filteredProjects.length }}</span>
          </button>
          <button class="tab-btn" :class="{ active: activeTab === 'rbac' }" @click="activeTab = 'rbac'">
            <span>RBAC</span>
          </button>
        </div>
      </div>
      <div class="toolbar-center">
        <div class="search-pill">
          <BaseIcon name="search" size="xs" class="search-icon" />
          <input type="text" v-model="tenancySearchQuery" placeholder="Search..." class="search-input-sleek" />
          <button v-if="tenancySearchQuery" @click="clearSearch" class="clear-btn"><BaseIcon name="x" size="xs" /></button>
        </div>
      </div>
      <div class="toolbar-right">
        <button v-if="activeTab === 'tenants'" class="btn btn-primary btn-sm" @click="showOrgModal = true">+ New Org</button>
        <button v-if="activeTab === 'projects'" class="btn btn-primary btn-sm" @click="showProjectModal = true">+ Create Project</button>
        <button v-if="activeTab === 'members'" class="btn btn-primary btn-sm" @click="openMemberDrawer()">+ Invite Member</button>
        <button class="btn btn-secondary btn-sm" @click="loadData" title="Refresh"><BaseIcon name="refresh-cw" size="xs" /></button>
      </div>
    </div>

    <!-- TAB 1: TENANTS LIST -->
    <div v-if="activeTab === 'tenants'" class="tab-content animate-fade-in">
      <div class="desktop-only-table desktop-only">
        <TenantListTable
          :organizations="filteredOrganizations"
          :stats="organizationStats"
          :loading="loading"
          @open-members="openMemberDrawer($event)"
          @open-rbac="openRbacModal($event)"
          @open-quota="openQuotaModal($event)"
          @delete-org="handleDeleteOrg($event)"
          @create-org="showOrgModal = true"
        />
      </div>
      <div class="mobile-only-stream mobile-only">
        <TenancyMobileCards
          :organizations="filteredOrganizations"
          :stats="organizationStats"
          @open-members="openMemberDrawer($event)"
          @open-rbac="openRbacModal($event)"
          @open-quota="openQuotaModal($event)"
          @delete-org="handleDeleteOrg($event)"
          @create-org="showOrgModal = true"
        />
      </div>
    </div>

    <!-- TAB 2: MEMBERS -->
    <div v-else-if="activeTab === 'members'" class="tab-content animate-fade-in">
      <div class="tenant-table-wrapper glass-panel">
        <div class="table-toolbar">
          <span class="tenant-count-badge">{{ filteredMembers.length }} Bound Members</span>
          <button class="btn btn-primary btn-sm" @click="openMemberDrawer()"><span>+ Invite Member</span></button>
        </div>
        <div class="table-scroll">
          <table class="tenant-table">
            <thead>
              <tr>
                <th class="th-left">User / Identity</th>
                <th>Assigned Role</th>
                <th>Resource Scope</th>
                <th>Organization ID</th>
                <th class="th-right">Management</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="m in filteredMembers" :key="m.id" class="tenant-row">
                <td class="td-left">
                  <div class="user-cell">
                    <div class="user-avatar">{{ m.user.charAt(0).toUpperCase() }}</div>
                    <div class="user-details">
                      <span class="user-name">{{ m.user }}</span>
                      <small class="user-verified">Verified Enterprise SSO</small>
                    </div>
                  </div>
                </td>
                <td>
                  <StatusBadge :status="m.role === 'admin' ? 'danger' : m.role === 'operator' ? 'warning' : m.role === 'auditor' ? 'violet' : 'active'" :label="m.role.toUpperCase()" />
                </td>
                <td><span class="scope-tag font-mono">{{ m.scope }}</span></td>
                <td><span class="text-muted font-mono">{{ m.orgId }}</span></td>
                <td class="td-right">
                  <button class="btn btn-secondary btn-sm" title="Revoke Member" @click="removeMember(m.id)"><span>Revoke</span></button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </div>

    <!-- TAB 3: PROJECTS WORKLOAD GRID -->
    <div v-else-if="activeTab === 'projects'" class="tab-content animate-fade-in">
      <div v-if="filteredProjects.length === 0" class="empty-list glass-panel">
        No project namespaces configured. Click "+ Create Project" above to allocate namespaces.
      </div>
      <div v-else class="projects-grid">
        <div v-for="proj in filteredProjects" :key="proj.id" class="project-card glass-panel">
          <div class="project-card-header">
            <div class="project-title-wrap">
              <BaseIcon name="box" size="sm" class="project-icon" />
              <div>
                <h3 class="project-name">{{ proj.name }}</h3>
                <small class="project-id font-mono">{{ proj.id }}</small>
              </div>
            </div>
            <StatusBadge status="healthy" label="ACTIVE" size="sm" />
          </div>
          <div class="project-body">
            <div class="project-stat-row">
              <span class="stat-label">Attached Org:</span> <span class="stat-value font-mono">{{ proj.orgId }}</span>
            </div>
            <div class="project-stat-row">
              <span class="stat-label">Live Workload Pods:</span> <span class="stat-value font-mono text-cyan">{{ proj.workloads }} pods</span>
            </div>
            <div class="project-envs">
              <span class="env-label">Environments:</span>
              <div class="env-badges">
                <span v-for="env in proj.envs" :key="env" class="env-chip" :class="'env-' + env">{{ env }}</span>
              </div>
            </div>
          </div>
          <div class="project-footer">
            <button class="btn btn-secondary btn-sm" @click="showFeedback('Exporting namespace config for ' + proj.name)">Inspect CRDs</button>
            <button class="btn btn-primary btn-sm" @click="showFeedback('Connecting to ' + proj.name + ' telemetry stream...')">View Pods</button>
          </div>
        </div>
      </div>
    </div>

    <!-- TAB 4: RBAC MATRIX -->
    <div v-else-if="activeTab === 'rbac'" class="tab-content animate-fade-in">
      <div class="rbac-container glass-panel">
        <div class="rbac-header">
          <div>
            <h3 class="rbac-title">Kubernetes RBAC Privilege Matrix</h3>
            <p class="rbac-sub">Click individual cells to toggle runtime access policies across the cluster mesh.</p>
          </div>
          <button class="btn btn-secondary btn-sm" @click="syncRbacToApi"><BaseIcon name="save" size="xs" /> <span>Sync to APIServer</span></button>
        </div>
        <div class="rbac-table-wrap">
          <table class="rbac-table">
            <thead>
              <tr>
                <th class="th-resource">Protected Resource & Action</th>
                <th v-for="role in rbacRoles" :key="role" class="th-role">
                  <span class="role-header-badge">{{ role.toUpperCase() }}</span>
                </th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="res in rbacResources" :key="res.key" class="rbac-row">
                <td class="td-resource">
                  <div class="resource-name">{{ res.label }}</div>
                  <small class="resource-key font-mono">{{ res.key }}</small>
                </td>
                <td v-for="role in rbacRoles" :key="role" class="td-perm" @click="toggleRbacPermission(role, res.key)">
                  <div class="perm-badge" :class="rbacMatrix[role]?.[res.key] ? 'perm-allowed' : 'perm-denied'">
                    <span v-if="rbacMatrix[role]?.[res.key]"><BaseIcon name="check" size="xs" /> ALLOWED</span>
                    <span v-else><BaseIcon name="x" size="xs" /> DENIED</span>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </div>

    <!-- MODALS & DRAWERS -->
    <CreateTenantModal v-model:show="showOrgModal" :is-submitting="isSubmitting" @create="handleCreateOrg($event)" />

    <ModalDrawer v-model:show="showProjectModal" title="Create Project Namespace" subtitle="Allocate cluster namespaces and resource quotas to an organization.">
      <form class="form-layout" @submit.prevent="handleCreateProject()">
        <div class="form-group">
          <label>Parent Organization</label>
          <select v-model="newProj.orgId" class="input-glass">
            <option v-for="org in organizations" :key="org.id" :value="org.id">{{ org.name }} ({{ org.id }})</option>
          </select>
        </div>
        <div class="form-group">
          <label>Project Name</label> <input v-model="newProj.name" type="text" placeholder="e.g. Payment Gateway Ingress" class="input-glass" required />
        </div>
        <div class="form-group">
          <label>Project Identifier</label> <input v-model="newProj.id" type="text" placeholder="e.g. proj-payment-gw" class="input-glass font-mono" required />
        </div>
        <div class="form-group">
          <label>Environments (comma separated)</label> <input v-model="newProj.envs" type="text" placeholder="dev, staging, prod" class="input-glass" />
        </div>
        <div class="form-group">
          <label>Initial Workload Pod Capacity</label> <input v-model="newProj.workloads" type="number" min="1" max="500" class="input-glass" />
        </div>
      </form>
      <template #footer="{ close }">
        <button class="btn btn-secondary" type="button" @click="close">Cancel</button>
        <button class="btn btn-primary" :disabled="isSubmitting" @click="handleCreateProject()">{{ isSubmitting ? 'Creating...' : 'Create Project' }}</button>
      </template>
    </ModalDrawer>

    <TenantMemberDrawer
      v-model:show="showMemberDrawer"
      :organizations="filteredOrganizations"
      :members="members"
      :selected-org-id="selectedOrgForDrawer"
      @invite="handleInviteMember($event)"
      @revoke="removeMember($event)"
    />

    <RolePermissionMatrixModal
      v-model:show="showRbacMatrixModal"
      :matrix="rbacMatrix"
      :roles="rbacRoles"
      :resources="rbacResources"
      :selected-role="selectedRoleForMatrix"
      @toggle="toggleRbacPermission"
      @sync="syncRbacToApi"
    />

    <!-- Quota Configuration Modal -->
    <ModalDrawer
      v-model:show="showQuotaModal"
      :title="selectedOrgForQuota ? `Resource Quota: ${selectedOrgForQuota.name}` : 'Tenant Resource Quota'"
      subtitle="Cluster compute boundaries, pod allocation limits, and namespace quotas."
    >
      <div v-if="selectedOrgForQuota" class="quota-modal-body">
        <div class="quota-info-banner">
          <span class="font-mono text-cyan">{{ selectedOrgForQuota.id }}</span>
          <span class="quota-tier-tag font-mono">{{ selectedOrgForQuota.tier }}</span>
        </div>

        <div class="quota-bar-group">
          <div class="quota-bar-header">
            <span class="quota-metric-name">Workload Pods</span>
            <span class="quota-metric-values font-mono">{{ organizationStats[selectedOrgForQuota.id]?.workloadCount ?? 0 }} / 50 pods</span>
          </div>
          <div class="quota-track">
            <div
              class="quota-fill quota-fill-cyan"
              :style="{ width: Math.min(100, ((organizationStats[selectedOrgForQuota.id]?.workloadCount ?? 0) / 50) * 100) + '%' }"
            ></div>
          </div>
        </div>

        <div class="quota-bar-group">
          <div class="quota-bar-header">
            <span class="quota-metric-name">Project Namespaces</span>
            <span class="quota-metric-values font-mono">{{ organizationStats[selectedOrgForQuota.id]?.projectCount ?? 0 }} / 10 namespaces</span>
          </div>
          <div class="quota-track">
            <div
              class="quota-fill quota-fill-emerald"
              :style="{ width: Math.min(100, ((organizationStats[selectedOrgForQuota.id]?.projectCount ?? 0) / 10) * 100) + '%' }"
            ></div>
          </div>
        </div>

        <div class="quota-bar-group">
          <div class="quota-bar-header">
            <span class="quota-metric-name">SSO Identity Seats</span>
            <span class="quota-metric-values font-mono">{{ organizationStats[selectedOrgForQuota.id]?.memberCount ?? 0 }} / 25 seats</span>
          </div>
          <div class="quota-track">
            <div
              class="quota-fill quota-fill-amber"
              :style="{ width: Math.min(100, ((organizationStats[selectedOrgForQuota.id]?.memberCount ?? 0) / 25) * 100) + '%' }"
            ></div>
          </div>
        </div>
      </div>
      <template #footer="{ close }">
        <button class="btn btn-secondary" type="button" @click="close">Close</button>
        <button class="btn btn-primary" type="button" @click="showFeedback('Resource quota saved for ' + selectedOrgForQuota?.name); close()">Save Quota</button>
      </template>
    </ModalDrawer>
  </div>
</template>

