/**
 * Audit Logs — Enterprise audit trail with search and filter.
 */
(function (global) {
  'use strict';

  var tableBody = document.getElementById('audit-table-body');
  var searchInput = document.getElementById('audit-search');
  var filterSelect = document.getElementById('audit-filter-action');

  function init() {
    AppState.on('auditLog', addRow);
    AppState.on('navigate', function (s) { if (s === 'audit-logs') renderAll(); });

    if (searchInput) searchInput.addEventListener('input', applyFilters);
    if (filterSelect) filterSelect.addEventListener('change', applyFilters);
  }

  function renderAll() {
    if (!tableBody) return;
    tableBody.innerHTML = '';
    var logs = AppState.getState().auditLogs || [];
    if (logs.length === 0) {
      tableBody.innerHTML = `<tr><td colspan="5" style="padding: 0;">` + UIComponents.emptyState({
        title: 'No Audit Logs Recorded',
        description: 'All system mutation actions, user accesses, and settings overrides will be logged here.',
        icon: '🕵️'
      }) + `</td></tr>`;
      return;
    }
    logs.forEach(function (log) { addRowDom(log); });
  }

  function addRow(log) {
    // If we previously had an empty state row, clean it
    if (tableBody && tableBody.querySelector('.empty-state-card')) {
      tableBody.innerHTML = '';
    }
    addRowDom(log);
    // Keep max 200 rows
    while (tableBody && tableBody.children.length > 200) {
      tableBody.removeChild(tableBody.lastChild);
    }
  }

  function addRowDom(log) {
    if (!tableBody) return;
    var tr = document.createElement('tr');
    tr.dataset.action = log.action || '';
    tr.dataset.searchText = [log.actor, log.action, log.target, log.result].join(' ').toLowerCase();

    tr.innerHTML =
      '<td style="font-family:var(--font-number);font-size:12px;color:var(--color-muted);white-space:nowrap">' + formatTimestamp(log.timestamp) + '</td>' +
      '<td>' + esc(log.actor || 'system') + '</td>' +
      '<td>' + actionBadge(log.action) + '</td>' +
      '<td><code style="font-size:12px">' + esc(log.target) + '</code></td>' +
      '<td>' + UIComponents.resultBadge(log.result) + '</td>';

    // Prepend (newest first)
    if (tableBody.firstChild) {
      tableBody.insertBefore(tr, tableBody.firstChild);
    } else {
      tableBody.appendChild(tr);
    }

    applyFilters();
  }

  function applyFilters() {
    if (!tableBody) return;
    var search = (searchInput ? searchInput.value.toLowerCase() : '');
    var actionFilter = (filterSelect ? filterSelect.value : '');

    var rows = tableBody.querySelectorAll('tr:not(.filter-empty-row)');
    var visibleCount = 0;
    rows.forEach(function (row) {
      var matchSearch = !search || (row.dataset.searchText && row.dataset.searchText.includes(search));
      var matchAction = !actionFilter || row.dataset.action === actionFilter;
      var show = matchSearch && matchAction;
      row.style.display = show ? '' : 'none';
      if (show) visibleCount++;
    });

    var existingEmpty = tableBody.querySelector('.filter-empty-row');
    if (existingEmpty) existingEmpty.remove();

    if (visibleCount === 0 && rows.length > 0) {
      var emptyTr = document.createElement('tr');
      emptyTr.className = 'filter-empty-row';
      emptyTr.innerHTML = `<td colspan="5" style="padding: 0;">` + UIComponents.emptyState({
        title: 'No Matching Logs',
        description: 'Try adjusting your search query or actions filter dropdown.',
        icon: '🔍'
      }) + `</td>`;
      tableBody.appendChild(emptyTr);
    }
  }

  function actionBadge(action) {
    var colors = { create: 'badge-healthy', update: 'badge-synced', delete: 'badge-down', test: 'badge-degraded', trigger: 'badge-synced', sync: 'badge-synced', rotate: 'badge-degraded', webhook: 'badge-synced' };
    var cls = colors[action] || 'badge-pending';
    return '<span class="badge ' + cls + '">' + esc(action || 'unknown') + '</span>';
  }

  function formatTimestamp(ts) {
    if (!ts) return '—';
    try { return new Date(ts).toLocaleString('en-US', { hour12: false, year: 'numeric', month: 'short', day: '2-digit', hour: '2-digit', minute: '2-digit', second: '2-digit' }); } catch(e) { return '—'; }
  }

  

  global.AuditLogsSection = { init: init };
})(window);
