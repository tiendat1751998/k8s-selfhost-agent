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
      path: '/alerts',
      name: 'alerts',
      component: () => import('../views/AlertsView.vue'),
      meta: { requiresAuth: true, title: 'Alerts & Channels' }
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
      alias: ['/totp', '/2fa', '/mfa'],
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

    // 7. Deprecated / Purged Bloat Routes (Strict Redirect to /)
    { path: '/promotions', redirect: '/' },
    { path: '/drift', redirect: '/' },
    { path: '/compliance', alias: ['/security/compliance', '/compliance-center'], redirect: '/' },
    { path: '/backup', redirect: '/' },
    { path: '/ai-hub', alias: ['/ai/providers', '/ai-providers'], redirect: '/' },
    { path: '/changes', redirect: '/' },
    { path: '/reports', redirect: '/' },
    { path: '/scaffolder', redirect: '/' },
    { path: '/plugins', redirect: '/' },
    { path: '/catalog', alias: ['/services'], redirect: '/' },
    { path: '/ecosystem', redirect: '/' },

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
