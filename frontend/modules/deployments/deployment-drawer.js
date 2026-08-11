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
      '</div>';
  }

  function renderSubTabPods(app) {
    return '<div class="empty-state"><div class="empty-state-text">No data available</div></div>';
  }

  function renderSubTabEvents(app) {
    return '<div class="empty-state"><div class="empty-state-text">No data available</div></div>';
  }

  function renderSubTabMetrics(app) {
    return '<div class="empty-state"><div class="empty-state-text">No data available</div></div>';
  }

  function renderSubTabEnv(app) {
    return '<div class="empty-state"><div class="empty-state-text">No data available</div></div>';
  }

  function renderSubTabGitOps(app) {
    return '<div class="empty-state"><div class="empty-state-text">No data available</div></div>';
  }

  function statusBadge(s) {
    if (s === 'healthy') return '<span class="badge badge-healthy">● Healthy</span>';
    if (s === 'degraded') return '<span class="badge badge-degraded">⚡ Degraded</span>';
    return '<span class="badge badge-down">🔴 Down</span>';
  }

  

  global.DeploymentDrawer = { create: createDetailsDrawer, open: openDetailsDrawer, close: closeDetailsDrawer };

})(window);
