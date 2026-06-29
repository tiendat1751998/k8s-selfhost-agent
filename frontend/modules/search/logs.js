/**
 * Log Stream Panel — Real-time log streaming with auto-scroll,
 * pause/resume, keyword filter, and 200-line buffer.
 * Optimized for high performance (60 FPS) rendering via requestAnimationFrame,
 * DOM caching, and filter debouncing.
 */
(function (global) {
  'use strict';

  const MAX_LINES = 200;
  const listEl = document.getElementById('log-list');
  const emptyEl = document.getElementById('log-empty');
  const countEl = document.getElementById('log-count');
  const filterInput = document.getElementById('log-filter');
  const pauseBtn = document.getElementById('log-pause-btn');
  const clearBtn = document.getElementById('log-clear-btn');

  let paused = false;
  let filterText = '';
  let pendingLogs = [];
  let renderRequested = false;
  let filterTimeout = null;

  // Cache rendered line structures to prevent DOM queries on filtering/flushing
  let renderedLines = [];

  function init() {
    AppState.on('log', onLog);

    if (pauseBtn) {
      pauseBtn.addEventListener('click', togglePause);
    }

    if (clearBtn) {
      clearBtn.addEventListener('click', clearLogs);
    }

    if (filterInput) {
      filterInput.addEventListener('input', function () {
        // Debounce input to prevent layout thrashing while typing
        if (filterTimeout) clearTimeout(filterTimeout);
        filterTimeout = setTimeout(() => {
          filterText = this.value.toLowerCase();
          applyFilter();
        }, 150);
      });
    }
  }

  function onLog(log) {
    if (paused) {
      pendingLogs.push(log);
      if (pendingLogs.length > MAX_LINES) pendingLogs.shift();
      return;
    }

    pendingLogs.push(log);

    // Batch UI updates using requestAnimationFrame for smooth 60 FPS performance
    if (!renderRequested) {
      renderRequested = true;
      requestAnimationFrame(flushLogs);
    }
  }

  function flushLogs() {
    renderRequested = false;
    if (pendingLogs.length === 0) return;

    if (emptyEl && emptyEl.style.display !== 'none') {
      emptyEl.style.display = 'none';
    }

    const fragment = document.createDocumentFragment();

    pendingLogs.forEach(log => {
      const lineEl = createLogLine(log);
      fragment.appendChild(lineEl);
      renderedLines.push({
        element: lineEl,
        text: log.text.toLowerCase()
      });
    });

    listEl.appendChild(fragment);
    pendingLogs = [];

    // Enforce log limit by pruning from head of cache and removing DOM elements
    while (renderedLines.length > MAX_LINES) {
      const removed = renderedLines.shift();
      if (removed && removed.element) {
        removed.element.remove();
      }
    }

    // Update line count label
    if (countEl) {
      countEl.textContent = renderedLines.length;
    }

    // Apply active filter instantly to new items
    applyFilter();

    // Smooth auto-scroll to bottom
    listEl.scrollTop = listEl.scrollHeight;
  }

  function createLogLine(log) {
    const line = document.createElement('div');
    line.className = 'log-line';

    const ts = document.createElement('span');
    ts.className = 'log-timestamp';
    ts.textContent = formatTimestamp(log.timestamp);

    const content = document.createElement('span');
    content.className = 'log-content ' + detectLevel(log.text);
    content.textContent = log.text;

    line.appendChild(ts);
    line.appendChild(content);

    return line;
  }

  function detectLevel(text) {
    const lower = text.toLowerCase();
    if (lower.includes('error') || lower.includes('fatal') || lower.includes('panic') || lower.includes('oomkilled')) return 'error';
    if (lower.includes('warn') || lower.includes('warning')) return 'warn';
    if (lower.includes('success') || lower.includes('resolved') || lower.includes('healthy')) return 'success';
    if (lower.includes('info')) return 'info';
    return '';
  }

  function formatTimestamp(ts) {
    if (!ts) return '';
    try {
      const d = new Date(ts);
      return d.toLocaleTimeString('en-US', { hour12: false, hour: '2-digit', minute: '2-digit', second: '2-digit' });
    } catch (e) { return ''; }
  }

  function applyFilter() {
    renderedLines.forEach(line => {
      const visible = !filterText || line.text.includes(filterText);
      line.element.style.display = visible ? '' : 'none';
    });
  }

  function togglePause() {
    paused = !paused;
    if (pauseBtn) {
      pauseBtn.textContent = paused ? '▶' : '⏸';
      pauseBtn.classList.toggle('active', paused);
    }

    if (!paused && pendingLogs.length > 0) {
      flushLogs();
    }
  }

  function clearLogs() {
    renderedLines.forEach(line => {
      if (line.element) line.element.remove();
    });
    renderedLines = [];
    if (emptyEl) emptyEl.style.display = '';
    if (countEl) countEl.textContent = '0';
    pendingLogs = [];
  }

  global.LogsPanel = { init };

})(window);
