const DeploymentPromotion = {
  init() {
    this.container = document.getElementById('deployment-promotion');
    if (!this.container) return;
    this.render();
    this.bindEvents();
    this.loadData();
  },

  render() {
    this.container.innerHTML = `
      <div class="promotion-container fade-in">
        <div class="promotion-header">
          <div>
            <h2>Deployment Promotion</h2>
            <p class="text-muted">Manage progressive release flow across environments</p>
          </div>
          <div class="promotion-actions">
            <button class="btn btn-outline" id="btn-refresh-promo">
              <span class="icon">🔄</span> Refresh
            </button>
            <button class="btn btn-primary" id="btn-new-promo">
              <span class="icon">🚀</span> New Promotion
            </button>
          </div>
        </div>

        <div class="promotion-pipeline">
          <div class="pipeline-stage" data-env="dev">
            <div class="stage-header">
              <span class="icon">💻</span> Development
            </div>
            <div class="stage-body" id="stage-dev">
              <!-- Cards here -->
            </div>
          </div>
          <div class="pipeline-arrow">➔</div>
          
          <div class="pipeline-stage" data-env="qa">
            <div class="stage-header">
              <span class="icon">🧪</span> QA / Testing
            </div>
            <div class="stage-body" id="stage-qa">
            </div>
          </div>
          <div class="pipeline-arrow">➔</div>
          
          <div class="pipeline-stage" data-env="staging">
            <div class="stage-header">
              <span class="icon">🎭</span> Staging
            </div>
            <div class="stage-body" id="stage-staging">
            </div>
          </div>
          <div class="pipeline-arrow">➔</div>
          
          <div class="pipeline-stage" data-env="production">
            <div class="stage-header production">
              <span class="icon">🌍</span> Production
            </div>
            <div class="stage-body" id="stage-production">
            </div>
          </div>
        </div>

      </div>
    `;
  },

  bindEvents() {
    const btnRefresh = document.getElementById('btn-refresh-promo');
    if (btnRefresh) {
      btnRefresh.addEventListener('click', () => {
        btnRefresh.classList.add('loading');
        setTimeout(() => {
          btnRefresh.classList.remove('loading');
          this.loadData();
        }, 800);
      });
    }
  },

  async loadData() {
    try {
      const json = await APIClient.get('/promotions');
      const items = json.data || [];
      this.renderPipeline(items);
      return;
    } catch (e) {
      console.error('Promotions API request failed:', e);
    }
    this.renderPipeline([]);
  },

  renderPipeline(data) {
    const stageMap = { dev: 'stage-dev', qa: 'stage-qa', staging: 'stage-staging', production: 'stage-production' };
    // Clear stages
    Object.values(stageMap).forEach(id => { const el = document.getElementById(id); if (el) el.innerHTML = ''; });

    if (!data || data.length === 0) {
      Object.keys(stageMap).forEach(key => {
        const id = stageMap[key];
        const el = document.getElementById(id);
        if (el) {
          el.innerHTML = `<div class="promo-card-empty" style="text-align:center;padding:var(--space-sm);color:var(--color-muted);font-size:12px;border:1px dashed var(--color-hairline);border-radius:6px;">No deployments in ${key}</div>`;
        }
      });
      return;
    }

    data.forEach(p => {
      const stageId = stageMap[p.to_env] || stageMap[p.from_env] || 'stage-dev';
      const el = document.getElementById(stageId);
      if (el) {
        const timeStr = p.created_at ? new Date(p.created_at).toLocaleString() : '';
        const needsApproval = p.status === 'pending';
        el.innerHTML += this.buildCard(p.service || '', p.version || '', timeStr, p.status || '', needsApproval);
      }
    });
  },

  buildCard(service, version, time, status, requiresApproval = false) {
    return `
      <div class="promo-card">
        <div class="promo-card-header">
          <strong>${Security.escapeHTML(service)}</strong>
          <span class="badge badge-outline">${Security.escapeHTML(version)}</span>
        </div>
        <div class="promo-card-body">
          <div class="text-sm text-muted">${Security.escapeHTML(time)}</div>
          <div class="text-sm ${requiresApproval ? 'text-warning' : 'text-success'}">${Security.escapeHTML(status)}</div>
        </div>
        ${requiresApproval ? `
        <div class="promo-card-actions">
          <button class="btn btn-sm btn-success w-full" onclick="alert('Promoted!')">Approve & Promote</button>
        </div>
        ` : ''}
      </div>
    `;
  }
};

window.DeploymentPromotion = DeploymentPromotion;
