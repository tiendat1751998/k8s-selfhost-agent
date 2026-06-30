/**
 * Observability Module — SLO Dashboard, Error Budgets, Traces, Events
 */
(function (global) {
  'use strict';

  /* ─── Render: SLO Dashboard ─── */
  function renderSLOCards(sloData) {
    var container = document.getElementById('obs-slo-grid');
    if (!container) return;

    if (!sloData || sloData.length === 0) {
      container.innerHTML = '<div style="grid-column:span 3;text-align:center;padding:48px 24px;color:var(--color-muted);">'
        + '<h4>No Active Service Level Objectives (SLOs) found</h4>'
        + '<p>Configure SLO definitions in the enterprise management pane to track service health.</p>'
        + '</div>';
      return;
    }

    container.innerHTML = sloData.map(function (slo) {
      var gaugeAngle = (slo.actual / 100) * 180;
      var statusColor = slo.budgetStatus === 'critical' ? '#ef4444' : (slo.budgetStatus === 'warning' ? '#eab308' : '#10b981');
      var complianceColor = slo.actual >= slo.target ? '#10b981' : '#ef4444';

      return '<div class="obs-slo-card">'
        + '<div class="obs-slo-header">'
        + '  <span class="obs-slo-service">' + esc(slo.service) + '</span>'
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
  function renderTraceTable() {
    var tbody = document.getElementById('obs-trace-tbody');
    if (!tbody) return;

    tbody.innerHTML = '<tr><td colspan="8" class="text-center" style="padding: 32px 16px; text-align: center;">'
      + '<div class="alert alert-danger" style="display:inline-flex;align-items:center;gap:12px;margin:0;max-width:600px;text-align:left;">'
      + '  <span class="icon" style="font-size:24px;">⚠️</span>'
      + '  <div>'
      + '    <h4 style="margin:0 0 4px;font-size:14px;color:var(--color-trading-down);font-weight:600;">Distributed tracing engine offline</h4>'
      + '    <p style="margin:0;font-size:12px;color:var(--color-muted);">Jaeger / OpenTelemetry endpoints are not configured in this cluster environment.</p>'
      + '  </div>'
      + '</div>'
      + '</td></tr>';
  }

  /* ─── Render: Events ─── */
  function renderEventStream(events) {
    var container = document.getElementById('obs-events-list');
    if (!container) return;

    if (!events || events.length === 0) {
      container.innerHTML = '<div class="text-center" style="padding: 24px; color: var(--color-muted);">No recent cluster events detected.</div>';
      return;
    }

    container.innerHTML = events.map(function (e) {
      var typeClass = e.type === 'Warning' ? 'obs-event-warning' : 'obs-event-normal';
      var icon = e.type === 'Warning' ? '⚠️' : '✅';
      return '<div class="obs-event-item ' + typeClass + '">'
        + '<div class="obs-event-icon">' + icon + '</div>'
        + '<div class="obs-event-body">'
        + '  <div class="obs-event-header-row">'
        + '    <span class="obs-event-reason">' + esc(e.reason) + '</span>'
        + '    <span class="obs-event-object">' + esc(e.object) + '</span>'
        + '    <span class="obs-event-age">' + esc(e.age) + '</span>'
        + '  </div>'
        + '  <div class="obs-event-message">' + esc(e.message) + '</div>'
        + '  ' + (e.namespace ? '<span class="obs-event-ns">ns/' + esc(e.namespace) + '</span>' : '')
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
    refresh: async function () {
      // 1. Load SLO snapshots
      try {
        const res = await fetch('/api/v1/observability/slo');
        if (res.ok) {
          const json = await res.json();
          const items = json.data || [];
          const mappedItems = items.map(function (item) {
            return {
              service: item.service,
              target: item.target,
              actual: item.actual,
              burnRate: item.burn_rate,
              budget: item.error_budget,
              budgetStatus: item.budget_status
            };
          });
          renderSLOCards(mappedItems);
        } else {
          renderSLOCards([]);
        }
      } catch (e) {
        console.error('Failed to fetch live SLO data:', e);
        renderSLOCards([]);
      }

      // 2. Render Jaeger offline warning
      renderTraceTable();

      // 3. Load events from /api/v1/explorer?kind=Event
      try {
        const res = await fetch('/api/v1/explorer?kind=Event');
        if (res.ok) {
          const json = await res.json();
          const items = json.data || [];
          const mappedEvents = items.map(function (item) {
            var labels = item.labels || {};
            return {
              type: item.status || 'Normal',
              reason: labels.reason || item.kind || 'Event',
              object: labels.object || item.name,
              namespace: item.namespace || '',
              message: labels.message || item.name || 'System event triggered',
              age: item.age || 'just now'
            };
          });
          renderEventStream(mappedEvents);
        } else {
          renderEventStream([]);
        }
      } catch (e) {
        console.error('Failed to fetch events:', e);
        renderEventStream([]);
      }
    },
    viewTrace: function (traceId) {
      // Offline fallback
      alert('Distributed tracing engine is offline. Trace waterfall is unavailable.');
    }
  };

  global.ObservabilityModule = ObservabilityModule;
})(window);
