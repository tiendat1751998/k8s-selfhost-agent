const HealthCenter = {
  init() {
    if (this.initialized) return;
    this.container = document.getElementById('health-center');
    if (!this.container) return;
    this.initialized = true;
    this.render();
    this.bindEvents();
    this.loadData();
    
    // Auto refresh every 30s when active
    if (this.refreshInterval) clearInterval(this.refreshInterval);
    this.refreshInterval = setInterval(() => this.loadData(), 30000);

    // Dynamic start/stop interval on tab switches to prevent leaks
    if (this.navListener) {
      this.navListener();
    }
    this.navListener = AppState.on('navigate', (section) => {
      if (section === 'health') {
        if (!this.refreshInterval) {
          this.loadData();
          this.refreshInterval = setInterval(() => this.loadData(), 30000);
        }
      } else {
        if (this.refreshInterval) {
          clearInterval(this.refreshInterval);
          this.refreshInterval = null;
        }
      }
    });
  },

  render() {
    this.container.innerHTML = `
      <div class="health-container fade-in">
        <div class="health-header">
          <div>
            <h2>Platform Health</h2>
            <p class="text-muted">Real-time status of internal platform components</p>
          </div>
          <div class="health-actions">
            <div class="last-updated" id="health-last-updated">Updating...</div>
            <button class="btn btn-outline" id="btn-refresh-health">
              <span class="icon">🔄</span> Refresh
            </button>
          </div>
        </div>

        <div class="health-grid" id="health-center-grid">
          <!-- Component cards injected here -->
        </div>

      </div>
    `;
  },

  bindEvents() {
    const btnRefresh = document.getElementById('btn-refresh-health');
    if (btnRefresh) {
      btnRefresh.addEventListener('click', async () => {
        btnRefresh.classList.add('loading');
        try {
          await this.loadData();
        } catch (e) {
          console.warn('Health refresh error', e);
        } finally {
          setTimeout(() => btnRefresh.classList.remove('loading'), 500);
        }
      });
    }
  },

  async loadData() {
    try {
      const json = await APIClient.get('/health');
      const items = json.data || [];
      this.renderComponents(items);
    } catch (e) {
      console.warn('Health API error:', e);
      const grid = document.getElementById('health-center-grid');
      if (grid) {
        grid.innerHTML = '<div class="text-error text-center p-4 w-full col-span-3">Failed to contact health center service ❌</div>';
      }
    }
    
    const timeSpan = document.getElementById('health-last-updated');
    if (timeSpan) {
      timeSpan.innerText = 'Last updated: ' + new Date().toLocaleTimeString();
    }
  },

  renderComponents(data) {
    const grid = document.getElementById('health-center-grid');
    if (!grid) return;

    const iconMap = {
      frontend: '🌐',
      backend: '⚙️',
      websocket: '⚡',
      database: '🗄️',
      ai: '🤖',
      gitops: '🐝',
      kubernetes: '☸️'
    };

    const nameMap = {
      frontend: 'Frontend UI Client',
      backend: 'Core API Server',
      websocket: 'Real-time WebSocket Hub',
      database: 'PostgreSQL Database Pool',
      gitops: 'Docker Swarm Provider',
      kubernetes: 'Kubernetes API Control Plane'
    };

    if (data.length === 0) {
      grid.innerHTML = '<div class="text-center w-full col-span-3 text-muted">No component health status reported.</div>';
      return;
    }

    grid.innerHTML = data.map(m => {
      const icon = iconMap[m.component] || '📦';
      const name = nameMap[m.id] || m.component || m.name || 'Component';
      const status = m.status || 'healthy';
      return `
      <div class="health-card ${status}">
        <div class="health-icon">${icon}</div>
        <div class="health-info">
          <h3>${name}</h3>
          <div class="health-status-text">
            <span class="status-indicator"></span>
            ${status.toUpperCase()}
          </div>
          <div class="health-message text-muted text-sm">${m.message || ''}</div>
        </div>
        <div class="health-metrics">
          <div class="metric">
            <span class="label">Last Checked</span>
            <span class="value">${m.last_checked_at ? new Date(m.last_checked_at).toLocaleTimeString() : 'N/A'}</span>
          </div>
        </div>
      </div>`;
    }).join('');
  }
};

window.HealthCenter = HealthCenter;
