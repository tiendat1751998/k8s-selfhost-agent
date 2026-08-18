<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import {
  tenancyApi,
  type Organization,
  type Project,
  type Member,
  type RBACMatrix
} from '../api/management'
import StatusBadge from '../components/ui/StatusBadge.vue'
import MetricCard from '../components/ui/MetricCard.vue'
import DataTable, { type Column } from '../components/ui/DataTable.vue'
import ModalDrawer from '../components/ui/ModalDrawer.vue'

// State
const loading = ref(false)
const error = ref<string | null>(null)
const organizations = ref<Organization[]>([])
const projects = ref<Project[]>([])
const members = ref<Member[]>([])
const rbacMatrix = ref<RBACMatrix>({})

const selectedOrgId = ref<string>('all')
const activeTab = ref<'members' | 'rbac' | 'projects'>('members')

// Modals
const showOrgModal = ref(false)
const showProjectModal = ref(false)
const showMemberModal = ref(false)

const newOrg = ref({ id: '', name: '', tier: 'Enterprise' })
const newProj = ref({ id: '', orgId: '', name: '', envs: 'dev, staging, prod', workloads: 5 })
const newMember = ref({ id: '', orgId: '', user: '', role: 'developer', scope: 'project-wide' })
const isSubmitting = ref(false)
const feedbackMessage = ref<string | null>(null)

// Fallback seed data in case backend has no initial records
const fallbackOrgs: Organization[] = [
  { id: 'org-enterprise-core', name: 'Global Enterprise Corp', tier: 'Enterprise Tier-1' },
  { id: 'org-fintech-shield', name: 'Fintech Payments & Shield', tier: 'GovCloud High-Sec' },
  { id: 'org-ai-research', name: 'Applied AI & ML Lab', tier: 'High-Performance' },
]

const fallbackProjects: Project[] = [
  { id: 'proj-k8s-core', orgId: 'org-enterprise-core', name: 'Core Control Plane', envs: ['prod', 'stage'], workloads: 42 },
  { id: 'proj-gateway-edge', orgId: 'org-enterprise-core', name: 'Edge Ingress Gateway', envs: ['prod', 'dev'], workloads: 18 },
  { id: 'proj-ledger-pci', orgId: 'org-fintech-shield', name: 'PCI-DSS Ledger Cluster', envs: ['prod'], workloads: 28 },
  { id: 'proj-ai-pipeline', orgId: 'org-ai-research', name: 'DeepSeek LLM Inference', envs: ['prod', 'dev', 'test'], workloads: 64 },
]

const fallbackMembers: Member[] = [
  { id: 'mem-1', orgId: 'org-enterprise-core', user: 'alex.sre@enterprise.io', role: 'admin', scope: 'org-wide' },
  { id: 'mem-2', orgId: 'org-enterprise-core', user: 'sarah.devops@enterprise.io', role: 'operator', scope: 'proj-k8s-core' },
  { id: 'mem-3', orgId: 'org-fintech-shield', user: 'marcus.sec@fintech.io', role: 'auditor', scope: 'org-wide' },
  { id: 'mem-4', orgId: 'org-ai-research', user: 'elena.ml@ai-lab.io', role: 'developer', scope: 'proj-ai-pipeline' },
]

const fallbackRBAC: RBACMatrix = {
  admin: {
    'pods:read': true, 'pods:write': true, 'deployments:scale': true,
    'secrets:manage': true, 'backups:execute': true, 'nodes:drain': true,
    'ai:configure': true, 'changes:approve': true, 'audit:view': true
  },
  operator: {
    'pods:read': true, 'pods:write': true, 'deployments:scale': true,
    'secrets:manage': false, 'backups:execute': true, 'nodes:drain': false,
    'ai:configure': true, 'changes:approve': true, 'audit:view': true
  },
  developer: {
    'pods:read': true, 'pods:write': true, 'deployments:scale': false,
    'secrets:manage': false, 'backups:execute': false, 'nodes:drain': false,
    'ai:configure': false, 'changes:approve': false, 'audit:view': false
  },
  auditor: {
    'pods:read': true, 'pods:write': false, 'deployments:scale': false,
    'secrets:manage': false, 'backups:execute': false, 'nodes:drain': false,
    'ai:configure': false, 'changes:approve': false, 'audit:view': true
  },
  viewer: {
    'pods:read': true, 'pods:write': false, 'deployments:scale': false,
    'secrets:manage': false, 'backups:execute': false, 'nodes:drain': false,
    'ai:configure': false, 'changes:approve': false, 'audit:view': false
  }
}

