/**
 * Search Analytics Panel Renderer
 */
(function (global) {
  'use strict';

  function renderAnalytics(queryCount, aiQueryCount) {
    document.getElementById('analytic-queries-count').textContent = queryCount;
    document.getElementById('analytic-ai-count').textContent = aiQueryCount;
    var pct = queryCount > 0 ? Math.round((aiQueryCount / queryCount) * 100) : 0;
    document.getElementById('analytic-ai-count').nextElementSibling.textContent = pct + '% of total search traffic';

    var targets = document.getElementById('analytic-targets');
    if (targets) {
      targets.innerHTML =
        '<div class="pipeline-detail-row"><span>1. pods/payment-svc-4d2e1</span><strong>14 searches</strong></div>' +
        '<div class="pipeline-detail-row"><span>2. deployments/api-server</span><strong>9 searches</strong></div>' +
        '<div class="pipeline-detail-row"><span>3. clusters/staging-1</span><strong>8 searches</strong></div>' +
        '<div class="pipeline-detail-row"><span>4. nodes/ip-10-0-1-78</span><strong>5 searches</strong></div>';
    }

    var errors = document.getElementById('analytic-errors');
    if (errors) {
      errors.innerHTML =
        '<div class="pipeline-detail-row"><span>1. Out Of Memory (OOMKilled)</span><strong style="color:var(--color-trading-down)">38 events</strong></div>' +
        '<div class="pipeline-detail-row"><span>2. Connection timeout to redis</span><strong style="color:var(--color-primary)">19 events</strong></div>' +
        '<div class="pipeline-detail-row"><span>3. Liveness probe threshold failed</span><strong style="color:var(--color-primary)">12 events</strong></div>' +
        '<div class="pipeline-detail-row"><span>4. ImagePullBackOff auth failed</span><strong style="color:var(--color-muted)">8 events</strong></div>';
    }
  }

  global.SearchAnalytics = {
    renderAnalytics: renderAnalytics
  };

})(window);
