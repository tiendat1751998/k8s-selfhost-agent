(function (global) {
  'use strict';

  const PodsSection = {
    init() {
      this.container = document.getElementById('kubernetes-pods');
      if (!this.container) return;
      this.render();
      
      AppState.on('navigate', (section) => {
        if (section === 'pods') {
          this.loadData();
        }
      });
    },

    render() {
      this.container.innerHTML = `
        <div class="panel fade-in" style="margin: var(--space-md); border: 1px solid var(--color-hairline); box-shadow: 0 4px 12px rgba(0,0,0,0.15);">
          <div class="panel-header" style="padding: var(--space-md); border-bottom: 1px solid var(--color-hairline);">
            <h3 style="margin: 0; font-size: 18px; color: var(--color-on-dark);">Kubernetes Pods</h3>
            <p style="margin: var(--space-xs) 0 0 0; font-size: 13px; color: var(--color-muted);">Real-time container workloads running on cluster nodes</p>
          </div>
          <div class="panel-body" style="padding: var(--space-md);">
            <div class="table-container" style="overflow-x: auto;">
              <table class="table" style="width: 100%; border-collapse: collapse; text-align: left;">
                <thead>
                  <tr style="border-bottom: 2px solid var(--color-hairline); color: var(--color-on-dark); font-size: 13px;">
                    <th style="padding: 12px 8px;">Name</th>
                    <th style="padding: 12px 8px;">Namespace</th>
                    <th style="padding: 12px 8px;">Status</th>
                    <th style="padding: 12px 8px;">Restarts</th>
                    <th style="padding: 12px 8px;">Node</th>
                    <th style="padding: 12px 8px;">Age</th>
                  </tr>
                </thead>
                <tbody id="pods-tbody" style="font-size: 13px;">
                  <tr><td colspan="6" class="text-center" style="padding: 24px; text-align: center; color: var(--color-muted);">Loading pods...</td></tr>
                </tbody>
              </table>
            </div>
          </div>
        </div>
      `;
    },

    async loadData() {
      const tbody = document.getElementById('pods-tbody');
      if (!tbody) return;

      try {
        const res = await fetch('/api/v1/explorer?kind=pod');
        if (!res.ok) throw new Error('Failed to fetch pods');
        
        const json = await res.json();
        const items = json.data || [];
        this.renderTable(items);
      } catch (e) {
        tbody.innerHTML = `<tr><td colspan="6" class="text-center text-danger" style="padding: 24px; text-align: center; color: var(--color-trading-down);">Error loading pods: ${e.message}</td></tr>`;
      }
    },

    renderTable(data) {
      const tbody = document.getElementById('pods-tbody');
      if (!tbody) return;

      if (data.length === 0) {
        tbody.innerHTML = `<tr><td colspan="6" class="text-center text-muted" style="padding: 24px; text-align: center; color: var(--color-muted);">No pods running in the cluster namespaces.</td></tr>`;
        return;
      }

      tbody.innerHTML = data.map(m => {
        const status = m.status || 'Running';
        const isRunning = status === 'Running';
        const isCompleted = status === 'Succeeded';
        
        let badgeColor = 'var(--color-trading-up)';
        if (!isRunning && !isCompleted) {
          badgeColor = 'var(--color-trading-down)';
        } else if (isCompleted) {
          badgeColor = 'var(--color-muted)';
        }

        const badgeClass = isRunning ? 'status-indicator' : (isCompleted ? 'status-indicator' : 'status-indicator offline');

        // Extract restart count or node name placeholders if not present
        const restarts = m.labels && m.labels['restarts'] ? m.labels['restarts'] : '0';
        const node = m.labels && m.labels['node'] ? m.labels['node'] : 'k8s-node-1';

        return `
          <tr style="border-bottom: 1px solid var(--color-hairline);">
            <td style="padding: 12px 8px; font-weight: 600; color: var(--color-on-dark);">${m.name || ''}</td>
            <td style="padding: 12px 8px; color: var(--color-muted);">${m.namespace || 'default'}</td>
            <td style="padding: 12px 8px;">
              <span class="badge" style="display: inline-flex; align-items: center; gap: 6px; padding: 4px 8px; border-radius: 4px; background: rgba(255,255,255,0.05); font-weight: 600; color: ${badgeColor};">
                <span class="${badgeClass}" style="width: 8px; height: 8px; border-radius: 50%; background: ${badgeColor};"></span>
                ${status}
              </span>
            </td>
            <td style="padding: 12px 8px; font-family: var(--font-number);">${restarts}</td>
            <td style="padding: 12px 8px; color: var(--color-muted); font-family: var(--font-number);">${node}</td>
            <td style="padding: 12px 8px; font-family: var(--font-number); color: var(--color-muted);">${m.age || ''}</td>
          </tr>
        `;
      }).join('');
    }
  };

  global.PodsSection = PodsSection;

})(window);
