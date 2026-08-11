/**
 * Git Trace Pipeline Visualization
 */
(function (global) {
  'use strict';

  function executeGitTrace() {
    var query = document.getElementById('search-git-query').value.trim().toLowerCase();
    var details = document.getElementById('search-git-trace-details');
    if (!details) return;

    if (!query) {
      details.innerHTML = '<div class="empty-state"><div class="empty-state-icon">🔀</div><div class="empty-state-text">Search for a commit or PR to trace deployment path</div></div>';
      return;
    }

    // Show empty state since no backend endpoint exists for tracing git commits yet
    details.innerHTML = '<div class="empty-state"><div class="empty-state-icon">📭</div><div class="empty-state-text">No trace data available for the given query.</div></div>';
  }

  function traceNode(title, sub, state) {
    var cls = state === 'healthy' ? 'badge-healthy' : 'badge-degraded';
    return '<div class="panel" style="flex-shrink:0;min-width:110px;text-align:center;padding:var(--space-sm);background:var(--color-surface-elevated);border-radius:var(--rounded-lg);">' +
      '<div style="font-size:12px;font-weight:600;">' + esc(title) + '</div>' +
      '<div class="badge ' + cls + '" style="font-size:10px;margin-top:4px;">' + esc(sub) + '</div>' +
    '</div>';
  }

  function traceArrow() {
    return '<div style="color:var(--color-primary);font-size:18px;font-weight:700;">➔</div>';
  }

  global.SearchGitTrace = {
    executeGitTrace: executeGitTrace
  };

})(window);
