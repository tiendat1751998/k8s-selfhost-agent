/**
 * Deployment Graph Mapping Visualizer
 */
(function (global) {
  'use strict';

  async function drawDeploymentGraph() {
    var imgName = document.getElementById('search-graph-image').value.trim();
    var canvas = document.getElementById('search-graph-canvas');
    if (!canvas) return;

    if (!imgName) {
      canvas.innerHTML = '<div class="empty-state"><div class="empty-state-icon">🕸️</div><div class="empty-state-text">Enter an image name to build mapping graph</div></div>';
      return;
    }

    canvas.innerHTML = '<div style="text-align:center;padding:20px;color:var(--color-muted);">Loading mapping data...</div>';

    var targets = [];
    try {
      var json = await APIClient.get('/explorer?kind=Deployment');
        var deployments = json.data || [];
        deployments.forEach(function(d) {
          var imageMatch = false;
          if (d.name && d.name.toLowerCase().indexOf(imgName.toLowerCase()) >= 0) imageMatch = true;
          if (imageMatch) {
            targets.push({ cluster: d.cluster || 'default', ns: d.namespace || 'default', workload: d.name, status: d.status === 'Ready' ? 'healthy' : 'degraded' });
          }
        });
      }
    } catch (e) {
      console.warn('Failed to fetch mapping data:', e);
    }

    if (targets.length === 0) {
      canvas.innerHTML = '<div class="empty-state"><div class="empty-state-icon">📭</div><div class="empty-state-text">No mapping data available for this image.</div></div>';
      return;
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
