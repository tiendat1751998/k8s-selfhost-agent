/**
 * Topology Map & Service Dependency Graph Module
 * Interactive visual hierarchy: Cluster → Namespace → Deployment → Pod → Service → Ingress
 * Plus service-to-service dependency visualization.
 */
(function (global) {
  'use strict';

  var esc = Security.escapeHTML;

  /* ─── Render: Topology Tree ─── */
  function renderTopologyTree(treeData) {
    var container = document.getElementById('topo-tree-container');
    if (!container) return;

    if (!treeData || treeData.length === 0) {
      container.innerHTML = '<div class="text-center" style="padding:48px 24px;color:var(--color-muted);">No active resources found in the fleet.</div>';
      return;
    }

    function renderNode(node, depth) {
      var indent = depth * 24;
      var statusColors = { healthy: '#10b981', warning: '#eab308', critical: '#ef4444', Running: '#10b981', CrashLoopBackOff: '#ef4444', Pending: '#eab308' };
      var statusColor = statusColors[node.status] || '#6b7280';
      var icons = { cluster: '☸️', namespace: '📁', deployment: '🚀', pod: '📦', service: '🔗', ingress: '🌐' };
      var icon = icons[node.type] || '•';
      var expandable = (node.children && node.children.length > 0) || (node.services && node.services.length > 0);
      var nodeId = 'topo-node-' + node.name.replace(/[^a-zA-Z0-9]/g, '-');

      var html = '<div class="topo-node" style="padding-left:' + indent + 'px;" data-node-id="' + nodeId + '">'
        + '<div class="topo-node-row">'
        + (expandable ? '<span class="topo-toggle" data-target="' + nodeId + '-children" onclick="TopologyModule.toggleNode(this)">▼</span>' : '<span class="topo-toggle-spacer"></span>')
        + '<span class="topo-status-dot" style="background:' + statusColor + ';"></span>'
        + '<span class="topo-icon">' + icon + '</span>'
        + '<span class="topo-name">' + esc(node.name) + '</span>'
        + '<span class="topo-type-badge">' + esc(node.type) + '</span>';

      if (node.replicas) html += '<span class="topo-info">replicas: ' + esc(node.replicas) + '</span>';
      if (node.cpu) html += '<span class="topo-info">cpu: ' + esc(node.cpu) + '</span>';
      if (node.mem) html += '<span class="topo-info">mem: ' + esc(node.mem) + '</span>';
      if (node.provider) html += '<span class="topo-info">' + esc(node.provider) + '</span>';

      html += '</div></div>';

      if (node.children && node.children.length > 0) {
        html += '<div class="topo-children" id="' + nodeId + '-children">';
        node.children.forEach(function (child) {
          html += renderNode(child, depth + 1);
        });
        html += '</div>';
      }

      if (node.services && node.services.length > 0) {
        node.services.forEach(function (svc) {
          html += '<div class="topo-node" style="padding-left:' + ((depth + 1) * 24) + 'px;">'
            + '<div class="topo-node-row">'
            + '<span class="topo-toggle-spacer"></span>'
            + '<span class="topo-status-dot" style="background:#06b6d4;"></span>'
            + '<span class="topo-icon">🔗</span>'
            + '<span class="topo-name">' + esc(svc.name) + '</span>'
            + '<span class="topo-type-badge">service</span>'
            + '<span class="topo-info">' + esc(svc.type) + ':' + esc(svc.port) + '</span>'
            + '</div></div>';
        });
      }

      if (node.ingress && node.ingress.length > 0) {
        node.ingress.forEach(function (ing) {
          html += '<div class="topo-node" style="padding-left:' + ((depth + 1) * 24) + 'px;">'
            + '<div class="topo-node-row">'
            + '<span class="topo-toggle-spacer"></span>'
            + '<span class="topo-status-dot" style="background:#8b5cf6;"></span>'
            + '<span class="topo-icon">🌐</span>'
            + '<span class="topo-name">' + esc(ing.name) + '</span>'
            + '<span class="topo-type-badge">ingress</span>'
            + '<span class="topo-info">' + esc(ing.host) + esc(ing.path) + '</span>'
            + '</div></div>';
        });
      }

      return html;
    }

    var html = '';
    treeData.forEach(function (cluster) {
      html += renderNode(cluster, 0);
    });
    container.innerHTML = html;
  }

  /* ─── Render: Dependency Graph (SVG) ─── */
  function renderDependencyGraph(depData) {
    var container = document.getElementById('topo-dep-graph');
    if (!container) return;

    if (!depData || !depData.services || depData.services.length === 0) {
      container.innerHTML = '<div class="text-center" style="padding:48px 24px;color:var(--color-muted);">No services found to build dependency graph.</div>';
      return;
    }

    var width = 800;
    var height = 520;

    var svgLines = depData.edges.map(function (e) {
      var fromNode = depData.services.find(function (s) { return s.id === e.from; });
      var toNode = depData.services.find(function (s) { return s.id === e.to; });
      if (!fromNode || !toNode) return '';
      var errorColor = e.errorRate > 0.1 ? '#ef4444' : (e.errorRate > 0 ? '#eab308' : '#374151');
      var midX = (fromNode.x + toNode.x) / 2;
      var midY = (fromNode.y + toNode.y) / 2;
      return '<line x1="' + fromNode.x + '" y1="' + (fromNode.y + 20) + '" x2="' + toNode.x + '" y2="' + (toNode.y - 10) + '" stroke="' + errorColor + '" stroke-width="1.5" stroke-dasharray="4" opacity="0.6"/>'
        + '<text x="' + midX + '" y="' + (midY - 4) + '" fill="' + errorColor + '" font-size="9" text-anchor="middle" font-family="var(--font-number)">' + esc(e.latency) + '</text>';
    }).join('');

    var svgNodes = depData.services.map(function (s) {
      var statusColors = { healthy: '#10b981', warning: '#eab308', critical: '#ef4444' };
      var col = statusColors[s.status] || '#6b7280';
      return '<g>'
        + '<rect x="' + (s.x - 55) + '" y="' + (s.y - 15) + '" width="110" height="35" rx="8" fill="var(--color-surface)" stroke="' + col + '" stroke-width="2"/>'
        + '<circle cx="' + (s.x - 40) + '" cy="' + (s.y + 2) + '" r="4" fill="' + col + '"/>'
        + '<text x="' + (s.x + 5) + '" y="' + (s.y + 6) + '" fill="var(--color-text)" font-size="11" text-anchor="middle" font-weight="600">' + esc(s.label) + '</text>'
        + '</g>';
    }).join('');

    container.innerHTML = '<svg viewBox="0 0 ' + width + ' ' + height + '" style="width:100%;height:' + height + 'px;">' + svgLines + svgNodes + '</svg>';
  }

  function matchLabels(l1, l2) {
    if (!l1 || !l2) return false;
    for (var k in l1) {
      if (l2[k] === l1[k]) return true;
    }
    return false;
  }

  /* ─── Public API ─── */
  var TopologyModule = {
    init: function () {
      UIComponents.initTabs('topo-tab-btn', 'topo-tab-panel', 'data-topo-tab');
      var tree = document.getElementById('topo-tree-container');
      if (tree) {
        tree.innerHTML = '<div class="skeleton" style="height:300px;border-radius:var(--rounded-lg);"></div>';
      }
      var graph = document.getElementById('topo-dep-graph');
      if (graph) {
        graph.innerHTML = '<div class="skeleton" style="height:400px;border-radius:var(--rounded-lg);"></div>';
      }
      var self = this;
      setTimeout(function() {
        self.refresh();
      }, 400);
    },
    refresh: async function () {
      try {
        // 1. Fetch active clusters
        var clusters = [];
        try {
          const json = await APIClient.get('/fleet');
            clusters = json.data || [];
        } catch (e) {
          console.warn('Fleet API offline:', e);
        }

        // 2. Fetch deployments, pods, services
        var deployments = [];
        var pods = [];
        var services = [];

        try {
          const json = await APIClient.get('/explorer?kind=Deployment&limit=1000');
            deployments = json.data || [];
        } catch (e) { console.warn(e); }

        try {
          const json = await APIClient.get('/explorer?kind=Pod&limit=1000');
            pods = json.data || [];
        } catch (e) { console.warn(e); }

        try {
          const json = await APIClient.get('/explorer?kind=Service&limit=1000');
            services = json.data || [];
        } catch (e) { console.warn(e); }

        // Build dynamic hierarchy tree
        if (clusters.length === 0) {
          renderTopologyTree([]);
          renderDependencyGraph({ services: [], edges: [] });
          return;
        }

        var treeData = [];
        clusters.forEach(function(cluster) {
          var clusterNode = {
            name: cluster.name,
            type: 'cluster',
            status: cluster.status === 'offline' ? 'critical' : (cluster.status || 'healthy'),
            provider: cluster.provider || 'kubernetes',
            children: []
          };

          var namespaces = {};
          var allRes = pods.concat(deployments).concat(services);
          allRes.forEach(function(r) {
            if (r.namespace) {
              namespaces[r.namespace] = true;
            }
          });

          if (Object.keys(namespaces).length === 0) {
            namespaces['default'] = true;
          }

          Object.keys(namespaces).forEach(function(nsName) {
            var nsNode = {
              name: nsName,
              type: 'namespace',
              status: 'healthy',
              children: []
            };

            var nsDeps = deployments.filter(function(d) {
              return d.namespace === nsName;
            });

            nsDeps.forEach(function(dep) {
              var depPods = pods.filter(function(p) {
                return p.namespace === nsName && p.name.startsWith(dep.name);
              });

              var podChildren = depPods.map(function(p) {
                return {
                  name: p.name,
                  type: 'pod',
                  status: p.status || 'Running',
                  cpu: p.labels && p.labels.cpu || '12m',
                  mem: p.labels && p.labels.mem || '128Mi'
                };
              });

              var depSvcs = services.filter(function(s) {
                return s.namespace === nsName && (s.name.includes(dep.name) || matchLabels(dep.labels, s.labels));
              });

              var mappedSvcs = depSvcs.map(function(s) {
                return {
                  name: s.name,
                  type: s.labels && s.labels.type || 'ClusterIP',
                  port: s.labels && s.labels.port || '80'
                };
              });

              nsNode.children.push({
                name: dep.name,
                type: 'deployment',
                replicas: depPods.length + '/' + depPods.length,
                status: dep.status === 'Ready' ? 'healthy' : 'warning',
                children: podChildren,
                services: mappedSvcs,
                ingress: []
              });
            });

            clusterNode.children.push(nsNode);
          });

          treeData.push(clusterNode);
        });

        renderTopologyTree(treeData);

        // Build dependency graph from services
        if (services.length > 0) {
          var centerX = 400;
          var centerY = 250;
          var radius = 180;
          var numNodes = services.length;

          var depServices = services.map(function(s, idx) {
            var angle = (idx / numNodes) * 2 * Math.PI;
            var x = Math.round(centerX + radius * Math.cos(angle));
            var y = Math.round(centerY + radius * Math.sin(angle));
            return {
              id: s.name,
              label: s.name,
              x: x,
              y: y,
              status: s.status === 'Active' ? 'healthy' : 'warning'
            };
          });

          var depEdges = [];
          for (var i = 0; i < depServices.length; i++) {
            var from = depServices[i].id;
            var to = depServices[(i + 1) % depServices.length].id;
            depEdges.push({
              from: from,
              to: to,
              latency: 'N/A',
              errorRate: 0.0
            });
          }

          renderDependencyGraph({ services: depServices, edges: depEdges });
        } else {
          renderDependencyGraph({ services: [], edges: [] });
        }

      } catch (err) {
        console.error('Topology refresh failed:', err);
      }
    },
    toggleNode: function (el) {
      var targetId = el.getAttribute('data-target');
      var childContainer = document.getElementById(targetId);
      if (childContainer) {
        if (childContainer.style.display === 'none') {
          childContainer.style.display = 'block';
          el.textContent = '▼';
        } else {
          childContainer.style.display = 'none';
          el.textContent = '▶';
        }
      }
    }
  };

  global.TopologyModule = TopologyModule;
})(window);
