import { createRouter, createWebHistory } from 'vue-router'
import LoginView from '../views/LoginView.vue'
import OverviewView from '../views/OverviewView.vue'
import IncidentsView from '../views/IncidentsView.vue'
import AgentsView from '../views/AgentsView.vue'
import SLOView from '../views/SLOView.vue'
import LogStreamView from '../views/LogStreamView.vue'
import FleetView from '../views/FleetView.vue'
import DeploymentsView from '../views/DeploymentsView.vue'
import PromotionsView from '../views/PromotionsView.vue'
import DockerSwarmView from '../views/DockerSwarmView.vue'
import ExplorerView from '../views/ExplorerView.vue'
import AuditView from '../views/AuditView.vue'
import DevSecOpsView from '../views/DevSecOpsView.vue'
import ComplianceView from '../views/ComplianceView.vue'
import DriftView from '../views/DriftView.vue'
import BackupRestoreView from '../views/BackupRestoreView.vue'
import AutomationView from '../views/AutomationView.vue'
import RunbooksView from '../views/RunbooksView.vue'
import CostFinOpsView from '../views/CostFinOpsView.vue'
import CapacityView from '../views/CapacityView.vue'
import TenancyRbacView from '../views/TenancyRbacView.vue'
import AIProviderHubView from '../views/AIProviderHubView.vue'
import ChangesView from '../views/ChangesView.vue'
import AlertsView from '../views/AlertsView.vue'
import ReportsView from '../views/ReportsView.vue'
import SettingsView from '../views/SettingsView.vue'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    // Authentication
    {
      path: '/login',
      name: 'login',
      component: LoginView,
      meta: { public: true }
    },

    // 1. Observability & Ops
    {
      path: '/',
      name: 'overview',
      component: OverviewView,
      meta: { requiresAuth: true }
    },
    {
      path: '/incidents',
      name: 'incidents',
      component: IncidentsView,
      meta: { requiresAuth: true }
    },
    {
      path: '/agents',
      name: 'agents',
      component: AgentsView,
      meta: { requiresAuth: true }
    },
    {
      path: '/slo',
      name: 'slo',
      component: SLOView,
      meta: { requiresAuth: true }
    },
    {
      path: '/logs',
      name: 'logs',
      component: LogStreamView,
      meta: { requiresAuth: true }
    },

    // 2. Compute & Fleet
    {
      path: '/fleet',
      name: 'fleet',
      component: FleetView,
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
      component: DeploymentsView,
      meta: { requiresAuth: true }
    },
    {
      path: '/workloads',
      name: 'workloads',
      component: DeploymentsView,
      meta: { requiresAuth: true }
    },
    {
      path: '/promotions',
      name: 'promotions',
      component: PromotionsView,
      meta: { requiresAuth: true }
    },
    {
      path: '/docker',
      name: 'docker',
      component: DockerSwarmView,
      meta: { requiresAuth: true }
    },
    {
      path: '/explorer',
      name: 'explorer',
      component: ExplorerView,
      meta: { requiresAuth: true }
    },

    // 3. Governance & Security
    {
      path: '/audit',
      name: 'audit',
      component: AuditView,
      meta: { requiresAuth: true }
    },
    {
      path: '/security',
      alias: ['/security/devsecops', '/devsecops'],
      name: 'security',
      component: DevSecOpsView,
      meta: { requiresAuth: true }
    },
    {
      path: '/compliance',
      alias: ['/security/compliance', '/compliance-center'],
      name: 'compliance',
      component: ComplianceView,
      meta: { requiresAuth: true }
    },
    {
      path: '/drift',
      name: 'drift',
      component: DriftView,
      meta: { requiresAuth: true }
    },
    {
      path: '/backup',
      name: 'backup',
      component: BackupRestoreView,
      meta: { requiresAuth: true }
    },

    // 4. Automation & FinOps
    {
      path: '/automation',
      name: 'automation',
      component: AutomationView,
      meta: { requiresAuth: true }
    },
    {
      path: '/runbooks',
      name: 'runbooks',
      component: RunbooksView,
      meta: { requiresAuth: true }
    },
    {
      path: '/cost',
      alias: ['/finops/cost', '/finops'],
      name: 'cost',
      component: CostFinOpsView,
      meta: { requiresAuth: true }
    },
    {
      path: '/capacity',
      name: 'capacity',
      component: CapacityView,
      meta: { requiresAuth: true }
    },

    // 5. Enterprise Management
    {
      path: '/tenancy',
      alias: ['/tenancy/rbac', '/rbac'],
      name: 'tenancy',
      component: TenancyRbacView,
      meta: { requiresAuth: true }
    },
    {
      path: '/ai-hub',
      alias: ['/ai/providers', '/ai-providers'],
      name: 'ai-hub',
      component: AIProviderHubView,
      meta: { requiresAuth: true }
    },
    {
      path: '/changes',
      name: 'changes',
      component: ChangesView,
      meta: { requiresAuth: true }
    },
    {
      path: '/alerts',
      name: 'alerts',
      component: AlertsView,
      meta: { requiresAuth: true }
    },
    {
      path: '/reports',
      name: 'reports',
      component: ReportsView,
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
      component: SettingsView,
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
