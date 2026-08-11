/**
 * Compliance & Security Posture Dashboard
 * Compliance scoring, policy violations, security posture, and secret management.
 */
(function (global) {
  'use strict';

  var frameworks = [];
  var violations = [];

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
    if (frameworks.length === 0) {
      container.innerHTML = '<div class="panel" style="grid-column:span 4;text-align:center;padding:var(--space-md);color:var(--color-muted);">No compliance frameworks found.</div>';
      return;
    }
    var overall = Math.round(frameworks.reduce(function(s,f){ return s+f.score; }, 0) / frameworks.length);
    var overallColor = overall >= 90 ? '#10b981' : (overall >= 75 ? '#eab308' : '#ef4444');

    container.innerHTML = '<div class="panel" style="padding:var(--space-lg);text-align:center;grid-column:span 2;">'
      + '<div style="font-size:48px;font-weight:700;color:' + overallColor + ';font-family:var(--font-number);">' + overall + '%</div>'
      + '<div style="font-size:14px;color:var(--color-muted);margin-top:4px;">Overall Compliance Score</div>'
      + '</div>'
      + frameworks.map(function(f) {
        var col = f.score >= 90 ? '#10b981' : (f.score >= 75 ? '#eab308' : '#ef4444');
        var passed = f.passed_checks !== undefined ? f.passed_checks : (f.passed || 0);
        var total = f.total_checks !== undefined ? f.total_checks : (f.total || 0);
        var failed = f.failed_checks !== undefined ? f.failed_checks : (f.failed || 0);
        var name = Security.escapeHTML(f.name);
        var icon = Security.escapeHTML(f.icon);
        return '<div class="panel" style="padding:var(--space-md);">'
          + '<div style="display:flex;align-items:center;gap:8px;margin-bottom:var(--space-sm);">'
          + '<span style="font-size:20px;">' + icon + '</span>'
          + '<strong style="font-size:13px;">' + name + '</strong>'
          + '</div>'
          + '<div style="font-size:28px;font-weight:700;color:' + col + ';font-family:var(--font-number);">' + f.score + '%</div>'
          + '<div style="font-size:11px;color:var(--color-muted);">' + passed + '/' + total + ' checks passed · ' + failed + ' failed</div>'
          + '<div style="margin-top:8px;height:4px;background:var(--color-surface-elevated);border-radius:2px;overflow:hidden;">'
          + '<div style="height:100%;width:' + f.score + '%;background:' + col + ';border-radius:2px;"></div>'
          + '</div></div>';
      }).join('');
  }

  function renderViolations() {
    var tbody = document.getElementById('comp-violations-tbody');
    if (!tbody) return;
    if (violations.length === 0) {
      tbody.innerHTML = '<tr><td colspan="5" class="text-center text-muted">No policy violations detected.</td></tr>';
      return;
    }
    var sevColors = { critical: '#ef4444', high: '#f97316', medium: '#eab308', low: '#6b7280' };
    tbody.innerHTML = violations.map(function(v) {
      var severity = Security.escapeHTML(v.severity);
      var policy = Security.escapeHTML(v.policy);
      var resource = Security.escapeHTML(v.resource);
      var namespace = Security.escapeHTML(v.namespace);
      var detected = UIComponents.timeAgo(v.detected_at) || Security.escapeHTML(v.detected);
      var col = sevColors[v.severity] || '#6b7280';
      return '<tr><td><span style="background:' + col + ';color:#fff;font-size:10px;padding:2px 8px;border-radius:4px;font-weight:600;">' + severity.toUpperCase() + '</span></td>'
        + '<td><strong style="font-size:12px;">' + policy + '</strong></td>'
        + '<td><code style="font-size:11px;">' + resource + '</code></td>'
        + '<td style="font-size:12px;">' + namespace + '</td>'
        + '<td style="font-size:12px;color:var(--color-muted);">' + detected + '</td></tr>';
    }).join('');
  }

  function renderSecurityPosture() {
    var container = document.getElementById('comp-posture-grid');
    if (!container) return;
    var statusColors = { healthy: '#10b981', warning: '#eab308', critical: '#ef4444' };
    container.innerHTML = securityPosture.map(function(s) {
      var col = statusColors[s.status] || '#eab308';
      var metric = Security.escapeHTML(s.metric);
      var value = Security.escapeHTML(s.value);
      return '<div class="panel" style="padding:var(--space-sm);border-left:3px solid ' + col + ';">'
        + '<div style="font-size:12px;color:var(--color-muted);">' + metric + '</div>'
        + '<div style="font-size:14px;font-weight:600;color:' + col + ';margin-top:4px;">' + value + '</div>'
        + '</div>';
    }).join('');
  }

  global.ComplianceModule = {
    init: function() {
      UIComponents.initTabs('comp-tab-btn', 'comp-tab-panel', 'data-comp-tab');
      this.refresh();
    },
    refresh: async function() {
      var scoreGrid = document.getElementById('comp-score-grid');
      var tbody = document.getElementById('comp-violations-tbody');
      
      // Show loading skeletons
      if (scoreGrid) {
        scoreGrid.innerHTML = '<div class="skeleton" style="height:200px;grid-column:span 4;border-radius:var(--rounded-lg);"></div>';
      }
      if (tbody) {
        tbody.innerHTML = '<tr><td colspan="5"><div class="skeleton" style="height:120px;border-radius:var(--rounded-lg);"></div></td></tr>';
      }
      renderSecurityPosture();

      try {
        const [frameworksRes, violationsRes] = await Promise.all([
          APIClient.get('/compliance/frameworks'),
          APIClient.get('/compliance/violations')
        ]);

        if (frameworksRes && frameworksRes.data) {
          frameworks = frameworksRes.data;
        } else {
          throw new Error('Invalid frameworks data format');
        }

        if (violationsRes && violationsRes.data) {
          violations = violationsRes.data;
        } else {
          throw new Error('Invalid violations data format');
        }

        renderComplianceScore();
        renderViolations();
      } catch (err) {
        console.error('[Compliance] Refresh failed:', err);
        if (scoreGrid) {
          scoreGrid.innerHTML = '<div class="panel" style="grid-column:span 4;padding:var(--space-md);color:var(--color-danger);border:1px solid rgba(239,68,68,0.2);background:rgba(239,68,68,0.1);border-radius:var(--rounded-lg);"><strong>Error:</strong> Failed to fetch compliance posture. Please refresh or try again.</div>';
        }
        if (tbody) {
          tbody.innerHTML = '<tr><td colspan="5" style="text-align:center;color:var(--color-danger);padding:var(--space-md);">Failed to load policy violations.</td></tr>';
        }
      }
    }
  };
})(window);
