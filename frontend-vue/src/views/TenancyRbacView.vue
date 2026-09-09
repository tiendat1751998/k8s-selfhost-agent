<script setup lang="ts">
import { useTenancyRbac } from '../composables/useTenancyRbac'
import MetricCard from '../components/ui/MetricCard.vue'
import StatusBadge from '../components/ui/StatusBadge.vue'
import ModalDrawer from '../components/ui/ModalDrawer.vue'
import TenantListTable from '../components/tenancy/TenantListTable.vue'
import TenancyMobileCards from '../components/tenancy/TenancyMobileCards.vue'
import RolePermissionMatrixModal from '../components/tenancy/RolePermissionMatrixModal.vue'
import TenantMemberDrawer from '../components/tenancy/TenantMemberDrawer.vue'
import CreateTenantModal from '../components/tenancy/CreateTenantModal.vue'
import '../assets/styles/views/tenancy.css'

const {
  loading,
  organizations,
  members,
  rbacMatrix,
  selectedOrgId,
  activeTab,
  showOrgModal,
  showProjectModal,
  showMemberDrawer,
  showRbacMatrixModal,
  selectedOrgForDrawer,
  selectedRoleForMatrix,
  newProj,
  isSubmitting,
  feedbackMessage,
  filteredProjects,
  filteredMembers,
  totalWorkloads,
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
    <!-- Header -->
    <div class="page-header desktop-header desktop-only">
      <div class="header-titles">
        <div class="header-badge">
          <span class="badge badge-cyan">Multi-Tenant Isolation</span>
          <span class="badge badge-emerald">ZeroTrust RBAC Matrix</span>
        </div>
        <h1 class="page-title">Enterprise Tenancy & RBAC Hub</h1>
        <p class="page-desc">
          Manage multi-organization workspace boundaries, project namespaces, member role bindings, and granular Kubernetes privilege matrices.
        </p>
      </div>

      <div class="header-actions">
        <button class="btn btn-secondary" @click="showOrgModal = true"><span>+ New Organization</span></button>
        <button class="btn btn-primary" @click="showProjectModal = true"><span>+ Create Project</span></button>
      </div>
    </div>

    <!-- Mobile 40px Command Bar (<640px) -->
    <div class="tenancy-mobile-command-bar mobile-only">
      <div class="command-bar-left">
        <span class="command-bar-title font-bold">🏢 Tenancy ({{ organizations.length }})</span>
      </div>
      <div class="command-bar-actions">
        <button
          class="btn-icon-cmd"
          title="Create Project"
          aria-label="Create Project"
          @click="showProjectModal = true"
        >
          <span>➕</span>
        </button>
        <button
          class="btn-icon-cmd"
          title="New Organization"
          aria-label="New Organization"
          @click="showOrgModal = true"
        >
          <span>🏢</span>
        </button>
      </div>
    </div>

    <!-- Mobile 20px Centered Micro-Telemetry Strip (<640px) -->
    <div class="tenancy-micro-telemetry mobile-only font-mono" role="status" aria-label="Tenancy Micro Telemetry">
      <span class="tel-item tel-orgs">🏢 {{ organizations.length }} orgs</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-projs">📁 {{ filteredProjects.length }} projs</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-wkls">📦 {{ totalWorkloads }} wkls</span>
      <span class="tel-sep">·</span>
      <span class="tel-item tel-mbrs">👥 {{ filteredMembers.length }} mbrs</span>
    </div>

    <!-- Alert Banner -->
    <div v-if="feedbackMessage" class="feedback-banner animate-fade-in">
      <span class="feedback-icon">✓</span>
      <span>{{ feedbackMessage }}</span>
    </div>

    <!-- Key Metrics Grid -->
    <div class="metrics-grid desktop-metrics desktop-only">
      <MetricCard title="Organizations" :value="organizations.length" trend="Multi-Org Isolated" trendType="neutral" />
      <MetricCard title="Active Projects" :value="filteredProjects.length" trend="+2 namespaces this week" trendType="positive" />
      <MetricCard title="Live Workloads" :value="totalWorkloads" trend="Replicas across pods" trendType="positive" />
      <MetricCard title="Active Members" :value="filteredMembers.length" trend="Mapped to RBAC Roles" trendType="neutral" />
    </div>

    <!-- Filter & Scope Bar -->
    <div class="scope-bar glass-panel">
      <div class="scope-left">
        <label class="scope-label">Active Organization Scope:</label>
        <select v-model="selectedOrgId" class="input-glass select-scope">
          <option value="all">🌐 All Organizations (Global Multi-Tenant)</option>
          <option v-for="org in organizations" :key="org.id" :value="org.id">
            🏢 {{ org.name }} ({{ org.tier }})
          </option>
        </select>
      </div>

      <div class="tab-pills">
        <button class="tab-btn" :class="{ active: activeTab === 'tenants' }" @click="activeTab = 'tenants'">
          <span>🏢 Organizations</span> <span class="tab-count">{{ organizations.length }}</span>
        </button>
        <button class="tab-btn" :class="{ active: activeTab === 'members' }" @click="activeTab = 'members'">
          <span>👥 Members & Roles</span> <span class="tab-count">{{ filteredMembers.length }}</span>
        </button>
        <button class="tab-btn" :class="{ active: activeTab === 'projects' }" @click="activeTab = 'projects'">
          <span>📁 Project Namespaces</span> <span class="tab-count">{{ filteredProjects.length }}</span>
        </button>
        <button class="tab-btn" :class="{ active: activeTab === 'rbac' }" @click="activeTab = 'rbac'">
          <span>🛡️ Granular RBAC Matrix</span>
        </button>
      </div>
    </div>

    <!-- TAB 1: TENANTS LIST -->
    <div v-if="activeTab === 'tenants'" class="tab-content animate-fade-in">
      <div class="desktop-only-table">
        <TenantListTable
          :organizations="organizations"
          :stats="organizationStats"
          :loading="loading"
          @open-members="openMemberDrawer($event)"
          @open-rbac="openRbacModal($event)"
          @open-quota="openQuotaModal($event)"
          @delete-org="handleDeleteOrg($event)"
          @create-org="showOrgModal = true"
        />
      </div>
      <div class="mobile-only-stream">
        <TenancyMobileCards
          :organizations="organizations"
          :stats="organizationStats"
          @open-members="openMemberDrawer($event)"
          @open-rbac="openRbacModal($event)"
          @open-quota="openQuotaModal($event)"
          @delete-org="handleDeleteOrg($event)"
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
              <span class="project-icon">📦</span>
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
          <button class="btn btn-secondary btn-sm" @click="syncRbacToApi"><span>💾 Sync to APIServer</span></button>
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
                    <span>{{ rbacMatrix[role]?.[res.key] ? '✓ ALLOWED' : '✕ DENIED' }}</span>
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
      :organizations="organizations"
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
  </div>
</template>
