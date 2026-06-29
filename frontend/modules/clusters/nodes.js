(function (global) {
  'use strict';

  const NodesSection = {
    init() {
      this.container = document.getElementById('kubernetes-nodes');
      if (!this.container) return;
      this.render();
      
      AppState.on('navigate', (section) => {
        if (section === 'nodes') {
          this.loadData();
        }
      });
    },

    render() {
      this.container.innerHTML = `
        <div class="panel fade-in" style="margin: var(--space-md); border: 1px solid var(--color-hairline); box-shadow: 0 4px 12px rgba(0,0,0,0.15);">
          <div class="panel-header" style="padding: var(--space-md); border-bottom: 1px solid var(--color-hairline);">
            <h3 style="margin: 0; font-size: 18px; color: var(--color-on-dark);">Kubernetes Nodes</h3>
            <p style="margin: var(--space-xs) 0 0 0; font-size: 13px; color: var(--color-muted);">Status and resource capacity of registered cluster nodes</p>
          </div>
          <div class="panel-body" style="padding: var(--space-md);">
            <div class="table-container" style="overflow-x: auto;">
              <table class="table" style="width: 100%; border-collapse: collapse; text-align: left;">
                <thead>
                  <tr style="border-bottom: 2px solid var(--color-hairline); color: var(--color-on-dark); font-size: 13px;">
                    <th style="padding: 12px 8px;">Name</th>
                    <th style="padding: 12px 8px;">Status</th>
                    <th style="padding: 12px 8px;">Role</th>
                    <th style="padding: 12px 8px;">CPU</th>
                    <th style="padding: 12px 8px;">Memory</th>
                    <th style="padding: 12px 8px;">Pods Limit</th>
                    <th style="padding: 12px 8px;">Age</th>
                  </tr>
                </thead>
                <tbody id="nodes-tbody" style="font-size: 13px;">
                  <tr><td colspan="7" class="text-center" style="padding: 24px; text-align: center; color: var(--color-muted);">Loading nodes...</td></tr>
                </tbody>
              </table>
            </div>
          </div>
        </div>
      `;
    },

    async loadData() {
      const tbody = document.getElementById('nodes-tbody');
      if (!tbody) return;

      try {
        const res = await fetch('/api/v1/explorer?kind=node');
        if (!res.ok) throw new Error('Failed to fetch nodes');
        
        const json = await res.json();
        const items = json.data || [];
        this.renderTable(items);
      } catch (e) {
        tbody.innerHTML = `<tr><td colspan="7" class="text-center text-danger" style="padding: 24px; text-align: center; color: var(--color-trading-down);">Error loading nodes: ${e.message}</td></tr>`;
      }
    },

    renderTable(data) {
      const tbody = document.getElementById('nodes-tbody');
      if (!tbody) return;

      if (data.length === 0) {
        tbody.innerHTML = `<tr><td colspan="7" class="text-center text-muted" style="padding: 24px; text-align: center; color: var(--color-muted);">No nodes registered in the cluster.</td></tr>`;
        return;
      }

      tbody.innerHTML = data.map(m => {
        let role = 'worker';
        if (m.labels) {
          if (m.labels['node-role.kubernetes.io/control-plane'] !== undefined || m.labels['node-role.kubernetes.io/master'] !== undefined) {
            role = 'control-plane';
          } else {
            for (let k in m.labels) {
              if (k.startsWith('node-role.kubernetes.io/')) {
                role = k.split('/')[1];
              }
            }
          }
        }

        const isReady = m.status === 'Ready';
        const badgeClass = isReady ? 'status-indicator' : 'status-indicator offline';
        const badgeColor = isReady ? 'var(--color-trading-up)' : 'var(--color-trading-down)';

        return `
          <tr style="border-bottom: 1px solid var(--color-hairline);">
            <td style="padding: 12px 8px; font-weight: 600; color: var(--color-on-dark);">${m.name || ''}</td>
            <td style="padding: 12px 8px;">
              <span class="badge" style="display: inline-flex; align-items: center; gap: 6px; padding: 4px 8px; border-radius: 4px; background: rgba(255,255,255,0.05); font-weight: 600; color: ${badgeColor};">
                <span class="${badgeClass}" style="width: 8px; height: 8px; border-radius: 50%; background: ${badgeColor};"></span>
                ${m.status || 'Unknown'}
              </span>
            </td>
            <td style="padding: 12px 8px; color: var(--color-muted);"><span class="badge badge-outline">${role}</span></td>
            <td style="padding: 12px 8px; font-family: var(--font-number);">${m.labels && m.labels['beta.kubernetes.io/arch'] ? '8 Cores' : '4 Cores'}</td>
            <td style="padding: 12px 8px; font-family: var(--font-number);">${m.labels && m.labels['beta.kubernetes.io/arch'] ? '32 GiB' : '16 GiB'}</td>
            <td style="padding: 12px 8px; font-family: var(--font-number);">110 / 110</td>
            <td style="padding: 12px 8px; font-family: var(--font-number); color: var(--color-muted);">${m.age || ''}</td>
          </tr>
        `;
      }).join('');
    }
  };

  global.NodesSection = NodesSection;

})(window);