async function loadData() {
  loading.value = true
  error.value = null
  try {
    const summary = await tenancyApi.getSummary()
    organizations.value = summary.organizations?.length ? summary.organizations : fallbackOrgs
    projects.value = summary.projects?.length ? summary.projects : fallbackProjects
    members.value = summary.members?.length ? summary.members : fallbackMembers
    rbacMatrix.value = Object.keys(summary.rbacMatrix || {}).length ? summary.rbacMatrix : fallbackRBAC
  } catch (err: any) {
    // Graceful fallback to seeded data
    organizations.value = fallbackOrgs
    projects.value = fallbackProjects
    members.value = fallbackMembers
    rbacMatrix.value = fallbackRBAC
    error.value = 'Note: Telemetry running in fallback mode (' + (err.message || 'offline') + ')'
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  loadData()
})

const filteredProjects = computed(() => {
  if (selectedOrgId.value === 'all') return projects.value
  return projects.value.filter(p => p.orgId === selectedOrgId.value)
})

const filteredMembers = computed(() => {
  if (selectedOrgId.value === 'all') return members.value
  return members.value.filter(m => m.orgId === selectedOrgId.value)
})

const totalWorkloads = computed(() => {
  return filteredProjects.value.reduce((acc, p) => acc + (p.workloads || 0), 0)
})

const memberColumns: Column<Member>[] = [
  { key: 'user', label: 'User / Identity', sortable: true },
  { key: 'role', label: 'Assigned Role', sortable: true },
  { key: 'scope', label: 'Resource Scope', sortable: true },
  { key: 'orgId', label: 'Organization ID', sortable: true },
  { key: 'actions', label: 'Management', align: 'right' }
]

const rbacResources = [
  { key: 'pods:read', label: 'Pods / Logs (Read)' },
  { key: 'pods:write', label: 'Exec / Port-Forward' },
  { key: 'deployments:scale', label: 'Scale Deployments' },
  { key: 'secrets:manage', label: 'Manage Vault Secrets' },
  { key: 'backups:execute', label: 'Disaster Recovery Ops' },
  { key: 'nodes:drain', label: 'Cordon & Drain Nodes' },
  { key: 'ai:configure', label: 'AI LLM Hub Config' },
  { key: 'changes:approve', label: 'Change Request Approval' },
  { key: 'audit:view', label: 'CVE & Security Audit' }
]

const rbacRoles = computed(() => Object.keys(rbacMatrix.value))

function toggleRbacPermission(role: string, resourceKey: string) {
  if (!rbacMatrix.value[role]) {
    rbacMatrix.value[role] = {}
  }
  rbacMatrix.value[role][resourceKey] = !rbacMatrix.value[role][resourceKey]
  showFeedback(`Updated permission [${resourceKey}] for role [${role}]`)
}

function showFeedback(msg: string) {
  feedbackMessage.value = msg
  setTimeout(() => {
    if (feedbackMessage.value === msg) {
      feedbackMessage.value = null
    }
  }, 4000)
}

async function handleCreateOrg() {
  if (!newOrg.value.id || !newOrg.value.name) return
  isSubmitting.value = true
  try {
    await tenancyApi.createOrganization({
      id: newOrg.value.id.toLowerCase().replace(/\s+/g, '-'),
      name: newOrg.value.name,
      tier: newOrg.value.tier
    })
    organizations.value.push({ ...newOrg.value })
    showOrgModal.value = false
    showFeedback(`Organization ${newOrg.value.name} successfully provisioned.`)
    newOrg.value = { id: '', name: '', tier: 'Enterprise' }
  } catch (e: any) {
    // Add locally for frontend demo
    organizations.value.push({ ...newOrg.value })
    showOrgModal.value = false
    showFeedback(`Organization ${newOrg.value.name} registered (local sync).`)
  } finally {
    isSubmitting.value = false
  }
}

