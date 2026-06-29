/**
 * Compliance & Security Posture Dashboard
 * Compliance scoring, policy violations, security posture, and secret management.
 */
(function (global) {
  'use strict';

  var frameworks = [
    { name: 'CIS Kubernetes Benchmark', score: 82, total: 120, passed: 98, failed: 22, icon: '🛡️' },
    { name: 'SOC 2 Type II', score: 91, total: 45, passed: 41, failed: 4, icon: '📋' },
    { name: 'PCI-DSS v4.0', score: 76, total: 80, passed: 61, failed: 19, icon: '💳' },
    { name: 'HIPAA', score: 88, total: 35, passed: 31, failed: 4, icon: '🏥' }
  ];

  var violations = [
    { severity: 'critical', policy: 'Privileged Container Detected', resource: 'pod/debug-tools-9x1c', namespace: 'staging', cluster: 'staging-cluster', detected: '2h ago' },
    { severity: 'high', policy: 'No Network Policy', resource: 'ns/dev-apps', namespace: 'dev-apps', cluster: 'staging-cluster', detected: '6h ago' },
    { severity: 'high', policy: 'Image Not Scanned', resource: 'deploy/legacy-api', namespace: 'production', cluster: 'prod-cluster-01', detected: '1d ago' },
    { severity: 'medium', policy: 'No Resource Limits', resource: 'deploy/batch-worker', namespace: 'staging', cluster: 'staging-cluster', detected: '2d ago' },
    { severity: 'medium', policy: 'Root User Container', resource: 'pod/migration-job-4c5d', namespace: 'production', cluster: 'prod-cluster-01', detected: '3d ago' },
    { severity: 'low', policy: 'Missing Labels', resource: 'deploy/test-runner', namespace: 'qa-testing', cluster: 'staging-cluster', detected: '5d ago' }
  ];

  var securityPosture = [
    { metric: 'Container Image Vulnerabilities', value: '14 Critical, 38 High', status: 'warning' },
    { metric: 'Privileged Containers', value: '2 detected', status: 'critical' },
    { metric: 'Network Policy Coverage', value: '78%', status: 'warning' },
    { metric: 'Pod Security Standards', value: '92% compliant', status: 'healthy' },
    { metric: 'RBAC Over-Permissions', value: '3 roles flagged', status: 'warning' },
    { metric: 'Secret Rotation Status', value: '5 expired, 2 expiring', status: 'critical' }
  ];

  function renderComplianceScore() {
    var container = document.getElementById('comp-score-grid');
    if (!container) return;
    var overall = Math.round(frameworks.reduce(function(s,f){ return s+f.score; }, 0) / frameworks.length);
    var overallColor = overall >= 90 ? '#10b981' : (overall >= 75 ? '#eab308' : '#ef4444');

    container.innerHTML = '<div class="panel" style="padding:var(--space-lg);text-align:center;grid-column:span 2;">'
      + '<div style="font-size:48px;font-weight:700;color:' + overallColor + ';font-family:var(--font-number);">' + overall + '%</div>'
      + '<div style="font-size:14px;color:var(--color-muted);margin-top:4px;">Overall Compliance Score</div>'
      + '</div>'
      + frameworks.map(function(f) {
        var col = f.score >= 90 ? '#10b981' : (f.score >= 75 ? '#eab308' : '#ef4444');
        return '<div class="panel" style="padding:var(--space-md);">'
          + '<div style="display:flex;align-items:center;gap:8px;margin-bottom:var(--space-sm);">'
          + '<span style="font-size:20px;">' + f.icon + '</span>'
          + '<strong style="font-size:13px;">' + f.name + '</strong>'
          + '</div>'
          + '<div style="font-size:28px;font-weight:700;color:' + col + ';font-family:var(--font-number);">' + f.score + '%</div>'
          + '<div style="font-size:11px;color:var(--color-muted);">' + f.passed + '/' + f.total + ' checks passed · ' + f.failed + ' failed</div>'
          + '<div style="margin-top:8px;height:4px;background:var(--color-surface-elevated);border-radius:2px;overflow:hidden;">'
          + '<div style="height:100%;width:' + f.score + '%;background:' + col + ';border-radius:2px;"></div>'
          + '</div></div>';
      }).join('');
  }

  function renderViolations() {
    var tbody = document.getElementById('comp-violations-tbody');
    if (!tbody) return;
    var sevColors = { critical: '#ef4444', high: '#f97316', medium: '#eab308', low: '#6b7280' };
    tbody.innerHTML = violations.map(function(v) {
      return '<tr><td><span style="background:' + sevColors[v.severity] + ';color:#fff;font-size:10px;padding:2px 8px;border-radius:4px;font-weight:600;">' + v.severity.toUpperCase() + '</span></td>'
        + '<td><strong style="font-size:12px;">' + v.policy + '</strong></td>'
        + '<td><code style="font-size:11px;">' + v.resource + '</code></td>'
        + '<td style="font-size:12px;">' + v.namespace + '</td>'
        + '<td style="font-size:12px;color:var(--color-muted);">' + v.detected + '</td></tr>';
    }).join('');
  }

  function renderSecurityPosture() {
    var container = document.getElementById('comp-posture-grid');
    if (!container) return;
    var statusColors = { healthy: '#10b981', warning: '#eab308', critical: '#ef4444' };
    container.innerHTML = securityPosture.map(function(s) {
      var col = statusColors[s.status];
      return '<div class="panel" style="padding:var(--space-sm);border-left:3px solid ' + col + ';">'
        + '<div style="font-size:12px;color:var(--color-muted);">' + s.metric + '</div>'
        + '<div style="font-size:14px;font-weight:600;color:' + col + ';margin-top:4px;">' + s.value + '</div>'
        + '</div>';
    }).join('');
  }

  global.ComplianceModule = {
    init: function() {
      UIComponents.initTabs('comp-tab-btn', 'comp-tab-panel', 'data-comp-tab');
      var scoreGrid = document.getElementById('comp-score-grid');
      if (scoreGrid) {
        scoreGrid.innerHTML = '<div class="skeleton" style="height:200px;grid-column:span 4;border-radius:var(--rounded-lg);"></div>';
      }
      var postureGrid = document.getElementById('comp-posture-grid');
      if (postureGrid) {
        postureGrid.innerHTML = '<div class="skeleton" style="height:150px;grid-column:span 3;border-radius:var(--rounded-lg);"></div>';
      }
      var tbody = document.getElementById('comp-violations-tbody');
      if (tbody) {
        tbody.innerHTML = '<tr><td colspan="5"><div class="skeleton" style="height:120px;border-radius:var(--rounded-lg);"></div></td></tr>';
      }
      var self = this;
      setTimeout(function() {
        self.refresh();
      }, 400);
    },
    refresh: function() { renderComplianceScore(); renderViolations(); renderSecurityPosture(); }
  };
})(window);
