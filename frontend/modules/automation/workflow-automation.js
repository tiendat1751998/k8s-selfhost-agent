/**
 * Workflow Automation Engine Module
 * Event-driven rules builder with triggers, actions, and execution history.
 */
(function (global) {
  'use strict';

  var rules = [];
  var executionLog = [];
  var loadingRules = false;
  var loadingHistory = false;

  // Intentional constants for dropdown configurations (no dynamic backend endpoint available)
  var triggerTypes = [
    { value: 'pod_restart', label: 'Pod Restart Count > Threshold' },
    { value: 'node_pressure', label: 'Node Resource Pressure' },
    { value: 'deployment_failure', label: 'Deployment Failure' },
    { value: 'high_cpu', label: 'High CPU Usage' },
    { value: 'high_memory', label: 'High Memory Usage' },
    { value: 'slo_breach', label: 'SLO Budget Breach' },
    { value: 'error_rate', label: 'Error Rate Spike' }
  ];

  var actionTypes = [
    { value: 'generate_rca', label: 'Generate RCA Report' },
    { value: 'send_notification', label: 'Send Notification' },
    { value: 'rollback', label: 'Auto-Rollback Deployment' },
    { value: 'scale_deployment', label: 'Scale Deployment' },
    { value: 'create_incident', label: 'Create Incident' },
    { value: 'cordon_node', label: 'Cordon Node' },
    { value: 'restart_pod', label: 'Restart Pod' }
  ];

  /* ─── Render Rules Table ─── */
  function renderRulesTable() {
    var tbody = document.getElementById('auto-rules-tbody');
    if (!tbody) return;

    if (loadingRules) {
      tbody.innerHTML = '<tr><td colspan="8" style="text-align:center;padding:var(--space-md);"><span class="loading loading-spinner"></span> Loading rules...</td></tr>';
      return;
    }

    if (rules.length === 0) {
      tbody.innerHTML = '<tr><td colspan="8" style="text-align:center;color:var(--color-muted);padding:var(--space-md);">No rules configured.</td></tr>';
      return;
    }

    tbody.innerHTML = rules.map(function (r) {
      var statusHtml = r.enabled
        ? '<span style="color:#10b981;font-weight:600;">● Active</span>'
        : '<span style="color:#6b7280;">○ Disabled</span>';
      
      var name = Security.escapeHTML(r.name);
      var triggerType = Security.escapeHTML(r.trigger_type || (r.trigger && r.trigger.type) || '');
      var condition = Security.escapeHTML((r.trigger_config && r.trigger_config.condition) || (r.trigger && r.trigger.condition) || '');
      var actionType = Security.escapeHTML(r.action_type || (r.action && r.action.type) || '');
      var executions = r.executions !== undefined ? r.executions : 0;
      
      var lastTriggeredVal = 'Never';
      if (r.last_triggered) {
        lastTriggeredVal = new Date(r.last_triggered).toLocaleString();
      } else if (r.lastTriggered) {
        lastTriggeredVal = r.lastTriggered;
      }

      return '<tr>'
        + '<td><strong>' + name + '</strong></td>'
        + '<td style="font-size:12px;">' + triggerType.replace(/_/g, ' ') + '</td>'
        + '<td style="font-size:12px;color:var(--color-muted);">' + condition + '</td>'
        + '<td style="font-size:12px;">' + actionType.replace(/_/g, ' ') + '</td>'
        + '<td>' + statusHtml + '</td>'
        + '<td style="font-family:var(--font-number);font-size:12px;">' + executions + '</td>'
        + '<td style="font-size:12px;color:var(--color-muted);">' + Security.escapeHTML(lastTriggeredVal) + '</td>'
        + '<td>'
        + '  <button class="btn btn-ghost btn-sm" onclick="WorkflowAutomation.toggleRule(\'' + r.id + '\', ' + r.enabled + ')">' + (r.enabled ? 'Disable' : 'Enable') + '</button>'
        + '  <button class="btn btn-ghost btn-sm" style="color:#ef4444;" onclick="WorkflowAutomation.deleteRule(\'' + r.id + '\')">✕</button>'
        + '</td>'
        + '</tr>';
    }).join('');
  }

  /* ─── Render Execution Log ─── */
  function renderExecutionLog() {
    var container = document.getElementById('auto-exec-log');
    if (!container) return;

    if (loadingHistory) {
      container.innerHTML = '<div style="text-align:center;padding:var(--space-md);"><span class="loading loading-spinner"></span> Loading history...</div>';
      return;
    }

    if (executionLog.length === 0) {
      container.innerHTML = '<div style="text-align:center;color:var(--color-muted);padding:var(--space-md);">No execution logs found.</div>';
      return;
    }

    container.innerHTML = executionLog.map(function (e) {
      var resultColor = e.result === 'success' ? '#10b981' : '#ef4444';
      var resultIcon = e.result === 'success' ? '✅' : '❌';
      
      var ruleName = Security.escapeHTML(e.rule_name || e.ruleName || 'Unknown Rule');
      var trigger = Security.escapeHTML(e.trigger_event || e.trigger || '');
      var action = Security.escapeHTML(e.action_taken || e.action || '');
      var timestamp = Security.escapeHTML(e.created_at ? new Date(e.created_at).toLocaleString() : (e.timestamp || ''));

      return '<div class="auto-exec-item">'
        + '<div class="auto-exec-icon">' + resultIcon + '</div>'
        + '<div class="auto-exec-body">'
        + '  <div class="auto-exec-header">'
        + '    <strong>' + ruleName + '</strong>'
        + '    <span style="font-size:11px;color:var(--color-muted);margin-left:auto;">' + timestamp + '</span>'
        + '  </div>'
        + '  <div style="font-size:12px;color:var(--color-muted);">Trigger: ' + trigger + '</div>'
        + '  <div style="font-size:12px;">Action: <span style="color:' + resultColor + ';">' + action + '</span></div>'
        + '</div>'
        + '</div>';
    }).join('');
  }

  /* ─── Create Rule Modal ─── */
  function showCreateRuleModal() {
    var triggerOptions = triggerTypes.map(function(t){ return '<option value="' + t.value + '">' + t.label + '</option>'; }).join('');
    var actionOptions = actionTypes.map(function(a){ return '<option value="' + a.value + '">' + a.label + '</option>'; }).join('');

    if (global.Modal && global.Modal.open) {
      global.Modal.open({
        title: '➕ Create Automation Rule',
        body: '<div style="padding:var(--space-xs);">'
          + '<div class="form-group"><label class="form-label">Rule Name</label><input type="text" class="form-select" id="auto-new-name" placeholder="e.g. Auto-RCA on OOMKill"></div>'
          + '<div style="display:grid;grid-template-columns:1fr 1fr;gap:var(--space-sm);">'
          + '<div class="form-group"><label class="form-label">Trigger Type</label><select class="form-select" id="auto-new-trigger">' + triggerOptions + '</select></div>'
          + '<div class="form-group"><label class="form-label">Threshold</label><input type="number" class="form-select" id="auto-new-threshold" value="5"></div>'
          + '</div>'
          + '<div class="form-group"><label class="form-label">Action</label><select class="form-select" id="auto-new-action">' + actionOptions + '</select></div>'
          + '<div style="display:flex;gap:8px;margin-top:var(--space-sm);">'
          + '<button id="auto-create-submit-btn" class="btn btn-primary btn-sm" onclick="WorkflowAutomation.createRule()">Create Rule</button>'
          + '<button class="btn btn-ghost btn-sm" onclick="Modal.close()">Cancel</button>'
          + '</div>'
          + '</div>'
      });
    }
  }

  /* ─── Public API ─── */
  var WorkflowAutomation = {
    init: function () {
      UIComponents.initTabs('auto-tab-btn', 'auto-tab-panel', 'data-auto-tab');
      this.refresh();
    },
    refresh: function () {
      var self = this;
      
      loadingRules = true;
      loadingHistory = true;
      renderRulesTable();
      renderExecutionLog();

      APIClient.get('/automation/rules')
        .then(function (res) {
          loadingRules = false;
          if (res && res.data) {
            rules = res.data;
          } else {
            console.error('Failed to parse automation rules response.');
          }
          renderRulesTable();
        })
        .catch(function (err) {
          loadingRules = false;
          console.error('Error fetching automation rules:', err);
          renderRulesTable();
        });

      APIClient.get('/automation/executions')
        .then(function (res) {
          loadingHistory = false;
          if (res && res.data) {
            executionLog = res.data;
          } else {
            console.error('Failed to parse executions response.');
          }
          renderExecutionLog();
        })
        .catch(function (err) {
          loadingHistory = false;
          console.error('Error fetching executions:', err);
          renderExecutionLog();
        });
    },
    showCreateModal: function () {
      showCreateRuleModal();
    },
    createRule: function () {
      var nameEl = document.getElementById('auto-new-name');
      var triggerEl = document.getElementById('auto-new-trigger');
      var thresholdEl = document.getElementById('auto-new-threshold');
      var actionEl = document.getElementById('auto-new-action');
      var submitBtn = document.getElementById('auto-create-submit-btn');

      if (!nameEl || !nameEl.value.trim()) { alert('Please enter a rule name.'); return; }
      
      var thresholdVal = thresholdEl ? thresholdEl.value : '5';
      var triggerTypeVal = triggerEl ? triggerEl.value : 'pod_restart';
      var actionTypeVal = actionEl ? actionEl.value : 'generate_rca';

      if (submitBtn) {
        submitBtn.disabled = true;
        submitBtn.textContent = 'Creating...';
      }

      var payload = {
        name: nameEl.value.trim(),
        enabled: true,
        trigger_type: triggerTypeVal,
        trigger_config: {
          condition: triggerTypeVal.replace(/_/g, ' ') + ' > ' + thresholdVal,
          threshold: thresholdVal
        },
        action_type: actionTypeVal,
        action_config: {}
      };

      var self = this;
      APIClient.post('/automation/rules', payload)
        .then(function (res) {
          if (res) {
            if (global.Modal) global.Modal.close();
            self.refresh();
          } else {
            alert('Failed to create rule.');
            if (submitBtn) {
              submitBtn.disabled = false;
              submitBtn.textContent = 'Create Rule';
            }
          }
        })
        .catch(function (err) {
          alert('Error creating rule: ' + err.message);
          if (submitBtn) {
            submitBtn.disabled = false;
            submitBtn.textContent = 'Create Rule';
          }
        });
    },
    toggleRule: function (id, currentEnabled) {
      var self = this;
      var newEnabled = !currentEnabled;
      APIClient.put('/automation/rules/' + id + '/toggle', { enabled: newEnabled })
        .then(function (res) {
          if (res && res.status === 'ok') {
            self.refresh();
          } else {
            alert('Failed to toggle rule.');
          }
        })
        .catch(function (err) {
          alert('Error toggling rule: ' + err.message);
        });
    },
    deleteRule: function (id) {
      if (!confirm('Are you sure you want to delete this rule?')) return;
      var self = this;
      APIClient.delete('/automation/rules/' + id)
        .then(function (res) {
          if (res && res.status === 'deleted') {
            self.refresh();
          } else {
            alert('Failed to delete rule.');
          }
        })
        .catch(function (err) {
          alert('Error deleting rule: ' + err.message);
        });
    }
  };

  global.WorkflowAutomation = WorkflowAutomation;
})(window);
