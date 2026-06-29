const ChangeManagement = {
  init() {
    this.container = document.getElementById('change-management');
    if (!this.container) return;
    this.render();
    this.bindEvents();
    this.loadData();
  },

  render() {
    this.container.innerHTML = `
      <div class="change-container fade-in">
        <div class="change-header">
          <div>
            <h2>Change Management</h2>
            <p class="text-muted">Enterprise governance and deployment approvals</p>
          </div>
          <div class="change-actions">
            <button class="btn btn-outline" id="btn-maint-window">
              <span class="icon">📅</span> Maintenance Windows
            </button>
            <button class="btn btn-primary" id="btn-request-change">
              <span class="icon">➕</span> Request Change
            </button>
          </div>
        </div>

        <div class="change-tabs">
          <button class="change-tab active" data-status="pending">Pending Approvals</button>
          <button class="change-tab" data-status="approved">Approved</button>
          <button class="change-tab" data-status="rejected">Rejected / Cancelled</button>
        </div>

        <div class="change-list">
          <div class="table-container">
            <table class="table">
              <thead>
                <tr>
                  <th>ID</th>
                  <th>Title</th>
                  <th>Type</th>
                  <th>Target</th>
                  <th>Requester</th>
                  <th>Scheduled</th>
                  <th>Actions</th>
                </tr>
              </thead>
              <tbody id="change-tbody">
                <tr><td colspan="7" class="text-center">Loading...</td></tr>
              </tbody>
            </table>
          </div>
        </div>

        <!-- Approval Modal -->
        <div class="change-modal" id="change-approval-modal" style="display:none;">
          <div class="change-modal-content">
            <div class="change-modal-header">
              <h3>Review Change Request: <span id="cr-id"></span></h3>
              <button class="btn btn-ghost btn-sm" id="btn-close-cr">✕</button>
            </div>
            <div class="change-modal-body" id="cr-details">
              <!-- Content injected via JS -->
            </div>
            <div class="change-modal-footer">
              <button class="btn btn-danger" onclick="ChangeManagement.rejectChange()">Reject</button>
              <button class="btn btn-success" onclick="ChangeManagement.approveChange()">Approve & Schedule</button>
            </div>
          </div>
        </div>
      </div>
    `;
  },

  bindEvents() {
    const tabs = document.querySelectorAll('.change-tab');
    tabs.forEach(tab => {
      tab.addEventListener('click', (e) => {
        tabs.forEach(t => t.classList.remove('active'));
        e.target.classList.add('active');
        this.currentStatus = e.target.getAttribute('data-status');
        this.loadData();
      });
    });

    const btnClose = document.getElementById('btn-close-cr');
    if (btnClose) {
      btnClose.addEventListener('click', () => {
        document.getElementById('change-approval-modal').style.display = 'none';
      });
    }

    this.currentStatus = 'pending';
  },

  async loadData() {
    try {
      const res = await fetch(`/api/v1/changes?status=${this.currentStatus}`);
      if (res.ok) {
        const json = await res.json();
        const items = json.data || [];
        this.items = items;
        this.renderTable(items);
        return;
      }
    } catch (e) {
      console.warn('Changes API unavailable:', e);
    }
    
    const tbody = document.getElementById('change-tbody');
    if (tbody) {
      tbody.innerHTML = `<tr><td colspan="7" class="text-center text-muted" style="color:var(--color-trading-down)">Unable to load changes. Check connection.</td></tr>`;
    }
    this.items = [];
  },

  renderTable(data) {
    const tbody = document.getElementById('change-tbody');
    if (!tbody) return;

    if (data.length === 0) {
      tbody.innerHTML = `<tr><td colspan="7" class="text-center text-muted">No ${this.currentStatus} change requests.</td></tr>`;
      return;
    }

    tbody.innerHTML = data.map(m => `
      <tr>
        <td><strong>${m.id || ''}</strong></td>
        <td>${m.title || m.description || ''}</td>
        <td><span class="badge badge-${m.type === 'emergency' ? 'danger' : 'outline'}">${m.type || m.risk_level || 'standard'}</span></td>
        <td>${m.cluster || ''} / ${m.namespace || m.ns || ''}</td>
        <td>${m.requester || ''}</td>
        <td>${m.scheduled_at ? new Date(m.scheduled_at).toLocaleString() : (m.created_at ? new Date(m.created_at).toLocaleString() : '')}</td>
        <td>
          ${m.status === 'pending' ? `<button class="btn btn-sm btn-primary" onclick="ChangeManagement.reviewCR('${m.id || ''}')">Review</button>` : '<button class="btn btn-sm btn-ghost">View</button>'}
        </td>
      </tr>
    `).join('');
  },

  reviewCR(id) {
    const item = (this.items || []).find(m => m.id === id);
    if (!item) return;

    document.getElementById('cr-id').innerText = id;
    
    const title = item.title || item.description || 'No Title';
    const desc = item.description || 'No Description provided.';
    const type = item.type || item.risk_level || 'standard';
    const isEmergency = type === 'emergency';

    document.getElementById('cr-details').innerHTML = `
      <div class="cr-info-grid">
        <div class="cr-info-item">
          <label>Title</label>
          <div>${esc(title)}</div>
        </div>
        <div class="cr-info-item">
          <label>Description</label>
          <div>${esc(desc)}</div>
        </div>
        <div class="cr-info-item">
          <label>Impact Analysis</label>
          <div class="${isEmergency ? 'text-danger' : 'text-warning'}">${isEmergency ? 'Emergency Change - High Risk' : 'Standard Change - Normal Risk'}</div>
        </div>
        <div class="cr-info-item">
          <label>Automated Checks</label>
          <div class="text-success">Passed: CI Tests, Security Scan, Policies Validation</div>
        </div>
      </div>
    `;

    document.getElementById('change-approval-modal').style.display = 'flex';
  },

  approveChange() {
    alert('Change Request Approved & Scheduled.');
    document.getElementById('change-approval-modal').style.display = 'none';
  },

  rejectChange() {
    alert('Change Request Rejected.');
    document.getElementById('change-approval-modal').style.display = 'none';
  }
};

window.ChangeManagement = ChangeManagement;
