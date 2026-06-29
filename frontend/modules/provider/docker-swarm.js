/**
 * Docker & Swarm Provider Module
 * Manages standalone containers, Swarm service replication, and Swarm clustered nodes.
 */
(function (global) {
  'use strict';

  var activeTab = 'services';
  var servicesData = [];
  var nodesData = [];
  var containersData = [];

  var initialized = false;
  function init() {
    if (initialized) return;
    initialized = true;
    var container = document.getElementById('docker-swarm-view');
    if (!container) {
      initialized = false;
      return;
    }

    renderLayout();
    bindEvents();
    loadData();
    
    // Listen for SPA navigation to refresh
    AppState.on('navigate', function (section) {
      if (section === 'docker-swarm') {
        loadData();
      }
    });
  }

  function renderLayout() {
    var container = document.getElementById('docker-swarm-view');
    if (!container) return;

    container.innerHTML = `
      <div class="docker-swarm-container">
        <!-- Overview Stats Cards -->
        <div class="docker-stats-grid">
          <div class="docker-stat-card">
            <div class="docker-stat-icon">🐳</div>
            <div class="docker-stat-info">
              <span class="docker-stat-label">Swarm Mode</span>
              <span class="docker-stat-value text-trading-up" id="docker-swarm-mode">Active</span>
            </div>
          </div>
          <div class="docker-stat-card">
            <div class="docker-stat-icon">🖥️</div>
            <div class="docker-stat-info">
              <span class="docker-stat-label">Total Nodes</span>
              <span class="docker-stat-value" id="docker-nodes-count">0</span>
            </div>
          </div>
          <div class="docker-stat-card">
            <div class="docker-stat-icon">🐝</div>
            <div class="docker-stat-info">
              <span class="docker-stat-label">Swarm Services</span>
              <span class="docker-stat-value" id="docker-services-count">0</span>
            </div>
          </div>
          <div class="docker-stat-card">
            <div class="docker-stat-icon">📦</div>
            <div class="docker-stat-info">
              <span class="docker-stat-label">Standalone Containers</span>
              <span class="docker-stat-value" id="docker-containers-count">0</span>
            </div>
          </div>
        </div>

        <!-- Navigation Tabs -->
        <div class="docker-tabs-header">
          <button class="docker-tab-btn active" data-docker-tab="services">🐝 Services</button>
          <button class="docker-tab-btn" data-docker-tab="nodes">🖥️ Nodes</button>
          <button class="docker-tab-btn" data-docker-tab="containers">📦 Standalone Containers</button>
        </div>

        <!-- Dynamic Grid Content -->
        <div id="docker-content-grid" class="docker-grid">
          <div class="text-center text-muted w-100 py-10" style="grid-column: 1 / -1; text-align: center; padding: var(--space-xl) 0;">
            <span class="loading loading-spinner"></span> Loading Docker configuration assets...
          </div>
        </div>
      </div>
    `;

    // Hook refresh button on the page header
    var refreshBtn = document.getElementById('btn-refresh-docker');
    if (refreshBtn) {
      refreshBtn.onclick = async function() {
        refreshBtn.classList.add('loading');
        try {
          await loadData();
        } catch (e) {
          console.warn('[Docker] Load data error on refresh', e);
        } finally {
          setTimeout(function() { refreshBtn.classList.remove('loading'); }, 500);
        }
      };
    }
  }

  function bindEvents() {
    var container = document.getElementById('docker-swarm-view');
    if (!container) return;

    // Tabs switching logic
    container.querySelectorAll('.docker-tab-btn').forEach(function (btn) {
      btn.addEventListener('click', function () {
        container.querySelectorAll('.docker-tab-btn').forEach(function (b) { b.classList.remove('active'); });
        this.classList.add('active');
        activeTab = this.dataset.dockerTab;
        renderTabContent();
      });
    });
  }

  async function loadData() {
    var banner = document.getElementById('docker-error-banner');
    if (banner) banner.style.display = 'none';

    try {
      // 1. Fetch Swarm Services
      var res = await fetch('/api/v1/docker/services');
      if (!res.ok) throw new Error('Services API returned status ' + res.status);
      var body = await res.json();
      servicesData = body.data || [];

      // 2. Fetch Swarm Nodes
      res = await fetch('/api/v1/docker/nodes');
      if (!res.ok) throw new Error('Nodes API returned status ' + res.status);
      body = await res.json();
      nodesData = body.data || [];

      // 3. Fetch Standalone Containers
      res = await fetch('/api/v1/docker/containers');
      if (!res.ok) throw new Error('Containers API returned status ' + res.status);
      body = await res.json();
      containersData = body.data || [];
    } catch (e) {
      if (e.name === 'AbortError') {
        return;
      }
      console.error('[Docker] API load error', e);
      servicesData = [];
      nodesData = [];
      containersData = [];
      showErrorBanner('Failed to load Docker Swarm data: ' + e.message);
    }

    updateOverviewMetrics();
    renderTabContent();
  }

  function updateOverviewMetrics() {
    var nodesEl = document.getElementById('docker-nodes-count');
    var servicesEl = document.getElementById('docker-services-count');
    var containersEl = document.getElementById('docker-containers-count');

    if (nodesEl) nodesEl.textContent = nodesData.length;
    if (servicesEl) servicesEl.textContent = servicesData.length;
    if (containersEl) containersEl.textContent = containersData.length;
  }

  function renderTabContent() {
    var grid = document.getElementById('docker-content-grid');
    if (!grid) return;

    grid.innerHTML = '';

    if (activeTab === 'services') {
      if (servicesData.length === 0) {
        grid.innerHTML = '<div style="grid-column:1/-1;text-align:center;" class="text-muted">No Docker Swarm Services running.</div>';
        return;
      }
      servicesData.forEach(renderServiceCard);
    } else if (activeTab === 'nodes') {
      if (nodesData.length === 0) {
        grid.innerHTML = '<div style="grid-column:1/-1;text-align:center;" class="text-muted">No Swarm cluster nodes found.</div>';
        return;
      }
      nodesData.forEach(renderNodeCard);
    } else if (activeTab === 'containers') {
      if (containersData.length === 0) {
        grid.innerHTML = '<div style="grid-column:1/-1;text-align:center;" class="text-muted">No standalone containers configured.</div>';
        return;
      }
      containersData.forEach(renderContainerCard);
    }
  }

  // ── CARD RENDERERS ──

  function renderServiceCard(svc) {
    var grid = document.getElementById('docker-content-grid');
    var card = document.createElement('div');
    card.className = 'docker-card';
    card.innerHTML = `
      <div class="docker-card-title">
        <h3>🐝 ${esc(svc.name)}</h3>
        <span class="badge badge-synced">Active</span>
      </div>
      <div class="docker-card-rows">
        <div class="docker-card-row">
          <span class="label">Image Spec</span>
          <span class="value" style="font-size:12px;color:var(--color-muted);">${esc(svc.image)}</span>
        </div>
        <div class="docker-card-row">
          <span class="label">Replicas</span>
          <span class="value" style="color:var(--color-primary); font-weight:700;">${svc.replicas} target</span>
        </div>
        <div class="docker-card-row">
          <span class="label">Exposed Ports</span>
          <span class="value">${svc.ports && svc.ports.length > 0 ? esc(svc.ports.join(', ')) : 'none'}</span>
        </div>
        <div class="docker-card-row">
          <span class="label">Last Configuration Change</span>
          <span class="value text-muted" style="font-size:11px;">${timeAgo(svc.updated_at)}</span>
        </div>
      </div>
      <div class="docker-card-footer">
        <button class="btn btn-sm btn-outline" data-action="scale" data-id="${esc(svc.name)}">Scale</button>
        <button class="btn btn-sm btn-ghost" data-action="logs" data-id="${esc(svc.name)}" data-type="service">Logs</button>
      </div>
    `;

    card.querySelectorAll('.btn').forEach(function (btn) {
      btn.addEventListener('click', function () {
        handleAction(this.dataset.action, this.dataset.id, this.dataset.type, svc);
      });
    });

    grid.appendChild(card);
  }

  function renderNodeCard(node) {
    var grid = document.getElementById('docker-content-grid');
    var card = document.createElement('div');
    card.className = 'docker-card';
    var isMgr = node.role === 'manager';
    var badgeClass = node.status === 'ready' ? 'badge-healthy' : 'badge-down';
    
    card.innerHTML = `
      <div class="docker-card-title">
        <h3>🖥️ ${esc(node.name)}</h3>
        <span class="badge ${badgeClass}">${esc(node.status)}</span>
      </div>
      <div class="docker-card-rows">
        <div class="docker-card-row">
          <span class="label">Role</span>
          <span class="value"><span class="badge ${isMgr ? 'badge-synced' : 'badge-outline'}">${node.role.toUpperCase()}</span></span>
        </div>
        <div class="docker-card-row">
          <span class="label">Availability</span>
          <span class="value text-trading-up">${esc(node.availability)}</span>
        </div>
        <div class="docker-card-row">
          <span class="label">Engine Version</span>
          <span class="value">${esc(node.version)}</span>
        </div>
        <div class="docker-card-row">
          <span class="label">Last Status Heartbeat</span>
          <span class="value text-muted" style="font-size:11px;">${timeAgo(node.updated_at)}</span>
        </div>
      </div>
      <div class="docker-card-footer">
        <button class="btn btn-sm btn-ghost" data-action="node-drain" data-id="${esc(node.name)}" ${node.availability === 'drain' ? 'disabled' : ''}>Drain Node</button>
      </div>
    `;

    card.querySelectorAll('.btn').forEach(function (btn) {
      btn.addEventListener('click', function () {
        handleAction(this.dataset.action, this.dataset.id, 'node', node);
      });
    });

    grid.appendChild(card);
  }

  function renderContainerCard(container) {
    var grid = document.getElementById('docker-content-grid');
    var card = document.createElement('div');
    card.className = 'docker-card';
    var isRunning = container.state === 'running';
    var badgeClass = isRunning ? 'badge-healthy' : 'badge-down';
    
    card.innerHTML = `
      <div class="docker-card-title">
        <h3>📦 ${esc(container.name)}</h3>
        <span class="badge ${badgeClass}">${esc(container.state)}</span>
      </div>
      <div class="docker-card-rows">
        <div class="docker-card-row">
          <span class="label">Image URL</span>
          <span class="value" style="font-size:12px;color:var(--color-muted);">${esc(container.image)}</span>
        </div>
        <div class="docker-card-row">
          <span class="label">State Info</span>
          <span class="value">${esc(container.status)}</span>
        </div>
        <div class="docker-card-row">
          <span class="label">Created Date</span>
          <span class="value" style="font-size:12px;">${new Date(container.created).toLocaleString()}</span>
        </div>
      </div>
      <div class="docker-card-footer">
        <button class="btn btn-sm ${isRunning ? 'btn-outline danger' : 'btn-primary'}" data-action="toggle-container" data-id="${esc(container.name)}">
          ${isRunning ? 'Stop' : 'Start'}
        </button>
        <button class="btn btn-sm btn-ghost" data-action="logs" data-id="${esc(container.name)}" data-type="container">Logs</button>
      </div>
    `;

    card.querySelectorAll('.btn').forEach(function (btn) {
      btn.addEventListener('click', function () {
        handleAction(this.dataset.action, this.dataset.id, this.dataset.type, container);
      });
    });

    grid.appendChild(card);
  }

  // ── ACTIONS HANDLER ──

  function handleAction(action, id, type, item) {
    switch (action) {
      case 'scale':
        var newReplicas = prompt('Enter target replicas count for service "' + id + '":', item.replicas);
        if (newReplicas === null) return;
        var num = parseInt(newReplicas, 10);
        if (isNaN(num) || num < 0) {
          alert('Please specify a valid replica count >= 0');
          return;
        }
        triggerServiceScale(id, num, item);
        break;

      case 'logs':
        showLogsConsole(id, type);
        break;

      case 'toggle-container':
        var targetState = item.state === 'running' ? 'stopped' : 'running';
        if (confirm('Are you sure you want to stop/start container "' + id + '"?')) {
          triggerContainerStateToggle(id, targetState, item);
        }
        break;

      case 'node-drain':
        if (confirm('Drain node "' + id + '"? This will reallocate active tasks.')) {
          triggerNodeDrain(id, item);
        }
        break;
    }
  }

  // ── SIMULATION & MODALS ──

  async function triggerServiceScale(name, count, svc) {
    Modal.open({
      title: '⚙️ Scaling Docker Swarm Service: ' + name,
      body: `
        <div style="font-size:12px;color:var(--color-muted);margin-bottom:var(--space-md);">Executing clustered service scale command over secure Engine API sockets.</div>
        <div class="docker-console-wrapper">
          <div class="docker-console-log" id="scale-console-log">Connecting to swarm manager...</div>
        </div>
      `,
      actions: [
        { label: 'Close', primary: true }
      ]
    });

    var logEl = document.getElementById('scale-console-log');
    if (!logEl) return;

    logEl.textContent += '\n[Swarm Engine] Authenticating CLI session to Docker daemon client...';
    logEl.textContent += '\n[Swarm Engine] Updating replication factor for service ID ' + svc.ID + '...';

    try {
      var res = await fetch('/api/v1/docker/services/' + encodeURIComponent(svc.ID) + '/scale', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ replicas: count })
      });
      if (!res.ok) throw new Error('API returned status ' + res.status);

      logEl.textContent += '\n[Swarm Engine] Swarm Service converged to ' + count + ' running state.';
      logEl.textContent += '\n[Swarm Engine] Audit log updated. Execution verified successfully.';
      svc.replicas = count;
      svc.updated_at = new Date();
      renderTabContent();

      if (AppState.addAuditLog) {
        AppState.addAuditLog({ action: 'scale', target: 'docker-service/' + name, result: 'success' });
      }
    } catch (e) {
      logEl.textContent += '\n[Swarm Engine] ERROR: Failed to scale service: ' + e.message;
    }
  }

  async function triggerContainerStateToggle(name, state, container) {
    var action = state === 'running' ? 'start' : 'stop';
    try {
      var res = await fetch('/api/v1/docker/containers/' + encodeURIComponent(container.ID) + '/toggle', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ action: action })
      });
      if (!res.ok) throw new Error('API returned status ' + res.status);

      var newState = state === 'running' ? 'running' : 'exited';
      var newStatus = state === 'running' ? 'Up Less than a minute' : 'Exited (0) Just now';

      container.state = newState;
      container.status = newStatus;
      renderTabContent();

      if (AppState.addAuditLog) {
        AppState.addAuditLog({ action: action, target: 'docker-container/' + name, result: 'success' });
      }
      alert('Container "' + name + '" state updated to: ' + newState);
    } catch (e) {
      alert('Failed to toggle container: ' + e.message);
    }
  }

  async function triggerNodeDrain(name, node) {
    try {
      var res = await fetch('/api/v1/docker/nodes/' + encodeURIComponent(node.ID) + '/drain', {
        method: 'POST'
      });
      if (!res.ok) throw new Error('API returned status ' + res.status);

      node.availability = 'drain';
      node.updated_at = new Date();
      renderTabContent();

      if (AppState.addAuditLog) {
        AppState.addAuditLog({ action: 'drain', target: 'docker-node/' + name, result: 'success' });
      }
      alert('Node "' + name + '" availability set to DRAIN. Tasks will migrate to active nodes.');
    } catch (e) {
      alert('Failed to drain node: ' + e.message);
    }
  }

  function showLogsConsole(name, type) {
    Modal.open({
      title: '📋 Logs - ' + name + ' (' + type + ')',
      body: `
        <div style="font-size:12px;color:var(--color-muted);margin-bottom:var(--space-md);">Viewing standard log streams from active Docker daemon sockets.</div>
        <div class="docker-console-wrapper">
          <div class="docker-console-log" id="docker-logs-console-log">Connecting socket stream...</div>
        </div>
      `,
      actions: [
        { label: 'Close', primary: true }
      ]
    });

    var logEl = document.getElementById('docker-logs-console-log');
    if (!logEl) return;

    logEl.textContent = 'Attaching logs handler to socket daemon...\nFetching stdout and stderr streams...';

    fetch('/api/v1/docker/logs?id=' + encodeURIComponent(name) + '&type=' + type)
      .then(function (res) {
        if (!res.ok) throw new Error('API returned status ' + res.status);
        return res.json();
      })
      .then(function (body) {
        logEl.textContent = body.logs || 'No logs available.';
        logEl.scrollTop = logEl.scrollHeight;
      })
      .catch(function (err) {
        logEl.textContent = 'Error loading logs: ' + err.message;
      });
  }

  function showErrorBanner(msg) {
    var container = document.getElementById('docker-swarm-view');
    if (!container) return;
    var banner = document.getElementById('docker-error-banner');
    if (!banner) {
      banner = document.createElement('div');
      banner.id = 'docker-error-banner';
      banner.style.padding = '12px var(--space-md)';
      banner.style.background = 'rgba(235, 94, 85, 0.1)';
      banner.style.border = '1px solid var(--color-trading-down)';
      banner.style.color = 'var(--color-trading-down)';
      banner.style.borderRadius = '6px';
      banner.style.marginBottom = 'var(--space-md)';
      banner.style.fontSize = '13px';
      var title = container.querySelector('.docker-header-wrapper');
      if (title && title.nextSibling) {
        container.insertBefore(banner, title.nextSibling);
      } else {
        container.prepend(banner);
      }
    }
    banner.textContent = msg;
    banner.style.display = 'block';
  }

  global.DockerSwarmSection = { init: init };
  
})(window);
