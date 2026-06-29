/**
 * Enterprise Feature Flag Manager
 * Handles dynamic activation/deactivation of modules/views.
 */
(function (global) {
  'use strict';

  var STORAGE_KEY = 'k8s_feature_flags';

  var DEFAULT_FLAGS = {
    'docker-swarm': true,
    'ai-ops': true,
    'cost-management': true,
    'compliance': true,
    'reporting': true,
    'change': true
  };

  var FLAG_METADATA = {
    'docker-swarm': {
      label: 'Docker & Swarm Provider',
      description: 'Enables support for Swarm clusters, services, and containers views.'
    },
    'ai-ops': {
      label: 'AI Ops & RCA Engine',
      description: 'Enables Copilot, automated Root Cause Analysis, and AI configuration.'
    },
    'cost-management': {
      label: 'Cost Management',
      description: 'Enables analysis of cluster resources spend and cost allocation reports.'
    },
    'compliance': {
      label: 'Compliance Monitoring',
      description: 'Enables regulatory frameworks (CIS, SOC2) and security audit scans.'
    },
    'reporting': {
      label: 'Reporting Center',
      description: 'Enables scheduling, generation, and downloading of cluster performance reports.'
    },
    'change': {
      label: 'Change Management',
      description: 'Enables tracking and approval flows for production environment updates.'
    }
  };

  var ROUTE_TO_FLAG_MAP = {
    'docker-swarm': 'docker-swarm',
    'ai-ops': 'ai-ops',
    'ai-providers': 'ai-ops',
    'correlation': 'ai-ops',
    'cost-management': 'cost-management',
    'compliance': 'compliance',
    'reporting': 'reporting',
    'change': 'change'
  };

  var flags = {};

  function init() {
    loadFlags();
    setupRoutingProtection();
  }

  function loadFlags() {
    var saved = {};
    try {
      var raw = localStorage.getItem(STORAGE_KEY);
      if (raw) saved = JSON.parse(raw);
    } catch (e) {
      console.warn('[FeatureFlags] Failed to parse saved flags:', e);
    }

    Object.keys(DEFAULT_FLAGS).forEach(function (key) {
      if (saved[key] !== undefined) {
        flags[key] = !!saved[key];
      } else {
        flags[key] = DEFAULT_FLAGS[key];
      }
    });
  }

  function saveFlags() {
    try {
      localStorage.setItem(STORAGE_KEY, JSON.stringify(flags));
    } catch (e) {
      console.error('[FeatureFlags] Failed to save flags:', e);
    }
  }

  function isEnabled(featureOrRoute) {
    var flagKey = ROUTE_TO_FLAG_MAP[featureOrRoute] || featureOrRoute;
    if (flags[flagKey] !== undefined) {
      return flags[flagKey];
    }
    return true; // default enable if not in map
  }

  function setEnabled(flagKey, enabled) {
    if (flags[flagKey] !== undefined) {
      flags[flagKey] = !!enabled;
      saveFlags();
      
      // Notify sidebar to re-render if it exists
      if (global.sidebarNav) {
        global.sidebarNav.render();
        global.sidebarNav.bindEvents();
      }

      // Check if current route is now disabled, if so, redirect to overview
      var currentHash = global.location.hash.replace('#', '').split('/')[0] || 'overview';
      if (!isEnabled(currentHash)) {
        global.location.hash = '#overview';
      }

      // Log action to audit logs if AppState is available
      if (global.AppState && typeof global.AppState.addAuditLog === 'function') {
        global.AppState.addAuditLog({
          action: enabled ? 'enable' : 'disable',
          target: 'system/feature-flag/' + flagKey,
          result: 'success'
        });
      }
    }
  }

  function getAll() {
    return Object.assign({}, flags);
  }

  function getMetadata() {
    return FLAG_METADATA;
  }

  function setupRoutingProtection() {
    // Intercept hash change and redirect if hash matches a disabled feature
    global.addEventListener('hashchange', function () {
      var hash = global.location.hash.replace('#', '').split('/')[0] || 'overview';
      if (!isEnabled(hash)) {
        console.warn('[FeatureFlags] Route disabled by feature flag:', hash);
        global.location.hash = '#overview';
      }
    });

    // Check on initial load
    var initialHash = global.location.hash.replace('#', '').split('/')[0] || 'overview';
    if (!isEnabled(initialHash)) {
      global.location.hash = '#overview';
    }
  }

  // Expose module
  global.FeatureFlags = {
    init: init,
    isEnabled: isEnabled,
    setEnabled: setEnabled,
    getAll: getAll,
    getMetadata: getMetadata
  };

  // Run initial loading
  init();

})(window);
