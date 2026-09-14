import { createRouter, createWebHistory } from 'vue-router'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    // Authentication
    {
      path: '/login',
      name: 'login',
      component: () => import('../views/LoginView.vue'),
      meta: { public: true, title: 'Login' }
    },

    // 1. Observability & Ops
    {
      path: '/',
      name: 'overview',
      component: () => import('../views/OverviewView.vue'),
      meta: { requiresAuth: true, title: 'Fleet Overview' }
    },
    {
      path: '/incidents',
      name: 'incidents',
      component: () => import('../views/IncidentsView.vue'),
      meta: { requiresAuth: true, title: 'Incidents & RCA' }
    },
    {
      path: '/agents',
      name: 'agents',
      component: () => import('../views/AgentsView.vue'),
      meta: { requiresAuth: true, title: 'Autonomous AI SRE Agents' }
    },
    {
      path: '/slo',
      name: 'slo',
      component: () => import('../views/SLOView.vue'),
      meta: { requiresAuth: true, title: 'SLOs & Error Budgets' }
    },
    {
      path: '/logs',
      name: 'logs',
      component: () => import('../views/LogStreamView.vue'),
      meta: { requiresAuth: true, title: 'Real-Time Log Stream' }
    },

    // 2. Compute & Fleet
    {
      path: '/fleet',
      name: 'fleet',
      component: () => import('../views/FleetView.vue'),
      meta: { requiresAuth: true, title: 'Fleet Multi-Cluster' }
    },
    {
      path: '/hosts',
      alias: ['/infra/hosts', '/infra-hosts'],
      name: 'infra-hosts',
      component: () => import('../views/InfraHostsView.vue'),
      meta: { requiresAuth: true, title: 'Infrastructure Hosts' }
    },
    {
      path: '/deployments',
      name: 'deployments',
      component: () => import('../views/DeploymentsView.vue'),
      meta: { requiresAuth: true, title: 'Deployments & Apps' }
    },
    {
      path: '/workloads',
      name: 'workloads',
      component: () => import('../views/DeploymentsView.vue'),
      meta: { requiresAuth: true, title: 'Workloads' }
    },
    {
      path: '/promotions',
      name: 'promotions',
      component: () => import('../views/PromotionsView.vue'),
      meta: { requiresAuth: true, title: 'Promotions Pipeline' }
    },
    {
      path: '/swarm',
      alias: ['/docker-swarm', '/docker', '/compute'],
      name: 'swarm',
      component: () => import('../views/DockerSwarmView.vue'),
      meta: { requiresAuth: true, title: 'Docker Swarm' }
    },
    {
      path: '/explorer',
      name: 'explorer',
      component: () => import('../views/ExplorerView.vue'),
      meta: { requiresAuth: true, title: 'Cluster Explorer' }
    },
    {
      path: '/helm',
      name: 'helm',
      component: () => import('../views/HelmCatalogView.vue'),
      meta: { requiresAuth: true, title: 'Helm Catalog' }
    },

    // 3. Governance & Security
    {
      path: '/audit',
      name: 'audit',
      component: () => import('../views/AuditView.vue'),
      meta: { requiresAuth: true, title: 'Audit & CVEs' }
    },
    {
      path: '/security',
      alias: ['/security/devsecops', '/devsecops'],
      name: 'security',
      component: () => import('../views/DevSecOpsView.vue'),
      meta: { requiresAuth: true, title: 'DevSecOps & Security' }
    },
    {
      path: '/compliance',
      alias: ['/security/compliance', '/compliance-center'],
      name: 'compliance',
      component: () => import('../views/ComplianceView.vue'),
      meta: { requiresAuth: true, title: 'Compliance & CIS' }
    },
    {
      path: '/drift',
      name: 'drift',
      component: () => import('../views/DriftView.vue'),
      meta: { requiresAuth: true, title: 'Config Drift & GitOps' }
    },
    {
      path: '/backup',
      name: 'backup',
      component: () => import('../views/BackupRestoreView.vue'),
      meta: { requiresAuth: true, title: 'Disaster Recovery & Backup' }
    },

    // 4. Automation & FinOps
    {
      path: '/automation',
      name: 'automation',
      component: () => import('../views/AutomationView.vue'),
      meta: { requiresAuth: true, title: 'Automation Rules' }
    },
    {
      path: '/runbooks',
      name: 'runbooks',
      component: () => import('../views/RunbooksView.vue'),
      meta: { requiresAuth: true, title: 'SRE Runbooks' }
    },
    {
      path: '/cost',
      alias: ['/finops/cost', '/finops'],
      name: 'cost',
      component: () => import('../views/CostFinOpsView.vue'),
      meta: { requiresAuth: true, title: 'Cost Optimization & FinOps' }
    },
    {
      path: '/capacity',
      name: 'capacity',
      component: () => import('../views/CapacityView.vue'),
      meta: { requiresAuth: true, title: 'Capacity Forecast' }
    },

    // 5. Enterprise Management
    {
      path: '/tenancy',
      alias: ['/tenancy/rbac', '/rbac'],
      name: 'tenancy',
      component: () => import('../views/TenancyRbacView.vue'),
      meta: { requiresAuth: true, title: 'Tenancy & RBAC' }
    },
    {
      path: '/ai-hub',
      alias: ['/ai/providers', '/ai-providers'],
      name: 'ai-hub',
      component: () => import('../views/AIProviderHubView.vue'),
      meta: { requiresAuth: true, title: 'AI Provider Hub' }
    },
    {
      path: '/changes',
      name: 'changes',
      component: () => import('../views/ChangesView.vue'),
      meta: { requiresAuth: true, title: 'Change Requests' }
    },
    {
      path: '/alerts',
      name: 'alerts',
      component: () => import('../views/AlertsView.vue'),
      meta: { requiresAuth: true, title: 'Alerts & Channels' }
    },
    {
      path: '/reports',
      name: 'reports',
      component: () => import('../views/ReportsView.vue'),
      meta: { requiresAuth: true, title: 'Reports Center' }
    },
    {
      path: '/catalog',
      name: 'ServiceCatalog',
      component: () => import('../views/ServiceCatalogView.vue'),
      meta: { requiresAuth: true, title: 'Service Catalog' }
    },
    {
      path: '/scaffolder',
      name: 'ScaffolderTemplates',
      component: () => import('../views/ScaffolderView.vue'),
      meta: { requiresAuth: true, title: 'Scaffolder Templates' }
    },
    {
      path: '/ecosystem',
      name: 'ecosystem',
      component: () => import('../views/EcosystemView.vue'),
      meta: { requiresAuth: true, title: 'Ecosystem Tools' }
    },
    {
      path: '/plugins',
      name: 'plugins',
      component: () => import('../views/PluginsView.vue'),
      meta: { requiresAuth: true, title: 'Plugin Hub' }
    },
    {
      path: '/settings',
      name: 'settings',
      component: () => import('../views/SettingsView.vue'),
      meta: { requiresAuth: true, title: 'System Settings' }
    },
    {
      path: '/settings/2fa-setup',
      name: 'totp-setup',
      component: () => import('../views/TOTPSetupView.vue'),
      meta: { requiresAuth: true, title: 'Two-Factor Authentication Setup' }
    },

    // 6. Generic Platform Fallback / Extension
    {
      path: '/platform/:feature?',
      alias: ['/platform'],
      name: 'platform',
      component: () => import('../views/GenericPlatformView.vue'),
      meta: { requiresAuth: true, title: 'Platform Telemetry' }
    },

    // Catch-All
    {
      path: '/:pathMatch(.*)*',
      name: 'not-found',
      redirect: '/'
    }
  ]
})

router.beforeEach((to, _from, next) => {
  const token = typeof window !== 'undefined' ? localStorage.getItem('k8s_token') : null
  const isAuthenticated = !!token

  if (to.meta.requiresAuth && !isAuthenticated) {
    next({ path: '/login', query: { redirect: to.fullPath } })
  } else if (to.path === '/login' && isAuthenticated) {
    next({ path: '/' })
  } else {
    next()
  }
})

router.afterEach((to) => {
  const title = (to.meta?.title as string) || ''
  if (title && typeof document !== 'undefined') {
    document.title = `${title} | K8s Self-Host Platform`
  }
})

export default router
