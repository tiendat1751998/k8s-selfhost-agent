/**
 * Personalization & Dashboard Customization Module
 */
(function (global) {
  'use strict';

  var WIDGETS_KEY = 'k8s_dashboard_widgets';
  var FAVORITES_KEY = 'k8s_favorites_clusters';
  var RECENT_KEY = 'k8s_recent_views';

  // Default widget visibility
  var defaultWidgets = {
    incidents: true,
    agent: true,
    logs: true,
    favorites: true,
    recent: true
  };

  var widgetPanelIds = {
    incidents: 'incident-panel',
    agent: 'agent-panel',
    logs: 'log-panel',
    favorites: 'favorites-panel',
    recent: 'recent-panel'
  };

  var sectionLabels = {
    'kubernetes': '☸️ Container Clusters',
    'docker-swarm': '🐳 Docker Swarm',
    'incidents': '🚨 Incidents Center',
    'ai-providers': '🤖 AI Providers',
    'settings': '⚙️ Settings & Prefs',
    'observability': '📊 Observability',
    'topology': '🗺️ Topology Map',
    'deployment-center': '🚀 Applications',
    'enterprise': '🏢 Enterprise Control'
  };

  function init() {
    var customizeBtn = document.getElementById('overview-customize-btn');
    if (customizeBtn) {
      customizeBtn.addEventListener('click', openCustomizeModal);
    }

    // Apply saved widget visibility
    applyWidgetVisibility();

    // Listen to AppState dynamic navigations to track recently viewed sections
    AppState.on('navigate', function (sectionId) {
      recordRecentView(sectionId);
    });

    // Listen to AppState kubernetes cluster load to refresh favorites
    AppState.on('kubernetes', function () {
      updateFavorites();
    });

    // Initial renders
    updateFavorites();
    updateRecentViews();
  }

  function getSavedWidgets() {
    var saved = {};
    try {
      var raw = localStorage.getItem(WIDGETS_KEY);
      if (raw) saved = JSON.parse(raw);
    } catch (e) { /* ignore */ }

    var widgets = {};
    Object.keys(defaultWidgets).forEach(function (key) {
      widgets[key] = saved[key] !== undefined ? saved[key] : defaultWidgets[key];
    });
    return widgets;
  }

  function applyWidgetVisibility() {
    var widgets = getSavedWidgets();
    Object.keys(widgets).forEach(function (key) {
      var el = document.getElementById(widgetPanelIds[key]);
      if (el) {
        el.style.display = widgets[key] ? '' : 'none';
      }
    });
  }

  function openCustomizeModal() {
    var widgets = getSavedWidgets();
    Modal.open({
      title: '⚙️ Customize Dashboard Widgets',
      body:
        '<div style="display:flex;flex-direction:column;gap:12px;padding:8px 0;">' +
          '<label style="display:flex;align-items:center;gap:8px;font-size:14px;cursor:pointer;">' +
            '<input type="checkbox" id="w-incidents" ' + (widgets.incidents ? 'checked' : '') + '> ' +
            '<span>🚨 Incidents Panel</span>' +
          '</label>' +
          '<label style="display:flex;align-items:center;gap:8px;font-size:14px;cursor:pointer;">' +
            '<input type="checkbox" id="w-agent" ' + (widgets.agent ? 'checked' : '') + '> ' +
            '<span>🤖 Agent Pipeline Panel</span>' +
          '</label>' +
          '<label style="display:flex;align-items:center;gap:8px;font-size:14px;cursor:pointer;">' +
            '<input type="checkbox" id="w-logs" ' + (widgets.logs ? 'checked' : '') + '> ' +
            '<span>📋 Live Logs Panel</span>' +
          '</label>' +
          '<label style="display:flex;align-items:center;gap:8px;font-size:14px;cursor:pointer;">' +
            '<input type="checkbox" id="w-favorites" ' + (widgets.favorites ? 'checked' : '') + '> ' +
            '<span>⭐ Favorites Panel</span>' +
          '</label>' +
          '<label style="display:flex;align-items:center;gap:8px;font-size:14px;cursor:pointer;">' +
            '<input type="checkbox" id="w-recent" ' + (widgets.recent ? 'checked' : '') + '> ' +
            '<span>👁️ Recently Viewed Panel</span>' +
          '</label>' +
        '</div>',
      actions: [
        { label: 'Cancel' },
        { label: 'Save Layout', primary: true, onClick: function () {
          var updated = {
            incidents: document.getElementById('w-incidents').checked,
            agent: document.getElementById('w-agent').checked,
            logs: document.getElementById('w-logs').checked,
            favorites: document.getElementById('w-favorites').checked,
            recent: document.getElementById('w-recent').checked
          };
          try {
            localStorage.setItem(WIDGETS_KEY, JSON.stringify(updated));
            applyWidgetVisibility();
            Modal.close();
          } catch (e) {
            alert('Failed to save layout preferences');
          }
        }}
      ]
    });
  }

  function updateFavorites() {
    var listEl = document.getElementById('favorites-list');
    if (!listEl) return;

    var favs = [];
    try {
      var raw = localStorage.getItem(FAVORITES_KEY);
      if (raw) favs = JSON.parse(raw);
    } catch (e) { /* ignore */ }

    if (favs.length === 0) {
      listEl.innerHTML = '<div class="empty-state"><div class="empty-state-icon">⭐</div><div class="empty-state-text">No favorites pinned</div><div class="empty-state-sub">Pin clusters from the Clusters section</div></div>';
      return;
    }

    var html = '<div style="display:grid;grid-template-columns:repeat(auto-fill, minmax(180px, 1fr));gap:12px;padding:8px 0;">';
    favs.forEach(function (name) {
      html +=
        '<div class="fav-item" style="padding:12px;background:var(--color-surface-elevated);border:1px solid var(--color-hairline);border-radius:var(--rounded-lg);cursor:pointer;display:flex;flex-direction:column;gap:4px;transition:border-color 0.2s;" data-id="' + esc(name) + '" onmouseover="this.style.borderColor=\'var(--color-primary)\'" onmouseout="this.style.borderColor=\'var(--color-hairline)\'">' +
          '<div style="display:flex;justify-content:space-between;align-items:center;">' +
            '<span style="font-weight:600;font-size:13px;color:var(--color-on-dark)">☸️ ' + esc(name) + '</span>' +
            '<span style="color:var(--color-primary);font-size:12px;">★</span>' +
          '</div>' +
          '<div style="font-size:11px;color:var(--color-muted)">Live cluster telemetry shortcut</div>' +
        '</div>';
    });
    html += '</div>';
    listEl.innerHTML = html;

    listEl.querySelectorAll('.fav-item').forEach(function (el) {
      el.addEventListener('click', function () {
        Router.navigate('kubernetes');
      });
    });
  }

  function recordRecentView(sectionId) {
    var label = sectionLabels[sectionId];
    if (!label) return;

    var list = [];
    try {
      var raw = localStorage.getItem(RECENT_KEY);
      if (raw) list = JSON.parse(raw);
    } catch (e) { /* ignore */ }

    // Remove existing entry for the same section
    list = list.filter(function (item) { return item.id !== sectionId; });
    
    // Unshift to front
    list.unshift({ id: sectionId, label: label, timestamp: Date.now() });

    // Cap to 5
    if (list.length > 5) list.pop();

    try {
      localStorage.setItem(RECENT_KEY, JSON.stringify(list));
    } catch (e) { /* ignore */ }

    updateRecentViews();
  }

  function updateRecentViews() {
    var listEl = document.getElementById('recent-list');
    if (!listEl) return;

    var list = [];
    try {
      var raw = localStorage.getItem(RECENT_KEY);
      if (raw) list = JSON.parse(raw);
    } catch (e) { /* ignore */ }

    if (list.length === 0) {
      listEl.innerHTML = '<div class="empty-state"><div class="empty-state-icon">👁️</div><div class="empty-state-text">No recently viewed sections</div></div>';
      return;
    }

    var html = '<div style="display:flex;flex-direction:column;gap:8px;padding:8px 0;">';
    list.forEach(function (item) {
      html +=
        '<div class="recent-item" style="display:flex;justify-content:space-between;align-items:center;padding:8px 12px;background:var(--color-surface-elevated);border-radius:var(--rounded-md);cursor:pointer;font-size:12px;font-weight:500;transition:background 0.2s;" data-id="' + esc(item.id) + '" onmouseover="this.style.background=\'var(--color-surface-hover)\'" onmouseout="this.style.background=\'var(--color-surface-elevated)\'">' +
          '<span>' + esc(item.label) + '</span>' +
          '<span style="font-size:10px;color:var(--color-muted);">' + timeAgo(item.timestamp) + '</span>' +
        '</div>';
    });
    html += '</div>';
    listEl.innerHTML = html;

    listEl.querySelectorAll('.recent-item').forEach(function (el) {
      el.addEventListener('click', function () {
        Router.navigate(this.dataset.id);
      });
    });
  }

  // Expose module APIs
  global.PersonalizationModule = {
    init: init,
    updateFavorites: updateFavorites,
    updateRecentViews: updateRecentViews
  };

})(window);
