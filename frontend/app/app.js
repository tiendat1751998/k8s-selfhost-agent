/**
 * App — Main entry point for Enterprise Configuration Control Plane.
 * Initialises all modules, connects WebSocket, manages connection status.
 */
(function () {
  'use strict';

  document.addEventListener('DOMContentLoaded', function () {
    // Core infrastructure
    TelemetryService.init();
    Modal.init();
    Router.init();

    // Operational panels (overview section)
    IncidentsPanel.init();
    AgentsPanel.init();
    LogsPanel.init();
    MetricsPanel.init();
    if (window.PersonalizationModule) {
      PersonalizationModule.init();
    }

    // Create connection banner dynamically
    var banner = document.createElement('div');
    banner.id = 'connection-lost-banner';
    banner.style.cssText = 'position:fixed;top:0;left:50%;transform:translateX(-50%) translateY(-100%);padding:8px 24px;border-radius:0 0 var(--rounded-md) var(--rounded-md);font-size:13px;font-weight:600;z-index:99999;transition:transform 0.3s cubic-bezier(0.16, 1, 0.3, 1);box-shadow:0 4px 12px rgba(0,0,0,0.5);display:flex;align-items:center;gap:8px;color:#fff;';
    banner.innerHTML = '<span>⚠️</span> <span>Connection lost. Reconnecting...</span>';
    document.body.appendChild(banner);

    // Connection status indicator
    AppState.on('connection', function (status) {
      var dot = document.getElementById('ws-dot');
      var label = document.getElementById('ws-label');
      if (dot) dot.className = 'status-dot ' + status;
      if (label) {
        switch (status) {
          case 'online':
            label.textContent = 'Connected';
            banner.style.transform = 'translateX(-50%) translateY(-100%)';
            break;
          case 'offline':
            label.textContent = 'Disconnected';
            banner.style.background = 'var(--color-trading-down)';
            banner.querySelector('span:last-child').textContent = 'Control plane disconnected. Viewing cached data offline.';
            banner.style.transform = 'translateX(-50%) translateY(0)';
            break;
          case 'connecting':
            label.textContent = 'Connecting…';
            banner.style.background = '#f0b90b';
            banner.querySelector('span:last-child').textContent = 'Connecting to control plane...';
            banner.style.transform = 'translateX(-50%) translateY(0)';
            break;
        }
      }
    });

    // Listen for config data from WebSocket
    AppState.on('connection', function (status) {
      if (status === 'online') {
        // Request initial config data
        WSClient.send({ type: 'get_config' });
      }
    });

    // Refresh button
    var refreshBtn = document.getElementById('refresh-btn');
    if (refreshBtn) {
      refreshBtn.addEventListener('click', function () {
        WSClient.send({ type: 'get_config' });
        AppState.addAuditLog({ action: 'refresh', target: 'system/config', result: 'success' });
        var section = document.getElementById('section-deployment-center');
        if (window.DeploymentCatalog && section && section.classList.contains('active')) {
          window.DeploymentCatalog.loadInitialApps();
        }
      });
    }

    // WebSocket connection
    var wsProtocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
    var token = localStorage.getItem('k8s_token');
    var wsUrl = wsProtocol + '//' + window.location.host + '/ws' + (token ? '?token=' + encodeURIComponent(token) : '');
    WSClient.connect(wsUrl);

    console.log('[App] K8S Control Plane — Enterprise Edition initialised');
  });

})();
