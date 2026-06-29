/**
 * Metrics Panel — Chart.js based real-time metrics display.
 * Throttled to prevent UI lag on rapid metrics packets.
 */
(function (global) {
  'use strict';

  let cpuChart = null;
  let memChart = null;
  let lastUpdate = 0;
  let updateTimeout = null;

  function init() {
    AppState.on('metrics', updateMetrics);
    AppState.on('stats', updateStats);
  }

  function updateStats(stats) {
    setTextById('stat-critical', stats.critical);
    setTextById('stat-warning', stats.warning);
    setTextById('stat-resolved', stats.resolved);
    setTextById('stat-agents', stats.agentRuns);
  }

  function updateMetrics(metrics) {
    const now = Date.now();
    // Throttle metric updates to at most once per 1 second to optimize CPU/rendering
    if (now - lastUpdate < 1000) {
      if (updateTimeout) clearTimeout(updateTimeout);
      updateTimeout = setTimeout(() => {
        doUpdateMetrics(metrics);
      }, 1000 - (now - lastUpdate));
      return;
    }
    doUpdateMetrics(metrics);
  }

  function doUpdateMetrics(metrics) {
    lastUpdate = Date.now();
    if (updateTimeout) clearTimeout(updateTimeout);
    
    // Future: update Chart.js charts when metrics canvas elements are present
  }

  function setTextById(id, value) {
    const el = document.getElementById(id);
    if (el) el.textContent = value;
  }

  global.MetricsPanel = { init };

})(window);
