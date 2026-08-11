/**
 * Notification Center Module
 * Multi-channel notification system with bell icon, inbox drawer, and channel configuration.
 */
(function (global) {
  'use strict';

  var notifications = [];
  var channels = [];
  var notificationHistory = [];

  var channelIcons = {
    slack: '💬',
    email: '📧',
    teams: '🟦',
    telegram: '✈️',
    webhook: '🔗'
  };

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
      var title = Security.escapeHTML(n.title || n.message || '');
      var severity = Security.escapeHTML(n.severity || '');
      var channelId = Security.escapeHTML(n.channel_id || '');
      var ch = channels.find(function(c) { return c.id === n.channel_id; });
      var channelName = ch ? ch.name : channelId;
      var timestamp = UIComponents.timeAgo(n.created_at) || Security.escapeHTML(n.created_at || '');

      return '<div style="padding:10px 16px;border-bottom:1px solid var(--color-hairline);background:' + bg + ';cursor:pointer;" onclick="NotificationCenter.markRead(\'' + n.id + '\')">'
        + '<div style="display:flex;align-items:center;gap:6px;margin-bottom:2px;">'
        + '<span style="width:8px;height:8px;border-radius:50%;background:' + (sevColors[n.severity] || '#6b7280') + ';flex-shrink:0;"></span>'
        + '<strong style="font-size:13px;flex:1;">' + title + '</strong>'
        + (!n.read ? '<span style="width:6px;height:6px;border-radius:50%;background:var(--color-primary);"></span>' : '')
        + '</div>'
        + '<div style="font-size:11px;color:var(--color-muted);margin-left:14px;">' + timestamp + ' · ' + Security.escapeHTML(channelName) + '</div>'
        + '</div>';
    }).join('');
  }

  /* ─── Channels Config ─── */
  function renderChannels() {
    var container = document.getElementById('notif-channels-grid');
    if (!container) return;

    var html = channels.map(function(ch) {
      var isConfigured = ch.enabled || (ch.webhook_url && ch.webhook_url.length > 0) || (ch.config && Object.keys(ch.config).length > 0);
      var statusDot = isConfigured ? '<span style="color:#10b981;">● Connected</span>' : '<span style="color:#6b7280;">○ Not configured</span>';
      var icon = channelIcons[ch.type] || '🔔';
      var name = Security.escapeHTML(ch.name);

      return '<div class="panel" style="padding:var(--space-md);">'
        + '<div style="display:flex;align-items:center;gap:8px;margin-bottom:var(--space-sm);">'
        + '<span style="font-size:24px;">' + icon + '</span>'
        + '<div><strong>' + name + '</strong><br>' + statusDot + '</div>'
        + '</div>'
        + '<button class="btn btn-ghost btn-sm" onclick="NotificationCenter.configureChannel(\'' + ch.id + '\')">' + (isConfigured ? 'Reconfigure' : 'Configure') + '</button>'
        + '<button class="btn btn-ghost btn-sm" style="color:var(--color-danger);margin-left:8px;" onclick="NotificationCenter.deleteChannel(\'' + ch.id + '\')">Delete</button>'
        + '</div>';
    }).join('');

    // Add "Add Webhook" card at the end
    html += '<div class="panel" style="padding:var(--space-md);display:flex;flex-direction:column;justify-content:center;align-items:center;border:1px dashed var(--color-hairline);min-height:120px;cursor:pointer;" onclick="NotificationCenter.configureChannel(null)">'
      + '<span style="font-size:24px;">➕</span>'
      + '<strong style="margin-top:8px;font-size:13px;">Add Webhook</strong>'
      + '</div>';

    container.innerHTML = html;
  }

  /* ─── History ─── */
  function renderHistory() {
    var tbody = document.getElementById('notif-history-tbody');
    if (!tbody) return;

    if (notificationHistory.length === 0) {
      tbody.innerHTML = '<tr><td colspan="5" style="padding:0;">' + UIComponents.emptyState({
        title: 'No Notification History',
        description: 'Audit logs of sent channel notifications and webhooks will appear here.',
        icon: '📋'
      }) + '</td></tr>';
      return;
    }

    tbody.innerHTML = notificationHistory.map(function(h) {
      var statusColor = h.status === 'delivered' ? '#10b981' : (h.status === 'failed' ? '#ef4444' : '#eab308');
      var title = Security.escapeHTML(h.title || h.message || '');
      var channelId = Security.escapeHTML(h.channel_id || '');
      var ch = channels.find(function(c) { return c.id === h.channel_id; });
      var channelName = ch ? ch.name : channelId;
      var status = Security.escapeHTML(h.status || '');
      var timestamp = UIComponents.timeAgo(h.created_at) || Security.escapeHTML(h.created_at || '');

      return '<tr>'
        + '<td style="font-size:12px;">' + title + '</td>'
        + '<td style="font-size:12px;">' + Security.escapeHTML(channelName) + '</td>'
        + '<td><span style="color:' + statusColor + ';font-weight:600;font-size:12px;">' + status + '</span></td>'
        + '<td style="font-size:12px;color:var(--color-muted);">' + timestamp + '</td>'
        + '<td>' + (h.status === 'failed' ? '<button class="btn btn-ghost btn-sm" style="font-size:11px;" onclick="alert(\'Retrying notification...\')">Retry</button>' : '') + '</td>'
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
    refresh: async function() {
      var channelsGrid = document.getElementById('notif-channels-grid');
      var historyBody = document.getElementById('notif-history-tbody');
      var inboxList = document.getElementById('notif-inbox-list');

      // Show skeletons during fetch
      if (channelsGrid) {
        channelsGrid.innerHTML = '<div class="skeleton" style="height:120px;border-radius:var(--rounded-lg);"></div>'
          + '<div class="skeleton" style="height:120px;border-radius:var(--rounded-lg);"></div>'
          + '<div class="skeleton" style="height:120px;border-radius:var(--rounded-lg);"></div>'
          + '<div class="skeleton" style="height:120px;border-radius:var(--rounded-lg);"></div>';
      }
      if (historyBody) {
        historyBody.innerHTML = '<tr><td colspan="5"><div class="skeleton" style="height:120px;border-radius:var(--rounded-lg);"></div></td></tr>';
      }
      if (inboxList) {
        inboxList.innerHTML = '<div class="skeleton" style="height:80px;margin:8px;border-radius:var(--rounded-lg);"></div>';
      }

      try {
        const [notifsRes, historyRes, channelsRes] = await Promise.all([
          APIClient.get('/notifications'),
          APIClient.get('/notifications/history'),
          APIClient.get('/notifications/channels')
        ]);

        if (notifsRes && notifsRes.data) {
          notifications = notifsRes.data;
        } else {
          throw new Error('Invalid notifications data format');
        }

        if (historyRes && historyRes.data) {
          notificationHistory = historyRes.data;
        } else {
          throw new Error('Invalid history data format');
        }

        if (channelsRes && channelsRes.data) {
          channels = channelsRes.data;
        } else {
          throw new Error('Invalid channels data format');
        }

        updateBellIcon();
        renderInbox();
        renderChannels();
        renderHistory();
      } catch (err) {
        console.error('[NotificationCenter] Refresh failed:', err);
        // Show error alert blocks
        if (channelsGrid) {
          channelsGrid.innerHTML = '<div class="panel" style="grid-column:span 4;padding:var(--space-md);color:var(--color-danger);border:1px solid rgba(239,68,68,0.2);background:rgba(239,68,68,0.1);border-radius:var(--rounded-lg);"><strong>Error:</strong> Failed to fetch notification channels. Please try again.</div>';
        }
        if (historyBody) {
          historyBody.innerHTML = '<tr><td colspan="5" style="text-align:center;color:var(--color-danger);padding:var(--space-md);">Failed to load notification history.</td></tr>';
        }
        if (inboxList) {
          inboxList.innerHTML = '<div style="padding:var(--space-md);text-align:center;color:var(--color-danger);">Failed to load notifications.</div>';
        }
      }
    },
    markRead: async function(id) {
      try {
        await APIClient.put('/notifications/' + id + '/read');
        this.refresh();
      } catch (err) {
        console.error('[NotificationCenter] Mark read failed:', err);
      }
    },
    markAllRead: async function() {
      try {
        await APIClient.put('/notifications/read-all');
        this.refresh();
      } catch (err) {
        console.error('[NotificationCenter] Mark all read failed:', err);
      }
    },
    configureChannel: function(chId) {
      if (global.Modal && global.Modal.open) {
        var ch = chId ? channels.find(function(c){ return c.id === chId; }) : {
          id: '',
          type: 'webhook',
          name: 'Custom Webhook',
          webhook_url: ''
        };
        var icon = (channelIcons[ch.type] || '🔔');
        var name = Security.escapeHTML(ch.name);
        var webhookUrl = Security.escapeHTML(ch.webhook_url || '');

        global.Modal.open({
          title: icon + (ch.id ? ' Configure ' : ' Add ') + name,
          body: '<div style="padding:var(--space-xs);">'
            + '<div id="notif-modal-error" style="display:none;padding:8px 12px;margin-bottom:12px;color:var(--color-danger);border:1px solid rgba(239,68,68,0.2);background:rgba(239,68,68,0.1);border-radius:var(--rounded-lg);font-size:12px;"></div>'
            + '<div class="form-group"><label class="form-label">Channel Name</label>'
            + '<input type="text" id="notif-channel-name-input" class="form-select" placeholder="Webhook Name" value="' + name + '" ' + (ch.id ? 'disabled' : '') + '></div>'
            + '<div class="form-group"><label class="form-label">Webhook URL</label>'
            + '<input type="text" id="notif-webhook-url-input" class="form-select" placeholder="https://..." value="' + webhookUrl + '"></div>'
            + '<div style="display:flex;gap:8px;margin-top:var(--space-sm);">'
            + '<button id="notif-save-btn" class="btn btn-primary btn-sm" onclick="NotificationCenter.saveChannel(\'' + ch.id + '\', this)">Save</button>'
            + '<button class="btn btn-ghost btn-sm" onclick="Modal.close();">Cancel</button>'
            + '</div></div>'
        });
      }
    },
    saveChannel: async function(chId, btn) {
      if (btn) {
        btn.disabled = true;
        btn.textContent = 'Saving...';
      }

      var nameInput = document.getElementById('notif-channel-name-input');
      var urlInput = document.getElementById('notif-webhook-url-input');
      
      var name = nameInput ? nameInput.value.trim() : 'Custom Webhook';
      var webhookUrl = urlInput ? urlInput.value.trim() : '';

      if (!webhookUrl) {
        var errBlock = document.getElementById('notif-modal-error');
        if (errBlock) {
          errBlock.textContent = 'Webhook URL is required.';
          errBlock.style.display = 'block';
        }
        if (btn) {
          btn.disabled = false;
          btn.textContent = 'Save';
        }
        return;
      }

      var payload = {
        type: chId ? (channels.find(function(c) { return c.id === chId; })?.type || 'webhook') : 'webhook',
        name: name,
        webhook_url: webhookUrl,
        enabled: true
      };

      try {
        if (chId) {
          // Reconfiguring existing
          await APIClient.delete('/notifications/channels/' + chId);
        }
        var res = await APIClient.post('/notifications/channels', payload);
        if (res) {
          if (global.Modal && global.Modal.close) {
            global.Modal.close();
          }
          this.refresh();
        } else {
          throw new Error('Server error');
        }
      } catch (err) {
        console.error('[NotificationCenter] Save channel failed:', err);
        var errBlock = document.getElementById('notif-modal-error');
        if (errBlock) {
          errBlock.textContent = 'Failed to save configuration. Please try again.';
          errBlock.style.display = 'block';
        }
        if (btn) {
          btn.disabled = false;
          btn.textContent = 'Save';
        }
      }
    },
    deleteChannel: async function(chId) {
      if (!confirm('Are you sure you want to delete this channel?')) return;
      try {
        await APIClient.delete('/notifications/channels/' + chId);
        this.refresh();
      } catch (err) {
        console.error('[NotificationCenter] Delete channel failed:', err);
        alert('Failed to delete channel: ' + err.message);
      }
    }
  };

  global.NotificationCenter = NotificationCenter;
})(window);
