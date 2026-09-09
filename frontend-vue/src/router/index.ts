import { createRouter, createWebHistory } from 'vue-router'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    // Authentication
    {
      path: '/login',
      name: 'login',
      component: () => import('../views/LoginView.vue'),
      meta: { public: true }
    },

    // 1. Observability & Ops
    {
      path: '/',
      name: 'overview',
      component: () => import('../views/OverviewView.vue'),
      meta: { requiresAuth: true }
    },
    {
      path: '/incidents',
      name: 'incidents',
      component: () => import('../views/IncidentsView.vue'),
      meta: { requiresAuth: true }
    },

    {
      path: '/slo',
      name: 'slo',
      component: () => import('../views/SLOView.vue'),
      meta: { requiresAuth: true }
    },
    {
      path: '/logs',
      name: 'logs',
      component: () => import('../views/LogStreamView.vue'),
      meta: { requiresAuth: true }
    },

    // 2. Compute & Fleet
    {
      path: '/fleet',
      name: 'fleet',
      component: () => import('../views/FleetView.vue'),
      meta: { requiresAuth: true }
    },
    {
      path: '/hosts',
      name: 'infra-hosts',
      component: () => import('../views/InfraHostsView.vue'),
      meta: { requiresAuth: true }
    },
    {
      path: '/deployments',
      name: 'deployments',
      component: () => import('../views/DeploymentsView.vue'),
      meta: { requiresAuth: true }
    },
    {
      path: '/workloads',
      name: 'workloads',
      component: () => import('../views/DeploymentsView.vue'),
      meta: { requiresAuth: true }
    },
    {
      path: '/promotions',
      name: 'promotions',
      component: () => import('../views/PromotionsView.vue'),
      meta: { requiresAuth: true }
    },
    {
      path: '/docker',
      name: 'docker',
      component: () => import('../views/DockerSwarmView.vue'),
      meta: { requiresAuth: true }
    },
    {
      path: '/explorer',
      name: 'explorer',
      component: () => import('../views/ExplorerView.vue'),
      meta: { requiresAuth: true }
    },
    {
      path: '/helm',
      name: 'helm',
      component: () => import('../views/HelmCatalogView.vue'),
      meta: { requiresAuth: true }
    },

    // 3. Governance & Security
    {
      path: '/audit',
      name: 'audit',
      component: () => import('../views/AuditView.vue'),
      meta: { requiresAuth: true }
    },
    {
      path: '/security',
      alias: ['/security/devsecops', '/devsecops'],
      name: 'security',
      component: () => import('../views/DevSecOpsView.vue'),
      meta: { requiresAuth: true }
    },
    {
      path: '/compliance',
      alias: ['/security/compliance', '/compliance-center'],
      name: 'compliance',
      component: () => import('../views/ComplianceView.vue'),
      meta: { requiresAuth: true }
    },
    {
      path: '/drift',
      name: 'drift',
      component: () => import('../views/DriftView.vue'),
      meta: { requiresAuth: true }
    },
    {
      path: '/backup',
      name: 'backup',
      component: () => import('../views/BackupRestoreView.vue'),
      meta: { requiresAuth: true }
    },

    // 4. Automation & FinOps
    {
      path: '/automation',
      name: 'automation',
      component: () => import('../views/AutomationView.vue'),
      meta: { requiresAuth: true }
    },
    {
      path: '/runbooks',
      name: 'runbooks',
      component: () => import('../views/RunbooksView.vue'),
      meta: { requiresAuth: true }
    },
    {
      path: '/cost',
      alias: ['/finops/cost', '/finops'],
      name: 'cost',
      component: () => import('../views/CostFinOpsView.vue'),
      meta: { requiresAuth: true }
    },
    {
      path: '/capacity',
      name: 'capacity',
      component: () => import('../views/CapacityView.vue'),
      meta: { requiresAuth: true }
    },

    // 5. Enterprise Management
    {
      path: '/tenancy',
      alias: ['/tenancy/rbac', '/rbac'],
      name: 'tenancy',
      component: () => import('../views/TenancyRbacView.vue'),
      meta: { requiresAuth: true }
    },
    {
      path: '/ai-hub',
      alias: ['/ai/providers', '/ai-providers'],
      name: 'ai-hub',
      component: () => import('../views/AIProviderHubView.vue'),
      meta: { requiresAuth: true }
    },
    {
      path: '/changes',
      name: 'changes',
      component: () => import('../views/ChangesView.vue'),
      meta: { requiresAuth: true }
    },
    {
      path: '/alerts',
      name: 'alerts',
      component: () => import('../views/AlertsView.vue'),
      meta: { requiresAuth: true }
    },
    {
      path: '/reports',
      name: 'reports',
      component: () => import('../views/ReportsView.vue'),
      meta: { requiresAuth: true }
    },
    {
      path: '/catalog',
      name: 'ServiceCatalog',
      component: () => import('../views/ServiceCatalogView.vue'),
      meta: { requiresAuth: true }
    },
    {
      path: '/scaffolder',
      name: 'ScaffolderTemplates',
      component: () => import('../views/ScaffolderView.vue'),
      meta: { requiresAuth: true }
    },
    {
      path: '/ecosystem',
      name: 'ecosystem',
      component: () => import('../views/EcosystemView.vue'),
      meta: { requiresAuth: true }
    },
    {
      path: '/plugins',
      name: 'plugins',
      component: () => import('../views/PluginsView.vue'),
      meta: { requiresAuth: true }
    },
    {
      path: '/settings',
      name: 'settings',
      component: () => import('../views/SettingsView.vue'),
      meta: { requiresAuth: true }
    },
    {
      path: '/settings/2fa-setup',
      name: 'totp-setup',
      component: () => import('../views/TOTPSetupView.vue'),
      meta: { requiresAuth: true }
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

export default router
