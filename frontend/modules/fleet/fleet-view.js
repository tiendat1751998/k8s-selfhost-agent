const FleetView = {
  init() {
    this.container = document.getElementById('fleet-view');
    if (!this.container) return;
    this.render();
    this.bindEvents();
    setTimeout(() => {
      this.loadData();
    }, 400);
  },

  render() {
    this.container.innerHTML = `
      <div class="fleet-container fade-in">
        <div class="fleet-header">
          <div>
            <h2>Multi-Cluster Fleet View</h2>
            <p class="text-muted">Manage clusters across all environments and regions</p>
          </div>
          <div class="fleet-actions">
            <button class="btn btn-outline" id="btn-fleet-scan">
              <span class="icon">🛡️</span> Security Scan All
            </button>
            <button class="btn btn-primary" id="btn-add-cluster">
              <span class="icon">➕</span> Register Cluster
            </button>
          </div>
        </div>

        <div class="fleet-filters">
          <div class="form-control">
            <input type="text" class="input input-bordered" placeholder="Search clusters..." id="fleet-search">
          </div>
          <div class="form-control">
            <select class="select select-bordered" id="fleet-group">
              <option value="">All Groups</option>
              <option value="production">Production</option>
              <option value="staging">Staging</option>
              <option value="edge">Edge</option>
            </select>
          </div>
        </div>

        <div class="fleet-grid" id="fleet-grid">
          <!-- Cluster cards will be rendered here -->
          <div class="skeleton" style="height:200px; grid-column: span 3; border-radius:var(--rounded-lg);"></div>
        </div>
      </div>
    `;
  },

  bindEvents() {
    const btnScan = document.getElementById('btn-fleet-scan');
    if (btnScan) {
      btnScan.addEventListener('click', () => {
        alert('Initiated security scan across all active clusters.');
      });
    }

    const searchInput = document.getElementById('fleet-search');
    const groupSelect = document.getElementById('fleet-group');
    const btnAdd = document.getElementById('btn-add-cluster');

    if (btnAdd) {
      btnAdd.addEventListener('click', () => {
        const mainAddBtn = document.getElementById('add-cluster-btn');
        if (mainAddBtn) {
          mainAddBtn.click();
        } else {
          alert('Please go to the Infrastructure > Clusters section to add a cluster');
        }
      });
    }

    const filterFn = () => this.filterCards(searchInput.value, groupSelect.value);

    if (searchInput) searchInput.addEventListener('input', filterFn);
    if (groupSelect) groupSelect.addEventListener('change', filterFn);
  },

  async loadData() {
    try {
      const res = await fetch('/api/v1/fleet');
      if (!res.ok) throw new Error('API request failed');
      
      const json = await res.json();
      const items = json.data || [];
      this.mocks = items;
      this.filterCards('', '');
    } catch (e) {
      this.showError('Failed to load fleet data: ' + e.message);
    }
  },

  showError(msg) {
    const grid = document.getElementById('fleet-grid');
    if (grid) {
      grid.innerHTML = `<div class="text-center text-danger w-100 py-10">${msg}</div>`;
    }
  },


  filterCards(query, group) {
    const grid = document.getElementById('fleet-grid');
    if (!grid) return;

    let filtered = this.mocks;
    if (query) {
      query = query.toLowerCase();
      filtered = filtered.filter(c => c.name.toLowerCase().includes(query) || c.id.toLowerCase().includes(query));
    }
    if (group) {
      filtered = filtered.filter(c => c.group === group);
    }

    if (filtered.length === 0) {
      grid.innerHTML = '<div class="text-center text-muted w-100 py-10">No clusters found matching filters.</div>';
      return;
    }

    grid.innerHTML = filtered.map(c => `
      <div class="fleet-card ${c.status === 'active' ? 'border-primary' : (c.status === 'upgrading' ? 'border-warning' : '')}">
        <div class="fleet-card-header">
          <h3>${c.name}</h3>
          <span class="badge badge-${c.status === 'active' ? 'success' : (c.status === 'upgrading' ? 'warning' : 'ghost')}">${c.status.toUpperCase()}</span>
        </div>
        <div class="fleet-details">
          <div class="fleet-detail">
            <span class="label">Group</span>
            <span class="value">${c.group}</span>
          </div>
          <div class="fleet-detail">
            <span class="label">Region</span>
            <span class="value">${c.region}</span>
          </div>
          <div class="fleet-detail">
            <span class="label">Provider</span>
            <span class="value">${c.provider.toUpperCase()}</span>
          </div>
          <div class="fleet-detail">
            <span class="label">K8s Version</span>
            <span class="value">${c.version}</span>
          </div>
          <div class="fleet-detail">
            <span class="label">Nodes</span>
            <span class="value">${c.nodes}</span>
          </div>
        </div>
        <div class="fleet-card-footer">
          <button class="btn btn-sm btn-ghost">Dashboard</button>
          <button class="btn btn-sm btn-outline" ${c.status === 'upgrading' ? 'disabled' : ''}>Upgrade</button>
        </div>
      </div>
    `).join('');
  }
};

window.FleetView = FleetView;
