/**
 * Deployment Graph Mapping Visualizer
 */
(function (global) {
  'use strict';

  function drawDeploymentGraph() {
    var imgName = document.getElementById('search-graph-image').value.trim();
    var canvas = document.getElementById('search-graph-canvas');
    if (!canvas) return;

    if (!imgName) {
      canvas.innerHTML = '<div class="empty-state"><div class="empty-state-icon">🕸️</div><div class="empty-state-text">Enter an image name to build mapping graph</div></div>';
      return;
    }

    var targets = [];
    if (imgName.toLowerCase().indexOf('nginx') >= 0) {
      targets = [
        { cluster: 'prod-us-east', ns: 'default', workload: 'nginx-prod', status: 'healthy' },
        { cluster: 'prod-eu-west', ns: 'default', workload: 'api-server', status: 'healthy' }
      ];
    } else if (imgName.toLowerCase().indexOf('auth') >= 0) {
      targets = [{ cluster: 'staging-1', ns: 'auth', workload: 'auth-svc-staging', status: 'degraded' }];
    } else if (imgName.toLowerCase().indexOf('payment') >= 0) {
      targets = [{ cluster: 'swarm-cluster', ns: 'manager', workload: 'payment-worker', status: 'down' }];
    } else {
      targets = [{ cluster: 'prod-us-east', ns: 'production', workload: imgName + '-deployment', status: 'healthy' }];
    }

    var nodesHtml = '';
    targets.forEach(function (t) {
      var cls = t.status === 'healthy' ? 'badge-healthy' : t.status === 'degraded' ? 'badge-degraded' : 'badge-down';
      nodesHtml +=
        '<div style="display:flex;flex-direction:column;align-items:center;">' +
          '<div style="width:2px;height:40px;background:var(--color-primary);"></div>' +
          '<div class="panel" style="padding:var(--space-sm);text-align:center;background:var(--color-surface-elevated);border:1px solid var(--color-hairline);border-radius:var(--rounded-lg);min-width:140px;">' +
            '<div style="font-size:12px;font-weight:600;">☸️ ' + esc(t.cluster) + '</div>' +
            '<div style="font-size:11px;color:var(--color-muted);margin:2px 0;">ns: ' + esc(t.ns) + '</div>' +
            '<div class="badge ' + cls + '" style="font-size:10px;">' + esc(t.workload) + '</div>' +
          '</div>' +
        '</div>';
    });

    canvas.innerHTML =
      '<div style="display:flex;flex-direction:column;align-items:center;">' +
        '<div class="panel" style="padding:var(--space-md);background:var(--color-surface);border:2px solid var(--color-primary);border-radius:var(--rounded-lg);z-index:10;">' +
          '<div style="font-size:13px;font-weight:600;">📦 Image: ' + esc(imgName) + '</div>' +
        '</div>' +
        '<div style="display:flex;gap:var(--space-lg);position:relative;">' +
          nodesHtml +
        '</div>' +
      '</div>';
  }

  global.SearchGraphVisualizer = {
    drawDeploymentGraph: drawDeploymentGraph
  };

})(window);
