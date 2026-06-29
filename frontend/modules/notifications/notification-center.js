/**
 * Notification Center Module
 * Multi-channel notification system with bell icon, inbox drawer, and channel configuration.
 */
(function (global) {
  'use strict';

  var notifications = [
    { id: 'n-001', title: 'Critical: payment-api OOMKilled', severity: 'critical', channel: 'slack', message: 'Pod payment-api-7f8a was OOMKilled in production namespace', timestamp: '2026-06-25 12:15', read: false },
    { id: 'n-002', title: 'Deployment: order-service scaled to 5', severity: 'info', channel: 'email', message: 'Auto-scaling triggered for order-service due to high CPU', timestamp: '2026-06-25 11:30', read: false },
    { id: 'n-003', title: 'Warning: Node memory pressure', severity: 'warning', channel: 'slack', message: 'worker-node-03 memory usage at 94%', timestamp: '2026-06-25 08:45', read: true },
    { id: 'n-004', title: 'SLO Breach: order-service budget at 12%', severity: 'critical', channel: 'teams', message: 'Error budget for order-service SLO dropped below 15% threshold', timestamp: '2026-06-25 06:20', read: true },
    { id: 'n-005', title: 'Backup completed successfully', severity: 'info', channel: 'email', message: 'Daily backup snap-20260625-01 completed for prod-cluster-01', timestamp: '2026-06-25 04:00', read: true },
    { id: 'n-006', title: 'New deployment: analytics-v2.3.0', severity: 'info', channel: 'telegram', message: 'analytics-engine deployed version v2.3.0 to staging', timestamp: '2026-06-24 22:10', read: true }
  ];

  var channels = [
    { id: 'slack', name: 'Slack', icon: '💬', configured: true, webhook: 'https://hooks.slack.com/services/T00.../B00.../xxx', channel: '#sre-alerts' },
    { id: 'email', name: 'Email', icon: '📧', configured: true, smtp: 'smtp.company.com:587', recipients: 'sre-team@company.com' },
    { id: 'teams', name: 'Microsoft Teams', icon: '🟦', configured: true, webhook: 'https://company.webhook.office.com/...' },
    { id: 'telegram', name: 'Telegram', icon: '✈️', configured: false, botToken: '', chatId: '' }
  ];

  var notificationHistory = [
    { id: 'h-001', notification: 'Critical: payment-api OOMKilled', channel: 'Slack #sre-alerts', status: 'delivered', timestamp: '2026-06-25 12:15:02' },
    { id: 'h-002', notification: 'Critical: payment-api OOMKilled', channel: 'Email sre-team@company.com', status: 'delivered', timestamp: '2026-06-25 12:15:05' },
    { id: 'h-003', notification: 'Deployment: order-service scaled', channel: 'Email sre-team@company.com', status: 'delivered', timestamp: '2026-06-25 11:30:12' },
    { id: 'h-004', notification: 'Warning: Node memory pressure', channel: 'Slack #sre-alerts', status: 'delivered', timestamp: '2026-06-25 08:45:03' },
    { id: 'h-005', notification: 'SLO Breach: order-service', channel: 'Teams webhook', status: 'failed', timestamp: '2026-06-25 06:20:15' },
    { id: 'h-006', notification: 'SLO Breach: order-service', channel: 'Teams webhook', status: 'retried', timestamp: '2026-06-25 06:20:45' }
  ];

  /* ─── Bell Icon ─── */
  function updateBellIcon() {
    var unread = notifications.filter(function(n){ return !n.read; }).length;
    var badge = document.getElementById('notif-bell-badge');
    if (badge) {
      badge.textContent = unread;
      badge.style.display = unread > 0 ? 'flex' : 'none';
    }
  }

  function createBellIcon() {
    var topBarRight = document.querySelector('.top-bar-right');
    if (!topBarRight || document.getElementById('notif-bell')) return;
    var bell = document.createElement('div');
    bell.id = 'notif-bell';
    bell.style.cssText = 'position:relative;cursor:pointer;font-size:18px;margin-right:8px;';
    bell.innerHTML = '🔔<span id="notif-bell-badge" style="position:absolute;top:-6px;right:-8px;background:#ef4444;color:#fff;font-size:9px;font-weight:700;min-width:16px;height:16px;border-radius:8px;display:flex;align-items:center;justify-content:center;">0</span>';
    bell.addEventListener('click', function(){ toggleInbox(); });
    topBarRight.insertBefore(bell, topBarRight.firstChild);
    updateBellIcon();
  }

  /* ─── Inbox Drawer ─── */
  function toggleInbox() {
    var drawer = document.getElementById('notif-inbox-drawer');
    if (!drawer) return;
    drawer.style.display = drawer.style.display === 'none' ? 'block' : 'none';
    if (drawer.style.display === 'block') renderInbox();
  }

  function createInboxDrawer() {
    if (document.getElementById('notif-inbox-drawer')) return;
    var drawer = document.createElement('div');
    drawer.id = 'notif-inbox-drawer';
    drawer.style.cssText = 'display:none;position:fixed;top:50px;right:16px;width:380px;max-height:500px;background:var(--color-bg);border:1px solid var(--color-hairline);border-radius:12px;box-shadow:0 12px 40px rgba(0,0,0,0.5);z-index:9999;overflow:hidden;';
    drawer.innerHTML = '<div style="padding:12px 16px;border-bottom:1px solid var(--color-hairline);display:flex;align-items:center;"><strong>🔔 Notifications</strong><span id="notif-inbox-count" style="margin-left:8px;font-size:11px;color:var(--color-muted);"></span><button class="btn btn-ghost btn-sm" style="margin-left:auto;font-size:11px;" onclick="NotificationCenter.markAllRead()">Mark all read</button></div>'
      + '<div id="notif-inbox-list" style="max-height:400px;overflow-y:auto;"></div>';
    document.body.appendChild(drawer);
  }

  function renderInbox() {
    var list = document.getElementById('notif-inbox-list');
    var count = document.getElementById('notif-inbox-count');
    if (!list) return;
    var unread = notifications.filter(function(n){ return !n.read; }).length;
    if (count) count.textContent = unread + ' unread';

    if (notifications.length === 0) {
      list.innerHTML = UIComponents.emptyState({
        title: 'Inbox is Empty',
        description: 'You have no system notifications or channel alerts at this time.',
        icon: '🔔'
      });
      return;
    }

    list.innerHTML = notifications.map(function(n) {
      var sevColors = { critical: '#ef4444', warning: '#eab308', info: '#06b6d4' };
      var bg = n.read ? 'transparent' : 'var(--color-surface-elevated)';
      return '<div style="padding:10px 16px;border-bottom:1px solid var(--color-hairline);background:' + bg + ';cursor:pointer;" onclick="NotificationCenter.markRead(\'' + n.id + '\')">'
        + '<div style="display:flex;align-items:center;gap:6px;margin-bottom:2px;">'
        + '<span style="width:8px;height:8px;border-radius:50%;background:' + (sevColors[n.severity] || '#6b7280') + ';flex-shrink:0;"></span>'
        + '<strong style="font-size:13px;flex:1;">' + n.title + '</strong>'
        + (!n.read ? '<span style="width:6px;height:6px;border-radius:50%;background:var(--color-primary);"></span>' : '')
        + '</div>'
        + '<div style="font-size:11px;color:var(--color-muted);margin-left:14px;">' + n.timestamp + ' · ' + n.channel + '</div>'
        + '</div>';
    }).join('');
  }

  /* ─── Channels Config ─── */
  function renderChannels() {
    var container = document.getElementById('notif-channels-grid');
    if (!container) return;
    container.innerHTML = channels.map(function(ch) {
      var statusDot = ch.configured ? '<span style="color:#10b981;">● Connected</span>' : '<span style="color:#6b7280;">○ Not configured</span>';
      return '<div class="panel" style="padding:var(--space-md);">'
        + '<div style="display:flex;align-items:center;gap:8px;margin-bottom:var(--space-sm);">'
        + '<span style="font-size:24px;">' + ch.icon + '</span>'
        + '<div><strong>' + ch.name + '</strong><br>' + statusDot + '</div>'
        + '</div>'
        + '<button class="btn btn-ghost btn-sm" onclick="NotificationCenter.configureChannel(\'' + ch.id + '\')">' + (ch.configured ? 'Reconfigure' : 'Configure') + '</button>'
        + '</div>';
    }).join('');
  }

  /* ─── History ─── */
  function renderHistory() {
    var tbody = document.getElementById('notif-history-tbody');
    if (!tbody) return;

    if (notificationHistory.length === 0) {
      tbody.innerHTML = `<tr><td colspan="5" style="padding:0;">` + UIComponents.emptyState({
        title: 'No Notification History',
        description: 'Audit logs of sent channel notifications and webhooks will appear here.',
        icon: '📋'
      }) + `</td></tr>`;
      return;
    }

    tbody.innerHTML = notificationHistory.map(function(h) {
      var statusColor = h.status === 'delivered' ? '#10b981' : (h.status === 'failed' ? '#ef4444' : '#eab308');
      return '<tr>'
        + '<td style="font-size:12px;">' + h.notification + '</td>'
        + '<td style="font-size:12px;">' + h.channel + '</td>'
        + '<td><span style="color:' + statusColor + ';font-weight:600;font-size:12px;">' + h.status + '</span></td>'
        + '<td style="font-size:12px;color:var(--color-muted);">' + h.timestamp + '</td>'
        + '<td>' + (h.status === 'failed' ? '<button class="btn btn-ghost btn-sm" style="font-size:11px;">Retry</button>' : '') + '</td>'
        + '</tr>';
    }).join('');
  }

  /* ─── Public API ─── */
  var NotificationCenter = {
    init: function() {
      createBellIcon();
      createInboxDrawer();
      UIComponents.initTabs('notif-tab-btn', 'notif-tab-panel', 'data-notif-tab');
      this.refresh();
    },
    refresh: function() {
      renderChannels();
      renderHistory();
      updateBellIcon();
    },
    markRead: function(id) {
      notifications.forEach(function(n){ if (n.id === id) n.read = true; });
      updateBellIcon();
      renderInbox();
    },
    markAllRead: function() {
      notifications.forEach(function(n){ n.read = true; });
      updateBellIcon();
      renderInbox();
    },
    configureChannel: function(chId) {
      if (global.Modal && global.Modal.open) {
        var ch = channels.find(function(c){ return c.id === chId; });
        global.Modal.open({
          title: ch.icon + ' Configure ' + ch.name,
          body: '<div style="padding:var(--space-xs);">'
            + '<div class="form-group"><label class="form-label">Webhook URL</label><input type="text" class="form-select" placeholder="https://..." value="' + (ch.webhook || '') + '"></div>'
            + '<div style="display:flex;gap:8px;margin-top:var(--space-sm);">'
            + '<button class="btn btn-primary btn-sm" onclick="alert(\'Configuration saved for ' + ch.name + '\');Modal.close();">Save</button>'
            + '<button class="btn btn-ghost btn-sm" onclick="Modal.close();">Cancel</button>'
            + '</div></div>'
        });
      }
    }
  };

  global.NotificationCenter = NotificationCenter;
})(window);