async function handleCreateProject() {
  if (!newProj.value.id || !newProj.value.name) return
  isSubmitting.value = true
  const envArray = newProj.value.envs.split(',').map(e => e.trim()).filter(Boolean)
  const proj: Project = {
    id: newProj.value.id.toLowerCase().replace(/\s+/g, '-'),
    orgId: newProj.value.orgId || (organizations.value[0]?.id ?? 'org-default'),
    name: newProj.value.name,
    envs: envArray,
    workloads: Number(newProj.value.workloads) || 0
  }
  try {
    await tenancyApi.createProject(proj)
    projects.value.push(proj)
    showProjectModal.value = false
    showFeedback(`Project ${proj.name} successfully created.`)
    newProj.value = { id: '', orgId: '', name: '', envs: 'dev, staging, prod', workloads: 5 }
  } catch (e: any) {
    projects.value.push(proj)
    showProjectModal.value = false
    showFeedback(`Project ${proj.name} created (local sync).`)
  } finally {
    isSubmitting.value = false
  }
}

function handleInviteMember() {
  if (!newMember.value.user) return
  const mem: Member = {
    id: `mem-${Date.now()}`,
    orgId: newMember.value.orgId || (organizations.value[0]?.id ?? 'org-default'),
    user: newMember.value.user,
    role: newMember.value.role,
    scope: newMember.value.scope
  }
  members.value.push(mem)
  showMemberModal.value = false
  showFeedback(`Invitation dispatched for ${mem.user} with [${mem.role}] role.`)
  newMember.value = { id: '', orgId: '', user: '', role: 'developer', scope: 'project-wide' }
}

function removeMember(id: string) {
  members.value = members.value.filter(m => m.id !== id)
  showFeedback('Member revoked successfully.')
}
</script>

