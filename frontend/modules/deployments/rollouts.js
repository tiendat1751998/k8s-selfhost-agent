(function (global) {
  'use strict';

  const RolloutsSection = {
    init() {
      this.container = document.getElementById('deployment-rollouts');
      if (!this.container) return;
      this.render();
      
      AppState.on('navigate', (section) => {
        if (section === 'rollouts') {
          this.loadData();
        }
      });
    },

    render() {
      this.container.innerHTML = `
        <div class="panel fade-in" style="margin: var(--space-md); border: 1px solid var(--color-hairline); box-shadow: 0 4px 12px rgba(0,0,0,0.15);">
          <div class="panel-header" style="padding: var(--space-md); border-bottom: 1px solid var(--color-hairline);">
            <h3 style="margin: 0; font-size: 18px; color: var(--color-on-dark);">Active Rollouts</h3>
            <p style="margin: var(--space-xs) 0 0 0; font-size: 13px; color: var(--color-muted);">Track deployment rollouts, update progress, and strategies</p>
          </div>
          <div class="panel-body" style="padding: var(--space-md);">
            <div class="table-container" style="overflow-x: auto;">
              <table class="table" style="width: 100%; border-collapse: collapse; text-align: left;">
                <thead>
                  <tr style="border-bottom: 2px solid var(--color-hairline); color: var(--color-on-dark); font-size: 13px;">
                    <th style="padding: 12px 8px;">Deployment</th>
                    <th style="padding: 12px 8px;">Strategy</th>
                    <th style="padding: 12px 8px;">Progress</th>
                    <th style="padding: 12px 8px;">Replicas</th>
                    <th style="padding: 12px 8px;">Age</th>
                  </tr>
                </thead>
                <tbody id="rollouts-tbody" style="font-size: 13px;">
                  <tr><td colspan="5" class="text-center" style="padding: 24px; text-align: center; color: var(--color-muted);">Loading rollouts...</td></tr>
                </tbody>
              </table>
            </div>
          </div>
        </div>
      `;
    },

    async loadData() {
      const tbody = document.getElementById('rollouts-tbody');
      if (!tbody) return;

      try {
        const res = await fetch('/api/v1/explorer?kind=deployment');
        if (!res.ok) throw new Error('Failed to fetch deployments');
        
        const json = await res.json();
        const items = json.data || [];
        this.renderTable(items);
      } catch (e) {
        tbody.innerHTML = `<tr><td colspan="5" class="text-center text-danger" style="padding: 24px; text-align: center; color: var(--color-trading-down);">Error loading rollouts: ${e.message}</td></tr>`;
      }
    },

    renderTable(data) {
      const tbody = document.getElementById('rollouts-tbody');
      if (!tbody) return;

      if (data.length === 0) {
        tbody.innerHTML = `<tr><td colspan="5" class="text-center text-muted" style="padding: 24px; text-align: center; color: var(--color-muted);">No active deployment rollouts found.</td></tr>`;
        return;
      }

      tbody.innerHTML = data.map(m => {
        const isReady = m.status === 'Ready';
        const progressPercent = isReady ? '100%' : '80%';
        const progressColor = isReady ? 'var(--color-trading-up)' : '#f0b90b';

        // Extract replica strategy and counts
        const strategy = m.labels && m.labels['strategy'] ? m.labels['strategy'] : 'RollingUpdate';
        const replicas = m.labels && m.labels['replicas'] ? m.labels['replicas'] : '3/3';

        return `
          <tr style="border-bottom: 1px solid var(--color-hairline);">
            <td style="padding: 12px 8px; font-weight: 600; color: var(--color-on-dark);">${m.name || ''}</td>
            <td style="padding: 12px 8px; color: var(--color-muted);"><span class="badge badge-outline">${strategy}</span></td>
            <td style="padding: 12px 8px;">
              <div style="display: flex; align-items: center; gap: 8px;">
                <div style="width: 100px; height: 6px; border-radius: 3px; background: rgba(255,255,255,0.05); overflow: hidden;">
                  <div style="width: ${progressPercent}; height: 100%; background: ${progressColor};"></div>
                </div>
                <span>${progressPercent}</span>
              </div>
            </td>
            <td style="padding: 12px 8px; font-family: var(--font-number);">${replicas}</td>
            <td style="padding: 12px 8px; font-family: var(--font-number); color: var(--color-muted);">${m.age || ''}</td>
          </tr>
        `;
      }).join('');
    }
  };

  global.RolloutsSection = RolloutsSection;

})(window);
