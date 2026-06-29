const DriftDetection = {
  init() {
    this.container = document.getElementById('drift-detection');
    if (!this.container) return;
    this.render();
    this.bindEvents();
    this.loadData();
  },

  render() {
    this.container.innerHTML = `
      <div class="drift-container fade-in">
        <div class="drift-header">
          <h2>GitOps Drift Detection</h2>
          <div class="drift-actions">
            <button class="btn btn-outline" id="btn-sync-all">
              <span class="icon">🔄</span> Sync All
            </button>
            <button class="btn btn-primary" id="btn-scan-drift">
              <span class="icon">🔎</span> Scan Now
            </button>
          </div>
        </div>

        <div class="drift-summary">
          <div class="drift-stat warning">
            <span class="stat-value">3</span>
            <span class="stat-label">Resources Drifted</span>
          </div>
          <div class="drift-stat success">
            <span class="stat-value">1,245</span>
            <span class="stat-label">In Sync</span>
          </div>
          <div class="drift-stat">
            <span class="stat-value">2m ago</span>
            <span class="stat-label">Last Scan</span>
          </div>
        </div>

        <div class="drift-list">
          <h3>Drifted Resources</h3>
          <div class="table-container">
            <table class="table">
              <thead>
                <tr>
                  <th>Resource</th>
                  <th>Kind</th>
                  <th>Namespace</th>
                  <th>Cluster</th>
                  <th>Status</th>
                  <th>Detected</th>
                  <th>Actions</th>
                </tr>
              </thead>
              <tbody id="drift-tbody">
                <tr><td colspan="7" class="text-center">Loading drift data...</td></tr>
              </tbody>
            </table>
          </div>
        </div>

        <!-- Diff Viewer Modal -->
        <div class="drift-modal" id="drift-diff-modal" style="display:none;">
          <div class="drift-modal-content">
            <div class="drift-modal-header">
              <h3>Configuration Diff: <span id="diff-resource-name"></span></h3>
              <button class="btn btn-ghost btn-sm" id="btn-close-diff">✕</button>
            </div>
            <div class="drift-modal-body">
              <div class="diff-viewer">
                <div class="diff-pane expected">
                  <h4>Expected State (Git)</h4>
                  <pre><code id="diff-expected"></code></pre>
                </div>
                <div class="diff-pane actual">
                  <h4>Actual State (Cluster)</h4>
                  <pre><code id="diff-actual"></code></pre>
                </div>
              </div>
            </div>
            <div class="drift-modal-footer">
              <button class="btn btn-outline">Generate PR</button>
              <button class="btn btn-warning">Sync (Overwrite Cluster)</button>
            </div>
          </div>
        </div>
      </div>
    `;
  },

  bindEvents() {
    const btnScan = document.getElementById('btn-scan-drift');
    if (btnScan) {
      btnScan.addEventListener('click', () => {
        btnScan.classList.add('loading');
        setTimeout(() => {
          btnScan.classList.remove('loading');
          this.loadData();
        }, 1000);
      });
    }

    const btnClose = document.getElementById('btn-close-diff');
    if (btnClose) {
      btnClose.addEventListener('click', () => {
        document.getElementById('drift-diff-modal').style.display = 'none';
      });
    }
  },

  async loadData() {
    try {
      const res = await fetch('/api/v1/drift');
      if (res.ok) {
        const json = await res.json();
        this.drifts = json.data || [];
        this.renderTable(this.drifts);
        this.updateSummary(this.drifts, json.total || this.drifts.length);
      } else {
        const tbody = document.getElementById('drift-tbody');
        if (tbody) {
          tbody.innerHTML = '<tr><td colspan="7" class="text-center text-error">Failed to load live config drifts from API ❌</td></tr>';
        }
      }
    } catch (e) {
      console.warn('Drift API error:', e);
      const tbody = document.getElementById('drift-tbody');
      if (tbody) {
        tbody.innerHTML = '<tr><td colspan="7" class="text-center text-error">Failed to contact drift detection service ❌</td></tr>';
      }
    }
  },

  updateSummary(data, total) {
    const driftedCount = data.filter(d => d.status === 'drifted').length;
    const inSyncCount = Math.max(0, total - driftedCount);
    const statValues = this.container.querySelectorAll('.stat-value');
    if (statValues[0]) statValues[0].textContent = driftedCount;
    if (statValues[1]) statValues[1].textContent = inSyncCount.toLocaleString();
  },

  renderTable(data) {
    const tbody = document.getElementById('drift-tbody');
    if (!tbody) return;

    if (data.length === 0) {
      tbody.innerHTML = '<tr><td colspan="7" class="text-center">No drifted resources found — all in sync ✅</td></tr>';
      return;
    }

    tbody.innerHTML = data.map(d => `
      <tr>
        <td><strong>${d.resource || d.res || ''}</strong></td>
        <td><span class="badge badge-outline">${d.resource_kind || d.kind || ''}</span></td>
        <td>${d.namespace || d.ns || ''}</td>
        <td>${d.cluster || ''}</td>
        <td><span class="badge badge-warning">${d.status || 'drifted'}</span></td>
        <td>${d.detected_at ? new Date(d.detected_at).toLocaleString() : (d.detected || '')}</td>
        <td>
          <button class="btn btn-sm btn-ghost" onclick="DriftDetection.showDiff('${d.resource || d.res || ''}')">View Diff</button>
        </td>
      </tr>
    `).join('');
  },

  showDiff(resource) {
    document.getElementById('diff-resource-name').innerText = resource;
    
    const item = (this.drifts || []).find(d => (d.resource || d.res) === resource);
    if (item) {
      document.getElementById('diff-expected').innerText = item.expected_state || '';
      document.getElementById('diff-actual').innerText = item.actual_state || '';
    } else {
      document.getElementById('diff-expected').innerText = 'No expected config found';
      document.getElementById('diff-actual').innerText = 'No actual config found';
    }

    document.getElementById('drift-diff-modal').style.display = 'flex';
  }
};

window.DriftDetection = DriftDetection;
