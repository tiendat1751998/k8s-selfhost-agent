/**
 * Topology Map & Service Dependency Graph Module
 * Interactive visual hierarchy: Cluster → Namespace → Deployment → Pod → Service → Ingress
 * Plus service-to-service dependency visualization.
 */
(function (global) {
  'use strict';

  /* ─── Mock Topology Data ─── */
  function generateTopologyData() {
    var state = (global.AppState && global.AppState.getState()) || {};
    var clusters = state.kubernetes;
    if (!Array.isArray(clusters) || clusters.length === 0) {
      clusters = [
        { name: 'prod-cluster-01', provider: 'aws', status: 'healthy' },
        { name: 'staging-cluster', provider: 'gcp', status: 'healthy' },
        { name: 'dev-cluster', provider: 'azure', status: 'warning' }
      ];
    }

    var topologyTree = clusters.map(function (cluster) {
      return {
        name: cluster.name,
        type: 'cluster',
        status: cluster.status || 'healthy',
        provider: cluster.provider || 'kubernetes',
        children: [
          {
            name: 'production', type: 'namespace', status: 'healthy',
            children: [
              { name: 'payment-api', type: 'deployment', replicas: '3/3', status: 'healthy',
                children: [
                  { name: 'payment-api-7f8a1', type: 'pod', status: 'Running', cpu: '120m', mem: '256Mi' },
                  { name: 'payment-api-7f8a2', type: 'pod', status: 'Running', cpu: '95m', mem: '230Mi' },
                  { name: 'payment-api-7f8a3', type: 'pod', status: 'Running', cpu: '110m', mem: '245Mi' }
                ],
                services: [{ name: 'payment-api-svc', type: 'ClusterIP', port: '8080' }],
                ingress: [{ name: 'payment-api-ing', host: 'api.payments.example.com', path: '/api/v1/payments' }]
              },
              { name: 'order-service', type: 'deployment', replicas: '2/2', status: 'healthy',
                children: [
                  { name: 'order-svc-5d2b1', type: 'pod', status: 'Running', cpu: '80m', mem: '180Mi' },
                  { name: 'order-svc-5d2b2', type: 'pod', status: 'Running', cpu: '75m', mem: '175Mi' }
                ],
                services: [{ name: 'order-svc', type: 'ClusterIP', port: '8081' }],
                ingress: []
              },
              { name: 'user-service', type: 'deployment', replicas: '2/3', status: 'warning',
                children: [
                  { name: 'user-svc-3a4b1', type: 'pod', status: 'Running', cpu: '60m', mem: '150Mi' },
                  { name: 'user-svc-3a4b2', type: 'pod', status: 'CrashLoopBackOff', cpu: '0m', mem: '0Mi' }
                ],
                services: [{ name: 'user-svc', type: 'ClusterIP', port: '8082' }],
                ingress: [{ name: 'user-ing', host: 'api.users.example.com', path: '/api/v1/users' }]
              }
            ]
          },
          {
            name: 'monitoring', type: 'namespace', status: 'healthy',
            children: [
              { name: 'prometheus', type: 'deployment', replicas: '1/1', status: 'healthy',
                children: [{ name: 'prometheus-0', type: 'pod', status: 'Running', cpu: '200m', mem: '512Mi' }],
                services: [{ name: 'prometheus-svc', type: 'ClusterIP', port: '9090' }],
                ingress: []
              }
            ]
          },
          {
            name: 'kube-system', type: 'namespace', status: 'healthy',
            children: [
              { name: 'coredns', type: 'deployment', replicas: '2/2', status: 'healthy',
                children: [
                  { name: 'coredns-abc1', type: 'pod', status: 'Running', cpu: '10m', mem: '64Mi' },
                  { name: 'coredns-abc2', type: 'pod', status: 'Running', cpu: '12m', mem: '68Mi' }
                ],
                services: [{ name: 'kube-dns', type: 'ClusterIP', port: '53' }],
                ingress: []
              }
            ]
          }
        ]
      };
    });

    return topologyTree;
  }

  function generateDependencyData() {
    return {
      services: [
        { id: 'gateway', label: 'API Gateway', x: 400, y: 50, status: 'healthy' },
        { id: 'payment-api', label: 'Payment API', x: 200, y: 180, status: 'healthy' },
        { id: 'order-service', label: 'Order Service', x: 600, y: 180, status: 'healthy' },
        { id: 'user-service', label: 'User Service', x: 400, y: 180, status: 'warning' },
        { id: 'inventory-api', label: 'Inventory API', x: 300, y: 310, status: 'healthy' },
        { id: 'notification-svc', label: 'Notifications', x: 500, y: 310, status: 'healthy' },
        { id: 'postgres-db', label: 'PostgreSQL', x: 200, y: 440, status: 'healthy' },
        { id: 'redis-cache', label: 'Redis Cache', x: 400, y: 440, status: 'healthy' },
        { id: 'rabbitmq', label: 'RabbitMQ', x: 600, y: 440, status: 'healthy' }
      ],
      edges: [
        { from: 'gateway', to: 'payment-api', latency: '12ms', errorRate: 0.1 },
        { from: 'gateway', to: 'order-service', latency: '8ms', errorRate: 0.0 },
        { from: 'gateway', to: 'user-service', latency: '15ms', errorRate: 2.1 },
        { from: 'payment-api', to: 'postgres-db', latency: '3ms', errorRate: 0.0 },
        { from: 'payment-api', to: 'redis-cache', latency: '1ms', errorRate: 0.0 },
        { from: 'order-service', to: 'inventory-api', latency: '5ms', errorRate: 0.0 },
        { from: 'order-service', to: 'rabbitmq', latency: '2ms', errorRate: 0.0 },
        { from: 'user-service', to: 'postgres-db', latency: '4ms', errorRate: 0.5 },
        { from: 'inventory-api', to: 'postgres-db', latency: '3ms', errorRate: 0.0 },
        { from: 'notification-svc', to: 'rabbitmq', latency: '2ms', errorRate: 0.0 },
        { from: 'order-service', to: 'notification-svc', latency: '6ms', errorRate: 0.0 }
      ]
    };
  }

  /* ─── Render: Topology Tree ─── */
  function renderTopologyTree(treeData) {
    var container = document.getElementById('topo-tree-container');
    if (!container) return;

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
        + '<span class="topo-name">' + node.name + '</span>'
        + '<span class="topo-type-badge">' + node.type + '</span>';

      if (node.replicas) html += '<span class="topo-info">replicas: ' + node.replicas + '</span>';
      if (node.cpu) html += '<span class="topo-info">cpu: ' + node.cpu + '</span>';
      if (node.mem) html += '<span class="topo-info">mem: ' + node.mem + '</span>';
      if (node.provider) html += '<span class="topo-info">' + node.provider + '</span>';

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
            + '<span class="topo-name">' + svc.name + '</span>'
            + '<span class="topo-type-badge">service</span>'
            + '<span class="topo-info">' + svc.type + ':' + svc.port + '</span>'
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
            + '<span class="topo-name">' + ing.name + '</span>'
            + '<span class="topo-type-badge">ingress</span>'
            + '<span class="topo-info">' + ing.host + ing.path + '</span>'
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

    var width = 800;
    var height = 520;

    var svgLines = depData.edges.map(function (e) {
      var fromNode = depData.services.find(function (s) { return s.id === e.from; });
      var toNode = depData.services.find(function (s) { return s.id === e.to; });
      if (!fromNode || !toNode) return '';
      var errorColor = e.errorRate > 1 ? '#ef4444' : (e.errorRate > 0 ? '#eab308' : '#374151');
      var midX = (fromNode.x + toNode.x) / 2;
      var midY = (fromNode.y + toNode.y) / 2;
      return '<line x1="' + fromNode.x + '" y1="' + (fromNode.y + 20) + '" x2="' + toNode.x + '" y2="' + (toNode.y - 10) + '" stroke="' + errorColor + '" stroke-width="1.5" stroke-dasharray="4" opacity="0.6"/>'
        + '<text x="' + midX + '" y="' + (midY - 4) + '" fill="' + errorColor + '" font-size="9" text-anchor="middle" font-family="var(--font-number)">' + e.latency + '</text>';
    }).join('');

    var svgNodes = depData.services.map(function (s) {
      var statusColors = { healthy: '#10b981', warning: '#eab308', critical: '#ef4444' };
      var col = statusColors[s.status] || '#6b7280';
      return '<g>'
        + '<rect x="' + (s.x - 55) + '" y="' + (s.y - 15) + '" width="110" height="35" rx="8" fill="var(--color-surface)" stroke="' + col + '" stroke-width="2"/>'
        + '<circle cx="' + (s.x - 40) + '" cy="' + (s.y + 2) + '" r="4" fill="' + col + '"/>'
        + '<text x="' + (s.x + 5) + '" y="' + (s.y + 6) + '" fill="var(--color-text)" font-size="11" text-anchor="middle" font-weight="600">' + s.label + '</text>'
        + '</g>';
    }).join('');

    container.innerHTML = '<svg viewBox="0 0 ' + width + ' ' + height + '" style="width:100%;height:' + height + 'px;">' + svgLines + svgNodes + '</svg>';
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
    refresh: function () {
      renderTopologyTree(generateTopologyData());
      renderDependencyGraph(generateDependencyData());
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
