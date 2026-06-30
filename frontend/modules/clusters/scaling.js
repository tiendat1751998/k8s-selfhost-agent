(function (global) {
  'use strict';

  const ScalingSection = {
    init() {
      this.container = document.getElementById('kubernetes-scaling');
      if (!this.container) return;
      this.render();
      
      AppState.on('navigate', (section) => {
        if (section === 'scaling') {
          this.loadData();
        }
      });
    },

    render() {
      this.container.innerHTML = `
        <div class="panel fade-in" style="margin: var(--space-md); border: 1px solid var(--color-hairline); box-shadow: 0 4px 12px rgba(0,0,0,0.15);">
          <div class="panel-header" style="padding: var(--space-md); border-bottom: 1px solid var(--color-hairline);">
            <h3 style="margin: 0; font-size: 18px; color: var(--color-on-dark);">Horizontal Pod Autoscaling (HPA)</h3>
            <p style="margin: var(--space-xs) 0 0 0; font-size: 13px; color: var(--color-muted);">Automatic scaling policies and resource utilization thresholds</p>
          </div>
          <div class="panel-body" style="padding: var(--space-md);">
            <div class="table-container" style="overflow-x: auto;">
              <table class="table" style="width: 100%; border-collapse: collapse; text-align: left;">
                <thead>
                  <tr style="border-bottom: 2px solid var(--color-hairline); color: var(--color-on-dark); font-size: 13px;">
                    <th style="padding: 12px 8px;">Name</th>
                    <th style="padding: 12px 8px;">Target Ref</th>
                    <th style="padding: 12px 8px;">Min Replicas</th>
                    <th style="padding: 12px 8px;">Max Replicas</th>
                    <th style="padding: 12px 8px;">Current Replicas</th>
                    <th style="padding: 12px 8px;">Metrics (Current / Target)</th>
                  </tr>
                </thead>
                <tbody id="scaling-tbody" style="font-size: 13px;">
                  <tr><td colspan="6" class="text-center" style="padding: 24px; text-align: center; color: var(--color-muted);">Loading scaling policies...</td></tr>
                </tbody>
              </table>
            </div>
          </div>
        </div>
      `;
    },

    async loadData() {
      const tbody = document.getElementById('scaling-tbody');
      if (!tbody) return;

      try {
        const res = await fetch('/api/v1/explorer?kind=hpa');
        if (!res.ok) throw new Error('Failed to fetch scaling policies');
        
        const json = await res.json();
        let items = json.data || [];
        
        if (items.length === 0) {
          tbody.innerHTML = `<tr><td colspan="6" class="text-center" style="padding: 24px; text-align: center; color: var(--color-muted);">No active HPA (Horizontal Pod Autoscaler) rules found.</td></tr>`;
          return;
        }
        
        this.renderTable(items);
      } catch (e) {
        tbody.innerHTML = `<tr><td colspan="6" class="text-center" style="padding: 24px; text-align: center; color: var(--color-muted);">No active HPA (Horizontal Pod Autoscaler) rules found on the current host. Detail: ${e.message}</td></tr>`;
      }
    },

    renderTable(data) {
      const tbody = document.getElementById('scaling-tbody');
      if (!tbody) return;

      tbody.innerHTML = data.map(m => `
        <tr style="border-bottom: 1px solid var(--color-hairline);">
          <td style="padding: 12px 8px; font-weight: 600; color: var(--color-on-dark);">${m.name || ''}</td>
          <td style="padding: 12px 8px; color: var(--color-muted);"><span class="badge badge-outline">${m.target || m.target_ref || 'Deployment'}</span></td>
          <td style="padding: 12px 8px; font-family: var(--font-number);">${m.min || 1}</td>
          <td style="padding: 12px 8px; font-family: var(--font-number);">${m.max || 10}</td>
          <td style="padding: 12px 8px; font-family: var(--font-number); font-weight: 600; color: var(--color-trading-up);">${m.current || 1}</td>
          <td style="padding: 12px 8px; font-family: var(--font-number); color: var(--color-muted);">${m.metrics || 'CPU: 0% / 80%'}</td>
        </tr>
      `).join('');
    }
  };

  global.ScalingSection = ScalingSection;

})(window);
