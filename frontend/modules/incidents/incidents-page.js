(function (global) {
  'use strict';

  const IncidentsPage = {
    init() {
      this.container = document.getElementById('incidents-page');
      if (!this.container) return;
      this.render();
      
      AppState.on('navigate', (section) => {
        if (section === 'incidents') {
          this.loadData();
        }
      });
    },

    render() {
      this.container.innerHTML = `
        <div class="panel fade-in" style="margin: var(--space-md); border: 1px solid var(--color-hairline); box-shadow: 0 4px 12px rgba(0,0,0,0.15);">
          <div class="panel-header" style="padding: var(--space-md); border-bottom: 1px solid var(--color-hairline);">
            <h3 style="margin: 0; font-size: 18px; color: var(--color-on-dark);">Incidents Center</h3>
            <p style="margin: var(--space-xs) 0 0 0; font-size: 13px; color: var(--color-muted);">Browse, inspect, and manage system and cluster incidents</p>
          </div>
          <div class="panel-body" style="padding: var(--space-md);">
            <div class="table-container" style="overflow-x: auto;">
              <table class="table" style="width: 100%; border-collapse: collapse; text-align: left;">
                <thead>
                  <tr style="border-bottom: 2px solid var(--color-hairline); color: var(--color-on-dark); font-size: 13px;">
                    <th style="padding: 12px 8px;">Incident ID</th>
                    <th style="padding: 12px 8px;">Pod/Resource</th>
                    <th style="padding: 12px 8px;">Severity</th>
                    <th style="padding: 12px 8px;">Status</th>
                    <th style="padding: 12px 8px;">Message</th>
                    <th style="padding: 12px 8px;">Age</th>
                    <th style="padding: 12px 8px; text-align: right;">Actions</th>
                  </tr>
                </thead>
                <tbody id="incidents-tbody" style="font-size: 13px;">
                  <tr><td colspan="7" class="text-center" style="padding: 24px; text-align: center; color: var(--color-muted);">Loading incidents...</td></tr>
                </tbody>
              </table>
            </div>
          </div>
        </div>
      `;
    },

    async loadData() {
      const tbody = document.getElementById('incidents-tbody');
      if (!tbody) return;

      try {
        const res = await fetch('/api/v1/incidents');
        if (!res.ok) throw new Error('Failed to fetch incidents');
        
        const json = await res.json();
        // The API returns incidents list under "data" or direct array
        const items = Array.isArray(json.data) ? json.data : (Array.isArray(json) ? json : []);
        this.incidents = items;
        this.renderTable(items);
      } catch (e) {
        tbody.innerHTML = `<tr><td colspan="7" class="text-center text-danger" style="padding: 24px; text-align: center; color: var(--color-trading-down);">Error loading incidents: ${e.message}</td></tr>`;
      }
    },

    renderTable(data) {
      const tbody = document.getElementById('incidents-tbody');
      if (!tbody) return;

      if (data.length === 0) {
        tbody.innerHTML = `<tr><td colspan="7" style="padding: var(--space-lg) 0;">` + UIComponents.emptyState({
          title: 'No incidents recorded',
          description: 'All clusters and components are operating within normal parameters.',
          icon: '🛡️',
          actionText: 'Scan for Incidents',
          actionId: 'scan-incidents-btn'
        }) + `</td></tr>`;

        setTimeout(() => {
          const btn = document.getElementById('scan-incidents-btn');
          if (btn) {
            btn.addEventListener('click', () => this.loadData());
          }
        }, 50);
        return;
      }

      tbody.innerHTML = data.map((m, idx) => {
        const severity = m.severity || 'info';
        let severityColor = 'var(--color-muted)';
        if (severity === 'critical') severityColor = 'var(--color-trading-down)';
        else if (severity === 'warning') severityColor = '#f0b90b';
        
        const status = m.status || 'open';
        let statusColor = 'var(--color-trading-down)';
        if (status === 'resolved') statusColor = 'var(--color-trading-up)';
        else if (status === 'investigating') statusColor = '#f0b90b';

        const idShort = (m.id || '').substring(0, 8) || `INC-${idx + 1}`;
        const resource = m.podName || m.resourceName || m.pod || 'cluster';
        const ageStr = formatAge(m.timestamp);

        return `
          <tr style="border-bottom: 1px solid var(--color-hairline);">
            <td style="padding: 12px 8px; font-family: var(--font-number); color: var(--color-muted);">${idShort}</td>
            <td style="padding: 12px 8px; font-weight: 600; color: var(--color-on-dark);">${resource}</td>
            <td style="padding: 12px 8px;">
              <span class="badge" style="padding: 2px 6px; border-radius: 4px; background: rgba(255,255,255,0.05); font-weight: 600; color: ${severityColor};">
                ${severity.toUpperCase()}
              </span>
            </td>
            <td style="padding: 12px 8px;">
              <span class="badge" style="padding: 2px 6px; border-radius: 4px; background: rgba(255,255,255,0.05); font-weight: 600; color: ${statusColor};">
                ${status.toUpperCase()}
              </span>
            </td>
            <td style="padding: 12px 8px; max-width: 300px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; color: var(--color-on-dark);">${m.message || m.type || ''}</td>
            <td style="padding: 12px 8px; font-family: var(--font-number); color: var(--color-muted);">${ageStr}</td>
            <td style="padding: 12px 8px; text-align: right;">
              <button class="btn btn-sm btn-ghost view-detail-btn" data-index="${idx}" style="padding: 4px 8px;">View Details</button>
            </td>
          </tr>
        `;
      }).join('');

      tbody.querySelectorAll('.view-detail-btn').forEach(btn => {
        btn.addEventListener('click', (e) => {
          const index = parseInt(e.target.dataset.index);
          const incident = this.incidents[index];
          if (incident && window.IncidentDetailModule && typeof window.IncidentDetailModule.openDrawer === 'function') {
            window.IncidentDetailModule.openDrawer(incident, index);
          }
        });
      });
    }
  };

  function formatAge(ts) {
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

  global.IncidentsPage = IncidentsPage;

})(window);
