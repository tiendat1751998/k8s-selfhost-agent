/**
 * Settings Section — Global configuration with localStorage persistence,
 * connection status indicators, export/import, and danger zone actions.
 */
(function (global) {
  'use strict';

  var STORAGE_KEY = 'k8s_control_plane_settings';

  // Intentional default settings configuration
  var defaults = {
    theme: 'dark',
    refreshInterval: '10',
    logRetention: '200',
    incidentBuffer: '50',
    notifications: 'critical',
    dateFormat: 'relative',
    timezone: 'local'
  };

  var fieldIds = {
    theme: 'setting-theme',
    refreshInterval: 'setting-refresh',
    logRetention: 'setting-log-retention',
    incidentBuffer: 'setting-incident-buffer',
    notifications: 'setting-notifications',
    dateFormat: 'setting-date-format',
    timezone: 'setting-timezone'
  };

  function init() {
    loadSettings();
    bindEvents();
    renderFeatureFlags();
    AppState.on('navigate', function (section) {
      if (section === 'settings') {
        updateConnectionStatus();
        renderFeatureFlags();
      }
    });
    AppState.on('connectionHealth', updateConnectionStatus);
    AppState.on('connection', updateConnectionStatus);
    // Initial update after data loads
    setTimeout(updateConnectionStatus, 1000);
    setTimeout(updateConnectionStatus, 4000);
  }

  function renderFeatureFlags() {
    var container = document.getElementById('feature-flags-container');
    if (!container) return;

    if (!global.FeatureFlags) {
      container.innerHTML = '<div style="color:var(--color-trading-down)">FeatureFlags module not loaded</div>';
      return;
    }

    var flags = global.FeatureFlags.getAll();
    var metadata = global.FeatureFlags.getMetadata();
    var html = '';

    Object.keys(metadata).forEach(function (key) {
      var item = metadata[key];
      var checked = flags[key] ? 'checked' : '';
      html += `
        <div class="switch-container">
          <div class="switch-info">
            <span class="switch-label">` + Security.escapeHTML(item.label) + `</span>
            <span class="switch-description">` + Security.escapeHTML(item.description) + `</span>
          </div>
          <label class="switch">
            <input type="checkbox" class="feature-flag-toggle" data-flag="` + Security.escapeHTML(key) + `" ` + checked + `>
            <span class="slider"></span>
          </label>
        </div>
      `;
    });

    container.innerHTML = html;

    // Bind change events to all switches
    var toggles = container.querySelectorAll('.feature-flag-toggle');
    toggles.forEach(function (toggle) {
      toggle.addEventListener('change', function () {
        var flagKey = this.getAttribute('data-flag');
        var enabled = this.checked;
        global.FeatureFlags.setEnabled(flagKey, enabled);
      });
    });
  }

  function loadSettings() {
    var saved = {};
    try {
      var raw = localStorage.getItem(STORAGE_KEY);
      if (raw) saved = JSON.parse(raw);
    } catch (e) { /* ignore */ }

    var settings = {};
    Object.keys(defaults).forEach(function (key) {
      settings[key] = saved[key] || defaults[key];
    });

    // Apply to UI
    Object.keys(fieldIds).forEach(function (key) {
      var el = document.getElementById(fieldIds[key]);
      if (el) el.value = settings[key];
    });

    // Publish to state
    AppState.setSettings(settings);
    return settings;
  }

  function saveSettings() {
    var settings = {};
    Object.keys(fieldIds).forEach(function (key) {
      var el = document.getElementById(fieldIds[key]);
      if (el) settings[key] = el.value;
    });

    try {
      localStorage.setItem(STORAGE_KEY, JSON.stringify(settings));
    } catch (e) {
      console.error('[Settings] Failed to save:', e);
    }

    AppState.setSettings(settings);
    AppState.addAuditLog({ action: 'update', target: 'system/settings', result: 'success' });
    showToast('Settings saved successfully');
  }

  function bindEvents() {
    var saveBtn = document.getElementById('settings-save-btn');
    var exportBtn = document.getElementById('settings-export-btn');
    var importBtn = document.getElementById('settings-import-btn');
    var importFile = document.getElementById('settings-import-file');
    var resetBtn = document.getElementById('settings-reset-btn');
    var clearCacheBtn = document.getElementById('settings-clear-cache-btn');

    if (saveBtn) saveBtn.addEventListener('click', saveSettings);
    if (exportBtn) exportBtn.addEventListener('click', exportSettings);
    if (importBtn) importBtn.addEventListener('click', function () {
      if (importFile) importFile.click();
    });
    if (importFile) importFile.addEventListener('change', importSettings);
    if (resetBtn) resetBtn.addEventListener('click', function () {
      if (confirm('Reset all settings to defaults? This cannot be undone.')) {
        localStorage.removeItem(STORAGE_KEY);
        loadSettings();
        AppState.addAuditLog({ action: 'delete', target: 'system/settings', result: 'success' });
        showToast('Settings reset to defaults');
      }
    });
    if (clearCacheBtn) clearCacheBtn.addEventListener('click', function () {
      if (confirm('Clear all cached data? The page will reload.')) {
        Object.keys(localStorage).forEach(function (key) {
          if (key.startsWith('k8s_')) localStorage.removeItem(key);
        });
        AppState.addAuditLog({ action: 'delete', target: 'system/cache', result: 'success' });
        location.reload();
      }
    });
  }

  function exportSettings() {
    var settings = {};
    Object.keys(fieldIds).forEach(function (key) {
      var el = document.getElementById(fieldIds[key]);
      if (el) settings[key] = el.value;
    });

    var state = AppState.getState();
    var exportData = {
      version: '1.0',
      exportedAt: new Date().toISOString(),
      settings: settings,
      kubernetes: state.kubernetes || [],
      gitProviders: state.gitProviders || [],
      cicd: state.cicd || [],
      aiProviders: state.aiProviders || []
    };

    var blob = new Blob([JSON.stringify(exportData, null, 2)], { type: 'application/json' });
    var url = URL.createObjectURL(blob);
    var a = document.createElement('a');
    a.href = url;
    a.download = 'k8s-control-plane-config-' + new Date().toISOString().slice(0, 10) + '.json';
    a.click();
    URL.revokeObjectURL(url);

    AppState.addAuditLog({ action: 'export', target: 'system/config', result: 'success' });
    showToast('Configuration exported');
  }

  function importSettings(e) {
    var file = e.target.files && e.target.files[0];
    if (!file) return;

    var reader = new FileReader();
    reader.onload = function (ev) {
      try {
        var data = JSON.parse(ev.target.result);
        if (!data || !data.settings) {
          showToast('Invalid config file', true);
          return;
        }

        // Apply settings
        Object.keys(fieldIds).forEach(function (key) {
          var el = document.getElementById(fieldIds[key]);
          if (el && data.settings[key]) el.value = data.settings[key];
        });

        saveSettings();

        // Import providers if present
        if (data.kubernetes) AppState.setKubernetes(data.kubernetes);
        if (data.gitProviders) AppState.setGitProviders(data.gitProviders);
        if (data.cicd) AppState.setCicd(data.cicd);
        if (data.aiProviders) AppState.setAiProviders(data.aiProviders);

        AppState.addAuditLog({ action: 'import', target: 'system/config', result: 'success' });
        showToast('Configuration imported successfully');
      } catch (err) {
        showToast('Failed to import: ' + err.message, true);
      }
    };
    reader.readAsText(file);
    e.target.value = ''; // reset file input
  }

  function updateConnectionStatus() {
    var state = AppState.getState();
    var conn = state.connection || 'offline';
    var health = state.connectionHealth || {};

    setStatusEl('settings-ws-status', conn === 'online' ? 'healthy' : conn === 'connecting' ? 'degraded' : 'down');
    setStatusEl('settings-k8s-status', health.k8s ? health.k8s.status : 'unknown');
    setStatusEl('settings-git-status', health.git ? health.git.status : 'unknown');
    setStatusEl('settings-cicd-status', health.cicd ? health.cicd.status : 'unknown');
    setStatusEl('settings-ai-status', health.ai ? health.ai.status : 'unknown');
    setStatusEl('settings-db-status', 'unknown');
    setStatusEl('settings-redis-status', 'unknown');
    setStatusEl('settings-nats-status', 'unknown');
  }

  function setStatusEl(id, status) {
    var el = document.getElementById(id);
    if (!el) return;
    var colors = { healthy: 'var(--color-trading-up)', degraded: '#f0b90b', down: 'var(--color-trading-down)', unknown: 'var(--color-muted)' };
    var labels = { healthy: '● Online', degraded: '● Degraded', down: '● Offline', unknown: '○ Unknown' };
    el.style.color = colors[status] || colors.unknown;
    el.textContent = labels[status] || labels.unknown;
  }

  function showToast(msg, isError) {
    var toast = document.createElement('div');
    toast.style.cssText = 'position:fixed;bottom:24px;right:24px;padding:12px 24px;border-radius:8px;font-size:14px;font-weight:500;z-index:9999;animation:fadeIn .3s ease;' +
      (isError ? 'background:var(--color-trading-down);color:#fff;' : 'background:var(--color-primary);color:#0b0e11;');
    toast.textContent = msg;
    document.body.appendChild(toast);
    setTimeout(function () {
      toast.style.opacity = '0';
      toast.style.transition = 'opacity .3s ease';
      setTimeout(function () { toast.remove(); }, 300);
    }, 3000);
  }

  global.SettingsSection = { init: init };
})(window);
