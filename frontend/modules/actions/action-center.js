/**
 * Action Center — Operations hub for K8s resource management.
 * Tabs: Pods, Deployments, StatefulSets, Nodes, History
 * Features: Confirmation modals, execution console with progress/logs, action history.
 */
(function (global) {
  'use strict';

  var actionHistory = [];
  var consoleEl, consoleLog, consoleProgress, consoleStatus, consoleAction, consoleDuration, consoleClose;

  function init() {
    consoleEl = document.getElementById('action-console');
    consoleLog = document.getElementById('action-console-log');
    consoleProgress = document.getElementById('action-console-progress');
    consoleStatus = document.getElementById('action-console-status');
    consoleAction = document.getElementById('action-console-action');
    consoleDuration = document.getElementById('action-console-duration');
    consoleClose = document.getElementById('action-console-close');

    if (consoleClose) consoleClose.addEventListener('click', function () { consoleEl.style.display = 'none'; });

    // Tab switching — scope to action center section
    var sectionEl = document.getElementById('section-action-center');
    if (sectionEl) {
      sectionEl.querySelectorAll('.action-tab').forEach(function (tab) {
        tab.addEventListener('click', function () {
          sectionEl.querySelectorAll('.action-tab').forEach(function (t) { t.classList.remove('active'); });
          sectionEl.querySelectorAll('.action-tab-content').forEach(function (c) { c.classList.remove('active'); });
          tab.classList.add('active');
          var target = document.getElementById('tab-' + tab.dataset.tab);
          if (target) target.classList.add('active');
        });
      });
    }

    // Populate cluster/namespace selects from state
    AppState.on('kubernetes', populateFilters);
    AppState.on('navigate', function (section) {
      if (section === 'action-center') {
        populateFilters(AppState.getState().kubernetes);
        loadActiveData();
      }
    });

    // Always load action data on init
    loadActiveData();
  }

  function populateFilters(clusters) {
    var sel = document.getElementById('action-cluster-select');
    if (!sel || !clusters) return;
    var current = sel.value;
    sel.innerHTML = '<option value="">All Clusters</option>';
    clusters.forEach(function (c) {
      sel.innerHTML += '<option value="' + esc(c.name) + '">' + esc(c.name) + '</option>';
    });
    sel.value = current;
  }

  async function loadActiveData() {
    const kinds = ['pod', 'deployment', 'statefulset', 'node'];
    const renderers = {
      pod: renderPods,
      deployment: renderDeployments,
      statefulset: renderStatefulSets,
      node: renderNodes
    };

    for (const kind of kinds) {
      try {
        const res = await fetch('/api/v1/explorer?kind=' + kind);
        if (res.ok) {
          const json = await res.json();
          renderers[kind](json.data || []);
        } else {
          renderers[kind]([]);
        }
      } catch (e) {
        console.warn('Failed to load ' + kind + ' data:', e);
        renderers[kind]([]);
      }
    }

    renderHistory();
  }

  // ── POD ACTIONS ──
  function renderPods(pods) {
    var body = document.getElementById('action-pods-body');
    var empty = document.getElementById('action-pods-empty');
    if (!body) return;
    body.innerHTML = '';
    if (pods.length === 0) { if (empty) empty.style.display = ''; return; }
    if (empty) empty.style.display = 'none';
    pods.forEach(function (pod) {
      var tr = document.createElement('tr');
      var restarts = pod.restarts !== undefined ? pod.restarts : 0;
      var age = pod.age || 'N/A';
      tr.innerHTML =
        '<td><strong>' + esc(pod.name) + '</strong></td>' +
        '<td><code style="font-size:12px;color:var(--color-muted)">' + esc(pod.namespace) + '</code></td>' +
        '<td>' + podStatusBadge(pod.status) + '</td>' +
        '<td style="font-family:var(--font-number)">' + restarts + '</td>' +
        '<td style="font-size:12px;color:var(--color-muted)">' + esc(age) + '</td>' +
        '<td><div class="action-group">' +
          '<button class="action-btn" data-action="restart" data-kind="pod" data-name="' + esc(pod.name) + '">Restart</button>' +
          '<button class="action-btn" data-action="logs" data-kind="pod" data-name="' + esc(pod.name) + '">Logs</button>' +
          '<button class="action-btn" data-action="yaml" data-kind="pod" data-name="' + esc(pod.name) + '">YAML</button>' +
          '<button class="action-btn" data-action="diagnostics" data-kind="pod" data-name="' + esc(pod.name) + '">Diag</button>' +
          '<button class="action-btn danger" data-action="delete" data-kind="pod" data-name="' + esc(pod.name) + '">Delete</button>' +
        '</div></td>';
      bindActionButtons(tr);
      body.appendChild(tr);
    });
  }

  // ── DEPLOYMENT ACTIONS ──
  function renderDeployments(deps) {
    var body = document.getElementById('action-deploys-body');
    var empty = document.getElementById('action-deploys-empty');
    if (!body) return;
    body.innerHTML = '';
    if (deps.length === 0) { if (empty) empty.style.display = ''; return; }
    if (empty) empty.style.display = 'none';
    deps.forEach(function (dep) {
      var tr = document.createElement('tr');
      var replicas = dep.replicas !== undefined ? dep.replicas : 1;
      var available = dep.available !== undefined ? dep.available : 1;
      tr.innerHTML =
        '<td><strong>' + esc(dep.name) + '</strong></td>' +
        '<td><code style="font-size:12px;color:var(--color-muted)">' + esc(dep.namespace) + '</code></td>' +
        '<td style="font-family:var(--font-number)">' + replicas + '</td>' +
        '<td style="font-family:var(--font-number)">' + available + '/' + replicas + '</td>' +
        '<td>' + deployStatusBadge(dep.status) + '</td>' +
        '<td><div class="action-group">' +
          '<button class="action-btn" data-action="restart" data-kind="deployment" data-name="' + esc(dep.name) + '">Restart</button>' +
          '<button class="action-btn" data-action="scale" data-kind="deployment" data-name="' + esc(dep.name) + '">Scale</button>' +
          '<button class="action-btn" data-action="rollback" data-kind="deployment" data-name="' + esc(dep.name) + '">Rollback</button>' +
          '<button class="action-btn" data-action="pause" data-kind="deployment" data-name="' + esc(dep.name) + '">Pause</button>' +
          '<button class="action-btn" data-action="resume" data-kind="deployment" data-name="' + esc(dep.name) + '">Resume</button>' +
        '</div></td>';
      bindActionButtons(tr);
      body.appendChild(tr);
    });
  }

  // ── STATEFULSET ACTIONS ──
  function renderStatefulSets(stsList) {
    var body = document.getElementById('action-sts-body');
    var empty = document.getElementById('action-sts-empty');
    if (!body) return;
    body.innerHTML = '';
    if (stsList.length === 0) { if (empty) empty.style.display = ''; return; }
    if (empty) empty.style.display = 'none';
    stsList.forEach(function (sts) {
      var tr = document.createElement('tr');
      var replicas = sts.replicas !== undefined ? sts.replicas : 1;
      var ready = sts.ready !== undefined ? sts.ready : 1;
      var storage = sts.storage !== undefined ? sts.storage : 'N/A';
      tr.innerHTML =
        '<td><strong>' + esc(sts.name) + '</strong></td>' +
        '<td><code style="font-size:12px;color:var(--color-muted)">' + esc(sts.namespace) + '</code></td>' +
        '<td style="font-family:var(--font-number)">' + replicas + '</td>' +
        '<td style="font-family:var(--font-number)">' + ready + '/' + replicas + '</td>' +
        '<td style="font-size:12px">' + esc(storage) + '</td>' +
        '<td><div class="action-group">' +
          '<button class="action-btn" data-action="restart" data-kind="statefulset" data-name="' + esc(sts.name) + '">Restart</button>' +
          '<button class="action-btn" data-action="scale" data-kind="statefulset" data-name="' + esc(sts.name) + '">Scale</button>' +
          '<button class="action-btn" data-action="storage" data-kind="statefulset" data-name="' + esc(sts.name) + '">Storage</button>' +
        '</div></td>';
      bindActionButtons(tr);
      body.appendChild(tr);
    });
  }

  // ── NODE ACTIONS ──
  function renderNodes(nodes) {
    var body = document.getElementById('action-nodes-body');
    var empty = document.getElementById('action-nodes-empty');
    if (!body) return;
    body.innerHTML = '';
    if (nodes.length === 0) { if (empty) empty.style.display = ''; return; }
    if (empty) empty.style.display = 'none';
    nodes.forEach(function (node) {
      var tr = document.createElement('tr');
      var role = node.role !== undefined ? node.role : 'worker';
      var cpu = node.cpu !== undefined ? node.cpu : 'N/A';
      var memory = node.memory !== undefined ? node.memory : 'N/A';
      var podsCount = node.pods !== undefined ? node.pods : 0;
      tr.innerHTML =
        '<td><strong>' + esc(node.name) + '</strong></td>' +
        '<td><span class="badge badge-' + (role === 'control-plane' ? 'synced' : 'healthy') + '">' + esc(role) + '</span></td>' +
        '<td>' + nodeStatusBadge(node.status) + '</td>' +
        '<td style="font-family:var(--font-number)">' + esc(cpu) + '</td>' +
        '<td style="font-family:var(--font-number)">' + esc(memory) + '</td>' +
        '<td style="font-family:var(--font-number)">' + podsCount + '</td>' +
        '<td><div class="action-group">' +
          '<button class="action-btn" data-action="cordon" data-kind="node" data-name="' + esc(node.name) + '">Cordon</button>' +
          '<button class="action-btn" data-action="uncordon" data-kind="node" data-name="' + esc(node.name) + '">Uncordon</button>' +
          '<button class="action-btn" data-action="drain" data-kind="node" data-name="' + esc(node.name) + '">Drain</button>' +
          '<button class="action-btn" data-action="diagnostics" data-kind="node" data-name="' + esc(node.name) + '">Diag</button>' +
        '</div></td>';
      bindActionButtons(tr);
      body.appendChild(tr);
    });
  }

  // ── ACTION EXECUTION ENGINE ──
  function bindActionButtons(parent) {
    parent.querySelectorAll('.action-btn').forEach(function (btn) {
      btn.addEventListener('click', function () {
        var action = this.dataset.action;
        var kind = this.dataset.kind;
        var name = this.dataset.name;
        confirmAndExecute(action, kind, name);
      });
    });
  }

  function confirmAndExecute(action, kind, name) {
    var actionLabel = action.charAt(0).toUpperCase() + action.slice(1);
    var isDangerous = ['delete', 'drain', 'cordon'].indexOf(action) >= 0;

    // Special handling for scale
    if (action === 'scale') {
      Modal.open({
        title: '📐 Scale ' + kind + ': ' + name,
        body: '<div class="form-group"><label class="form-label">Target Replicas</label><input type="number" class="form-select" id="scale-replicas" min="0" max="100" value="3"></div>',
        actions: [
          { label: 'Cancel' },
          { label: 'Scale', primary: true, onClick: function () {
            var replicas = document.getElementById('scale-replicas').value;
            executeAction(action, kind, name, { replicas: replicas });
          }}
        ]
      });
      return;
    }

    // Special handling for logs/yaml — show in modal directly
    if (action === 'logs') {
      showLogsModal(name);
      return;
    }
    if (action === 'yaml') {
      showYamlModal(kind, name);
      return;
    }
    if (action === 'storage') {
      showStorageModal(name);
      return;
    }

    Modal.open({
      title: (isDangerous ? '⚠️ ' : '▶ ') + actionLabel + ' ' + kind + ': ' + name,
      body: '<p style="color:var(--color-text-secondary)">Are you sure you want to <strong>' + esc(actionLabel.toLowerCase()) + '</strong> ' + kind + ' <code>' + esc(name) + '</code>?</p>' +
            (isDangerous ? '<p style="color:var(--color-trading-down);font-size:13px;">⚠ This is a destructive action and cannot be undone.</p>' : ''),
      actions: [
        { label: 'Cancel' },
        { label: actionLabel, primary: true, onClick: function () { executeAction(action, kind, name); } }
      ]
    });
  }

  function executeAction(action, kind, name, params) {
    var startTime = Date.now();
    var entry = {
      action: action,
      kind: kind,
      name: name,
      params: params || {},
      status: 'running',
      startTime: startTime,
      logs: []
    };

    // Show execution console
    showConsole(entry);

    // Simulate execution steps
    var steps = getExecutionSteps(action, kind, name, params);
    var stepIndex = 0;

    function runStep() {
      if (stepIndex >= steps.length) {
        entry.status = 'success';
        entry.duration = Date.now() - startTime;
        updateConsole(entry, 100);
        addToHistory(entry);
        AppState.addAuditLog({ action: action, target: kind + '/' + name, result: 'success' });
        return;
      }

      var step = steps[stepIndex];
      entry.logs.push(step.log);
      updateConsole(entry, Math.round(((stepIndex + 1) / steps.length) * 100));
      stepIndex++;
      setTimeout(runStep, step.delay);
    }

    setTimeout(runStep, 300);
  }

  function getExecutionSteps(action, kind, name, params) {
    var base = [
      { log: '$ kubectl ' + action + ' ' + kind + ' ' + name, delay: 200 },
      { log: '→ Connecting to cluster...', delay: 400 },
      { log: '→ Authenticating...', delay: 300 },
    ];

    switch (action) {
      case 'restart':
        base.push({ log: '→ Rolling restart initiated for ' + kind + '/' + name, delay: 500 });
        base.push({ log: '→ Waiting for rollout...', delay: 800 });
        base.push({ log: '✓ ' + kind + '/' + name + ' successfully restarted', delay: 200 });
        break;
      case 'delete':
        base.push({ log: '→ Deleting ' + kind + '/' + name + '...', delay: 600 });
        base.push({ log: '→ Waiting for termination...', delay: 500 });
        base.push({ log: '✓ ' + kind + '/' + name + ' deleted', delay: 200 });
        break;
      case 'scale':
        var r = params && params.replicas || 3;
        base.push({ log: '→ Scaling ' + kind + '/' + name + ' to ' + r + ' replicas...', delay: 500 });
        base.push({ log: '→ Waiting for pods...', delay: 800 });
        base.push({ log: '✓ Scaled to ' + r + ' replicas', delay: 200 });
        break;
      case 'rollback':
        base.push({ log: '→ Fetching rollout history...', delay: 400 });
        base.push({ log: '→ Rolling back to previous revision...', delay: 600 });
        base.push({ log: '✓ Rollback complete', delay: 200 });
        break;
      case 'cordon':
        base.push({ log: '→ Cordoning node ' + name + '...', delay: 400 });
        base.push({ log: '✓ Node ' + name + ' cordoned (unschedulable)', delay: 200 });
        break;
      case 'uncordon':
        base.push({ log: '→ Uncordoning node ' + name + '...', delay: 400 });
        base.push({ log: '✓ Node ' + name + ' uncordoned (schedulable)', delay: 200 });
        break;
      case 'drain':
        base.push({ log: '→ Evicting pods from node ' + name + '...', delay: 800 });
        base.push({ log: '→ Waiting for pod evictions...', delay: 1000 });
        base.push({ log: '✓ Node ' + name + ' drained successfully', delay: 200 });
        break;
      case 'diagnostics':
        base.push({ log: '→ Collecting diagnostics for ' + kind + '/' + name + '...', delay: 600 });
        base.push({ log: '→ CPU: 67% | Memory: 72% | Disk: 45%', delay: 300 });
        base.push({ log: '→ Network: OK | DNS: OK | Kubelet: running', delay: 300 });
        base.push({ log: '✓ Diagnostics complete — no issues found', delay: 200 });
        break;
      default:
        base.push({ log: '→ Executing ' + action + '...', delay: 500 });
        base.push({ log: '✓ Done', delay: 200 });
    }
    return base;
  }

  function showConsole(entry) {
    if (!consoleEl) return;
    consoleEl.style.display = '';
    if (consoleAction) consoleAction.textContent = entry.action.toUpperCase() + ' ' + entry.kind + '/' + entry.name;
    if (consoleStatus) { consoleStatus.textContent = 'Running'; consoleStatus.className = 'badge badge-degraded'; }
    if (consoleProgress) consoleProgress.style.width = '0%';
    if (consoleLog) consoleLog.textContent = 'Initializing...';
    if (consoleDuration) consoleDuration.textContent = '';
  }

  function updateConsole(entry, progress) {
    if (consoleProgress) consoleProgress.style.width = progress + '%';
    if (consoleLog) consoleLog.textContent = entry.logs.join('\n');
    if (consoleDuration) consoleDuration.textContent = ((Date.now() - entry.startTime) / 1000).toFixed(1) + 's';
    if (entry.status === 'success' && consoleStatus) {
      consoleStatus.textContent = 'Success';
      consoleStatus.className = 'badge badge-healthy';
    } else if (entry.status === 'failed' && consoleStatus) {
      consoleStatus.textContent = 'Failed';
      consoleStatus.className = 'badge badge-down';
    }
    if (consoleLog) consoleLog.scrollTop = consoleLog.scrollHeight;
  }

  // ── HISTORY ──
  function addToHistory(entry) {
    actionHistory.unshift({
      time: new Date().toISOString(),
      action: entry.action,
      target: entry.kind + '/' + entry.name,
      status: entry.status,
      duration: ((entry.duration || 0) / 1000).toFixed(1) + 's'
    });
    if (actionHistory.length > 100) actionHistory.pop();
    renderHistory();
  }

  function renderHistory() {
    var body = document.getElementById('action-history-body');
    var empty = document.getElementById('action-history-empty');
    if (!body) return;
    body.innerHTML = '';
    if (actionHistory.length === 0) { if (empty) empty.style.display = ''; return; }
    if (empty) empty.style.display = 'none';
    actionHistory.forEach(function (h) {
      var tr = document.createElement('tr');
      tr.innerHTML =
        '<td style="font-size:12px;color:var(--color-muted)">' + timeFormat(h.time) + '</td>' +
        '<td><span class="badge badge-synced">' + esc(h.action) + '</span></td>' +
        '<td><code style="font-size:12px">' + esc(h.target) + '</code></td>' +
        '<td>' + UIComponents.resultBadge(h.status) + '</td>' +
        '<td style="font-family:var(--font-number);font-size:12px">' + esc(h.duration) + '</td>' +
        '<td><button class="action-btn">View</button></td>';
      body.appendChild(tr);
    });
  }

  // ── SPECIAL MODALS ──
  function showLogsModal(name) {
    Modal.open({
      title: '📋 Logs: ' + name,
      body: '<pre class="ai-test-response" style="max-height:400px;overflow-y:auto;font-size:12px;">' +
        '2026-06-25T08:55:01Z [INFO] Starting server on :8080\n' +
        '2026-06-25T08:55:02Z [INFO] Connected to database\n' +
        '2026-06-25T08:55:02Z [INFO] Health check passed\n' +
        '2026-06-25T08:55:15Z [WARN] High memory usage: 89%\n' +
        '2026-06-25T08:55:30Z [ERROR] Connection timeout to redis:6379\n' +
        '2026-06-25T08:55:31Z [INFO] Reconnecting to redis...\n' +
        '2026-06-25T08:55:32Z [INFO] Redis connection restored\n' +
        '2026-06-25T08:56:00Z [INFO] Request processed: GET /api/v1/pods (200)\n' +
        '</pre>',
      actions: [{ label: 'Close', primary: true }]
    });
  }

  function showYamlModal(kind, name) {
    Modal.open({
      title: '📄 YAML: ' + kind + '/' + name,
      body: '<pre class="ai-test-response" style="max-height:400px;overflow-y:auto;font-size:12px;">' +
        'apiVersion: v1\n' +
        'kind: Pod\n' +
        'metadata:\n' +
        '  name: ' + esc(name) + '\n' +
        '  namespace: production\n' +
        '  labels:\n' +
        '    app: ' + esc(name.split('-')[0]) + '\n' +
        'spec:\n' +
        '  containers:\n' +
        '  - name: app\n' +
        '    image: registry.internal/' + esc(name.split('-')[0]) + ':latest\n' +
        '    resources:\n' +
        '      requests:\n' +
        '        cpu: 250m\n' +
        '        memory: 256Mi\n' +
        '      limits:\n' +
        '        cpu: 500m\n' +
        '        memory: 512Mi\n' +
        '</pre>',
      actions: [{ label: 'Close', primary: true }]
    });
  }

  function showStorageModal(name) {
    Modal.open({
      title: '💾 Storage: ' + name,
      body: '<div class="pipeline-detail"><div class="pipeline-detail-row"><span class="pipeline-detail-label">PVC Count</span><span class="pipeline-detail-value">3</span></div><div class="pipeline-detail-row"><span class="pipeline-detail-label">Total Size</span><span class="pipeline-detail-value">300Gi</span></div><div class="pipeline-detail-row"><span class="pipeline-detail-label">Used</span><span class="pipeline-detail-value">187Gi (62%)</span></div><div class="pipeline-detail-row"><span class="pipeline-detail-label">Storage Class</span><span class="pipeline-detail-value">gp3-encrypted</span></div><div class="pipeline-detail-row"><span class="pipeline-detail-label">Status</span><span class="pipeline-detail-value"><span class="badge badge-healthy">Bound</span></span></div></div>',
      actions: [{ label: 'Close', primary: true }]
    });
  }

  // ── HELPERS ──
  function podStatusBadge(s) {
    return s === 'Running' ? '<span class="badge badge-healthy">' + esc(s) + '</span>' :
           (s === 'CrashLoopBackOff' || s === 'OOMKilled' ? '<span class="badge badge-down">' + esc(s) + '</span>' :
           '<span class="badge badge-degraded">' + esc(s) + '</span>');
  }

  function deployStatusBadge(s) {
    return s === 'Available' ? '<span class="badge badge-healthy">' + esc(s) + '</span>' :
           (s === 'Progressing' ? '<span class="badge badge-degraded">' + esc(s) + '</span>' :
           '<span class="badge badge-down">' + esc(s) + '</span>');
  }

  function nodeStatusBadge(s) {
    return s === 'Ready' ? '<span class="badge badge-healthy">' + esc(s) + '</span>' : '<span class="badge badge-down">' + esc(s) + '</span>';
  }

  function timeFormat(ts) {
    try { return new Date(ts).toLocaleTimeString(); } catch (e) { return ts; }
  }

  global.ActionCenterSection = { init: init };
})(window);
