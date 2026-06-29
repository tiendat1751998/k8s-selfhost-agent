/**
 * Incidents Panel — DOM updater for live incident stream.
 */
(function (global) {
  'use strict';

  const listEl = document.getElementById('incident-list');
  const emptyEl = document.getElementById('incident-empty');
  const countEl = document.getElementById('incident-count');
  let currentFilter = 'all';

  function init() {
    // Severity filter buttons
    document.querySelectorAll('#incident-panel [data-filter]').forEach(btn => {
      btn.addEventListener('click', () => {
        document.querySelectorAll('#incident-panel [data-filter]').forEach(b => b.classList.remove('active'));
        btn.classList.add('active');
        currentFilter = btn.dataset.filter;
        applyFilter();
      });
    });

    // Listen for new incidents
    AppState.on('incident', renderIncident);
  }

  function renderIncident(inc) {
    if (emptyEl) emptyEl.style.display = 'none';

    const card = document.createElement('div');
    card.className = 'incident-card';
    card.dataset.severity = inc.severity || 'info';

    const severity = inc.severity || 'info';
    const type = inc.type || 'Unknown';
    const cluster = inc.cluster || inc.clusterName || '—';
    const ns = inc.namespace || '—';
    const pod = inc.podName || '';
    const time = formatTime(inc.timestamp);

    card.innerHTML =
      '<div class="incident-severity ' + severity + '"></div>' +
      '<div class="incident-info">' +
        '<div class="incident-type">' + esc(type) + '</div>' +
        '<div class="incident-meta">' +
          '<span>📍 ' + esc(cluster) + '</span>' +
          '<span>📦 ' + esc(ns) + '</span>' +
          (pod ? '<span>🔹 ' + esc(pod) + '</span>' : '') +
        '</div>' +
      '</div>' +
      '<div class="incident-time">' + time + '</div>';

    // Prepend
    if (listEl.firstChild && listEl.firstChild !== emptyEl) {
      listEl.insertBefore(card, listEl.firstChild);
    } else {
      listEl.appendChild(card);
    }

    // Enforce limit in DOM (max 50 cards)
    while (listEl.querySelectorAll('.incident-card').length > 50) {
      const last = listEl.querySelector('.incident-card:last-child');
      if (last) last.remove();
    }

    updateCount();
    applyFilter();
  }

  function applyFilter() {
    const cards = listEl.querySelectorAll('.incident-card');
    cards.forEach(card => {
      if (currentFilter === 'all' || card.dataset.severity === currentFilter) {
        card.style.display = '';
      } else {
        card.style.display = 'none';
      }
    });
  }

  function updateCount() {
    const count = listEl.querySelectorAll('.incident-card').length;
    countEl.textContent = count;
  }

  function formatTime(ts) {
    if (!ts) return 'now';
    try {
      const d = new Date(ts);
      const now = new Date();
      const diffMs = now - d;
      if (diffMs < 60000) return Math.floor(diffMs / 1000) + 's ago';
      if (diffMs < 3600000) return Math.floor(diffMs / 60000) + 'm ago';
      if (diffMs < 86400000) return Math.floor(diffMs / 3600000) + 'h ago';
      return d.toLocaleDateString();
    } catch (e) { return 'now'; }
  }

  global.IncidentsPanel = { init };

})(window);
