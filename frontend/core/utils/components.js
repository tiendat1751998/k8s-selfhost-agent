/**
 * UI Component Helpers — Reusable DOM rendering utilities.
 * Eliminates repetitive HTML string construction across module scripts.
 */
const UIComponents = {

  /**
   * Render a data table with optional pagination.
   * @param {Object} opts - { columns: [{key, label, render?}], data: [], emptyText?, tableId?, tbodyId? }
   */
  renderTable(opts) {
    const { columns = [], data = [], emptyText = 'No data available.', tbodyId } = opts;
    const tbody = tbodyId ? document.getElementById(tbodyId) : null;
    if (!tbody) return '';

    if (data.length === 0) {
      tbody.innerHTML = `<tr><td colspan="${columns.length}" class="text-center text-muted">${emptyText}</td></tr>`;
      return;
    }

    tbody.innerHTML = data.map(row => {
      const cells = columns.map(col => {
        const val = col.render ? col.render(row) : (row[col.key] || '');
        return `<td>${val}</td>`;
      }).join('');
      return `<tr>${cells}</tr>`;
    }).join('');
  },

  /**
   * Render a status badge.
   * @param {string} status - The status text.
   * @param {string} [variant] - Badge variant (success, danger, warning, info, outline, ghost).
   */
  badge(status, variant) {
    if (!variant) {
      const map = {
        healthy: 'success', active: 'success', running: 'success', completed: 'success', ready: 'success',
        critical: 'danger', error: 'danger', failed: 'danger',
        warning: 'warning', degraded: 'warning', pending: 'warning', drifted: 'warning', upgrading: 'warning',
        medium: 'info', info: 'info', generating: 'info',
      };
      variant = map[(status || '').toLowerCase()] || 'outline';
    }
    return `<span class="badge badge-${variant}">${(status || '').toUpperCase()}</span>`;
  },

  /**
   * Render a metric summary card.
   * @param {Object} opts - { value, label, variant?, icon? }
   */
  statCard(opts) {
    const { value = '0', label = '', variant = '', icon = '' } = opts;
    return `
      <div class="stat-card ${variant}">
        ${icon ? `<span class="stat-icon">${icon}</span>` : ''}
        <span class="stat-value">${value}</span>
        <span class="stat-label">${label}</span>
      </div>`;
  },

  /**
   * Render a grid of stat cards into a container.
   * @param {string} containerId - DOM element ID for the stats grid.
   * @param {Array} stats - Array of { value, label, variant?, icon? }
   */
  renderStats(containerId, stats) {
    const el = document.getElementById(containerId);
    if (!el) return;
    el.innerHTML = stats.map(s => this.statCard(s)).join('');
  },

  /**
   * Render a modal dialog.
   * @param {Object} opts - { modalId, title, body, footer? }
   */
  showModal(opts) {
    const { modalId, title = '', body = '', footer = '' } = opts;
    const modal = document.getElementById(modalId);
    if (!modal) return;

    const titleEl = modal.querySelector('.modal-header h3, [class*="modal-header"] h3');
    if (titleEl) titleEl.textContent = title;

    const bodyEl = modal.querySelector('.modal-body, [class*="modal-body"]');
    if (bodyEl) bodyEl.innerHTML = body;

    if (footer) {
      const footerEl = modal.querySelector('.modal-footer, [class*="modal-footer"]');
      if (footerEl) footerEl.innerHTML = footer;
    }

    modal.style.display = 'flex';
  },

  /**
   * Format a date string or Date object to locale string.
   * @param {string|Date} d - The date to format.
   * @param {string} [fallback=''] - Fallback text if date is null/invalid.
   */
  formatDate(d, fallback = '') {
    if (!d) return fallback;
    try {
      const date = new Date(d);
      const settings = (window.AppState && typeof window.AppState.getState === 'function')
        ? window.AppState.getState().settings
        : {};
      const tz = settings.timezone || 'local';
      const format = settings.dateFormat || 'relative';

      if (format === 'iso') {
        return tz === 'utc' ? date.toISOString() : date.toLocaleString('sv-SE').replace(' ', 'T');
      }

      const options = tz === 'utc' ? { timeZone: 'UTC' } : {};
      return date.toLocaleString(undefined, options);
    } catch (e) {
      return fallback;
    }
  },

  /**
   * Format relative time (e.g., "5m ago").
   * @param {string|Date} d - The date to calculate from.
   */
  timeAgo(d) {
    if (!d) return '';
    const settings = (window.AppState && typeof window.AppState.getState === 'function')
      ? window.AppState.getState().settings
      : {};
    if (settings.dateFormat === 'absolute') {
      return this.formatDate(d);
    }
    if (settings.dateFormat === 'iso') {
      return new Date(d).toISOString();
    }

    const now = Date.now();
    const diff = now - new Date(d).getTime();
    if (diff < 0) return 'just now';
    const mins = Math.floor(diff / 60000);
    if (mins < 1) return 'just now';
    if (mins < 60) return `${mins}m ago`;
    const hrs = Math.floor(mins / 60);
    if (hrs < 24) return `${hrs}h ago`;
    const days = Math.floor(hrs / 24);
    return `${days}d ago`;
  },

  /**
   * Escape HTML entities to prevent XSS.
   * @param {string} str - Raw string.
   */
  escapeHtml(str) {
    if (!str) return '';
    const div = document.createElement('div');
    div.textContent = str;
    return div.innerHTML;
  },

  /**
   * Render a themed empty state card.
   * @param {Object} opts - { title, description, icon?, actionText?, actionId? }
   */
  emptyState(opts) {
    const { title = 'No records found', description = 'Try adjusting your filters or reloading.', icon = '🔍', actionText = '', actionId = '' } = opts;
    return `
      <div class="empty-state-card" style="display:flex;flex-direction:column;align-items:center;justify-content:center;padding:var(--space-xl);text-align:center;background:var(--color-surface-elevated);border:1px dashed var(--color-hairline);border-radius:var(--rounded-lg);margin:var(--space-md) 0;animation:fadeIn 0.3s ease;">
        <span class="empty-state-icon" style="font-size:36px;margin-bottom:var(--space-md);filter:drop-shadow(0 4px 6px rgba(0,0,0,0.15));">${icon}</span>
        <h4 class="empty-state-title" style="font-size:var(--text-body-lg);font-weight:700;color:var(--color-on-dark);margin:0 0 var(--space-xs) 0;">` + title + `</h4>
        <p class="empty-state-description" style="font-size:var(--text-body-sm);color:var(--color-muted);margin:0 0 var(--space-md) 0;max-width:320px;line-height:1.5;">` + description + `</p>
        ` + (actionText && actionId ? `<button class="btn btn-primary btn-sm" id="` + actionId + `">` + actionText + `</button>` : '') + `
      </div>
    `;
  },

  /**
   * Centralized tab selection handler.
   */
  initTabs(btnClass, panelClass, attrName) {
    document.querySelectorAll('.' + btnClass).forEach(function (tab) {
      tab.addEventListener('click', function () {
        document.querySelectorAll('.' + btnClass).forEach(function (t) { t.classList.remove('active'); });
        tab.classList.add('active');
        const target = tab.getAttribute(attrName);
        document.querySelectorAll('.' + panelClass).forEach(function (p) {
          p.style.display = p.id === target ? 'block' : 'none';
        });
      });
    });
  },

  /**
   * Centralized execution result status badge.
   */
  resultBadge(s) {
    if (s === 'success' || s === 'delivered') return '<span class="badge badge-healthy">✓ Success</span>';
    if (s === 'failed') return '<span class="badge badge-down">✗ Failed</span>';
    return '<span class="badge badge-degraded">' + Security.escapeHTML(s) + '</span>';
  }
};

window.UIComponents = UIComponents;
window.esc = Security.escapeHTML;
window.timeAgo = UIComponents.timeAgo;

