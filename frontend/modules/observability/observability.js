/**
 * Observability Module — SLO Dashboard, Error Budgets, Traces, Events
 */
(function (global) {
  'use strict';

  /* ─── Mock Data ─── */
  function generateSLOData() {
    return [
      { service: 'payment-api', target: 99.95, actual: 99.92, burnRate: 1.8, budget: 32, budgetStatus: 'warning' },
      { service: 'user-service', target: 99.9, actual: 99.97, burnRate: 0.3, budget: 85, budgetStatus: 'healthy' },
      { service: 'order-service', target: 99.95, actual: 99.88, burnRate: 2.4, budget: 12, budgetStatus: 'critical' },
      { service: 'inventory-api', target: 99.9, actual: 99.93, burnRate: 0.7, budget: 67, budgetStatus: 'healthy' },
      { service: 'notification-svc', target: 99.5, actual: 99.78, burnRate: 0.2, budget: 94, budgetStatus: 'healthy' },
      { service: 'analytics-engine', target: 99.9, actual: 99.85, burnRate: 1.5, budget: 45, budgetStatus: 'warning' }
    ];
  }

  function generateTraceData() {
    var services = ['payment-api', 'user-service', 'order-service', 'gateway', 'inventory-api'];
    var statuses = ['OK', 'OK', 'OK', 'ERROR', 'OK', 'TIMEOUT'];
    var traces = [];
    for (var i = 0; i < 15; i++) {
      var d = new Date();
      d.setMinutes(d.getMinutes() - Math.floor(Math.random() * 120));
      var status = statuses[Math.floor(Math.random() * statuses.length)];
      traces.push({
        traceId: 'trace-' + Math.random().toString(36).substr(2, 12),
        service: services[Math.floor(Math.random() * services.length)],
        operation: ['GET /api/v1/orders', 'POST /api/v1/payments', 'GET /api/v1/users', 'PUT /api/v1/inventory'][Math.floor(Math.random() * 4)],
        duration: Math.round(5 + Math.random() * 995),
        status: status,
        spans: Math.floor(3 + Math.random() * 12),
        timestamp: d.toISOString().slice(11, 19)
      });
    }
    return traces.sort(function (a, b) { return a.timestamp < b.timestamp ? 1 : -1; });
  }

  function generateEventData() {
    var events = [
      { type: 'Warning', reason: 'BackOff', object: 'pod/payment-api-7f8a', namespace: 'production', message: 'Back-off restarting failed container', age: '2m' },
      { type: 'Normal', reason: 'Scheduled', object: 'pod/order-svc-5d2b', namespace: 'production', message: 'Successfully assigned pod to node-3', age: '5m' },
      { type: 'Warning', reason: 'Unhealthy', object: 'pod/analytics-9x1c', namespace: 'monitoring', message: 'Liveness probe failed: connection refused', age: '8m' },
      { type: 'Normal', reason: 'Pulled', object: 'pod/user-svc-3a4b', namespace: 'production', message: 'Successfully pulled image "user-svc:v2.1.0"', age: '12m' },
      { type: 'Normal', reason: 'Created', object: 'pod/cache-warmer-1b2c', namespace: 'staging', message: 'Created container cache-warmer', age: '15m' },
      { type: 'Warning', reason: 'FailedMount', object: 'pod/data-proc-8e9f', namespace: 'production', message: 'MountVolume.SetUp failed for volume "config-vol"', age: '18m' },
      { type: 'Normal', reason: 'ScalingReplicaSet', object: 'deploy/inventory-api', namespace: 'production', message: 'Scaled up replica set inventory-api-6d7e to 5', age: '22m' },
      { type: 'Warning', reason: 'OOMKilling', object: 'pod/ml-worker-4c5d', namespace: 'ml-jobs', message: 'Memory cgroup out of memory: Killed process 1234', age: '30m' },
      { type: 'Normal', reason: 'Completed', object: 'job/backup-daily', namespace: 'kube-system', message: 'Job completed successfully', age: '45m' },
      { type: 'Normal', reason: 'NodeReady', object: 'node/worker-node-05', namespace: '', message: 'Node worker-node-05 status is now: NodeReady', age: '1h' }
    ];
    return events;
  }

  /* ─── Render: SLO Dashboard ─── */
  function renderSLOCards(sloData) {
    var container = document.getElementById('obs-slo-grid');
    if (!container) return;

    container.innerHTML = sloData.map(function (slo) {
      var gaugeAngle = (slo.actual / 100) * 180;
      var statusColor = slo.budgetStatus === 'critical' ? '#ef4444' : (slo.budgetStatus === 'warning' ? '#eab308' : '#10b981');
      var complianceColor = slo.actual >= slo.target ? '#10b981' : '#ef4444';

      return '<div class="obs-slo-card">'
        + '<div class="obs-slo-header">'
        + '  <span class="obs-slo-service">' + slo.service + '</span>'
        + '  <span class="obs-slo-target">Target: ' + slo.target + '%</span>'
        + '</div>'
        + '<div class="obs-slo-gauge-wrap">'
        + '  <div class="obs-slo-gauge">'
        + '    <svg viewBox="0 0 120 70" class="obs-gauge-svg">'
        + '      <path d="M10 65 A50 50 0 0 1 110 65" fill="none" stroke="var(--color-hairline)" stroke-width="8" stroke-linecap="round"/>'
        + '      <path d="M10 65 A50 50 0 0 1 110 65" fill="none" stroke="' + complianceColor + '" stroke-width="8" stroke-linecap="round" stroke-dasharray="' + (gaugeAngle * 0.87) + ' 157" />'
        + '    </svg>'
        + '    <div class="obs-gauge-value" style="color:' + complianceColor + ';">' + slo.actual + '%</div>'
        + '  </div>'
        + '</div>'
        + '<div class="obs-slo-metrics">'
        + '  <div class="obs-slo-metric"><span class="obs-metric-label">Burn Rate</span><span class="obs-metric-value">' + slo.burnRate + 'x</span></div>'
        + '  <div class="obs-slo-metric"><span class="obs-metric-label">Error Budget</span>'
        + '    <div class="obs-budget-bar"><div class="obs-budget-fill" style="width:' + slo.budget + '%;background:' + statusColor + ';"></div></div>'
        + '    <span class="obs-metric-value" style="color:' + statusColor + ';">' + slo.budget + '%</span>'
        + '  </div>'
        + '</div>'
        + '</div>';
    }).join('');
  }

  /* ─── Render: Traces ─── */
  function renderTraceTable(traces) {
    var tbody = document.getElementById('obs-trace-tbody');
    if (!tbody) return;

    tbody.innerHTML = traces.map(function (t) {
      var statusClass = t.status === 'OK' ? 'obs-status-ok' : (t.status === 'ERROR' ? 'obs-status-error' : 'obs-status-timeout');
      var durationColor = t.duration > 500 ? '#ef4444' : (t.duration > 200 ? '#eab308' : '#10b981');
      return '<tr>'
        + '<td><code class="obs-trace-id">' + t.traceId + '</code></td>'
        + '<td><strong>' + t.service + '</strong></td>'
        + '<td style="font-size:12px;">' + t.operation + '</td>'
        + '<td style="font-family:var(--font-number);color:' + durationColor + ';">' + t.duration + 'ms</td>'
        + '<td><span class="' + statusClass + '">' + t.status + '</span></td>'
        + '<td style="font-family:var(--font-number);">' + t.spans + '</td>'
        + '<td style="font-size:12px;color:var(--color-muted);">' + t.timestamp + '</td>'
        + '<td><button class="btn btn-ghost btn-sm" onclick="ObservabilityModule.viewTrace(\'' + t.traceId + '\')">View</button></td>'
        + '</tr>';
    }).join('');
  }

  /* ─── Render: Events ─── */
  function renderEventStream(events) {
    var container = document.getElementById('obs-events-list');
    if (!container) return;

    container.innerHTML = events.map(function (e) {
      var typeClass = e.type === 'Warning' ? 'obs-event-warning' : 'obs-event-normal';
      var icon = e.type === 'Warning' ? '⚠️' : '✅';
      return '<div class="obs-event-item ' + typeClass + '">'
        + '<div class="obs-event-icon">' + icon + '</div>'
        + '<div class="obs-event-body">'
        + '  <div class="obs-event-header-row">'
        + '    <span class="obs-event-reason">' + e.reason + '</span>'
        + '    <span class="obs-event-object">' + e.object + '</span>'
        + '    <span class="obs-event-age">' + e.age + '</span>'
        + '  </div>'
        + '  <div class="obs-event-message">' + e.message + '</div>'
        + '  ' + (e.namespace ? '<span class="obs-event-ns">ns/' + e.namespace + '</span>' : '')
        + '</div>'
        + '</div>';
    }).join('');
  }

  /* ─── Public API ─── */
  var ObservabilityModule = {
    init: function () {
      UIComponents.initTabs('obs-tab-btn', 'obs-tab-panel', 'data-obs-tab');
      var grid = document.getElementById('obs-slo-grid');
      if (grid) {
        grid.innerHTML = '<div class="skeleton" style="height:240px;grid-column:span 3;border-radius:var(--rounded-lg);"></div>';
      }
      var tbody = document.getElementById('obs-trace-tbody');
      if (tbody) {
        tbody.innerHTML = '<tr><td colspan="7"><div class="skeleton" style="height:120px;border-radius:var(--rounded-lg);"></div></td></tr>';
      }
      var self = this;
      setTimeout(function() {
        self.refresh();
      }, 400);
    },
    refresh: function () {
      renderSLOCards(generateSLOData());
      renderTraceTable(generateTraceData());
      renderEventStream(generateEventData());
    },
    viewTrace: function (traceId) {
      if (global.Modal && global.Modal.open) {
        var spans = [
          { service: 'gateway', operation: 'HTTP GET', duration: 245, start: 0 },
          { service: 'payment-api', operation: 'processPayment', duration: 180, start: 15 },
          { service: 'user-service', operation: 'validateUser', duration: 45, start: 20 },
          { service: 'inventory-api', operation: 'checkStock', duration: 90, start: 65 },
          { service: 'order-service', operation: 'createOrder', duration: 120, start: 95 },
          { service: 'notification-svc', operation: 'sendConfirmation', duration: 35, start: 200 }
        ];
        var maxDur = 260;
        var waterfallHtml = spans.map(function (s) {
          var left = Math.round((s.start / maxDur) * 100);
          var width = Math.max(Math.round((s.duration / maxDur) * 100), 4);
          var colors = { gateway: '#6366f1', 'payment-api': '#f43f5e', 'user-service': '#06b6d4', 'inventory-api': '#10b981', 'order-service': '#f97316', 'notification-svc': '#8b5cf6' };
          var color = colors[s.service] || '#6b7280';
          return '<div style="display:flex;align-items:center;gap:8px;margin-bottom:6px;font-size:12px;">'
            + '<div style="width:120px;text-align:right;color:var(--color-muted);flex-shrink:0;">' + s.service + '</div>'
            + '<div style="flex:1;height:20px;background:var(--color-surface);border-radius:4px;position:relative;">'
            + '  <div style="position:absolute;left:' + left + '%;width:' + width + '%;height:100%;background:' + color + ';border-radius:4px;display:flex;align-items:center;justify-content:center;color:#fff;font-size:10px;font-weight:600;">' + s.duration + 'ms</div>'
            + '</div>'
            + '</div>';
        }).join('');

        global.Modal.open({
          title: 'Trace: ' + traceId,
          body: '<div style="padding:var(--space-sm);">'
            + '<h4 style="margin:0 0 var(--space-sm);font-size:13px;color:var(--color-muted);text-transform:uppercase;">Span Waterfall</h4>'
            + waterfallHtml
            + '</div>'
        });
      }
    }
  };

  global.ObservabilityModule = ObservabilityModule;
})(window);
