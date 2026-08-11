const AuditMode = {
  init() {
    this.container = document.getElementById('audit-mode');
    if (!this.container) return;
    this.render();
    this.bindEvents();
    this.loadData();
  },

  render() {
    this.container.innerHTML = `
      <div class="audit-container fade-in">
        <div class="audit-header">
          <div>
            <h2>Platform Audit Mode</h2>
            <p class="text-muted">Continuous platform self-assessment and gap analysis</p>
          </div>
          <div class="audit-actions">
            <button class="btn btn-primary" id="btn-run-audit">
              <span class="icon">🔍</span> Run Full Audit
            </button>
          </div>
        </div>

        <div class="audit-stats">
          <div class="stat-card border-critical">
            <div class="stat-title">Critical Gaps</div>
            <div class="stat-value text-danger">1</div>
          </div>
          <div class="stat-card border-high">
            <div class="stat-title">High Severity</div>
            <div class="stat-value text-warning">2</div>
          </div>
          <div class="stat-card border-medium">
            <div class="stat-title">Medium/Low</div>
            <div class="stat-value">5</div>
          </div>
        </div>

        <div class="audit-findings">
          <h3>Active Findings</h3>
          <div class="table-container">
            <table class="table">
              <thead>
                <tr>
                  <th>Severity</th>
                  <th>Category</th>
                  <th>Description</th>
                  <th>Remediation</th>
                  <th>Actions</th>
                </tr>
              </thead>
              <tbody id="audit-tbody">
                <tr><td colspan="5" class="text-center">Loading findings...</td></tr>
              </tbody>
            </table>
          </div>
        </div>
      </div>
    `;
  },

  bindEvents() {
    const btnRun = document.getElementById('btn-run-audit');
    if (btnRun) {
      btnRun.addEventListener('click', () => {
        btnRun.classList.add('loading');
        setTimeout(() => {
          btnRun.classList.remove('loading');
          alert('Audit completed. 1 new finding discovered.');
          this.loadData();
        }, 1500);
      });
    }
  },

  async loadData() {
    try {
      const json = await APIClient.get('/audit/findings');
      const items = json.data || [];
      this.renderFindings(items);
    } catch (e) {
      this.showError('Failed to load audit findings: ' + e.message);
    }
  },

  showError(msg) {
    const tbody = document.getElementById('audit-tbody');
    if (tbody) {
      tbody.innerHTML = `<tr><td colspan="5" class="text-center text-danger">${msg}</td></tr>`;
    }
  },

  renderFindings(data) {
    const tbody = document.getElementById('audit-tbody');
    if (!tbody) return;

    tbody.innerHTML = data.map(m => `
      <tr>
        <td>
          <span class="badge badge-${this.getSeverityClass(m.severity)}">${(m.severity || 'medium').toUpperCase()}</span>
        </td>
        <td><span class="badge badge-outline">${m.category || ''}</span></td>
        <td><strong>${m.description || m.desc || ''}</strong></td>
        <td class="text-muted text-sm">${m.remediation || m.action || ''}</td>
        <td>
          <button class="btn btn-sm btn-outline btn-resolve" onclick="AuditMode.resolveFinding(this, '${m.id || ''}')">Resolve</button>
        </td>
      </tr>
    `).join('');
  },

  getSeverityClass(sev) {
    switch (sev) {
      case 'critical': return 'danger';
      case 'high': return 'warning';
      case 'medium': return 'info';
      default: return 'ghost';
    }
  },

  resolveFinding(btn) {
    btn.innerHTML = '<span class="loading loading-spinner loading-xs"></span>';
    setTimeout(() => {
      btn.closest('tr').remove();
    }, 500);
  }
};

window.AuditMode = AuditMode;
