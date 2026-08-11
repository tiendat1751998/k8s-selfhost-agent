const ResourceExplorer = {
  init() {
    this.container = document.getElementById('resource-explorer');
    if (!this.container) return;
    this.render();
    this.bindEvents();
    
    // Parse kind from route (e.g. #explorer/Node)
    const hashParts = window.location.hash.replace('#', '').split('/');
    if (hashParts.length > 1 && hashParts[0] === 'explorer') {
      const kind = hashParts[1];
      const kindSelect = document.getElementById('search-kind');
      if (kindSelect) {
        // match case insensitively
        Array.from(kindSelect.options).forEach(opt => {
          if (opt.value.toLowerCase() === kind.toLowerCase()) {
            kindSelect.value = opt.value;
          }
        });
      }
    }
    
    this.loadData();
    
    // Listen for navigation events to update kind if already mounted
    if (window.AppState) {
      window.AppState.on('navigate', (route) => {
        const parts = route.split('/');
        if (parts[0] === 'explorer' && parts[1]) {
          const kindSelect = document.getElementById('search-kind');
          if (kindSelect) {
            Array.from(kindSelect.options).forEach(opt => {
              if (opt.value.toLowerCase() === parts[1].toLowerCase()) {
                kindSelect.value = opt.value;
              }
            });
            this.loadData();
          }
        }
      });
    }
  },

  render() {
    this.container.innerHTML = `
      <div class="explorer-container fade-in">
        <div class="explorer-header">
          <div>
            <h2>Resource Explorer</h2>
            <p class="text-muted">Global search across all clusters and namespaces</p>
          </div>
          <div class="explorer-actions">
            <button class="btn btn-outline">
              <span class="icon">🔖</span> Saved Searches
            </button>
            <button class="btn btn-primary" id="btn-search">
              <span class="icon">🔍</span> Search
            </button>
          </div>
        </div>

        <div class="explorer-filters">
          <div class="filter-group">
            <label>Query</label>
            <input type="text" class="input input-bordered" id="search-query" placeholder="Name or tag...">
          </div>
          <div class="filter-group">
            <label>Kind</label>
            <select class="select select-bordered" id="search-kind">
              <option value="">Any Kind</option>
              <option value="Pod">Pod</option>
              <option value="Deployment">Deployment</option>
              <option value="Service">Service</option>
              <option value="Ingress">Ingress</option>
              <option value="Node">Node</option>
              <option value="PersistentVolumeClaim">PVC</option>
            </select>
          </div>
          <div class="filter-group">
            <label>Cluster</label>
            <select class="select select-bordered" id="search-cluster">
              <option value="">All Clusters</option>
            </select>
          </div>
        </div>

        <div class="explorer-results">
          <div class="results-header">
            <h3>Search Results <span class="badge badge-primary" id="results-count">0</span></h3>
          </div>
          <div class="table-container">
            <table class="table">
              <thead>
                <tr>
                  <th>Kind</th>
                  <th>Name</th>
                  <th>Namespace</th>
                  <th>Cluster</th>
                  <th>Status</th>
                  <th>Age</th>
                  <th>Actions</th>
                </tr>
              </thead>
              <tbody id="explorer-tbody">
                <tr><td colspan="7" class="text-center">Enter search criteria</td></tr>
              </tbody>
            </table>
          </div>
        </div>
      </div>
    `;
  },

  bindEvents() {
    const btnSearch = document.getElementById('btn-search');
    if (btnSearch) {
      btnSearch.addEventListener('click', () => {
        btnSearch.classList.add('loading');
        setTimeout(() => {
          btnSearch.classList.remove('loading');
          this.loadData();
        }, 600);
      });
    }

    const inputs = ['search-query', 'search-kind', 'search-cluster'];
    inputs.forEach(id => {
      const el = document.getElementById(id);
      if (el) {
        el.addEventListener('keypress', (e) => {
          if (e.key === 'Enter') this.loadData();
        });
      }
    });
  },

  async loadData() {
    const query = document.getElementById('search-query')?.value || '';
    const kind = document.getElementById('search-kind')?.value || '';
    const cluster = document.getElementById('search-cluster')?.value || '';

    try {
      const qs = new URLSearchParams({ q: query, kind, cluster }).toString();
      const json = await APIClient.get(`/explorer?${qs}`);
      const items = json.data || [];
      this.renderTable(items, json.total || items.length);
    } catch (e) {
      this.showError('Failed to load explorer data: ' + e.message);
    }
  },

  showError(msg) {
    const tbody = document.getElementById('explorer-tbody');
    const countSpan = document.getElementById('results-count');
    if (tbody) tbody.innerHTML = `<tr><td colspan="7" class="text-center text-danger">${msg}</td></tr>`;
    if (countSpan) countSpan.innerText = '0';
  },

  renderTable(data, total) {
    const tbody = document.getElementById('explorer-tbody');
    const countSpan = document.getElementById('results-count');
    if (!tbody) return;
    if (countSpan) countSpan.innerText = total || data.length;

    if (data.length === 0) {
      tbody.innerHTML = `<tr><td colspan="7" style="padding: var(--space-md) 0;">` + UIComponents.emptyState({
        title: 'No Resources Found',
        description: 'No Kubernetes workloads, namespaces, or node assets matched the query filters.',
        icon: '🧭'
      }) + `</td></tr>`;
      return;
    }

    tbody.innerHTML = data.map(m => `
      <tr>
        <td><span class="badge badge-outline">${m.kind || m.resource_kind || ''}</span></td>
        <td><strong>${m.name || ''}</strong></td>
        <td>${m.namespace || m.ns || '-'}</td>
        <td>${m.cluster || ''}</td>
        <td><span class="badge badge-success">${m.status || 'Active'}</span></td>
        <td>${m.created_at ? new Date(m.created_at).toLocaleDateString() : (m.age || '')}</td>
        <td>
          <button class="btn btn-sm btn-ghost">View Details</button>
        </td>
      </tr>
    `).join('');
  }
};

window.ResourceExplorer = ResourceExplorer;
