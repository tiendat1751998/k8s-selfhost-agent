const EventCorrelation = {
  init() {
    this.container = document.getElementById('event-correlation');
    if (!this.container) return;
    this.render();
    this.bindEvents();
    this.loadData();
  },

  render() {
    this.container.innerHTML = `
      <div class="correlation-container fade-in">
        <div class="correlation-header">
          <div>
            <h2>Event Correlation Engine</h2>
            <p class="text-muted">AI-driven noise reduction and root cause analysis</p>
          </div>
          <div class="correlation-actions">
            <select class="select select-bordered" id="correlation-filter">
              <option value="active">Active Issues</option>
              <option value="resolved">Resolved</option>
              <option value="all">All Time</option>
            </select>
            <button class="btn btn-outline" id="btn-refresh-correlation">
              <span class="icon">🔄</span> Refresh
            </button>
          </div>
        </div>

        <div class="correlation-metrics">
          <div class="metric-card">
            <div class="metric-title">Raw Events Reduced</div>
            <div class="metric-value text-success">94%</div>
            <div class="metric-subtext">14,230 events → 12 actionable issues</div>
          </div>
          <div class="metric-card">
            <div class="metric-title">Active Chains</div>
            <div class="metric-value text-warning">3</div>
            <div class="metric-subtext">Currently investigating</div>
          </div>
          <div class="metric-card">
            <div class="metric-title">Auto-resolved</div>
            <div class="metric-value">45</div>
            <div class="metric-subtext">In the last 24h</div>
          </div>
        </div>

        <div class="correlation-chains" id="correlation-chains">
          <div class="skeleton h-32 w-full mb-4"></div>
          <div class="skeleton h-32 w-full"></div>
        </div>
      </div>
    `;
  },

  bindEvents() {
    const btnRefresh = document.getElementById('btn-refresh-correlation');
    if (btnRefresh) {
      btnRefresh.addEventListener('click', () => {
        btnRefresh.classList.add('loading');
        setTimeout(() => {
          btnRefresh.classList.remove('loading');
          this.loadData();
        }, 800);
      });
    }

    const filter = document.getElementById('correlation-filter');
    if (filter) {
      filter.addEventListener('change', () => this.loadData());
    }
  },

  async loadData() {
    try {
      const json = await APIClient.get('/correlation');
      const items = json.data || [];
      this.renderChains(items);
      return;
    } catch (e) {
      console.error('Correlation API request failed:', e);
    }
    this.renderChains([]);
  },

  renderChains(data) {
    const chainsContainer = document.getElementById('correlation-chains');
    if (!chainsContainer) return;

    if (!data || data.length === 0) {
      chainsContainer.innerHTML = `
        <div class="empty-state" style="padding:var(--space-lg);text-align:center;color:var(--color-muted);">
          <div style="font-size:48px;margin-bottom:var(--space-sm);">🔗</div>
          <h3 style="margin:0;font-weight:600;">No Correlation Chains Found</h3>
          <p style="font-size:13px;max-width:400px;margin:var(--space-xs) auto 0;">The event correlation engine is active. Any correlated incident groups will appear here.</p>
        </div>
      `;
      return;
    }

    chainsContainer.innerHTML = data.map(m => {
      const severity = m.severity || 'medium';
      const eventCount = m.event_count || (m.event_ids ? m.event_ids.length : 0);
      const timeAgo = m.correlated_at ? new Date(m.correlated_at).toLocaleString() : '';
      return `
      <div class="correlation-chain-card ${severity}">
        <div class="chain-header">
          <div class="chain-title-area">
            <span class="badge badge-${severity === 'critical' ? 'danger' : 'warning'}">${severity.toUpperCase()}</span>
            <h3>${m.title || ''}</h3>
            <span class="chain-id">${m.id || ''}</span>
          </div>
          <div class="chain-meta">
            <span class="badge badge-outline">${eventCount} events correlated</span>
            <span class="text-muted text-sm">${timeAgo}</span>
          </div>
        </div>
        <div class="chain-root-cause">
          <strong>🧠 AI Probable Root Cause:</strong> ${m.root_cause || 'Analysis pending...'}
        </div>
        <div class="chain-footer">
          <div class="chain-context">
            <span><strong>Cluster:</strong> ${m.cluster || ''}</span>
            <span><strong>Namespace:</strong> ${m.namespace || ''}</span>
          </div>
          <div class="chain-actions">
            <button class="btn btn-sm btn-outline">View RCA</button>
            <button class="btn btn-sm btn-primary">Run Playbook</button>
          </div>
        </div>
      </div>`;
    }).join('');
  }
};

window.EventCorrelation = EventCorrelation;
