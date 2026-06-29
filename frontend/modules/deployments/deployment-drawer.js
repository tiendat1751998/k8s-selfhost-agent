/**
 * Deployment Center — Details Drawer Component
 */
(function (global) {
  'use strict';

  function createDetailsDrawer() {
    global.DeploymentState.drawerOverlay = document.createElement('div');
    global.DeploymentState.drawerOverlay.id = 'app-drawer-overlay';
    global.DeploymentState.drawerOverlay.style.cssText = 'position:fixed;top:0;left:0;width:100%;height:100%;background:rgba(0,0,0,0.5);z-index:998;display:none;';
    global.DeploymentState.drawerOverlay.addEventListener('click', closeDetailsDrawer);

    global.DeploymentState.drawerEl = document.createElement('div');
    global.DeploymentState.drawerEl.id = 'app-drawer';
    global.DeploymentState.drawerEl.style.cssText = 'position:fixed;top:0;right:-650px;width:650px;height:100%;background:var(--color-surface);border-left:1px solid var(--color-hairline);z-index:999;overflow-y:auto;transition:right .3s ease;box-shadow:-4px 0 20px rgba(0,0,0,0.3);';

    document.body.appendChild(global.DeploymentState.drawerOverlay);
    document.body.appendChild(global.DeploymentState.drawerEl);
  }

  function openDetailsDrawer(app, idx) {
    global.DeploymentState.currentApp = app;
    renderDrawerContent(app, idx);
    global.DeploymentState.drawerOverlay.style.display = '';
    setTimeout(function () { global.DeploymentState.drawerEl.style.right = '0px'; }, 10);
  }

  function closeDetailsDrawer() {
    global.DeploymentState.drawerEl.style.right = '-650px';
    setTimeout(function () { global.DeploymentState.drawerOverlay.style.display = 'none'; }, 300);
    global.DeploymentState.currentApp = null;
  }

  function renderDrawerContent(app, idx) {
    global.DeploymentState.drawerEl.innerHTML =
      '<div style="padding:var(--space-lg);">' +
        // Header
        '<div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:var(--space-md);">' +
          '<div><h3 style="margin:0;color:var(--color-text)">🚀 ' + esc(app.name) + '</h3>' +
          '<div style="margin-top:4px;"><span class="badge badge-synced">' + esc(app.team.toUpperCase()) + '</span> ' +
            statusBadge(app.status) + '</div></div>' +
          '<button class="btn btn-ghost btn-sm" id="app-drawer-close" style="font-size:18px;">✕</button>' +
        '</div>' +
        // Tab switches
        '<div style="display:flex;gap:4px;border-bottom:1px solid var(--color-hairline);margin-bottom:var(--space-md);overflow-x:auto;">' +
          '<button class="btn btn-ghost btn-sm sub-tab active" data-sub="overview">Overview</button>' +
          '<button class="btn btn-ghost btn-sm sub-tab" data-sub="pods">Pods</button>' +
          '<button class="btn btn-ghost btn-sm sub-tab" data-sub="events">Events</button>' +
          '<button class="btn btn-ghost btn-sm sub-tab" data-sub="metrics">Metrics</button>' +
          '<button class="btn btn-ghost btn-sm sub-tab" data-sub="env">Environment</button>' +
          '<button class="btn btn-ghost btn-sm sub-tab" data-sub="gitops">GitOps & History</button>' +
        '</div>' +
        // Panel contents
        '<div id="app-sub-contents">' +
          renderSubTabOverview(app) +
        '</div>' +
        // Bottom Actions
        '<div style="border-top:1px solid var(--color-hairline);margin-top:var(--space-lg);padding-top:var(--space-md);display:flex;gap:var(--space-xs);flex-wrap:wrap;">' +
          '<button class="btn btn-primary btn-sm app-act-btn" data-act="scale">Scale</button>' +
          '<button class="btn btn-ghost btn-sm app-act-btn" data-act="restart">Restart</button>' +
          '<button class="btn btn-ghost btn-sm app-act-btn" data-act="rollback">Rollback</button>' +
          '<button class="btn btn-ghost btn-sm app-act-btn" data-act="pause">Pause</button>' +
          '<button class="btn btn-ghost btn-sm app-act-btn" data-act="resume">Resume</button>' +
          '<button class="btn btn-ghost btn-sm app-act-btn" data-act="yaml">View YAML</button>' +
          '<button class="btn btn-ghost btn-sm app-act-btn danger" data-act="delete" style="margin-left:auto;">Delete</button>' +
        '</div>' +
      '</div>';

    // Bind sub-tabs click
    global.DeploymentState.drawerEl.querySelectorAll('.sub-tab').forEach(function (btn) {
      btn.addEventListener('click', function () {
        global.DeploymentState.drawerEl.querySelectorAll('.sub-tab').forEach(function (b) { b.classList.remove('active'); });
        this.classList.add('active');
        var tab = this.dataset.sub;
        var contentEl = document.getElementById('app-sub-contents');
        if (tab === 'overview') contentEl.innerHTML = renderSubTabOverview(app);
        if (tab === 'pods') contentEl.innerHTML = renderSubTabPods(app);
        if (tab === 'events') contentEl.innerHTML = renderSubTabEvents(app);
        if (tab === 'metrics') contentEl.innerHTML = renderSubTabMetrics(app);
        if (tab === 'env') contentEl.innerHTML = renderSubTabEnv(app);
        if (tab === 'gitops') contentEl.innerHTML = renderSubTabGitOps(app);
      });
    });

    // Bind actions
    global.DeploymentState.drawerEl.querySelectorAll('.app-act-btn').forEach(function (btn) {
      btn.addEventListener('click', function () {
        var act = this.dataset.act;
        if (global.DeploymentCatalog && global.DeploymentCatalog.handleCatalogAction) {
          global.DeploymentCatalog.handleCatalogAction(act, idx);
        }
      });
    });

    document.getElementById('app-drawer-close').addEventListener('click', closeDetailsDrawer);
  }

  function renderSubTabOverview(app) {
    return '' +
      '<div class="pipeline-detail">' +
        '<div class="pipeline-detail-row"><span class="pipeline-detail-label">Application Name</span><span class="pipeline-detail-value"><strong>' + esc(app.name) + '</strong></span></div>' +
        '<div class="pipeline-detail-row"><span class="pipeline-detail-label">Target Platform</span><span class="pipeline-detail-value">' + esc(app.type) + '</span></div>' +
        '<div class="pipeline-detail-row"><span class="pipeline-detail-label">Cluster</span><span class="pipeline-detail-value">' + esc(app.target) + '</span></div>' +
        (app.namespace ? '<div class="pipeline-detail-row"><span class="pipeline-detail-label">Namespace</span><span class="pipeline-detail-value">' + esc(app.namespace) + '</span></div>' : '') +
        '<div class="pipeline-detail-row"><span class="pipeline-detail-label">Image Name</span><span class="pipeline-detail-value"><code>' + esc(app.image) + '</code></span></div>' +
        '<div class="pipeline-detail-row"><span class="pipeline-detail-label">Desired Replicas</span><span class="pipeline-detail-value">' + app.replicas + '</span></div>' +
        '<div class="pipeline-detail-row"><span class="pipeline-detail-label">CPU Request</span><span class="pipeline-detail-value">' + esc(app.cpu) + '</span></div>' +
        '<div class="pipeline-detail-row"><span class="pipeline-detail-label">Memory Request</span><span class="pipeline-detail-value">' + esc(app.memory) + '</span></div>' +
        '<div class="pipeline-detail-row"><span class="pipeline-detail-label">Container Port</span><span class="pipeline-detail-value">' + app.port + '</span></div>' +
        '<div class="pipeline-detail-row"><span class="pipeline-detail-label">Network Sync Mode</span><span class="pipeline-detail-value">' + esc(app.netType) + '</span></div>' +
      '</div>' +
      '<div style="margin-top:var(--space-md);background:rgba(14,203,129,0.04);border:1px solid rgba(14,203,129,0.1);padding:var(--space-sm);border-radius:6px;font-size:13px;">' +
        '<strong>🧠 AI Health Audit:</strong> Workload matches current utilization metrics. Recommended limits are CPU: 500m, Memory: 512Mi. No anomalies detected.' +
      '</div>';
  }

  function renderSubTabPods(app) {
    var rows = '';
    for (var i = 1; i <= app.replicas; i++) {
      var podName = app.name + '-' + Math.random().toString(36).substring(2, 7);
      rows += '<tr>' +
        '<td><strong>' + podName + '</strong></td>' +
        '<td><span class="badge badge-healthy">Running</span></td>' +
        '<td style="font-family:var(--font-number)">0</td>' +
        '<td>3h</td>' +
        '<td><button class="action-btn" onclick="alert(\'Opening live logs...\')">Logs</button></td>' +
      '</tr>';
    }

    if (app.replicas === 0) {
      return '<div class="empty-state"><div class="empty-state-text">No active active pods (scaled to 0)</div></div>';
    }

    return '' +
      '<div class="enterprise-table-wrap">' +
        '<table class="enterprise-table">' +
          '<thead><tr><th>Pod Name</th><th>Status</th><th>Restarts</th><th>Age</th><th>Actions</th></tr></thead>' +
          '<tbody>' + rows + '</tbody>' +
        '</table>' +
      '</div>';
  }

  function renderSubTabEvents(app) {
    return '' +
      '<div style="border-left:2px solid var(--color-hairline);padding-left:var(--space-md);">' +
        eventRow('10m ago', 'Normal', 'Scheduled', 'Successfully scheduled to node ip-10-0-1-78') +
        eventRow('9m ago', 'Normal', 'Pulled', 'Container image pulled successfully') +
        eventRow('9m ago', 'Normal', 'Created', 'Created container app') +
        eventRow('9m ago', 'Normal', 'Started', 'Started container app') +
      '</div>';
  }

  function eventRow(time, type, reason, msg) {
    return '<div style="margin-bottom:var(--space-sm);position:relative;">' +
      '<div style="position:absolute;left:-21px;top:4px;width:10px;height:10px;border-radius:50%;background:var(--color-primary);"></div>' +
      '<div style="font-size:11px;color:var(--color-muted);">' + esc(time) + ' · ' + esc(type) + '</div>' +
      '<div style="font-size:13px;"><strong>' + esc(reason) + '</strong> — ' + esc(msg) + '</div>' +
    '</div>';
  }

  function renderSubTabMetrics(app) {
    return '' +
      '<div style="display:grid;grid-template-columns:1fr 1fr;gap:var(--space-md);">' +
        '<div class="panel"><div class="panel-header"><div class="panel-title" style="font-size:12px;">CPU Utilization</div></div>' +
        '<div class="panel-body" style="height:120px;display:flex;align-items:center;justify-content:center;color:var(--color-primary);font-size:24px;font-family:var(--font-number);">18%</div></div>' +
        '<div class="panel"><div class="panel-header"><div class="panel-title" style="font-size:12px;">Memory Utilization</div></div>' +
        '<div class="panel-body" style="height:120px;display:flex;align-items:center;justify-content:center;color:var(--color-info);font-size:24px;font-family:var(--font-number);">42%</div></div>' +
      '</div>';
  }

  function renderSubTabEnv(app) {
    return '' +
      '<div class="enterprise-table-wrap">' +
        '<table class="enterprise-table">' +
          '<thead><tr><th>Key</th><th>Value</th></tr></thead>' +
          '<tbody>' +
            '<tr><td><code>APP_ENV</code></td><td><code>production</code></td></tr>' +
            '<tr><td><code>DB_CONNECTION_TIMEOUT</code></td><td><code>30s</code></td></tr>' +
            '<tr><td><code>LOG_LEVEL</code></td><td><code>info</code></td></tr>' +
          '</tbody>' +
        '</table>' +
      '</div>';
  }

  function renderSubTabGitOps(app) {
    return '' +
      '<div class="pipeline-detail" style="margin-bottom:var(--space-md);">' +
        '<div class="pipeline-detail-row"><span>ArgoCD Status:</span><span class="badge badge-healthy">Synced</span></div>' +
        '<div class="pipeline-detail-row"><span>Git Repo Branch:</span><span><code>main</code></span></div>' +
        '<div class="pipeline-detail-row"><span>Auto-Remediation PR:</span><span><a href="#" style="color:var(--color-primary);text-decoration:none;">PR #142 (Merged)</a></span></div>' +
      '</div>' +
      '<h4 style="margin:var(--space-md) 0 var(--space-xs);font-size:13px;color:var(--color-muted);">Rollout History</h4>' +
      '<div style="font-size:12px;line-height:1.6;">' +
        '<div><strong>v1.2.0</strong> — 2 hours ago by admin (Scale deployment to ' + app.replicas + ')</div>' +
        '<div style="color:var(--color-muted)">v1.1.9 — 1 day ago by gitops (Update image tag to v2.4)</div>' +
        '<div style="color:var(--color-muted)">v1.1.8 — 3 days ago by system (Auto-heal OOMKilled crash)</div>' +
      '</div>';
  }

  function statusBadge(s) {
    if (s === 'healthy') return '<span class="badge badge-healthy">● Healthy</span>';
    if (s === 'degraded') return '<span class="badge badge-degraded">⚡ Degraded</span>';
    return '<span class="badge badge-down">🔴 Down</span>';
  }

  

  global.DeploymentDrawer = { create: createDetailsDrawer, open: openDetailsDrawer, close: closeDetailsDrawer };

})(window);