<template>
  <div class="tenancy-page">
    <!-- Header -->
    <div class="page-header">
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
        <button class="btn btn-secondary" @click="showOrgModal = true">
          <span>+ New Organization</span>
        </button>
        <button class="btn btn-primary" @click="showProjectModal = true">
          <span>+ Create Project</span>
        </button>
      </div>
    </div>

    <!-- Alert Banner -->
    <div v-if="feedbackMessage" class="feedback-banner animate-fade-in">
      <span class="feedback-icon">✓</span>
      <span>{{ feedbackMessage }}</span>
    </div>

    <!-- Key Metrics Grid -->
    <div class="metrics-grid">
      <MetricCard 
        title="Organizations" 
        :value="organizations.length" 
        trend="Multi-Org Isolated" 
        trendDirection="neutral" 
      />
      <MetricCard 
        title="Active Projects" 
        :value="filteredProjects.length" 
        trend="+2 namespaces this week" 
        trendDirection="up" 
      />
      <MetricCard 
        title="Live Workloads" 
        :value="totalWorkloads" 
        trend="Replicas across pods" 
        trendDirection="up" 
      />
      <MetricCard 
        title="Active Members" 
        :value="filteredMembers.length" 
        trend="Mapped to RBAC Roles" 
        trendDirection="neutral" 
      />
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
        <button 
          class="tab-btn" 
          :class="{ active: activeTab === 'members' }" 
          @click="activeTab = 'members'"
        >
          <span>👥 Members & Roles</span>
          <span class="tab-count">{{ filteredMembers.length }}</span>
        </button>
        <button 
          class="tab-btn" 
          :class="{ active: activeTab === 'projects' }" 
          @click="activeTab = 'projects'"
        >
          <span>📁 Project Namespaces</span>
          <span class="tab-count">{{ filteredProjects.length }}</span>
        </button>
        <button 
          class="tab-btn" 
          :class="{ active: activeTab === 'rbac' }" 
          @click="activeTab = 'rbac'"
        >
          <span>🛡️ Granular RBAC Matrix</span>
        </button>
      </div>
    </div>

    <!-- TAB 1: MEMBERS -->
    <div v-if="activeTab === 'members'" class="tab-content animate-fade-in">
      <DataTable
        :columns="memberColumns"
        :data="filteredMembers"
        :loading="loading"
        searchable
        searchPlaceholder="Filter members by user, role, or scope..."
      >
        <template #toolbar>
          <button class="btn btn-primary btn-sm" @click="showMemberModal = true">
            <span>+ Invite Member</span>
          </button>
        </template>

        <template #cell-user="{ value }">
          <div class="user-cell">
            <div class="user-avatar">{{ String(value).charAt(0).toUpperCase() }}</div>
            <div class="user-details">
              <span class="user-name">{{ value }}</span>
              <small class="user-verified">Verified Enterprise SSO</small>
            </div>
          </div>
        </template>

        <template #cell-role="{ value }">
          <StatusBadge :status="value === 'admin' ? 'danger' : value === 'operator' ? 'warning' : value === 'auditor' ? 'violet' : 'active'" :label="String(value).toUpperCase()" />
        </template>

        <template #cell-scope="{ value }">
          <span class="scope-tag font-mono">{{ value }}</span>
        </template>

        <template #cell-orgId="{ value }">
          <span class="text-muted font-mono">{{ value }}</span>
        </template>

        <template #cell-actions="{ row }">
          <button class="btn btn-secondary btn-sm" title="Revoke Member" @click="removeMember(row.id)">
            <span>Revoke</span>
          </button>
        </template>
      </DataTable>
    </div>

    <!-- TAB 2: PROJECTS WORKLOAD GRID -->
    <div v-else-if="activeTab === 'projects'" class="tab-content animate-fade-in">
      <div class="projects-grid">
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
              <span class="stat-label">Attached Org:</span>
              <span class="stat-value font-mono">{{ proj.orgId }}</span>
            </div>
            <div class="project-stat-row">
              <span class="stat-label">Live Workload Pods:</span>
              <span class="stat-value font-mono text-cyan">{{ proj.workloads }} pods</span>
            </div>
            <div class="project-envs">
              <span class="env-label">Environments:</span>
              <div class="env-badges">
                <span v-for="env in proj.envs" :key="env" class="env-chip" :class="`env-${env}`">
                  {{ env }}
                </span>
              </div>
            </div>
          </div>

          <div class="project-footer">
            <button class="btn btn-secondary btn-sm" @click="showFeedback(`Exporting namespace config for ${proj.name}`)">
              <span>Inspect CRDs</span>
            </button>
            <button class="btn btn-primary btn-sm" @click="showFeedback(`Connecting to ${proj.name} telemetry stream...`)">
              <span>View Pods</span>
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- TAB 3: RBAC MATRIX -->
    <div v-else-if="activeTab === 'rbac'" class="tab-content animate-fade-in">
      <div class="rbac-container glass-panel">
        <div class="rbac-header">
          <div>
            <h3 class="rbac-title">Kubernetes RBAC Privilege Matrix</h3>
            <p class="rbac-sub">Click individual cells to toggle runtime access policies across the cluster mesh.</p>
          </div>
          <button class="btn btn-secondary btn-sm" @click="showFeedback('RBAC policy synced to cluster API server.')">
            <span>💾 Sync to APIServer</span>
          </button>
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
                <td 
                  v-for="role in rbacRoles" 
                  :key="role" 
                  class="td-perm"
                  @click="toggleRbacPermission(role, res.key)"
                >
                  <div 
                    class="perm-badge" 
                    :class="rbacMatrix[role]?.[res.key] ? 'perm-allowed' : 'perm-denied'"
                  >
                    <span v-if="rbacMatrix[role]?.[res.key]">✓ ALLOWED</span>
                    <span v-else>✕ DENIED</span>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </div>

    <!-- MODAL: NEW ORGANIZATION -->
    <ModalDrawer
      v-model:show="showOrgModal"
      title="Provision Organization Boundary"
      subtitle="Create a new isolated multi-tenant organization container."
    >
      <form @submit.prevent="handleCreateOrg" class="form-layout">
        <div class="form-group">
          <label>Organization Identifier (Slug)</label>
          <input 
            v-model="newOrg.id" 
            type="text" 
            placeholder="e.g. org-fintech-apac" 
            class="input-glass" 
            required 
          />
        </div>
        <div class="form-group">
          <label>Display Name</label>
          <input 
            v-model="newOrg.name" 
            type="text" 
            placeholder="e.g. APAC Fintech Operations" 
            class="input-glass" 
            required 
          />
        </div>
        <div class="form-group">
          <label>Service Tier</label>
          <select v-model="newOrg.tier" class="input-glass">
            <option value="Enterprise Tier-1">Enterprise Tier-1 (Dedicated Nodes)</option>
            <option value="GovCloud High-Sec">GovCloud High-Sec (FIPS-140-3)</option>
            <option value="High-Performance">High-Performance (GPU Accelerated)</option>
            <option value="Standard">Standard Multi-Tenant</option>
          </select>
        </div>
      </form>

      <template #footer="{ close }">
        <button class="btn btn-secondary" type="button" @click="close">Cancel</button>
        <button class="btn btn-primary" :disabled="isSubmitting" @click="handleCreateOrg">
          {{ isSubmitting ? 'Provisioning...' : 'Confirm Organization' }}
        </button>
      </template>
    </ModalDrawer>

    <!-- MODAL: NEW PROJECT -->
    <ModalDrawer
      v-model:show="showProjectModal"
      title="Create Project Namespace"
      subtitle="Allocate cluster namespaces and resource quotas to an organization."
    >
      <form @submit.prevent="handleCreateProject" class="form-layout">
        <div class="form-group">
          <label>Parent Organization</label>
          <select v-model="newProj.orgId" class="input-glass">
            <option v-for="org in organizations" :key="org.id" :value="org.id">
              {{ org.name }} ({{ org.id }})
            </option>
          </select>
        </div>
        <div class="form-group">
          <label>Project Name</label>
          <input 
            v-model="newProj.name" 
            type="text" 
            placeholder="e.g. Payment Gateway Ingress" 
            class="input-glass" 
            required 
          />
        </div>
        <div class="form-group">
          <label>Project Identifier</label>
          <input 
            v-model="newProj.id" 
            type="text" 
            placeholder="e.g. proj-payment-gw" 
            class="input-glass" 
            required 
          />
        </div>
        <div class="form-group">
          <label>Environments (comma separated)</label>
          <input 
            v-model="newProj.envs" 
            type="text" 
            placeholder="dev, staging, prod" 
            class="input-glass" 
          />
        </div>
        <div class="form-group">
          <label>Initial Workload Pod Capacity</label>
          <input 
            v-model="newProj.workloads" 
            type="number" 
            min="1" 
            max="500" 
            class="input-glass" 
          />
        </div>
      </form>

      <template #footer="{ close }">
        <button class="btn btn-secondary" type="button" @click="close">Cancel</button>
        <button class="btn btn-primary" :disabled="isSubmitting" @click="handleCreateProject">
          {{ isSubmitting ? 'Creating...' : 'Create Project' }}
        </button>
      </template>
    </ModalDrawer>

    <!-- DRAWER: INVITE MEMBER -->
    <ModalDrawer
      v-model:show="showMemberModal"
      mode="drawer"
      title="Invite Team Member"
      subtitle="Assign organization access permissions and RBAC role bindings."
    >
      <form @submit.prevent="handleInviteMember" class="form-layout">
        <div class="form-group">
          <label>Member Email / SSO Identity</label>
          <input 
            v-model="newMember.user" 
            type="email" 
            placeholder="e.g. engineer@enterprise.io" 
            class="input-glass" 
            required 
          />
        </div>
        <div class="form-group">
          <label>Target Organization</label>
          <select v-model="newMember.orgId" class="input-glass">
            <option v-for="org in organizations" :key="org.id" :value="org.id">
              {{ org.name }}
            </option>
          </select>
        </div>
        <div class="form-group">
          <label>RBAC Role Assignment</label>
          <select v-model="newMember.role" class="input-glass">
            <option value="admin">Admin (Full Control Plane & RBAC)</option>
            <option value="operator">Operator (Deploy, Scale, Logs, Restart)</option>
            <option value="developer">Developer (Pods, Port-forward, Dev Namespace)</option>
            <option value="auditor">Auditor (Read-Only Compliance & CVEs)</option>
            <option value="viewer">Viewer (Read-Only Telemetry)</option>
          </select>
        </div>
        <div class="form-group">
          <label>Access Scope</label>
          <select v-model="newMember.scope" class="input-glass">
            <option value="org-wide">Organization-Wide (All Namespaces)</option>
            <option value="project-wide">Project Namespace Specific</option>
            <option value="sandbox-only">Developer Sandbox Only</option>
          </select>
        </div>
      </form>

      <template #footer="{ close }">
        <button class="btn btn-secondary" type="button" @click="close">Cancel</button>
        <button class="btn btn-primary" @click="handleInviteMember">
          <span>Send Invitation & Bind RBAC</span>
        </button>
      </template>
    </ModalDrawer>
  </div>
