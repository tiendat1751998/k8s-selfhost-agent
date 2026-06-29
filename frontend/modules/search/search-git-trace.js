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

    var hash = 'c7f82b1';
    var desc = 'PR #142 (Merged): increase memory limit to 512Mi';
    var cluster = 'prod-us-east';
    var author = 'admin';

    if (query.indexOf('token') >= 0 || query.indexOf('auth') >= 0 || query.indexOf('140') >= 0) {
      hash = 'a3d2e14';
      desc = 'PR #140 (Merged): fix authentication token expiration';
      cluster = 'prod-eu-west';
      author = 'dev-team';
    } else if (query.indexOf('ci') >= 0 || query.indexOf('138') >= 0) {
      hash = 'b8f921d';
      desc = 'PR #138 (Merged): setup initial CI/CD pipeline';
      cluster = 'prod-us-east';
      author = 'sre-team';
    } else if (query.indexOf('probe') >= 0 || query.indexOf('liveness') >= 0 || query.indexOf('145') >= 0) {
      hash = 'd9c3e41';
      desc = 'PR #145 (Open): update liveness probes in deployment config';
      cluster = 'staging-1';
      author = 'oncall';
    }

    details.innerHTML =
      '<div style="background:var(--color-surface-card);border:1px solid var(--color-hairline);padding:var(--space-md);border-radius:var(--rounded-lg);">' +
        '<h4>Trace pipeline: ' + esc(hash) + '</h4>' +
        '<p style="font-size:13px;color:var(--color-muted);margin-bottom:var(--space-md);">' + esc(desc) + ' | Author: <strong>' + esc(author) + '</strong></p>' +
        '<div style="display:flex;align-items:center;gap:var(--space-sm);overflow-x:auto;padding-bottom:var(--space-sm);">' +
          traceNode('💻 Commit', hash, 'healthy') +
          traceArrow() +
          traceNode('🔀 PR Approved', 'Auto-Merged', 'healthy') +
          traceArrow() +
          traceNode('📦 Image Push', 'registry:latest', 'healthy') +
          traceArrow() +
          traceNode('🚀 ArgoCD Sync', 'Synced', 'healthy') +
          traceArrow() +
          traceNode('☸️ Cluster', cluster, 'healthy') +
        '</div>' +
      '</div>';
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
