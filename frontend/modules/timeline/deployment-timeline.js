/**
 * Deployment Timeline Module
 * Unified chronological view of deployments, rollbacks, incidents, and commits.
 */
(function (global) {
  'use strict';

  var currentEvents = [];

  function getEventTypeInfo(type) {
    switch (type) {
      case 'deployment':
        return { icon: '🚀', color: '#6366f1' };
      case 'incident':
        return { icon: '🔴', color: '#ef4444' };
      case 'rollback':
        return { icon: '⏪', color: '#f97316' };
      case 'commit':
        return { icon: '📝', color: '#10b981' };
      case 'config':
        return { icon: '🔧', color: '#8b5cf6' };
      default:
        return { icon: '❓', color: '#6b7280' };
    }
  }

  function renderTimeline(events) {
    var container = document.getElementById('timeline-container');
    if (!container) return;

    if (!events || events.length === 0) {
      container.innerHTML = '<div style="text-align:center;padding:48px 24px;color:var(--color-muted);">'
        + '<h4>No events found</h4>'
        + '<p>Ensure cluster operations are running normally.</p>'
        + '</div>';
      return;
    }

    var grouped = {};
    events.forEach(function(ev) {
      var date = 'Unknown';
      var time = '';
      if (ev.created_at) {
        var d = new Date(ev.created_at);
        var year = d.getFullYear();
        var month = String(d.getMonth() + 1).padStart(2, '0');
        var day = String(d.getDate()).padStart(2, '0');
        var hours = String(d.getHours()).padStart(2, '0');
        var minutes = String(d.getMinutes()).padStart(2, '0');
        date = year + '-' + month + '-' + day;
        time = hours + ':' + minutes;
      }
      if (!grouped[date]) grouped[date] = [];
      
      var typeInfo = getEventTypeInfo(ev.type);

      grouped[date].push({
        id: ev.id,
        title: ev.title,
        detail: ev.detail,
        namespace: ev.namespace,
        time: time,
        icon: typeInfo.icon,
        color: typeInfo.color
      });
    });

    var html = '';
    Object.keys(grouped).sort().reverse().forEach(function(date) {
      html += '<div style="margin-bottom:var(--space-md);">'
        + '<div style="font-size:12px;font-weight:600;color:var(--color-muted);margin-bottom:8px;padding:4px 0;border-bottom:1px dashed var(--color-hairline);">' + Security.escapeHTML(date) + '</div>';
      grouped[date].forEach(function(ev) {
        html += '<div class="timeline-event" '
          + 'onclick="DeploymentTimeline.showDetail(\'' + Security.escapeHTML(ev.id) + '\')">'
          + '<div style="width:32px;height:32px;border-radius:50%;background:' + ev.color + '20;display:flex;align-items:center;justify-content:center;font-size:16px;flex-shrink:0;">' + ev.icon + '</div>'
          + '<div style="flex:1;min-width:0;">'
          + '<div style="display:flex;align-items:center;gap:8px;">'
          + '<strong style="font-size:13px;">' + Security.escapeHTML(ev.title) + '</strong>'
          + (ev.namespace ? '<span style="font-size:10px;background:var(--color-surface);border:1px solid var(--color-hairline);padding:1px 6px;border-radius:3px;">' + Security.escapeHTML(ev.namespace) + '</span>' : '')
          + '<span style="font-size:11px;color:var(--color-muted);margin-left:auto;">' + Security.escapeHTML(ev.time) + '</span>'
          + '</div>'
          + '<div style="font-size:12px;color:var(--color-muted);margin-top:2px;">' + Security.escapeHTML(ev.detail) + '</div>'
          + '</div></div>';
      });
      html += '</div>';
    });
    container.innerHTML = html;
  }

  global.DeploymentTimeline = {
    init: function() { this.refresh(); },
    refresh: async function() {
      var container = document.getElementById('timeline-container');
      if (container) {
        container.innerHTML = '<div class="skeleton" style="height:40px;margin-bottom:12px;"></div>'
          + '<div class="skeleton" style="height:40px;margin-bottom:12px;"></div>'
          + '<div class="skeleton" style="height:40px;"></div>';
      }

      var rangeSelect = document.getElementById('timeline-range');
      var range = rangeSelect ? rangeSelect.value : '7d';
      
      var typeSelect = document.getElementById('timeline-type');
      var type = typeSelect ? typeSelect.value : '';

      var url = '/timeline?range=' + encodeURIComponent(range);
      if (type) {
        url += '&type=' + encodeURIComponent(type);
      }

      try {
        const json = await APIClient.get(url);
        const events = (json && json.data) || [];
        currentEvents = events;
        renderTimeline(events);
      } catch (e) {
        console.error('Failed to fetch timeline events:', e);
        if (container) {
          container.innerHTML = '<div style="text-align:center;padding:24px;color:var(--color-trading-down);">'
            + '⚠️ Failed to load deployment timeline: ' + Security.escapeHTML(e.message)
            + '</div>';
        }
      }
    },
    showDetail: async function(id) {
      var detail = document.getElementById('timeline-detail');
      if (detail) {
        detail.innerHTML = '<div class="skeleton" style="height:60px;"></div>';
      }

      try {
        const ev = await APIClient.get('/timeline/' + encodeURIComponent(id));
        if (!ev) {
          if (detail) {
            detail.innerHTML = '<span class="text-muted-sm">Event details not found.</span>';
          }
          return;
        }

        var typeInfo = getEventTypeInfo(ev.type);
        var title = Security.escapeHTML(ev.title || '');
        var detailText = Security.escapeHTML(ev.detail || '');
        var namespace = Security.escapeHTML(ev.namespace || '');
        var cluster = Security.escapeHTML(ev.cluster || '');
        var type = Security.escapeHTML(ev.type || '');
        
        var dateStr = '';
        var timeStr = '';
        if (ev.created_at) {
          var d = new Date(ev.created_at);
          var year = d.getFullYear();
          var month = String(d.getMonth() + 1).padStart(2, '0');
          var day = String(d.getDate()).padStart(2, '0');
          var hours = String(d.getHours()).padStart(2, '0');
          var minutes = String(d.getMinutes()).padStart(2, '0');
          dateStr = year + '-' + month + '-' + day;
          timeStr = hours + ':' + minutes;
        }

        if (detail) {
          detail.innerHTML = '<div style="display:flex;align-items:center;gap:8px;margin-bottom:8px;">'
            + '<span style="font-size:24px;">' + typeInfo.icon + '</span>'
            + '<div><strong style="font-size:14px;">' + title + '</strong><br><span style="font-size:12px;color:var(--color-muted);">' + dateStr + ' ' + timeStr + ' · ' + type + '</span></div></div>'
            + '<p style="font-size:13px;margin:0 0 8px 0;">' + detailText + '</p>'
            + (namespace ? '<span style="font-size:11px;color:var(--color-muted);margin-right:12px;">Namespace: ' + namespace + '</span>' : '')
            + (cluster ? '<span style="font-size:11px;color:var(--color-muted);">Cluster: ' + cluster + '</span>' : '');
        }
      } catch (e) {
        console.error('Failed to load event detail:', e);
        if (detail) {
          detail.innerHTML = '<span class="text-danger" style="font-size:12px;">⚠️ Failed to load event details: ' + Security.escapeHTML(e.message) + '</span>';
        }
      }
    }
  };
})(window);