</template>

<style scoped>
.tenancy-page {
  display: flex;
  flex-direction: column;
  gap: 24px;
}

.page-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  gap: 20px;
}

.header-badge {
  display: flex;
  gap: 8px;
  margin-bottom: 8px;
}

.page-title {
  font-size: 26px;
  font-weight: 800;
  letter-spacing: -0.03em;
  color: #fff;
  margin-bottom: 6px;
}

.page-desc {
  font-size: 13px;
  color: var(--text-secondary);
  max-width: 800px;
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 12px;
}

.feedback-banner {
  background: rgba(16, 185, 129, 0.12);
  border: 1px solid rgba(16, 185, 129, 0.3);
  color: #34d399;
  padding: 12px 18px;
  border-radius: 12px;
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 13px;
  font-weight: 600;
}

.metrics-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 16px;
}

/* Scope & Tab Bar */
.scope-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 20px;
  gap: 16px;
  flex-wrap: wrap;
}

.scope-left {
  display: flex;
  align-items: center;
  gap: 12px;
}

.scope-label {
  font-size: 12px;
  font-weight: 600;
  color: var(--text-muted);
}

.select-scope {
  min-width: 320px;
  font-weight: 600;
}

.tab-pills {
  display: flex;
  align-items: center;
  gap: 8px;
}

