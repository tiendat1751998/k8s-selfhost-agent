window.FleetView = createDataTablePage({
  idPrefix: 'fleet',
  containerId: 'fleet-view',
  title: 'Multi-Cluster Fleet View',
  description: 'Manage clusters across all environments and regions',
  viewType: 'grid',
  actions: [
    { id: 'btn-fleet-scan', label: 'Security Scan All', icon: 'dY>,?' },
    { id: 'btn-add-cluster', label: 'Register Cluster', icon: 'z ', primary: true }
  ],
  filtersHtml: `
    <div class="form-control" style="flex:1;">
      <input type="text" class="input input-bordered w-100" placeholder="Search clusters..." id="fleet-search">
    </div>
    <div class="form-control" style="width:200px;">
      <select class="select select-bordered w-100" id="fleet-group">
        <option value="">All Groups</option>
        <option value="production">Production</option>
        <option value="staging">Staging</option>
        <option value="edge">Edge</option>
      </select>
    </div>
  `,
  endpoint: '/fleet',
  emptyMessage: 'No clusters found matching filters.',
  renderRow: function(c) {
    return `
      <div class="fleet-card ${c.status === 'active' ? 'border-primary' : (c.status === 'upgrading' ? 'border-warning' : '')}" style="background:var(--color-surface); padding:16px; border-radius:8px; border:1px solid var(--color-hairline);">
        <div class="fleet-card-header" style="display:flex; justify-content:space-between; align-items:center; margin-bottom:12px;">
          <h3 style="margin:0; font-size:16px;">${c.name}</h3>
          <span class="badge badge-${c.status === 'active' ? 'success' : (c.status === 'upgrading' ? 'warning' : 'ghost')}">${c.status.toUpperCase()}</span>
        </div>
        <div class="fleet-details" style="display:flex; flex-direction:column; gap:8px; margin-bottom:16px;">
          <div style="display:flex; justify-content:space-between;"><span class="text-muted">Group</span><span>${c.group}</span></div>
          <div style="display:flex; justify-content:space-between;"><span class="text-muted">Region</span><span>${c.region}</span></div>
          <div style="display:flex; justify-content:space-between;"><span class="text-muted">Provider</span><span>${c.provider.toUpperCase()}</span></div>
          <div style="display:flex; justify-content:space-between;"><span class="text-muted">K8s Version</span><span>${c.version}</span></div>
          <div style="display:flex; justify-content:space-between;"><span class="text-muted">Nodes</span><span>${c.nodes}</span></div>
        </div>
        <div class="fleet-card-footer" style="display:flex; gap:8px;">
          <button class="btn btn-sm btn-ghost" style="flex:1;">Dashboard</button>
          <button class="btn btn-sm btn-outline" style="flex:1;" ${c.status === 'upgrading' ? 'disabled' : ''}>Upgrade</button>
        </div>
      </div>
    `;
  },
  bindEvents: function() {
    const btnScan = document.getElementById('btn-fleet-scan');
    if (btnScan) btnScan.addEventListener('click', () => alert('Initiated security scan across all active clusters.'));

    const btnAdd = document.getElementById('btn-add-cluster');
    if (btnAdd) {
      btnAdd.addEventListener('click', () => {
        const mainAddBtn = document.getElementById('add-cluster-btn');
        if (mainAddBtn) mainAddBtn.click();
        else alert('Please go to the Infrastructure > Clusters section to add a cluster');
      });
    }

    const searchInput = document.getElementById('fleet-search');
    const groupSelect = document.getElementById('fleet-group');

    const filterFn = () => {
      const query = searchInput ? searchInput.value.toLowerCase() : '';
      const group = groupSelect ? groupSelect.value : '';
      let filtered = this.data || [];
      if (query) filtered = filtered.filter(c => c.name.toLowerCase().includes(query) || c.id.toLowerCase().includes(query));
      if (group) filtered = filtered.filter(c => c.group === group);
      this.renderContent(filtered);
    };

    if (searchInput) searchInput.addEventListener('input', filterFn);
    if (groupSelect) groupSelect.addEventListener('change', filterFn);
  }
});
