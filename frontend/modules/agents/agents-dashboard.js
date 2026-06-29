/**
 * Multi-Agent Execution Framework Dashboard
 * Controls task backlog, dependency tracking, metrics, and live logs.
 */
(function (global) {
  'use strict';

  var container = null;

  function init() {
    container = document.getElementById('agents-page');
    if (!container) return;

    renderLayout();
    bindEvents();
    refresh();

    // Subscribe to real-time events from AppState
    if (global.AppState) {
      global.AppState.on('agent', function () {
        refresh();
      });
    }
  }

  function renderLayout() {
    container.innerHTML = `
      <div style="display:flex;flex-direction:column;gap:var(--space-md);height:100%;padding:var(--space-xs);">
        <!-- Metric Cards Row -->
        <div style="display:grid;grid-template-columns:repeat(4, 1fr);gap:var(--space-md);">
          <div class="panel" style="padding:var(--space-md);border-left:3px solid var(--color-primary);">
            <div style="font-size:var(--text-caption);color:var(--color-muted);">Current Phase & Module</div>
            <h3 id="agent-meta-phase" style="margin:8px 0 0 0;font-size:16px;">—</h3>
            <span id="agent-meta-module" style="font-size:12px;color:var(--color-muted);">—</span>
          </div>
          <div class="panel" style="padding:var(--space-md);border-left:3px solid #10b981;">
            <div style="font-size:var(--text-caption);color:var(--color-muted);">Repository Health</div>
            <h3 id="agent-meta-health" style="margin:8px 0 0 0;font-size:24px;color:#10b981;">100.0%</h3>
          </div>
          <div class="panel" style="padding:var(--space-md);border-left:3px solid #f59e0b;">
            <div style="font-size:var(--text-caption);color:var(--color-muted);">Technical Debt</div>
            <h3 id="agent-meta-debt" style="margin:8px 0 0 0;font-size:24px;color:#f59e0b;">0.0h</h3>
          </div>
          <div class="panel" style="padding:var(--space-md);border-left:3px solid #3b82f6;">
            <div style="font-size:var(--text-caption);color:var(--color-muted);">Architecture & Quality</div>
            <h3 style="margin:8px 0 0 0;font-size:16px;">
              Arch: <span id="agent-meta-arch" style="color:#3b82f6;">100%</span> | 
              QA: <span id="agent-meta-qa" style="color:#10b981;">100%</span>
            </h3>
          </div>
        </div>

        <!-- Main Workspace splits -->
        <div style="display:grid;grid-template-columns:1.5fr 1fr;gap:var(--space-md);flex-grow:1;min-height:500px;">
          <!-- Left side: Task Queue & Creation -->
          <div class="panel" style="display:flex;flex-direction:column;padding:var(--space-md);">
            <div style="display:flex;justify-content:between;align-items:center;margin-bottom:var(--space-md);">
              <h3 style="margin:0;font-size:15px;">📋 Tasks Backlog</h3>
              <button class="btn btn-primary btn-sm" id="btn-add-task" style="margin-left:auto;">+ Add Task</button>
            </div>
            <div style="flex-grow:1;overflow-y:auto;max-height:600px;" id="tasks-list-container">
              <div class="empty-state"><div class="empty-state-icon">📋</div><div class="empty-state-text">No tasks created yet</div></div>
            </div>
          </div>

          <!-- Right side: Run History & Console Logs -->
          <div style="display:flex;flex-direction:column;gap:var(--space-md);">
            <div class="panel" style="flex:1;display:flex;flex-direction:column;padding:var(--space-md);">
              <h3 style="margin:0 0 var(--space-md) 0;font-size:15px;">🤖 Agent Executions</h3>
              <div style="flex-grow:1;overflow-y:auto;max-height:300px;" id="runs-list-container">
                <div class="empty-state"><div class="empty-state-icon">🤖</div><div class="empty-state-text">No executions recorded</div></div>
              </div>
            </div>

            <div class="panel" style="flex:1;display:flex;flex-direction:column;padding:var(--space-md);background:#0a0e17;border:1px solid var(--color-hairline);">
              <h3 style="margin:0 0 var(--space-sm) 0;font-size:13px;color:#cbd5e1;display:flex;justify-content:between;">
                📟 Pipeline Live Logs
                <button class="btn btn-ghost btn-sm" id="btn-clear-logs" style="margin-left:auto;color:#cbd5e1;padding:2px 8px;font-size:10px;">Clear</button>
              </h3>
              <div id="agent-run-logs" style="flex-grow:1;overflow-y:auto;max-height:250px;font-family:monospace;font-size:11px;color:#38bdf8;padding:var(--space-xs);line-height:1.6;white-space:pre-wrap;">[Console Idle] Waiting for next step...</div>
            </div>
          </div>
        </div>
      </div>
    `;
  }

  function bindEvents() {
    var btnAdd = document.getElementById('btn-add-task');
    if (btnAdd) {
      btnAdd.addEventListener('click', showCreateTaskModal);
    }

    var btnClear = document.getElementById('btn-clear-logs');
    if (btnClear) {
      btnClear.addEventListener('click', function () {
        var logEl = document.getElementById('agent-run-logs');
        if (logEl) logEl.innerHTML = '[Console Cleared]\n';
      });
    }
  }

  function refresh() {
    // 1. Fetch project state
    fetch('/api/v1/agents/state')
      .then(function(r){ return r.json(); })
      .then(function(state) {
        updateStateUI(state);
      })
      .catch(function(err){ console.error('Error fetching agent state:', err); });

    // 2. Fetch tasks
    fetch('/api/v1/agents/tasks')
      .then(function(r){ return r.json(); })
      .then(function(resp) {
        renderTasks(resp.data || []);
      })
      .catch(function(err){ console.error('Error fetching agent tasks:', err); });

    // 3. Fetch runs
    fetch('/api/v1/agents/runs')
      .then(function(r){ return r.json(); })
      .then(function(resp) {
        renderRuns(resp.data || []);
      })
      .catch(function(err){ console.error('Error fetching agent runs:', err); });
  }

  function updateStateUI(state) {
    if (!state) return;
    document.getElementById('agent-meta-phase').textContent = state.current_phase || '—';
    document.getElementById('agent-meta-module').textContent = state.current_module || '—';
    document.getElementById('agent-meta-health').textContent = (state.repository_health || 100.0).toFixed(1) + '%';
    document.getElementById('agent-meta-debt').textContent = (state.technical_debt || 0.0).toFixed(1) + 'h';
    document.getElementById('agent-meta-arch').textContent = (state.architecture_score || 100.0).toFixed(0) + '%';
    document.getElementById('agent-meta-qa').textContent = (state.quality_score || 100.0).toFixed(0) + '%';
  }

  function renderTasks(tasks) {
    var container = document.getElementById('tasks-list-container');
    if (!container) return;

    if (tasks.length === 0) {
      container.innerHTML = '<div class="empty-state"><div class="empty-state-icon">📋</div><div class="empty-state-text">No tasks created yet</div></div>';
      return;
    }

    container.innerHTML = '<div style="display:flex;flex-direction:column;gap:var(--space-sm);">' + tasks.map(function (t) {
      var statusColor = getStatusColor(t.status);
      var deps = t.dependencies && t.dependencies.length > 0 
        ? '<div style="font-size:11px;color:var(--color-muted);margin-top:4px;">Dependencies: <strong>' + t.dependencies.join(', ') + '</strong></div>' 
        : '';
      
      var subtasksHtml = t.subtasks && t.subtasks.length > 0 
        ? '<div style="margin-top:8px;padding-top:8px;border-top:1px solid var(--color-hairline);">' + t.subtasks.map(function(s){
            return '<div style="font-size:11px;display:flex;align-items:center;gap:6px;margin-bottom:2px;">'
              + '<span style="color:' + getStatusColor(s.status) + ';">' + (s.status === 'success' ? '✓' : '⏳') + '</span>'
              + '<span>' + s.title + ' (Complexity: ' + s.complexity + ')</span>'
              + '</div>';
          }).join('') + '</div>'
        : '';

      return '<div class="panel" style="padding:var(--space-md);border-left:3px solid ' + statusColor + ';">'
        + '<div style="display:flex;justify-content:between;align-items:center;">'
        + '<span style="font-weight:600;font-size:13px;color:var(--color-primary-light);">' + t.title + '</span>'
        + '<span class="badge" style="background:' + statusColor + '20;color:' + statusColor + ';border:1px solid ' + statusColor + '40;font-size:10px;padding:1px 6px;border-radius:3px;">' + t.status.toUpperCase() + '</span>'
        + '</div>'
        + '<div style="font-size:12px;color:var(--color-muted);margin-top:4px;">' + t.description + '</div>'
        + '<div style="display:flex;gap:var(--space-md);font-size:11px;color:var(--color-muted);margin-top:6px;">'
        + '<span>Phase: <strong>' + t.phase + '</strong></span>'
        + '<span>Module: <strong>' + t.module + '</strong></span>'
        + '</div>'
        + deps
        + subtasksHtml
        + '</div>';
    }).join('') + '</div>';
  }

  function renderRuns(runs) {
    var container = document.getElementById('runs-list-container');
    if (!container) return;

    if (runs.length === 0) {
      container.innerHTML = '<div class="empty-state"><div class="empty-state-icon">🤖</div><div class="empty-state-text">No executions recorded</div></div>';
      return;
    }

    container.innerHTML = '<table class="table" style="font-size:12px;">'
      + '<thead><tr><th>Agent</th><th>Status</th><th>Time</th><th>Action</th></tr></thead>'
      + '<tbody>' + runs.map(function (r) {
        var statusColor = getStatusColor(r.status);
        var date = new Date(r.created_at).toLocaleTimeString();
        return '<tr>'
          + '<td><strong>' + r.agent_type + '</strong></td>'
          + '<td><span style="color:' + statusColor + ';">● ' + r.status.toUpperCase() + '</span></td>'
          + '<td class="text-muted-sm">' + date + '</td>'
          + '<td><button class="btn btn-ghost btn-xs" onclick="AgentsDashboard.viewOutput(\'' + r.id + '\')">Logs</button></td>'
          + '</tr>';
      }).join('') + '</tbody></table>';
  }

  function viewOutput(runID) {
    fetch('/api/v1/agents/runs')
      .then(function(r){ return r.json(); })
      .then(function(resp) {
        var run = (resp.data || []).find(function(r) { return r.id === runID; });
        if (!run || !global.Modal) return;

        var content = run.output || run.error_detail || 'No logs generated.';
        global.Modal.open({
          title: '📟 Execution Logs: ' + run.agent_type,
          body: '<div style="background:#0a0e17;color:#38bdf8;padding:var(--space-md);border-radius:4px;font-family:monospace;font-size:11px;line-height:1.6;white-space:pre-wrap;max-height:500px;overflow-y:auto;">' + content + '</div>'
        });
      })
      .catch(function(err){ console.error('Error fetching run detail:', err); });
  }

  function showCreateTaskModal() {
    if (!global.Modal) return;

    global.Modal.open({
      title: '📋 Create Execution Task',
      body: '<form id="form-create-task" style="display:flex;flex-direction:column;gap:var(--space-sm);padding:var(--space-xs);">'
        + '<div class="form-group"><label class="form-label">Task Title</label><input type="text" class="form-select" id="task-title" required placeholder="e.g. Implement Drift Reconciliation"></div>'
        + '<div class="form-group"><label class="form-label">Description</label><textarea class="form-select" id="task-desc" rows="3" required placeholder="Outline task objective details..."></textarea></div>'
        + '<div style="display:grid;grid-template-columns:1fr 1fr;gap:var(--space-sm);">'
        + '<div class="form-group"><label class="form-label">Phase</label><input type="text" class="form-select" id="task-phase" value="Phase 11" required></div>'
        + '<div class="form-group"><label class="form-label">Module</label><input type="text" class="form-select" id="task-module" value="drift" required></div>'
        + '</div>'
        + '<div class="form-group"><label class="form-label">Feature Context</label><input type="text" class="form-select" id="task-feature" value="Reconciler" required></div>'
        + '<div class="form-group"><label class="form-label">Dependencies (comma-separated Task IDs)</label><input type="text" class="form-select" id="task-deps" placeholder="e.g. TASK-UUID-1, TASK-UUID-2"></div>'
        + '<button type="submit" class="btn btn-primary" style="margin-top:var(--space-sm);">Submit Task to Queue</button>'
        + '</form>'
    });

    document.getElementById('form-create-task').addEventListener('submit', function (e) {
      e.preventDefault();
      var depsVal = document.getElementById('task-deps').value;
      var depsArray = depsVal ? depsVal.split(',').map(function(s){ return s.trim(); }).filter(Boolean) : [];

      var payload = {
        title: document.getElementById('task-title').value,
        description: document.getElementById('task-desc').value,
        phase: document.getElementById('task-phase').value,
        module: document.getElementById('task-module').value,
        feature: document.getElementById('task-feature').value,
        dependencies: depsArray
      };

      fetch('/api/v1/agents/tasks', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload)
      })
      .then(function(r) { return r.json(); })
      .then(function() {
        if (global.Modal) global.Modal.close();
        refresh();
      })
      .catch(function(err) { console.error('Error creating task:', err); });
    });
  }

  function getStatusColor(status) {
    switch (status) {
      case 'success': return '#10b981';
      case 'inprogress': return 'var(--color-primary)';
      case 'running': return 'var(--color-primary)';
      case 'failed': return '#ef4444';
      case 'blocked': return '#f59e0b';
      default: return 'var(--color-muted)';
    }
  }

  global.AgentsDashboard = {
    init: init,
    refresh: refresh,
    viewOutput: viewOutput
  };

})(window);