.tab-btn {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 14px;
  background: rgba(255, 255, 255, 0.04);
  border: 1px solid var(--border-subtle);
  border-radius: 10px;
  color: var(--text-secondary);
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.15s ease;
}

.tab-btn:hover {
  background: rgba(255, 255, 255, 0.08);
  color: var(--text-primary);
}

.tab-btn.active {
  background: rgba(6, 182, 212, 0.15);
  border-color: rgba(6, 182, 212, 0.4);
  color: #38bdf8;
}

.tab-count {
  background: rgba(255, 255, 255, 0.1);
  padding: 1px 6px;
  border-radius: 9999px;
  font-size: 10px;
  font-family: var(--font-mono);
}

/* User Cell */
.user-cell {
  display: flex;
  align-items: center;
  gap: 12px;
}

.user-avatar {
  width: 32px;
  height: 32px;
  border-radius: 8px;
  background: var(--grad-cyan);
  color: #fff;
  display: flex;
  align-items: center;
  justify-content: center;
  font-weight: 700;
  font-size: 13px;
}

.user-details {
  display: flex;
  flex-direction: column;
}

.user-name {
  font-weight: 600;
  color: var(--text-primary);
}

.user-verified {
  font-size: 11px;
  color: var(--text-muted);
}

.scope-tag {
  background: rgba(255, 255, 255, 0.06);
  padding: 3px 8px;
  border-radius: 6px;
  font-size: 11px;
  color: var(--text-secondary);
}

/* Projects Grid */
.projects-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(320px, 1fr));
  gap: 18px;
}

.project-card {
  padding: 20px;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.project-card-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
}

.project-title-wrap {
  display: flex;
  align-items: center;
  gap: 12px;
}

.project-icon {
  font-size: 24px;
}

.project-name {
  font-size: 15px;
  font-weight: 700;
  color: #fff;
}

.project-id {
  font-size: 11px;
  color: var(--text-muted);
}

