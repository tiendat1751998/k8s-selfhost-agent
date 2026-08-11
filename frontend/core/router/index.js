/**
 * Router — SPA client-side routing via sidebar navigation.
 * Switches section visibility without page reload.
 */
(function (global) {
  'use strict';

  const sections = {};
  const sectionTitles = {
    'overview': 'Overview',
    'kubernetes': 'Container Clusters',
    'docker-swarm': 'Docker & Swarm Integration',
    'deployment-center': 'Deployment Center',
    'enterprise-search': 'Enterprise Search',
    'git-providers': 'Git Providers',
    'cicd': 'CI/CD Integrations',
    'ai-providers': 'AI Providers',
    'connection-health': 'Connection Health',
    'audit-logs': 'Audit Logs',
    'action-center': 'Action Center',
    'ai-ops': 'AI Operations',
    'gitops': 'GitOps',
    'enterprise': 'Enterprise',
    'cost-management': 'Cost Management',
    'observability': 'Observability',
    'topology': 'Topology Map',
    'workflow-automation': 'Workflow Automation',
    'notifications': 'Notification Center',
    'compliance': 'Compliance',
    'runbooks': 'Runbook Center',
    'timeline': 'Deployment Timeline',
    'platform-engineering': 'Platform Engineering',
    'capacity': 'Capacity Planning',
    'drift': 'Drift Detection',
    'correlation': 'Event Correlation',
    'change': 'Change Management',
    'promotion': 'Deployment Promotion',
    'explorer': 'Resource Explorer',
    'tagging': 'Tagging System',
    'reporting': 'Reporting Center',
    'health': 'Platform Health',
    'fleet': 'Multi-Cluster Fleet View',
    'audit': 'Platform Audit Mode',
    'settings': 'Settings',
    'nodes': 'Nodes',
    'pods': 'Pods',
    'rollouts': 'Rollouts',
    'scaling': 'Auto Scaling',
    'incidents': 'Incidents',
    'agents': 'Agent Execution Pipeline'
  };

  const moduleRegistry = {
    'kubernetes': { scripts: ['/modules/clusters/index.js'], globalObj: 'KubernetesSection' },
    'docker-swarm': { scripts: ['/modules/provider/docker-swarm.js?v=2'], globalObj: 'DockerSwarmSection' },
    'deployment-center': {
      scripts: [
        '/modules/deployments/deployment-catalog.js',
        '/modules/deployments/deployment-drawer.js',
        '/modules/deployments/deployment-wizard.js',
        '/modules/deployments/deployment-center.js'
      ],
      globalObj: 'DeploymentCenter'
    },
    'enterprise-search': {
      scripts: [
        '/modules/search/search-index.js',
        '/modules/search/search-adv-builder.js',
        '/modules/search/search-git-trace.js',
        '/modules/search/search-graph-visualizer.js',
        '/modules/search/search-analytics.js',
        '/modules/search/search-logs-ui.js',
        '/modules/search/search-autocomplete.js',
        '/modules/search/enterprise-search.js'
      ],
      globalObj: 'EnterpriseSearchSection'
    },
    'git-providers': { scripts: ['/modules/gitops/git-providers.js?v=prod'], globalObj: 'GitProvidersSection' },
    'cicd': { scripts: ['/modules/gitops/cicd.js?v=prod'], globalObj: 'CicdSection' },
    'ai-providers': { scripts: ['/modules/ai/ai-providers.js?v=2'], globalObj: 'AiProvidersSection' },
    'connection-health': { scripts: ['/modules/clusters/connection-health.js?v=prod'], globalObj: 'ConnectionHealthSection' },
    'audit-logs': { scripts: ['/modules/rbac/audit-logs.js'], globalObj: 'AuditLogsSection' },
    'action-center': { scripts: ['/modules/actions/action-center.js'], globalObj: 'ActionCenterSection' },
    'ai-ops': { scripts: ['/modules/ai/ai-ops.js'], globalObj: 'AiOpsSection' },
    'gitops': { scripts: ['/modules/gitops/gitops.js'], globalObj: 'GitOpsSection' },
    'enterprise': {
      scripts: [
        '/modules/platform/enterprise-tenancy.js',
        '/modules/platform/enterprise-rbac.js',
        '/modules/platform/enterprise-marketplace.js',
        '/modules/platform/enterprise-backup.js',
        '/modules/platform/enterprise-provisioning.js',
        '/modules/platform/enterprise.js'
      ],
      globalObj: 'EnterpriseSection'
    },
    'cost-management': { scripts: ['/modules/cost/cost-management.js?v=3'], globalObj: 'CostManager' },
    'observability': { scripts: ['/modules/observability/observability.js'], globalObj: 'ObservabilityModule' },
    'topology': { scripts: ['/modules/topology/topology.js?v=3'], globalObj: 'TopologyModule' },
    'workflow-automation': { scripts: ['/modules/automation/workflow-automation.js'], globalObj: 'WorkflowAutomation' },
    'notifications': { scripts: ['/modules/notifications/notification-center.js'], globalObj: 'NotificationCenter' },
    'compliance': { scripts: ['/modules/compliance/compliance.js'], globalObj: 'ComplianceModule' },
    'runbooks': { scripts: ['/modules/runbooks/runbook-center.js'], globalObj: 'RunbookCenter' },
    'timeline': { scripts: ['/modules/timeline/deployment-timeline.js'], globalObj: 'DeploymentTimeline' },
    'platform-engineering': { scripts: ['/modules/platform-eng/platform-engineering.js'], globalObj: 'PlatformEngineering' },
    'capacity': { scripts: ['/modules/capacity/capacity-planning.js?v=prod'], globalObj: 'CapacityPlanning' },
    'drift': { scripts: ['/modules/drift/drift-detection.js?v=prod'], globalObj: 'DriftDetection' },
    'correlation': { scripts: ['/modules/correlation/event-correlation.js?v=prod'], globalObj: 'EventCorrelation' },
    'change': { scripts: ['/modules/change/change-management.js?v=prod'], globalObj: 'ChangeManagement' },
    'promotion': { scripts: ['/modules/promotion/deployment-promotion.js?v=prod'], globalObj: 'DeploymentPromotion' },
    'explorer': { scripts: ['/modules/explorer/resource-explorer.js?v=prod'], globalObj: 'ResourceExplorer' },
    'tagging': { scripts: ['/modules/tagging/tagging-system.js?v=prod'], globalObj: 'TaggingSystem' },
    'reporting': { scripts: ['/modules/reporting/reporting-center.js?v=prod'], globalObj: 'ReportingCenter' },
    'health': { scripts: ['/modules/healthcenter/health-center.js?v=prod'], globalObj: 'HealthCenter' },
    'fleet': { scripts: ['/modules/fleet/fleet-view.js?v=prod'], globalObj: 'FleetView' },
    'audit': { scripts: ['/modules/audit/audit-mode.js'], globalObj: 'AuditMode' },
    'settings': { scripts: ['/modules/settings/settings.js'], globalObj: 'SettingsSection' },
    'nodes': { scripts: ['/modules/clusters/nodes.js'], globalObj: 'NodesSection' },
    'pods': { scripts: ['/modules/clusters/pods.js'], globalObj: 'PodsSection' },
    'rollouts': { scripts: ['/modules/deployments/rollouts.js'], globalObj: 'RolloutsSection' },
    'scaling': { scripts: ['/modules/clusters/scaling.js?v=prod'], globalObj: 'ScalingSection' },
    'incidents': { scripts: ['/modules/incidents/incidents-page.js'], globalObj: 'IncidentsPage' },
    'agents': { scripts: ['/modules/agents/agents-dashboard.js'], globalObj: 'AgentsDashboard' }
  };

  const loadedModules = {};

  function loadModule(sectionId) {
    const config = moduleRegistry[sectionId];
    if (!config) return Promise.resolve();

    if (loadedModules[sectionId]) {
      return Promise.resolve();
    }

    const loadScript = (src) => {
      return new Promise((resolve, reject) => {
        if (document.querySelector(`script[src="${src}"]`)) {
          resolve();
          return;
        }
        const script = document.createElement('script');
        script.src = src;
        script.onload = () => resolve();
        script.onerror = (err) => reject(new Error(`Failed to load script ${src}`));
        document.body.appendChild(script);
      });
    };

    let promise = Promise.resolve();
    config.scripts.forEach((src) => {
      promise = promise.then(() => loadScript(src));
    });

    return promise.then(async () => {
      const obj = window[config.globalObj];
      if (obj && typeof obj.init === 'function') {
        try {
          const initRes = obj.init();
          if (initRes instanceof Promise) {
            await initRes;
          }
        } catch (e) {
          console.error(`Error initializing module ${config.globalObj}:`, e);
        }
      }
      loadedModules[sectionId] = true;
    });
  }

  function init() {
    // Cache section elements
    document.querySelectorAll('.section').forEach(function (el) {
      var id = el.id.replace('section-', '');
      sections[id] = el;
    });

    // Sidebar click handler
    document.querySelectorAll('.sidebar-link').forEach(function (link) {
      link.addEventListener('click', function (e) {
        e.preventDefault();
        var section = this.dataset.section;
        navigate(section);
      });
    });

    // Handle hash change
    window.addEventListener('hashchange', function () {
      var hash = window.location.hash.replace('#', '') || 'overview';
      navigate(hash, true);
    });

    // Initial route from hash
    var initialSection = window.location.hash.replace('#', '') || 'overview';
    navigate(initialSection, true);
  }

  function navigate(sectionId, skipHash) {
    if (window.APIClient && typeof window.APIClient.abortAll === 'function') {
      window.APIClient.abortAll();
    }

    var targetSection = sectionId;
    
    var basePath = targetSection.split('/')[0] || 'overview';
    if (window.FeatureFlags && typeof window.FeatureFlags.isEnabled === 'function') {
      if (!window.FeatureFlags.isEnabled(basePath)) {
        console.warn(`[Router] Route "${basePath}" is disabled by feature flags.`);
        targetSection = 'overview';
        basePath = 'overview';
      }
    }
    
    loadModule(basePath).then(() => {
      // Re-scan sections to catch any new lazy-loaded sections
      document.querySelectorAll('.section').forEach(function (el) {
        var id = el.id.replace('section-', '');
        sections[id] = el;
      });

      if (!sections[basePath]) {
        basePath = 'overview';
        targetSection = 'overview';
      }

      // Hide all sections
      Object.values(sections).forEach(function (el) { el.classList.remove('active'); });

      // Show target
      if (sections[basePath]) {
        sections[basePath].classList.add('active');
      }

      // Update sidebar active state
      document.querySelectorAll('.sidebar-link').forEach(function (link) {
        link.classList.toggle('active', link.dataset.section === basePath || link.dataset.section === targetSection);
      });

      // Update page title
      var titleEl = document.getElementById('page-title');
      if (titleEl) titleEl.textContent = sectionTitles[basePath] || basePath;

      // Update hash without triggering hashchange
      if (!skipHash) {
        history.pushState(null, '', '#' + targetSection);
      }

      // Emit navigation event
      AppState.emit('navigate', targetSection);
    }).catch(err => {
      console.error('Error navigating to section:', err);
      // Fallback display logic on failure
      if (sections['overview']) {
        sections['overview'].classList.add('active');
      }
    });
  }

  global.Router = { init: init, navigate: navigate };

})(window);
