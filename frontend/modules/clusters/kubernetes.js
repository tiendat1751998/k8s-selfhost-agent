/**
 * Container Clusters — Enterprise table supporting Kubernetes, Docker, Docker Swarm.
 */
(function (global) {
  'use strict';

  var tableBody = document.getElementById('k8s-table-body');
  var emptyEl = document.getElementById('k8s-empty');
  var addBtn = document.getElementById('add-cluster-btn');

  var providerIcons = {
    'kubernetes': '☸️',
    'docker': '🐳',
    'docker_swarm': '🐝'
  };

  var providerLabels = {
    'kubernetes': 'Kubernetes',
    'docker': 'Docker',
    'docker_swarm': 'Docker Swarm'
  };

  var initialized = false;
  function init() {
    if (initialized) return;
    initialized = true;
    if (addBtn) addBtn.addEventListener('click', showAddModal);
    AppState.on('kubernetes', render);
    AppState.on('navigate', function (section) {
      if (section === 'kubernetes') loadClusters();
    });
  }

  function loadClusters() {
    var state = AppState.getState();
    render(state.kubernetes);
  }

  function render(clusters) {
    if (!tableBody) return;
    clusters = clusters || [];
    tableBody.innerHTML = '';

    if (clusters.length === 0) {
      if (emptyEl) emptyEl.style.display = '';
      tableBody.closest('.enterprise-table-wrap').style.display = 'none';
      return;
    }

    if (emptyEl) emptyEl.style.display = 'none';
    tableBody.closest('.enterprise-table-wrap').style.display = '';

    clusters.forEach(function (cluster) {
      var provider = cluster.provider || 'kubernetes';
      var icon = providerIcons[provider] || '📦';
      var label = providerLabels[provider] || provider;

      var favs = [];
      try {
        var rawFavs = localStorage.getItem('k8s_favorites_clusters');
        if (rawFavs) favs = JSON.parse(rawFavs);
      } catch (e) {}
      var isFav = favs.indexOf(cluster.name) >= 0;
      var starIcon = isFav ? '★' : '☆';
      var starColor = isFav ? 'var(--color-primary)' : 'var(--color-muted)';

      var tr = document.createElement('tr');
      tr.innerHTML =
        '<td><span class="cluster-fav-star" style="cursor:pointer;margin-right:8px;color:' + starColor + ';" data-id="' + esc(cluster.name) + '">' + starIcon + '</span><strong>' + esc(cluster.name) + '</strong></td>' +
        '<td>' + icon + ' <span class="badge badge-' + providerBadgeClass(provider) + '">' + esc(label) + '</span></td>' +
        '<td><code style="font-size:12px;color:var(--color-muted)">' + esc(cluster.endpoint || cluster.socketPath || '') + '</code></td>' +
        '<td>' + statusBadge(cluster.status) + '</td>' +
        '<td style="font-family:var(--font-number);font-size:12px;color:var(--color-muted)">' + timeAgo(cluster.lastCheck) + '</td>' +
        '<td style="font-family:var(--font-number)">' + (cluster.nodes || 0) + '</td>' +
        '<td style="font-family:var(--font-number)">' + (cluster.pods || cluster.containers || 0) + '</td>' +
        '<td style="font-family:var(--font-number)">' + (cluster.services || 0) + '</td>' +
        '<td><div class="action-group">' +
          '<button class="action-btn" data-action="detail" data-id="' + esc(cluster.name) + '">Details</button>' +
          '<button class="action-btn" data-action="test" data-id="' + esc(cluster.name) + '">Test</button>' +
          '<button class="action-btn danger" data-action="remove" data-id="' + esc(cluster.name) + '">Remove</button>' +
        '</div></td>';

      var starBtn = tr.querySelector('.cluster-fav-star');
      if (starBtn) {
        starBtn.addEventListener('click', function (e) {
          e.stopPropagation();
          toggleFavoriteCluster(this.dataset.id);
        });
      }

      tr.querySelectorAll('.action-btn').forEach(function (btn) {
        btn.addEventListener('click', function () { handleAction(this.dataset.action, this.dataset.id, cluster); });
      });

      tableBody.appendChild(tr);
    });
  }

  function toggleFavoriteCluster(name) {
    var favs = [];
    try {
      var rawFavs = localStorage.getItem('k8s_favorites_clusters');
      if (rawFavs) favs = JSON.parse(rawFavs);
    } catch (e) {}

    var idx = favs.indexOf(name);
    if (idx >= 0) {
      favs.splice(idx, 1);
    } else {
      favs.push(name);
    }
    localStorage.setItem('k8s_favorites_clusters', JSON.stringify(favs));

    // Force re-render of clusters table
    loadClusters();

    // Trigger update of overview favorites panel if it exists
    if (window.PersonalizationModule && typeof window.PersonalizationModule.updateFavorites === 'function') {
      window.PersonalizationModule.updateFavorites();
    }
  }

  async function handleAction(action, id, cluster) {
    switch (action) {
      case 'detail': showDetailModal(cluster); break;
      case 'test': testConnection(id); break;
      case 'remove':
        if (confirm('Remove cluster "' + id + '"?')) {
          try {
            var res = await fetch('/api/v1/fleet/' + id, { method: 'DELETE' });
            if (!res.ok) throw new Error('Failed to delete');
            var listRes = await fetch('/api/v1/fleet');
            if (listRes.ok) {
              var data = await listRes.json();
              if (data && data.data) AppState.setKubernetes(data.data);
            }
          } catch (e) {
            alert('Error deleting cluster: ' + e.message);
          }
        }
        break;
    }
  }

  async function testConnection(name) {
    try {
      var res = await fetch('/api/v1/fleet/' + name + '/test', { method: 'POST' });
      if (!res.ok) throw new Error('Connection failed');
      alert('Connection to "' + name + '" is healthy ✅');
    } catch (e) {
      alert('Connection test failed: ' + e.message);
    }
  }

  function showDetailModal(cluster) {
    var provider = cluster.provider || 'kubernetes';
    var icon = providerIcons[provider] || '📦';
    var label = providerLabels[provider] || provider;

    var endpointLabel = provider === 'docker' ? 'Docker Socket' : provider === 'docker_swarm' ? 'Swarm Manager' : 'API Endpoint';
    var endpointValue = cluster.endpoint || cluster.socketPath || '—';

    var extraRows = '';
    if (provider === 'docker' || provider === 'docker_swarm') {
      extraRows +=
        '<div class="pipeline-detail-row"><span class="pipeline-detail-label">Docker Version</span><span class="pipeline-detail-value">' + esc(cluster.dockerVersion || '—') + '</span></div>';
      if (provider === 'docker_swarm') {
        extraRows +=
          '<div class="pipeline-detail-row"><span class="pipeline-detail-label">Swarm Role</span><span class="pipeline-detail-value">' + (cluster.swarmManager ? 'Manager' : 'Worker') + '</span></div>' +
          '<div class="pipeline-detail-row"><span class="pipeline-detail-label">Services</span><span class="pipeline-detail-value">' + (cluster.services || 0) + '</span></div>';
      }
      extraRows +=
        '<div class="pipeline-detail-row"><span class="pipeline-detail-label">Containers</span><span class="pipeline-detail-value">' + (cluster.containers || cluster.pods || 0) + '</span></div>';
    }

    Modal.open({
      title: icon + ' ' + cluster.name + ' (' + label + ')',
      body:
        '<div style="display:grid;gap:var(--space-md)">' +
          '<div class="pipeline-detail">' +
            '<div class="pipeline-detail-row"><span class="pipeline-detail-label">Provider</span><span class="pipeline-detail-value">' + icon + ' ' + esc(label) + '</span></div>' +
            '<div class="pipeline-detail-row"><span class="pipeline-detail-label">' + endpointLabel + '</span><span class="pipeline-detail-value" style="font-size:12px">' + esc(endpointValue) + '</span></div>' +
            '<div class="pipeline-detail-row"><span class="pipeline-detail-label">Status</span><span class="pipeline-detail-value">' + statusBadge(cluster.status) + '</span></div>' +
            '<div class="pipeline-detail-row"><span class="pipeline-detail-label">Nodes</span><span class="pipeline-detail-value">' + (cluster.nodes || 0) + '</span></div>' +
            '<div class="pipeline-detail-row"><span class="pipeline-detail-label">CPU Usage</span><span class="pipeline-detail-value">' + (cluster.cpu || '—') + '</span></div>' +
            '<div class="pipeline-detail-row"><span class="pipeline-detail-label">Memory Usage</span><span class="pipeline-detail-value">' + (cluster.memory || '—') + '</span></div>' +
            extraRows +
          '</div>' +
          '<div>' +
            '<div class="form-label">Auth Config (masked)</div>' +
            '<pre class="ai-test-response"><span class="secret-value">••••••••••••••••••••••</span> <button class="secret-reveal-btn" onclick="this.previousElementSibling.textContent=\'' + (provider === 'docker' ? 'socket: ' + esc(endpointValue) : 'server: ' + esc(endpointValue)) + '\\ncertificate-authority-data: [REDACTED]\';this.remove()">Reveal</button></pre>' +
          '</div>' +
        '</div>',
      actions: [
        { label: 'Test Connection', onClick: function () { testConnection(cluster.name); }, closeOnClick: false },
        { label: 'Close', primary: true }
      ]
    });
  }

  function showAddModal() {
    Modal.open({
      title: '+ Add Container Cluster',
      body:
        '<div class="form-group"><label class="form-label">Provider</label><select class="form-select" id="new-cluster-provider"><option value="kubernetes">☸️ Kubernetes</option><option value="docker">🐳 Docker</option><option value="docker_swarm">🐝 Docker Swarm</option></select></div>' +
        '<div class="form-group"><label class="form-label">Cluster Name</label><input class="form-select" id="new-k8s-name" placeholder="prod-us-east"></div>' +
        '<div class="form-group"><label class="form-label">Group</label><select class="form-select" id="new-k8s-group"><option value="production">Production</option><option value="staging">Staging</option><option value="development">Development</option></select></div>' +
        '<div class="form-group"><label class="form-label">Region</label><input class="form-select" id="new-k8s-region" placeholder="us-east-1"></div>' +
        '<div class="form-group" id="endpoint-group"><label class="form-label">API Endpoint / Socket Path</label><input class="form-select" id="new-k8s-endpoint" placeholder="https://api.cluster.local:6443 or /var/run/docker.sock"></div>' +
        '<div class="form-group"><label class="form-label">Authentication</label><input class="form-select" id="new-k8s-token" type="password" placeholder="Bearer token / TLS cert path"></div>',
      actions: [
        { label: 'Cancel' },
        { label: 'Add Cluster', primary: true, closeOnClick: false, onClick: async function () {
          var provider = document.getElementById('new-cluster-provider').value;
          var name = document.getElementById('new-k8s-name').value;
          var group = document.getElementById('new-k8s-group').value;
          var region = document.getElementById('new-k8s-region').value;
          if (!name) { alert('Cluster Name is required'); return; }
          
          var payload = {
            id: 'cls-' + name.toLowerCase().replace(/[^a-z0-9]/g, '-'),
            name: name,
            group: group || 'production',
            region: region || 'unknown',
            provider: provider,
            status: 'pending',
            version: 'unknown',
            nodes: 0
          };

          try {
            var res = await fetch('/api/v1/fleet', {
              method: 'POST',
              headers: { 'Content-Type': 'application/json' },
              body: JSON.stringify(payload)
            });
            if (res.ok) {
              AppState.addAuditLog({ action: 'create', target: 'cluster/' + payload.id, result: 'success' });
              Modal.close();
              if (window.k8sTable && typeof window.k8sTable.showLoading === 'function') {
                k8sTable.showLoading(3);
              }
              var r = await fetch('/api/v1/fleet');
              if (r.ok) {
                var data = await r.json();
                if (data && data.data) AppState.setKubernetes(data.data);
              }
            } else {
              alert('Failed to add cluster');
            }
          } catch (e) {
            alert('Error adding cluster: ' + e);
          }
        }}
      ]
    });
  }

  function providerBadgeClass(provider) {
    switch (provider) {
      case 'kubernetes': return 'synced';
      case 'docker': return 'healthy';
      case 'docker_swarm': return 'degraded';
      default: return 'pending';
    }
  }

  function statusBadge(status) {
    var cls = 'badge-pending';
    if (status === 'healthy') cls = 'badge-healthy';
    else if (status === 'degraded') cls = 'badge-degraded';
    else if (status === 'down') cls = 'badge-down';
    return '<span class="badge ' + cls + '">' + esc(status || 'unknown') + '</span>';
  }

  global.KubernetesSection = { init: init };

})(window);
