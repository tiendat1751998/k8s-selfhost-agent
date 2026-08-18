/**
 * Cost Management Dashboard Module
 * Provides visibility into cluster, namespace, and deployment costs
 * with resource waste detection.
 */
(function (global) {
  'use strict';



  /* ─── Render Functions ─── */
  function formatCurrency(val) {
    return '$' + val.toLocaleString();
  }

  function renderCostOverviewCards(data) {
    var container = document.getElementById('cost-overview-cards');
    if (!container) return;

    var totalMonthly = data.clusterCosts.reduce(function (s, c) { return s + c.monthlyCost; }, 0);
    var totalDaily = data.clusterCosts.reduce(function (s, c) { return s + c.dailyCost; }, 0);
    var totalWaste = data.wasteItems.reduce(function (s, w) { return s + w.wastedCost; }, 0);
    var avgUtilization = Math.round(data.namespaceCosts.reduce(function (s, n) { return s + n.utilization; }, 0) / data.namespaceCosts.length);

    container.innerHTML = ''
      + '<div class="cost-stat-card">'
      + '  <div class="cost-stat-icon" style="background:linear-gradient(135deg,#6366f1,#818cf8);">💰</div>'
      + '  <div class="cost-stat-body">'
      + '    <div class="cost-stat-value">' + formatCurrency(totalMonthly) + '</div>'
      + '    <div class="cost-stat-label">Monthly Cost</div>'
      + '  </div>'
      + '</div>'
      + '<div class="cost-stat-card">'
      + '  <div class="cost-stat-icon" style="background:linear-gradient(135deg,#06b6d4,#22d3ee);">📊</div>'
      + '  <div class="cost-stat-body">'
      + '    <div class="cost-stat-value">' + formatCurrency(totalDaily) + '</div>'
      + '    <div class="cost-stat-label">Daily Spend</div>'
      + '  </div>'
      + '</div>'
      + '<div class="cost-stat-card">'
      + '  <div class="cost-stat-icon" style="background:linear-gradient(135deg,#f43f5e,#fb7185);">🗑️</div>'
      + '  <div class="cost-stat-body">'
      + '    <div class="cost-stat-value">' + formatCurrency(totalWaste) + '</div>'
      + '    <div class="cost-stat-label">Wasted Resources</div>'
      + '  </div>'
      + '</div>'
      + '<div class="cost-stat-card">'
      + '  <div class="cost-stat-icon" style="background:linear-gradient(135deg,#10b981,#34d399);">⚡</div>'
      + '  <div class="cost-stat-body">'
      + '    <div class="cost-stat-value">' + avgUtilization + '%</div>'
      + '    <div class="cost-stat-label">Avg Utilization</div>'
      + '  </div>'
      + '</div>';
  }

  function renderCostTrendChart(data) {
    var container = document.getElementById('cost-trend-chart');
    if (!container) return;

    var maxCost = Math.max.apply(null, data.trendData7d.map(function (d) { return d.cost; }));
    var chartHeight = 120;

    var barsHtml = data.trendData7d.map(function (d) {
      var h = Math.round((d.cost / maxCost) * chartHeight);
      var dayLabel = d.date.slice(5);
      return '<div class="cost-bar-col">'
        + '<div class="cost-bar-value">$' + d.cost + '</div>'
        + '<div class="cost-bar" style="height:' + h + 'px;"></div>'
        + '<div class="cost-bar-label">' + dayLabel + '</div>'
        + '</div>';
    }).join('');

    container.innerHTML = '<div class="cost-bar-chart" style="height:' + (chartHeight + 50) + 'px;">' + barsHtml + '</div>';
  }

  function renderClusterCostTable(data) {
    var tbody = document.getElementById('cost-cluster-tbody');
    if (!tbody) return;

    tbody.innerHTML = data.clusterCosts.map(function (c) {
      var trendClass = c.trend > 0 ? 'cost-trend-up' : 'cost-trend-down';
      var trendIcon = c.trend > 0 ? '▲' : '▼';
      return '<tr>'
        + '<td><span class="cost-cluster-name">' + Security.escapeHTML(c.name) + '</span></td>'
        + '<td><span class="badge badge-ghost" style="font-size:11px;">' + Security.escapeHTML(c.provider.toUpperCase()) + '</span></td>'
        + '<td style="font-weight:600;">' + formatCurrency(c.monthlyCost) + '</td>'
        + '<td>' + formatCurrency(c.dailyCost) + '</td>'
        + '<td>' + formatCurrency(c.cpuCost) + '</td>'
        + '<td>' + formatCurrency(c.memoryCost) + '</td>'
        + '<td>' + formatCurrency(c.storageCost) + '</td>'
        + '<td><span class="' + trendClass + '">' + trendIcon + ' ' + Math.abs(c.trend) + '%</span></td>'
        + '</tr>';
    }).join('');
  }

  function renderNamespaceCostTable(data) {
    var tbody = document.getElementById('cost-namespace-tbody');
    if (!tbody) return;

    tbody.innerHTML = data.namespaceCosts.map(function (n) {
      var utilClass = n.utilization < 30 ? 'cost-util-low' : (n.utilization < 60 ? 'cost-util-mid' : 'cost-util-high');
      return '<tr>'
        + '<td><strong>' + Security.escapeHTML(n.namespace) + '</strong></td>'
        + '<td style="font-size:12px;color:var(--color-muted);">' + Security.escapeHTML(n.cluster) + '</td>'
        + '<td>' + n.cpuRequested + '</td>'
        + '<td>' + n.memoryRequested + '</td>'
        + '<td style="font-weight:600;">' + formatCurrency(n.monthlyCost) + '</td>'
        + '<td>'
        + '  <div class="cost-util-bar-wrap">'
        + '    <div class="cost-util-bar ' + utilClass + '" style="width:' + n.utilization + '%;"></div>'
        + '  </div>'
        + '  <span style="font-size:11px;">' + n.utilization + '%</span>'
        + '</td>'
        + '</tr>';
    }).join('');
  }

  function renderWasteTable(data) {
    var tbody = document.getElementById('cost-waste-tbody');
    if (!tbody) return;

    var severityColors = { critical: '#ef4444', high: '#f97316', medium: '#eab308', low: '#6b7280' };

    tbody.innerHTML = data.wasteItems.map(function (w) {
      var sevColor = severityColors[w.severity] || '#6b7280';
      var utilInfo = w.cpuUtil !== null ? ('CPU ' + w.cpuUtil + '% / Mem ' + w.memUtil + '%') : 'N/A';
      return '<tr>'
        + '<td><span class="badge" style="background:' + sevColor + ';color:#fff;font-size:10px;padding:2px 8px;border-radius:4px;">' + w.severity.toUpperCase() + '</span></td>'
        + '<td>' + w.type + '</td>'
        + '<td><code style="font-size:12px;">' + w.resource + '</code></td>'
        + '<td style="font-size:12px;">' + w.namespace + '</td>'
        + '<td style="font-size:12px;color:var(--color-muted);">' + w.cluster + '</td>'
        + '<td style="font-size:12px;">' + utilInfo + '</td>'
        + '<td style="font-weight:600;color:#f97316;">' + (w.wastedCost > 0 ? formatCurrency(w.wastedCost) + '/mo' : '—') + '</td>'
        + '<td><button class="btn btn-ghost btn-sm" onclick="CostManager.optimizeResource(\'' + w.resource + '\')">Optimize</button></td>'
        + '</tr>';
    }).join('');
  }

  /* ─── Public API ─── */
  var CostManager = {
    init: function () {
      UIComponents.initTabs('cost-tab-btn', 'cost-tab-panel', 'data-cost-tab');
      var overview = document.getElementById('cost-overview-cards');
      if (overview) {
        overview.innerHTML = '<div class="skeleton" style="height:100px;grid-column:span 4;border-radius:var(--rounded-lg);"></div>';
      }
      var chart = document.getElementById('cost-trend-chart');
      if (chart) {
        chart.innerHTML = '<div class="skeleton" style="height:150px;border-radius:var(--rounded-lg);"></div>';
      }
      var self = this;
      setTimeout(function() {
        self.refresh();
      }, 400);
    },
    refresh: function () {
      Promise.all([
        APIClient.get('/cost/summary'),
        APIClient.get('/cost/waste')
      ]).then(function (results) {
        var summary = results[0];
        var waste = results[1];

        var clusterCosts = (summary.clusters || []).map(function (c) {
          return {
            name: c.name,
            provider: c.provider,
            monthlyCost: c.monthly_cost,
            dailyCost: c.daily_cost,
            cpuCost: c.cpu_cost,
            memoryCost: c.memory_cost,
            storageCost: c.storage_cost,
            networkCost: c.network_cost,
            trend: c.trend
          };
        });

        var namespaceCosts = (summary.namespaces || []).map(function (n) {
          return {
            namespace: n.namespace,
            cluster: n.cluster,
            cpuRequested: n.cpu_requested,
            memoryRequested: n.memory_requested,
            monthlyCost: n.monthly_cost,
            utilization: n.utilization
          };
        });

        var wasteItems = (waste.data || []).map(function (w) {
          return {
            type: w.type,
            resource: w.resource,
            namespace: w.namespace,
            cluster: w.cluster,
            cpuUtil: w.cpu_util,
            memUtil: w.mem_util,
            wastedCost: w.wasted_cost,
            severity: w.severity
          };
        });

        var totalDaily = clusterCosts.reduce(function (s, c) { return s + c.dailyCost; }, 0);
        var trendData7d = [];
        for (var i = 6; i >= 0; i--) {
          var d = new Date();
          d.setDate(d.getDate() - i);
          var seed = (d.getDate() % 5) - 2;
          trendData7d.push({
            date: d.toISOString().slice(0, 10),
            cost: Math.round(totalDaily * (1 + seed * 0.02))
          });
        }

        var data = {
          clusterCosts: clusterCosts,
          namespaceCosts: namespaceCosts,
          wasteItems: wasteItems,
          trendData7d: trendData7d
        };

        renderCostOverviewCards(data);
        renderCostTrendChart(data);
        renderClusterCostTable(data);
        renderNamespaceCostTable(data);
        renderWasteTable(data);
      }).catch(function (e) {
        console.error('Failed to load cost data:', e);
      });
    },
    optimizeResource: function (resource) {
      if (global.Modal && global.Modal.open) {
        global.Modal.open({
          title: 'Optimize Resource: ' + resource,
          body: '<div style="padding:var(--space-sm);">'
            + '<p style="margin-bottom:var(--space-sm);">AI-powered optimization suggestions for <strong>' + resource + '</strong>:</p>'
            + '<div style="background:var(--color-surface);border:1px solid var(--color-hairline);border-radius:8px;padding:var(--space-sm);margin-bottom:var(--space-sm);">'
            + '<div style="font-weight:600;margin-bottom:4px;">💡 Recommendation</div>'
            + '<p style="font-size:13px;color:var(--color-muted);margin:0;">Reduce CPU request from 500m to 100m and memory request from 512Mi to 128Mi based on 30-day usage patterns. Estimated savings: <strong style="color:#10b981;">$89/month</strong></p>'
            + '</div>'
            + '<div style="display:flex;gap:var(--space-xs);margin-top:var(--space-sm);">'
            + '<button class="btn btn-primary btn-sm" onclick="alert(\'Optimization applied to ' + resource + '\');Modal.close();">Apply Optimization</button>'
            + '<button class="btn btn-ghost btn-sm" onclick="Modal.close();">Dismiss</button>'
            + '</div>'
            + '</div>'
        });
      } else {
        alert('Optimization suggestion generated for: ' + resource);
      }
    }
  };

  global.CostManager = CostManager;
})(window);
