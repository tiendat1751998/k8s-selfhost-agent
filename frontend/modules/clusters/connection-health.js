/**
 * Connection Health — Real-time health dashboard with heartbeat animations.
 */
(function (global) {
  'use strict';

  var systems = ['k8s', 'git', 'cicd', 'ai'];

  function init() {
    AppState.on('connectionHealth', render);
    AppState.on('navigate', function (s) { if (s === 'connection-health') render(AppState.getState().connectionHealth); });
  }

  function render(health) {
    if (!health) return;

    systems.forEach(function (sys) {
      var data = health[sys];
      if (!data) return;

      var dot = document.getElementById('health-' + sys + '-dot');
      var text = document.getElementById('health-' + sys + '-text');
      var latency = document.getElementById('health-' + sys + '-latency');
      var last = document.getElementById('health-' + sys + '-last');

      if (dot) {
        dot.className = 'health-dot ' + (data.status || 'pending');
      }
      if (text) {
        text.textContent = capitalize(data.status || 'checking');
        text.style.color = statusColor(data.status);
      }
      if (latency) latency.textContent = data.latency || '—';
      if (last) last.textContent = timeAgo(data.lastCheck);
    });

    // Calculate overall health
    var healthyCount = 0;
    var totalCount = 0;
    systems.forEach(function (sys) {
      if (health[sys]) {
        totalCount++;
        if (health[sys].status === 'healthy') healthyCount++;
      }
    });

    var score = totalCount > 0 ? Math.round((healthyCount / totalCount) * 100) : 0;
    var overallEl = document.getElementById('health-overall');
    var badgeEl = document.getElementById('health-score-badge');
    if (overallEl) overallEl.textContent = 'System Health: ' + score + '%';
    if (badgeEl) {
      badgeEl.textContent = 'Health: ' + score + '%';
      badgeEl.style.background = score >= 75 ? 'rgba(14,203,129,0.15)' : score >= 50 ? 'rgba(252,213,53,0.15)' : 'rgba(246,70,93,0.15)';
      badgeEl.style.color = score >= 75 ? 'var(--color-trading-up)' : score >= 50 ? 'var(--color-primary)' : 'var(--color-trading-down)';
    }
  }

  function statusColor(status) {
    switch (status) {
      case 'healthy': return 'var(--color-trading-up)';
      case 'degraded': return 'var(--color-primary)';
      case 'down': return 'var(--color-trading-down)';
      default: return 'var(--color-muted)';
    }
  }

  function capitalize(s) {
    if (!s) return '—';
    return s.charAt(0).toUpperCase() + s.slice(1);
  }

  global.ConnectionHealthSection = { init: init };
})(window);