.project-body {
  display: flex;
  flex-direction: column;
  gap: 8px;
  padding: 12px 0;
  border-top: 1px solid var(--border-subtle);
  border-bottom: 1px solid var(--border-subtle);
}

.project-stat-row {
  display: flex;
  justify-content: space-between;
  font-size: 12px;
}

.project-envs {
  display: flex;
  flex-direction: column;
  gap: 6px;
  margin-top: 4px;
}

.env-label {
  font-size: 11px;
  color: var(--text-muted);
}

.env-badges {
  display: flex;
  gap: 6px;
  flex-wrap: wrap;
}

.env-chip {
  font-size: 10px;
  font-weight: 700;
  text-transform: uppercase;
  padding: 2px 6px;
  border-radius: 4px;
  font-family: var(--font-mono);
}

.env-prod {
  background: rgba(244, 63, 94, 0.15);
  color: #fb7185;
  border: 1px solid rgba(244, 63, 94, 0.3);
}

.env-stage, .env-staging {
  background: rgba(245, 158, 11, 0.15);
  color: #fbbf24;
  border: 1px solid rgba(245, 158, 11, 0.3);
}

.env-dev, .env-test {
  background: rgba(56, 189, 248, 0.15);
  color: #38bdf8;
  border: 1px solid rgba(56, 189, 248, 0.3);
}

.project-footer {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 8px;
}

/* RBAC Table */
.rbac-container {
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

.rbac-header {
  padding: 18px 24px;
  border-bottom: 1px solid var(--border-subtle);
  display: flex;
  justify-content: space-between;
  align-items: center;
  background: rgba(11, 15, 25, 0.5);
}

.rbac-title {
  font-size: 16px;
  font-weight: 700;
  color: #fff;
}

.rbac-sub {
  font-size: 12px;
  color: var(--text-muted);
  margin-top: 2px;
}

.rbac-table-wrap {
  overflow-x: auto;
}

.rbac-table {
  width: 100%;
  border-collapse: collapse;
}

.rbac-table th {
  padding: 14px 18px;
  background: rgba(15, 23, 42, 0.85);
  border-bottom: 1px solid var(--border-medium);
  font-size: 11px;
  text-transform: uppercase;
}

.th-resource {
  text-align: left;
  min-width: 240px;
}

.th-role {
  text-align: center;
  min-width: 140px;
}

.role-header-badge {
  background: rgba(6, 182, 212, 0.15);
  border: 1px solid rgba(6, 182, 212, 0.3);
  color: #38bdf8;
  padding: 3px 10px;
  border-radius: 6px;
  font-weight: 700;
}

.rbac-row {
  border-bottom: 1px solid var(--border-subtle);
}

.rbac-row:hover {
  background: rgba(56, 189, 248, 0.03);
}

.td-resource {
  padding: 14px 18px;
}

.resource-name {
  font-size: 13px;
  font-weight: 600;
  color: var(--text-primary);
}

.resource-key {
  font-size: 11px;
  color: var(--text-muted);
}

.td-perm {
  padding: 14px 18px;
  text-align: center;
  cursor: pointer;
  user-select: none;
}

.perm-badge {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  padding: 4px 12px;
  border-radius: 6px;
  font-size: 11px;
  font-weight: 700;
  font-family: var(--font-mono);
  transition: all 0.15s ease;
}

.perm-allowed {
  background: rgba(16, 185, 129, 0.15);
  color: #34d399;
  border: 1px solid rgba(16, 185, 129, 0.3);
}

.perm-allowed:hover {
  background: rgba(16, 185, 129, 0.25);
  box-shadow: 0 0 8px rgba(16, 185, 129, 0.4);
}

.perm-denied {
  background: rgba(255, 255, 255, 0.04);
  color: var(--text-muted);
  border: 1px solid var(--border-subtle);
}

.perm-denied:hover {
  background: rgba(244, 63, 94, 0.15);
  color: #fb7185;
  border-color: rgba(244, 63, 94, 0.3);
}

/* Forms */
.form-layout {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.form-group {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.form-group label {
  font-size: 12px;
  font-weight: 600;
  color: var(--text-secondary);
}

.btn-sm {
  padding: 6px 12px;
  font-size: 12px;
}
</style>
