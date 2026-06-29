const CapacityPlanning = {
  init() {
    this.container = document.getElementById('capacity-planning');
    if (!this.container) return;
    this.render();
    this.bindEvents();
    setTimeout(() => {
      this.loadData();
    }, 400);
  },

  render() {
    this.container.innerHTML = `
      <div class="capacity-container fade-in">
        <div class="capacity-header">
          <h2>Capacity Planning</h2>
          <div class="capacity-actions">
            <select class="select select-bordered" id="capacity-cluster-select">
              <option value="all">All Clusters</option>
              <option value="production-us-east">production-us-east</option>
              <option value="staging-eu-west">staging-eu-west</option>
            </select>
            <button class="btn btn-primary" id="btn-recalculate-capacity">
              <span class="icon">🔄</span> Recalculate Forecast
            </button>
          </div>
        </div>

        <div class="capacity-alerts" id="capacity-alerts">
          <!-- Warnings go here -->
        </div>

        <div class="capacity-metrics-grid" id="capacity-metrics-grid">
          <div class="skeleton h-32 w-full"></div>
          <div class="skeleton h-32 w-full"></div>
          <div class="skeleton h-32 w-full"></div>
        </div>

        <div class="capacity-charts-grid">
          <div class="capacity-card">
            <h3>CPU Forecast</h3>
            <div class="capacity-chart" id="cpu-forecast-chart">
              <!-- Mock chart -->
              <div class="mock-chart-line" style="height: 40%; left: 0;"></div>
              <div class="mock-chart-line" style="height: 50%; left: 25%;"></div>
              <div class="mock-chart-line" style="height: 65%; left: 50%;"></div>
              <div class="mock-chart-line warning" style="height: 85%; left: 75%;"></div>
              <div class="mock-chart-line critical" style="height: 98%; left: 100%;"></div>
            </div>
            <div class="chart-labels">
              <span>Today</span>
              <span>7d</span>
              <span>30d</span>
              <span>60d</span>
              <span>90d</span>
            </div>
          </div>
          
          <div class="capacity-card">
            <h3>Memory Forecast</h3>
            <div class="capacity-chart" id="mem-forecast-chart">
              <!-- Mock chart -->
              <div class="mock-chart-line" style="height: 60%; left: 0;"></div>
              <div class="mock-chart-line" style="height: 65%; left: 25%;"></div>
              <div class="mock-chart-line" style="height: 70%; left: 50%;"></div>
              <div class="mock-chart-line" style="height: 75%; left: 75%;"></div>
              <div class="mock-chart-line" style="height: 80%; left: 100%;"></div>
            </div>
            <div class="chart-labels">
              <span>Today</span>
              <span>7d</span>
              <span>30d</span>
              <span>60d</span>
              <span>90d</span>
            </div>
          </div>
        </div>
      </div>
    `;
  },

  bindEvents() {
    const btn = document.getElementById('btn-recalculate-capacity');
    if (btn) {
      btn.addEventListener('click', () => {
        btn.classList.add('loading');
        setTimeout(() => {
          btn.classList.remove('loading');
          this.loadData();
        }, 1000);
      });
    }

    const select = document.getElementById('capacity-cluster-select');
    if (select) {
      select.addEventListener('change', () => this.loadData());
    }
  },

  async loadData() {
    try {
      const res = await fetch('/api/v1/capacity');
      if (res.ok) {
        const json = await res.json();
        const items = json.data || [];
        if (items.length > 0) {
          this.renderMetrics(items);
          return;
        }
      }
    } catch (e) {
      console.warn('Capacity API unavailable:', e);
    }
    
    const alerts = document.getElementById('capacity-alerts');
    if (alerts) {
      alerts.innerHTML = `
        <div class="alert alert-danger">
          <span class="icon">⚠️</span>
          <div>
            <h4>Unable to load capacity planning data</h4>
            <p>Please check connection or ensure the metrics server is running on the cluster.</p>
          </div>
        </div>
      `;
    }
  },

  renderMetrics(data) {
    // 1. Render Alerts/Warnings
    const alerts = document.getElementById('capacity-alerts');
    if (alerts) {
      const warnings = data.filter(f => f.status === 'warning' || f.status === 'critical');
      if (warnings.length > 0) {
        alerts.innerHTML = warnings.map(w => `
          <div class="alert alert-${w.status === 'critical' ? 'danger' : 'warning'}">
            <span class="icon">⚠️</span>
            <div>
              <h4>${w.resource_type.toUpperCase()} Exhaustion Warning</h4>
              <p>Cluster ${esc(w.cluster)} is projected to run out of ${esc(w.resource_type)} capacity. Current: <strong>${w.current_usage.toFixed(1)}%</strong>. Status: <strong>${w.status}</strong>.</p>
            </div>
          </div>
        `).join('');
      } else {
        alerts.innerHTML = `
          <div class="alert alert-success">
            <span class="icon">✓</span>
            <div>
              <h4>Capacity Healthy</h4>
              <p>All resource pools are within safe utilization thresholds.</p>
            </div>
          </div>
        `;
      }
    }

    // 2. Render Metrics Grid
    const grid = document.getElementById('capacity-metrics-grid');
    if (grid) {
      grid.innerHTML = data.map(f => {
        let icon = '🧠';
        let iconColor = '#3b82f6';
        let bg = 'rgba(59,130,246,0.1)';
        if (f.resource_type === 'memory') {
          icon = '📝';
          iconColor = '#10b981';
          bg = 'rgba(16,185,129,0.1)';
        } else if (f.resource_type === 'storage') {
          icon = '💾';
          iconColor = '#f59e0b';
          bg = 'rgba(245,158,11,0.1)';
        }

        const current = f.current_usage || 0;
        const projected90d = f.forecast_90d || 0;
        const trendIcon = projected90d > current ? '↗' : '→';
        const trendClass = projected90d > current ? 'up' : 'flat';

        return `
          <div class="metric-card">
            <div class="metric-icon" style="background: ${bg}; color: ${iconColor};">${icon}</div>
            <div class="metric-content">
              <h4>${f.resource_type.toUpperCase()} Usage</h4>
              <div class="metric-value">${current.toFixed(1)}% <span class="trend ${trendClass}">${trendIcon} ${projected90d.toFixed(1)}% (90d)</span></div>
              <div class="progress-bar"><div class="progress-fill" style="width: ${current}%"></div></div>
            </div>
          </div>
        `;
      }).join('');
    }

    // 3. Render SVG Charts
    const cpuData = data.find(f => f.resource_type === 'cpu');
    if (cpuData) {
      this.drawSvgChart('cpu-forecast-chart', cpuData);
    }
    const memData = data.find(f => f.resource_type === 'memory');
    if (memData) {
      this.drawSvgChart('mem-forecast-chart', memData);
    }
  },

  drawSvgChart(elementId, f) {
    const el = document.getElementById(elementId);
    if (!el) return;

    const values = [
      f.current_usage || 0,
      f.forecast_7d || 0,
      f.forecast_30d || 0,
      f.forecast_90d || 0
    ];

    const xCoords = [20, 130, 250, 370];
    const points = values.map((val, idx) => {
      const y = 90 - (val * 0.8);
      const constrainedY = Math.max(10, Math.min(90, y));
      return { x: xCoords[idx], y: constrainedY, value: val };
    });

    const pathD = `M ${points[0].x},${points[0].y} L ${points[1].x},${points[1].y} L ${points[2].x},${points[2].y} L ${points[3].x},${points[3].y}`;
    
    let dotsAndLabels = '';
    let colorClass = f.status === 'critical' ? 'var(--color-trading-down)' : (f.status === 'warning' ? 'var(--color-trading-neutral)' : 'var(--color-trading-up)');
    if (!colorClass.startsWith('var')) {
      colorClass = 'var(--color-primary)';
    }

    points.forEach(pt => {
      dotsAndLabels += `
        <circle cx="${pt.x}" cy="${pt.y}" r="4" fill="${colorClass}" />
        <text x="${pt.x}" y="${pt.y - 8}" text-anchor="middle" font-size="10" fill="var(--color-text-secondary)" font-family="var(--font-number)">${pt.value.toFixed(1)}%</text>
      `;
    });

    el.innerHTML = `
      <svg viewBox="0 0 400 100" style="width:100%; height:100%; overflow:visible;">
        <line x1="0" y1="90" x2="400" y2="90" stroke="var(--color-hairline)" stroke-dasharray="2" />
        <line x1="0" y1="50" x2="400" y2="50" stroke="var(--color-hairline)" stroke-dasharray="2" />
        <line x1="0" y1="10" x2="400" y2="10" stroke="var(--color-hairline)" stroke-dasharray="2" />
        <path d="${pathD}" fill="none" stroke="${colorClass}" stroke-width="3" stroke-linecap="round" stroke-linejoin="round" />
        ${dotsAndLabels}
      </svg>
    `;
  }
};

window.CapacityPlanning = CapacityPlanning;
