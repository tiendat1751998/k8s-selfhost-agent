/**
 * Incident Detail Module — Drawer, workflow states, assignment, timeline.
 * P2-001 through P2-004 combined.
 */
(function (global) {
  'use strict';

  var drawerEl, drawerOverlay;
  var currentIncident = null;
  var incidentMeta = {}; // track workflow state, assignment, notes per incident

  function init() {
    createDrawer();

    // Attach click handlers to incident cards
    var listEl = document.getElementById('incident-list');
    if (listEl) {
      listEl.addEventListener('click', function (e) {
        var card = e.target.closest('.incident-card');
        if (card) {
          var idx = Array.from(listEl.querySelectorAll('.incident-card')).indexOf(card);
          var incidents = AppState.getState().incidents;
          if (incidents[idx]) openDrawer(incidents[idx], idx);
        }
      });
    }
  }

  function createDrawer() {
    // Overlay
    drawerOverlay = document.createElement('div');
    drawerOverlay.id = 'incident-drawer-overlay';
    drawerOverlay.style.cssText = 'position:fixed;top:0;left:0;width:100%;height:100%;background:rgba(0,0,0,0.5);z-index:998;display:none;';
    drawerOverlay.addEventListener('click', closeDrawer);

    // Drawer
    drawerEl = document.createElement('div');
    drawerEl.id = 'incident-drawer';
    drawerEl.style.cssText = 'position:fixed;top:0;right:-600px;width:600px;height:100%;background:var(--color-surface);border-left:1px solid var(--color-hairline);z-index:999;overflow-y:auto;transition:right .3s ease;box-shadow:-4px 0 20px rgba(0,0,0,0.3);';

    document.body.appendChild(drawerOverlay);
    document.body.appendChild(drawerEl);
  }

  function openDrawer(incident, idx) {
    currentIncident = incident;
    var key = incident.id || idx;
    if (!incidentMeta[key]) {
      incidentMeta[key] = {
        state: 'open',
        assignee: null,
        notes: [],
        labels: [],
        timeline: buildTimeline(incident)
      };
    }
    var meta = incidentMeta[key];

    drawerEl.innerHTML = renderDrawerContent(incident, meta, key);
    drawerOverlay.style.display = '';
    setTimeout(function () { drawerEl.style.right = '0px'; }, 10);

    bindDrawerEvents(key, meta);
  }

  function closeDrawer() {
    drawerEl.style.right = '-600px';
    setTimeout(function () { drawerOverlay.style.display = 'none'; }, 300);
    currentIncident = null;
  }

  function renderDrawerContent(inc, meta, key) {
    var severity = inc.severity || 'info';
    var type = inc.type || 'Unknown';
    var cluster = inc.cluster || inc.clusterName || '—';
    var ns = inc.namespace || '—';
    var pod = inc.podName || '—';

    return '' +
      '<div style="padding:var(--space-lg);">' +
        // Header
        '<div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:var(--space-md);">' +
          '<div><h3 style="margin:0;color:var(--color-text)">' + esc(type) + '</h3>' +
          '<div style="margin-top:4px;"><span class="badge badge-' + severityClass(severity) + '">' + esc(severity.toUpperCase()) + '</span> ' +
            '<span class="badge badge-' + stateClass(meta.state) + '">' + esc(meta.state.toUpperCase()) + '</span></div></div>' +
          '<button class="btn btn-ghost btn-sm" id="drawer-close" style="font-size:18px;">✕</button>' +
        '</div>' +

        // Resource Info
        '<div class="panel" style="margin-bottom:var(--space-md);">' +
          '<div class="panel-header"><div class="panel-title">Resource Details</div></div>' +
          '<div class="panel-body" style="padding:var(--space-md);">' +
            '<div class="pipeline-detail">' +
              '<div class="pipeline-detail-row"><span class="pipeline-detail-label">Cluster</span><span class="pipeline-detail-value">' + esc(cluster) + '</span></div>' +
              '<div class="pipeline-detail-row"><span class="pipeline-detail-label">Namespace</span><span class="pipeline-detail-value">' + esc(ns) + '</span></div>' +
              '<div class="pipeline-detail-row"><span class="pipeline-detail-label">Pod</span><span class="pipeline-detail-value">' + esc(pod) + '</span></div>' +
              '<div class="pipeline-detail-row"><span class="pipeline-detail-label">Time</span><span class="pipeline-detail-value">' + (inc.timestamp ? new Date(inc.timestamp).toLocaleString() : 'now') + '</span></div>' +
              (inc.message ? '<div class="pipeline-detail-row"><span class="pipeline-detail-label">Message</span><span class="pipeline-detail-value" style="max-width:300px;word-break:break-all;">' + esc(inc.message) + '</span></div>' : '') +
            '</div>' +
          '</div>' +
        '</div>' +

        // Workflow
        '<div class="panel" style="margin-bottom:var(--space-md);">' +
          '<div class="panel-header"><div class="panel-title">Workflow</div></div>' +
          '<div class="panel-body" style="padding:var(--space-md);">' +
            '<div style="display:flex;gap:var(--space-xs);margin-bottom:var(--space-sm);">' +
              workflowButton('open', meta.state) +
              workflowButton('investigating', meta.state) +
              workflowButton('mitigating', meta.state) +
              workflowButton('resolved', meta.state) +
            '</div>' +
          '</div>' +
        '</div>' +

        // Assignment
        '<div class="panel" style="margin-bottom:var(--space-md);">' +
          '<div class="panel-header"><div class="panel-title">Assignment</div></div>' +
          '<div class="panel-body" style="padding:var(--space-md);">' +
            '<div class="form-group" style="margin-bottom:var(--space-sm);">' +
              '<label class="form-label">Assignee</label>' +
              '<select class="form-select" id="drawer-assignee">' +
                '<option value="">Unassigned</option>' +
                '<option value="admin"' + (meta.assignee === 'admin' ? ' selected' : '') + '>admin</option>' +
                '<option value="sre-team"' + (meta.assignee === 'sre-team' ? ' selected' : '') + '>sre-team</option>' +
                '<option value="oncall"' + (meta.assignee === 'oncall' ? ' selected' : '') + '>oncall</option>' +
                '<option value="dev-team"' + (meta.assignee === 'dev-team' ? ' selected' : '') + '>dev-team</option>' +
              '</select>' +
            '</div>' +
            '<div class="form-group" style="margin-bottom:var(--space-sm);">' +
              '<label class="form-label">Add Note</label>' +
              '<div style="display:flex;gap:var(--space-xs);">' +
                '<input type="text" class="form-select" id="drawer-note" placeholder="Add a note..." style="flex:1;">' +
                '<button class="btn btn-primary btn-sm" id="drawer-add-note">Add</button>' +
              '</div>' +
            '</div>' +
            renderNotes(meta.notes) +
            '<div class="form-group">' +
              '<label class="form-label">Labels</label>' +
              '<div style="display:flex;gap:var(--space-xs);flex-wrap:wrap;" id="drawer-labels">' +
                meta.labels.map(function (l) { return '<span class="badge badge-synced">' + esc(l) + '</span>'; }).join('') +
                '<button class="btn btn-ghost btn-sm" id="drawer-add-label" style="font-size:11px;">+ Label</button>' +
              '</div>' +
            '</div>' +
          '</div>' +
        '</div>' +

        // Timeline
        '<div class="panel" style="margin-bottom:var(--space-md);">' +
          '<div class="panel-header"><div class="panel-title">Timeline</div></div>' +
          '<div class="panel-body" style="padding:var(--space-md);">' +
            renderTimeline(meta.timeline) +
          '</div>' +
        '</div>' +

        // Actions
        '<div class="panel">' +
          '<div class="panel-header"><div class="panel-title">Actions</div></div>' +
          '<div class="panel-body" style="padding:var(--space-md);display:flex;flex-wrap:wrap;gap:var(--space-xs);">' +
            '<button class="btn btn-ghost btn-sm drawer-action" data-action="ai-analyze">🤖 Analyze with AI</button>' +
            '<button class="btn btn-ghost btn-sm drawer-action" data-action="rca">🔍 Generate RCA</button>' +
            '<button class="btn btn-ghost btn-sm drawer-action" data-action="remediation">🛠 Generate Remediation</button>' +
            '<button class="btn btn-ghost btn-sm drawer-action" data-action="gitops-patch">📝 Create GitOps Patch</button>' +
            '<button class="btn btn-ghost btn-sm drawer-action" data-action="create-pr">🔀 Create PR</button>' +
          '</div>' +
        '</div>' +

      '</div>';
  }

  function bindDrawerEvents(key, meta) {
    // Close
    var closeBtn = document.getElementById('drawer-close');
    if (closeBtn) closeBtn.addEventListener('click', closeDrawer);

    // Workflow buttons
    drawerEl.querySelectorAll('.workflow-btn').forEach(function (btn) {
      btn.addEventListener('click', function () {
        meta.state = this.dataset.state;
        meta.timeline.push({ time: new Date().toISOString(), event: 'State → ' + meta.state, actor: 'admin' });
        AppState.addAuditLog({ action: 'update', target: 'incident/' + key + '/state', result: meta.state });
        openDrawer(currentIncident, key);
      });
    });

    // Assignee
    var assigneeEl = document.getElementById('drawer-assignee');
    if (assigneeEl) {
      assigneeEl.addEventListener('change', function () {
        meta.assignee = this.value;
        meta.timeline.push({ time: new Date().toISOString(), event: 'Assigned to ' + (meta.assignee || 'unassigned'), actor: 'admin' });
        AppState.addAuditLog({ action: 'assign', target: 'incident/' + key, result: meta.assignee || 'unassigned' });
      });
    }

    // Notes
    var addNoteBtn = document.getElementById('drawer-add-note');
    var noteInput = document.getElementById('drawer-note');
    if (addNoteBtn && noteInput) {
      addNoteBtn.addEventListener('click', function () {
        var text = noteInput.value.trim();
        if (text) {
          meta.notes.push({ text: text, time: new Date().toISOString(), actor: 'admin' });
          meta.timeline.push({ time: new Date().toISOString(), event: 'Note: ' + text, actor: 'admin' });
          openDrawer(currentIncident, key);
        }
      });
    }

    // Labels
    var addLabelBtn = document.getElementById('drawer-add-label');
    if (addLabelBtn) {
      addLabelBtn.addEventListener('click', function () {
        var label = prompt('Enter label:');
        if (label) {
          meta.labels.push(label.trim());
          meta.timeline.push({ time: new Date().toISOString(), event: 'Label added: ' + label.trim(), actor: 'admin' });
          openDrawer(currentIncident, key);
        }
      });
    }

    // Actions
    drawerEl.querySelectorAll('.drawer-action').forEach(function (btn) {
      btn.addEventListener('click', function () {
        var action = this.dataset.action;
        meta.timeline.push({ time: new Date().toISOString(), event: action + ' triggered', actor: 'admin' });
        AppState.addAuditLog({ action: action, target: 'incident/' + key, result: 'triggered' });
        this.textContent = '✅ ' + action;
        this.disabled = true;
      });
    });
  }

  function buildTimeline(inc) {
    var timeline = [];
    timeline.push({ time: inc.timestamp || new Date().toISOString(), event: 'Incident detected: ' + (inc.type || 'unknown'), actor: 'system' });
    if (inc.severity === 'critical') {
      timeline.push({ time: inc.timestamp || new Date().toISOString(), event: 'Alert escalated to on-call', actor: 'system' });
    }
    return timeline;
  }

  function renderTimeline(timeline) {
    if (!timeline.length) return '<div style="color:var(--color-muted);font-size:13px;">No events</div>';
    return '<div style="border-left:2px solid var(--color-primary);padding-left:var(--space-sm);">' +
      timeline.map(function (e) {
        return '<div style="margin-bottom:var(--space-sm);position:relative;">' +
          '<div style="position:absolute;left:-15px;top:4px;width:8px;height:8px;border-radius:50%;background:var(--color-primary);"></div>' +
          '<div style="font-size:12px;color:var(--color-muted);">' + timeShort(e.time) + ' · ' + esc(e.actor) + '</div>' +
          '<div style="font-size:13px;">' + esc(e.event) + '</div>' +
        '</div>';
      }).join('') +
    '</div>';
  }

  function renderNotes(notes) {
    if (!notes.length) return '';
    return '<div style="margin-bottom:var(--space-sm);">' +
      notes.map(function (n) {
        return '<div style="background:var(--color-bg);padding:8px;border-radius:4px;margin-bottom:4px;font-size:13px;">' +
          '<span style="color:var(--color-muted);font-size:11px;">' + timeShort(n.time) + ' · ' + esc(n.actor) + '</span><br>' +
          esc(n.text) +
        '</div>';
      }).join('') +
    '</div>';
  }

  function workflowButton(state, current) {
    var active = state === current;
    var colors = { open: '#ef4444', investigating: '#f59e0b', mitigating: '#3b82f6', resolved: '#22c55e' };
    return '<button class="btn btn-ghost btn-sm workflow-btn" data-state="' + state + '" style="' +
      (active ? 'background:' + colors[state] + ';color:#0b0e11;' : 'border-color:' + colors[state] + ';color:' + colors[state] + ';') +
      '">' + state.charAt(0).toUpperCase() + state.slice(1) + '</button>';
  }

  function severityClass(s) {
    if (s === 'critical') return 'down';
    if (s === 'warning') return 'degraded';
    return 'healthy';
  }

  function stateClass(s) {
    if (s === 'open') return 'down';
    if (s === 'investigating') return 'degraded';
    if (s === 'mitigating') return 'synced';
    return 'healthy';
  }

  function timeShort(ts) {
    try { return new Date(ts).toLocaleTimeString(); } catch (e) { return ts; }
  }

  

  global.IncidentDetailModule = { init: init, openDrawer: openDrawer };
})(window);
