/**
 * Telemetry Service — Client-side error telemetry and performance metrics.
 * Reports uncaught errors and performance data to /api/v1/telemetry.
 */
(function (global) {
  'use strict';

  const TELEMETRY_URL = '/api/v1/telemetry';
  const sentErrors = new Set();

  function init() {
    // 1. Hook up window.onerror to capture uncaught exceptions
    window.addEventListener('error', function (event) {
      if (!event.filename && !event.message) return;

      const errorKey = `${event.message}:${event.filename}:${event.lineno}:${event.colno}`;
      if (sentErrors.has(errorKey)) return;
      sentErrors.add(errorKey);

      report({
        type: 'error',
        message: event.message,
        source: event.filename,
        line: event.lineno,
        col: event.colno,
        stack: event.error ? event.error.stack : null,
        url: window.location.href,
        timestamp: new Date().toISOString()
      });
    });

    // 2. Hook up unhandled promise rejections
    window.addEventListener('unhandledrejection', function (event) {
      const reason = event.reason;
      let message = 'Unhandled Promise Rejection';
      let stack = null;

      if (reason) {
        if (reason instanceof Error) {
          message = reason.message;
          stack = reason.stack;
        } else if (typeof reason === 'string') {
          message = reason;
        } else {
          try {
            message = JSON.stringify(reason);
          } catch (e) {}
        }
      }

      const errorKey = `promise:${message}:${stack}`;
      if (sentErrors.has(errorKey)) return;
      sentErrors.add(errorKey);

      report({
        type: 'error',
        message: message,
        stack: stack,
        url: window.location.href,
        timestamp: new Date().toISOString()
      });
    });

    // 3. Hook up basic performance metrics on page load
    window.addEventListener('load', function () {
      setTimeout(reportPerformance, 1000);
    });
  }

  function reportPerformance() {
    let loadTime = null;
    let fcp = null;

    if (global.performance) {
      const perfData = global.performance.timing;
      if (perfData) {
        loadTime = perfData.loadEventEnd - perfData.navigationStart;
      }

      if (typeof global.PerformanceObserver === 'function') {
        try {
          const observer = new PerformanceObserver((list) => {
            const entries = list.getEntries();
            for (const entry of entries) {
              if (entry.name === 'first-contentful-paint') {
                fcp = entry.startTime;
                reportPerformancePayload(loadTime, fcp);
                observer.disconnect();
                break;
              }
            }
          });
          observer.observe({ type: 'paint', buffered: true });
        } catch (e) {
          reportPerformancePayload(loadTime, null);
        }
      } else {
        reportPerformancePayload(loadTime, null);
      }
    }
  }

  function reportPerformancePayload(loadTime, fcp) {
    const domNodes = document.getElementsByTagName('*').length;
    report({
      type: 'performance',
      url: window.location.href,
      timestamp: new Date().toISOString(),
      metrics: {
        loadTimeMs: loadTime,
        fcpMs: fcp,
        domNodeCount: domNodes
      }
    });
  }

  function report(data) {
    if (data.url && data.url.includes(TELEMETRY_URL)) return;

    if (typeof navigator.sendBeacon === 'function') {
      const blob = new Blob([JSON.stringify(data)], { type: 'application/json' });
      const success = navigator.sendBeacon(TELEMETRY_URL, blob);
      if (success) return;
    }

    fetch(TELEMETRY_URL, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data)
    }).catch(function (err) {
      console.warn('Telemetry delivery failed:', err);
    });
  }

  global.TelemetryService = { init: init, report: report };

})(window);
