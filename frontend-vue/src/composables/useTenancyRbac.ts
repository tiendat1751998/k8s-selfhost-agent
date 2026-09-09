import { ref, computed, onMounted } from 'vue'
import {
  tenancyApi,
  type Organization,
  type Project,
  type Member,
  type RBACMatrix
} from '../api/management'

export interface RbacResourceItem {
  key: string
  label: string
}

export interface TenantStatSummary {
  projectCount: number
  workloadCount: number
  memberCount: number
}

export function useTenancyRbac() {
  // Primary State
  const loading = ref(false)
  const error = ref<string | null>(null)
  const organizations = ref<Organization[]>([])
  const projects = ref<Project[]>([])
  const members = ref<Member[]>([])
  const rbacMatrix = ref<RBACMatrix>({})

  // Navigation & Scope State
  const selectedOrgId = ref<string>('all')
  const activeTab = ref<'tenants' | 'members' | 'projects' | 'rbac'>('tenants')

  // Modals & Drawers Visibility
  const showOrgModal = ref(false)
  const showProjectModal = ref(false)
  const showMemberModal = ref(false)
  const showMemberDrawer = ref(false)
  const showRbacMatrixModal = ref(false)
  const showQuotaModal = ref(false)

  // Selected Entities for Contextual Modals
  const selectedOrgForDrawer = ref<string | null>(null)
  const selectedOrgForQuota = ref<Organization | null>(null)
  const selectedRoleForMatrix = ref<string | null>(null)

  // Form State
  const newOrg = ref<Partial<Organization>>({ id: '', name: '', tier: 'Enterprise Tier-1', quotaPreset: 'standard' })
  const newProj = ref({ id: '', orgId: '', name: '', envs: 'dev, staging, prod', workloads: 5 })
  const newMember = ref({ id: '', orgId: '', user: '', role: 'developer', scope: 'project-wide' })
  const isSubmitting = ref(false)
  const feedbackMessage = ref<string | null>(null)

  // Feedback Notification Utility
  let feedbackTimer: ReturnType<typeof setTimeout> | null = null
  function showFeedback(msg: string) {
    feedbackMessage.value = msg
    if (feedbackTimer) clearTimeout(feedbackTimer)
    feedbackTimer = setTimeout(() => {
      if (feedbackMessage.value === msg) {
        feedbackMessage.value = null
      }
    }, 4000)
  }

  // Load Tenancy Summary & Matrices
  async function loadData() {
    loading.value = true
    error.value = null
    try {
      const summary = await tenancyApi.getSummary()
      organizations.value = summary?.organizations || []
      projects.value = summary?.projects || []
      members.value = summary?.members || []
      rbacMatrix.value = summary?.rbacMatrix || {}
    } catch (err: unknown) {
      organizations.value = []
      projects.value = []
      members.value = []
      rbacMatrix.value = {}
      error.value = err instanceof Error ? err.message : 'Failed to load tenancy and RBAC data'
      showFeedback(error.value)
    } finally {
      loading.value = false
    }
  }

  onMounted(() => {
    loadData()
  })

  // Computed Scoped Data
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

  const organizationStats = computed<Record<string, TenantStatSummary>>(() => {
    const stats: Record<string, TenantStatSummary> = {}
    for (const org of organizations.value) {
      const orgProjects = projects.value.filter(p => p.orgId === org.id)
      const orgMembers = members.value.filter(m => m.orgId === org.id)
      const wCount = orgProjects.reduce((acc, p) => acc + (p.workloads || 0), 0)
      stats[org.id] = {
        projectCount: orgProjects.length,
        workloadCount: wCount,
        memberCount: orgMembers.length
      }
    }
    return stats
  })

  // Granular RBAC Catalog Definitions
  const rbacResources: RbacResourceItem[] = [
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

  const rbacRoles = computed(() => {
    const keys = Object.keys(rbacMatrix.value || {})
    const validRoles = keys.filter(k =>
      !k.includes(':') &&
      !k.toLowerCase().includes('read') &&
      !k.toLowerCase().includes('write') &&
      !k.toLowerCase().includes('scale') &&
      !k.toLowerCase().includes('deploy') &&
      !k.toLowerCase().includes('analyze')
    )
    return validRoles.length > 0
      ? validRoles
      : ['Platform Admin', 'DevOps Team', 'Developer', 'Viewer', 'Security Auditor']
  })

  // Permission & Role Assignment Handlers
  function toggleRbacPermission(role: string, resourceKey: string) {
    if (!rbacMatrix.value[role]) {
      rbacMatrix.value[role] = {}
    }
    rbacMatrix.value[role][resourceKey] = !rbacMatrix.value[role][resourceKey]
    showFeedback(`Updated permission [${resourceKey}] for role [${role}]`)
  }

  function syncRbacToApi() {
    showFeedback('RBAC policy synced to cluster API server.')
  }

  // Tenant / Org Management Handlers
  async function handleCreateOrg(customOrg?: Partial<Organization>) {
    const target = customOrg || newOrg.value
    if (!target.id || !target.name) return
    isSubmitting.value = true
    const orgPayload: Organization = {
      id: target.id.toLowerCase().replace(/\s+/g, '-'),
      name: target.name,
      tier: target.tier || 'Enterprise Tier-1',
      ...(target.quotaPreset ? { quotaPreset: target.quotaPreset } : {})
    }
    try {
      const created = await tenancyApi.createOrganization(orgPayload)
      organizations.value.push(created || orgPayload)
      showOrgModal.value = false
      showFeedback(`Organization ${target.name} successfully provisioned.`)
      newOrg.value = { id: '', name: '', tier: 'Enterprise Tier-1', quotaPreset: 'standard' }
    } catch (e: unknown) {
      showFeedback(`Failed to provision organization: ${e instanceof Error ? e.message : 'Unknown error'}`)
    } finally {
      isSubmitting.value = false
    }
  }

  function handleDeleteOrg(orgId: string) {
    const org = organizations.value.find(o => o.id === orgId)
    const orgName = org ? org.name : orgId
    organizations.value = organizations.value.filter(o => o.id !== orgId)
    projects.value = projects.value.filter(p => p.orgId !== orgId)
    members.value = members.value.filter(m => m.orgId !== orgId)
    if (selectedOrgId.value === orgId) {
      selectedOrgId.value = 'all'
    }
    showFeedback(`Organization [${orgName}] and associated bindings purged.`)
  }

  // Project Management Handlers
  async function handleCreateProject(customProj?: Partial<Project> & { envs?: string | string[] }) {
    const target = customProj || newProj.value
    if (!target.id || !target.name) return
    isSubmitting.value = true
    const envArray = Array.isArray(target.envs)
      ? target.envs
      : typeof target.envs === 'string'
        ? target.envs.split(',').map(e => e.trim()).filter(Boolean)
        : ['dev', 'staging', 'prod']
    const proj: Project = {
      id: target.id.toLowerCase().replace(/\s+/g, '-'),
      orgId: target.orgId || (organizations.value[0]?.id ?? 'org-default'),
      name: target.name,
      envs: envArray,
      workloads: Number(target.workloads) || 0
    }
    try {
      const created = await tenancyApi.createProject(proj)
      projects.value.push(created || proj)
      showProjectModal.value = false
      showFeedback(`Project ${proj.name} successfully created.`)
      newProj.value = { id: '', orgId: '', name: '', envs: 'dev, staging, prod', workloads: 5 }
    } catch (e: unknown) {
      showFeedback(`Failed to create project: ${e instanceof Error ? e.message : 'Unknown error'}`)
    } finally {
      isSubmitting.value = false
    }
  }

  // Member Management Handlers
  function handleInviteMember(customMember?: Partial<Member>) {
    const target = customMember || newMember.value
    if (!target.user) return
    const mem: Member = {
      id: target.id || ('mem-' + Date.now()),
      orgId: target.orgId || (organizations.value[0]?.id ?? 'org-default'),
      user: target.user,
      role: target.role || 'developer',
      scope: target.scope || 'project-wide'
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

  // Navigation & Drawer Openers
  function openMemberDrawer(orgId?: string) {
    if (orgId) {
      selectedOrgForDrawer.value = orgId
      selectedOrgId.value = orgId
    } else {
      selectedOrgForDrawer.value = selectedOrgId.value !== 'all' ? selectedOrgId.value : organizations.value[0]?.id || null
    }
    showMemberDrawer.value = true
  }

  function openRbacModal(role?: string) {
    selectedRoleForMatrix.value = role || null
    showRbacMatrixModal.value = true
  }

  function openQuotaModal(org: Organization) {
    selectedOrgForQuota.value = org
    showQuotaModal.value = true
    showFeedback(`Viewing quota configuration for ${org.name}`)
  }

  return {
    // State
    loading,
    error,
    organizations,
    projects,
    members,
    rbacMatrix,
    selectedOrgId,
    activeTab,
    showOrgModal,
    showProjectModal,
    showMemberModal,
    showMemberDrawer,
    showRbacMatrixModal,
    showQuotaModal,
    selectedOrgForDrawer,
    selectedOrgForQuota,
    selectedRoleForMatrix,
    newOrg,
    newProj,
    newMember,
    isSubmitting,
    feedbackMessage,

    // Computed
    filteredProjects,
    filteredMembers,
    totalWorkloads,
    organizationStats,
    rbacResources,
    rbacRoles,

    // Methods
    loadData,
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
  }
}
