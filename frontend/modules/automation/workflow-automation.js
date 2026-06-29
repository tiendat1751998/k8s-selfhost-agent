/**
 * Workflow Automation Engine Module
 * Event-driven rules builder with triggers, actions, and execution history.
 */
(function (global) {
  'use strict';

  var rules = [
    { id: 'rule-001', name: 'Auto-RCA on Restart Loop', enabled: true, trigger: { type: 'pod_restart', condition: 'count > 5', threshold: 5 }, action: { type: 'generate_rca' }, lastTriggered: '2h ago', executions: 12 },
    { id: 'rule-002', name: 'Notify SRE on Node Pressure', enabled: true, trigger: { type: 'node_pressure', condition: 'memory > 90%', threshold: 90 }, action: { type: 'send_notification', channel: 'Slack #sre-alerts' }, lastTriggered: '6h ago', executions: 5 },
    { id: 'rule-003', name: 'Auto-Rollback on Failure', enabled: false, trigger: { type: 'deployment_failure', condition: 'health_check_failed', threshold: 0 }, action: { type: 'rollback' }, lastTriggered: '3d ago', executions: 2 },
    { id: 'rule-004', name: 'Scale on High CPU', enabled: true, trigger: { type: 'high_cpu', condition: 'cpu > 85%', threshold: 85 }, action: { type: 'scale_deployment', replicas: '+2' }, lastTriggered: '1h ago', executions: 8 },
    { id: 'rule-005', name: 'Create Incident on SLO Breach', enabled: true, trigger: { type: 'slo_breach', condition: 'budget < 10%', threshold: 10 }, action: { type: 'create_incident', severity: 'critical' }, lastTriggered: '12h ago', executions: 3 }
  ];

  var executionLog = [
    { ruleId: 'rule-001', ruleName: 'Auto-RCA on Restart Loop', trigger: 'payment-api pod restarted 6 times', action: 'Generated RCA report', result: 'success', timestamp: '2026-06-25 12:15:00' },
    { ruleId: 'rule-004', ruleName: 'Scale on High CPU', trigger: 'order-service CPU at 91%', action: 'Scaled from 3 to 5 replicas', result: 'success', timestamp: '2026-06-25 11:30:00' },
    { ruleId: 'rule-002', ruleName: 'Notify SRE on Node Pressure', trigger: 'worker-node-03 memory at 94%', action: 'Sent Slack notification to #sre-alerts', result: 'success', timestamp: '2026-06-25 08:45:00' },
    { ruleId: 'rule-001', ruleName: 'Auto-RCA on Restart Loop', trigger: 'cache-warmer pod restarted 8 times', action: 'Generated RCA report', result: 'success', timestamp: '2026-06-25 06:20:00' },
    { ruleId: 'rule-005', ruleName: 'Create Incident on SLO Breach', trigger: 'payment-api SLO budget at 8%', action: 'Created critical incident INC-2847', result: 'success', timestamp: '2026-06-24 22:10:00' },
    { ruleId: 'rule-003', ruleName: 'Auto-Rollback on Failure', trigger: 'analytics-worker health check failed', action: 'Rolled back to revision 4', result: 'success', timestamp: '2026-06-22 14:30:00' }
  ];

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

    tbody.innerHTML = rules.map(function (r) {
      var statusHtml = r.enabled
        ? '<span style="color:#10b981;font-weight:600;">● Active</span>'
        : '<span style="color:#6b7280;">○ Disabled</span>';
      return '<tr>'
        + '<td><strong>' + r.name + '</strong></td>'
        + '<td style="font-size:12px;">' + r.trigger.type.replace(/_/g, ' ') + '</td>'
        + '<td style="font-size:12px;color:var(--color-muted);">' + r.trigger.condition + '</td>'
        + '<td style="font-size:12px;">' + r.action.type.replace(/_/g, ' ') + '</td>'
        + '<td>' + statusHtml + '</td>'
        + '<td style="font-family:var(--font-number);font-size:12px;">' + r.executions + '</td>'
        + '<td style="font-size:12px;color:var(--color-muted);">' + r.lastTriggered + '</td>'
        + '<td>'
        + '  <button class="btn btn-ghost btn-sm" onclick="WorkflowAutomation.toggleRule(\'' + r.id + '\')">' + (r.enabled ? 'Disable' : 'Enable') + '</button>'
        + '  <button class="btn btn-ghost btn-sm" style="color:#ef4444;" onclick="WorkflowAutomation.deleteRule(\'' + r.id + '\')">✕</button>'
        + '</td>'
        + '</tr>';
    }).join('');
  }

  /* ─── Render Execution Log ─── */
  function renderExecutionLog() {
    var container = document.getElementById('auto-exec-log');
    if (!container) return;

    container.innerHTML = executionLog.map(function (e) {
      var resultColor = e.result === 'success' ? '#10b981' : '#ef4444';
      var resultIcon = e.result === 'success' ? '✅' : '❌';
      return '<div class="auto-exec-item">'
        + '<div class="auto-exec-icon">' + resultIcon + '</div>'
        + '<div class="auto-exec-body">'
        + '  <div class="auto-exec-header">'
        + '    <strong>' + e.ruleName + '</strong>'
        + '    <span style="font-size:11px;color:var(--color-muted);margin-left:auto;">' + e.timestamp + '</span>'
        + '  </div>'
        + '  <div style="font-size:12px;color:var(--color-muted);">Trigger: ' + e.trigger + '</div>'
        + '  <div style="font-size:12px;">Action: <span style="color:' + resultColor + ';">' + e.action + '</span></div>'
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
          + '<button class="btn btn-primary btn-sm" onclick="WorkflowAutomation.createRule()">Create Rule</button>'
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
      renderRulesTable();
      renderExecutionLog();
    },
    showCreateModal: function () {
      showCreateRuleModal();
    },
    createRule: function () {
      var name = document.getElementById('auto-new-name');
      var trigger = document.getElementById('auto-new-trigger');
      var threshold = document.getElementById('auto-new-threshold');
      var action = document.getElementById('auto-new-action');
      if (!name || !name.value.trim()) { alert('Please enter a rule name.'); return; }
      rules.push({
        id: 'rule-' + Date.now(),
        name: name.value.trim(),
        enabled: true,
        trigger: { type: trigger.value, condition: trigger.value.replace(/_/g, ' ') + ' > ' + threshold.value, threshold: parseInt(threshold.value) },
        action: { type: action.value },
        lastTriggered: 'Never',
        executions: 0
      });
      if (global.Modal) global.Modal.close();
      this.refresh();
    },
    toggleRule: function (id) {
      rules.forEach(function (r) { if (r.id === id) r.enabled = !r.enabled; });
      this.refresh();
    },
    deleteRule: function (id) {
      rules = rules.filter(function (r) { return r.id !== id; });
      this.refresh();
    }
  };

  global.WorkflowAutomation = WorkflowAutomation;
})(window);
