/**
 * Deployment Timeline Module
 * Unified chronological view of deployments, rollbacks, incidents, and commits.
 */
(function (global) {
  'use strict';

  function generateTimelineEvents() {
    return [
      { id: 'ev-01', type: 'deployment', icon: '🚀', color: '#6366f1', title: 'payment-api v2.3.0', detail: 'Deployed to production, 3/3 pods ready', time: '12:15', date: '2026-06-25', namespace: 'production' },
      { id: 'ev-02', type: 'incident', icon: '🔴', color: '#ef4444', title: 'OOMKilled: ml-worker', detail: 'Memory limit exceeded in ml-jobs namespace', time: '11:30', date: '2026-06-25', namespace: 'ml-jobs' },
      { id: 'ev-03', type: 'rollback', icon: '⏪', color: '#f97316', title: 'Rollback: analytics-worker', detail: 'Rolled back from v1.8.0 to v1.7.2 due to crash loop', time: '10:45', date: '2026-06-25', namespace: 'staging' },
      { id: 'ev-04', type: 'commit', icon: '📝', color: '#10b981', title: 'feat: add retry logic', detail: 'PR #428 by @dev-lead merged to main', time: '09:20', date: '2026-06-25', namespace: '' },
      { id: 'ev-05', type: 'config', icon: '🔧', color: '#8b5cf6', title: 'ConfigMap updated: payment-config', detail: 'Added new retry timeout configuration', time: '08:00', date: '2026-06-25', namespace: 'production' },
      { id: 'ev-06', type: 'deployment', icon: '🚀', color: '#6366f1', title: 'order-service v3.1.0', detail: 'Blue-green deployment completed', time: '22:30', date: '2026-06-24', namespace: 'production' },
      { id: 'ev-07', type: 'incident', icon: '🔴', color: '#ef4444', title: 'SLO breach: order-service', detail: 'Error budget dropped below 10%', time: '18:15', date: '2026-06-24', namespace: 'production' },
      { id: 'ev-08', type: 'deployment', icon: '🚀', color: '#6366f1', title: 'user-service v1.5.0', detail: 'Canary deployment started', time: '14:00', date: '2026-06-24', namespace: 'staging' },
      { id: 'ev-09', type: 'commit', icon: '📝', color: '#10b981', title: 'fix: connection pool leak', detail: 'PR #425 by @backend-dev merged', time: '10:30', date: '2026-06-24', namespace: '' },
      { id: 'ev-10', type: 'config', icon: '🔧', color: '#8b5cf6', title: 'Secret rotated: db-credentials', detail: 'Automated rotation via vault-agent', time: '04:00', date: '2026-06-24', namespace: 'production' }
    ];
  }

  function renderTimeline(events) {
    var container = document.getElementById('timeline-container');
    if (!container) return;

    var grouped = {};
    events.forEach(function(ev) {
      if (!grouped[ev.date]) grouped[ev.date] = [];
      grouped[ev.date].push(ev);
    });

    var html = '';
    Object.keys(grouped).sort().reverse().forEach(function(date) {
      html += '<div style="margin-bottom:var(--space-md);">'
        + '<div style="font-size:12px;font-weight:600;color:var(--color-muted);margin-bottom:8px;padding:4px 0;border-bottom:1px dashed var(--color-hairline);">' + date + '</div>';
      grouped[date].forEach(function(ev) {
        html += '<div class="timeline-event" style="display:flex;align-items:flex-start;gap:12px;padding:8px 0;cursor:pointer;transition:background 0.1s;border-radius:6px;padding:8px;" '
          + 'onmouseover="this.style.background=\'var(--color-surface-elevated)\'" onmouseout="this.style.background=\'transparent\'" '
          + 'onclick="DeploymentTimeline.showDetail(\'' + ev.id + '\')">'
          + '<div style="width:32px;height:32px;border-radius:50%;background:' + ev.color + '20;display:flex;align-items:center;justify-content:center;font-size:16px;flex-shrink:0;">' + ev.icon + '</div>'
          + '<div style="flex:1;min-width:0;">'
          + '<div style="display:flex;align-items:center;gap:8px;">'
          + '<strong style="font-size:13px;">' + ev.title + '</strong>'
          + (ev.namespace ? '<span style="font-size:10px;background:var(--color-surface);border:1px solid var(--color-hairline);padding:1px 6px;border-radius:3px;">' + ev.namespace + '</span>' : '')
          + '<span style="font-size:11px;color:var(--color-muted);margin-left:auto;">' + ev.time + '</span>'
          + '</div>'
          + '<div style="font-size:12px;color:var(--color-muted);margin-top:2px;">' + ev.detail + '</div>'
          + '</div></div>';
      });
      html += '</div>';
    });
    container.innerHTML = html;
  }

  global.DeploymentTimeline = {
    init: function() { this.refresh(); },
    refresh: function() { renderTimeline(generateTimelineEvents()); },
    showDetail: function(id) {
      var ev = generateTimelineEvents().find(function(e){ return e.id === id; });
      if (!ev) return;
      var detail = document.getElementById('timeline-detail');
      if (detail) {
        detail.innerHTML = '<div style="display:flex;align-items:center;gap:8px;margin-bottom:8px;">'
          + '<span style="font-size:24px;">' + ev.icon + '</span>'
          + '<div><strong style="font-size:14px;">' + ev.title + '</strong><br><span style="font-size:12px;color:var(--color-muted);">' + ev.date + ' ' + ev.time + ' · ' + ev.type + '</span></div></div>'
          + '<p style="font-size:13px;margin:0;">' + ev.detail + '</p>'
          + (ev.namespace ? '<span style="font-size:11px;color:var(--color-muted);">Namespace: ' + ev.namespace + '</span>' : '');
      }
    }
  };
})(window);
