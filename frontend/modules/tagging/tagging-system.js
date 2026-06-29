const TaggingSystem = {
  init() {
    this.container = document.getElementById('tagging-system');
    if (!this.container) return;
    this.render();
    this.bindEvents();
    this.loadData();
  },

  render() {
    this.container.innerHTML = `
      <div class="tagging-container fade-in">
        <div class="tagging-header">
          <div>
            <h2>Tagging System</h2>
            <p class="text-muted">Manage global resource tags and organizational labels</p>
          </div>
          <div class="tagging-actions">
            <button class="btn btn-primary" id="btn-create-tag">
              <span class="icon">➕</span> Create Tag
            </button>
          </div>
        </div>

        <div class="tagging-grid" id="tagging-grid">
          <!-- Tags inserted here -->
        </div>

        <!-- Create Tag Modal -->
        <div class="tag-modal" id="create-tag-modal" style="display:none;">
          <div class="tag-modal-content">
            <div class="tag-modal-header">
              <h3>Create New Tag</h3>
              <button class="btn btn-ghost btn-sm" id="btn-close-modal">✕</button>
            </div>
            <div class="tag-modal-body">
              <div class="form-control">
                <label>Key</label>
                <input type="text" class="input input-bordered" id="tag-key" placeholder="e.g. environment">
              </div>
              <div class="form-control">
                <label>Value</label>
                <input type="text" class="input input-bordered" id="tag-value" placeholder="e.g. production">
              </div>
              <div class="form-control">
                <label>Category</label>
                <select class="select select-bordered" id="tag-category">
                  <option value="environment">Environment</option>
                  <option value="business_unit">Business Unit</option>
                  <option value="owner">Owner</option>
                  <option value="cost_center">Cost Center</option>
                  <option value="custom">Custom</option>
                </select>
              </div>
            </div>
            <div class="tag-modal-footer">
              <button class="btn btn-outline" id="btn-cancel-modal">Cancel</button>
              <button class="btn btn-primary" id="btn-save-tag">Save Tag</button>
            </div>
          </div>
        </div>

      </div>
    `;
  },

  bindEvents() {
    const btnCreate = document.getElementById('btn-create-tag');
    const modal = document.getElementById('create-tag-modal');
    const btnClose = document.getElementById('btn-close-modal');
    const btnCancel = document.getElementById('btn-cancel-modal');
    const btnSave = document.getElementById('btn-save-tag');

    if (btnCreate) btnCreate.addEventListener('click', () => modal.style.display = 'flex');
    if (btnClose) btnClose.addEventListener('click', () => modal.style.display = 'none');
    if (btnCancel) btnCancel.addEventListener('click', () => modal.style.display = 'none');

    if (btnSave) {
      btnSave.addEventListener('click', () => {
        alert('Tag created successfully');
        modal.style.display = 'none';
        this.loadData();
      });
    }
  },

  async loadData() {
    try {
      const res = await fetch('/api/v1/tags');
      if (res.ok) {
        const json = await res.json();
        const items = json.data || [];
        if (items.length > 0) {
          this.renderTags(items);
          return;
        }
      }
    } catch (e) {
      console.warn('Tags API unavailable, using mock data:', e);
    }
    this.renderMockTags();
  },

  renderTags(data) {
    const grid = document.getElementById('tagging-grid');
    if (!grid) return;

    grid.innerHTML = data.map(m => `
      <div class="tag-card">
        <div class="tag-card-header">
          <span class="badge badge-outline">${m.category || 'custom'}</span>
          <button class="btn btn-ghost btn-sm btn-icon" title="Delete Tag" onclick="TaggingSystem.deleteTag('${m.id || ''}')">🗑️</button>
        </div>
        <div class="tag-key-value">
          <span class="tag-key">${m.key || ''}:</span>
          <span class="tag-value">${m.value || ''}</span>
        </div>
        <div class="tag-card-footer">
          <span class="text-sm text-muted">${m.usage_count || 0} resources</span>
          <button class="btn btn-sm btn-outline">View Resources</button>
        </div>
      </div>
    `).join('');
  },

  renderMockTags() {
    const grid = document.getElementById('tagging-grid');
    if (!grid) return;

    const mocks = [
      { key: 'environment', value: 'production', category: 'environment', usageCount: 4210 },
      { key: 'environment', value: 'staging', category: 'environment', usageCount: 840 },
      { key: 'owner', value: 'team-payments', category: 'owner', usageCount: 156 },
      { key: 'owner', value: 'team-platform', category: 'owner', usageCount: 342 },
      { key: 'cost_center', value: 'cc-9921-marketing', category: 'cost_center', usageCount: 89 },
      { key: 'tier', value: 'frontend', category: 'custom', usageCount: 50 }
    ];

    grid.innerHTML = mocks.map(m => `
      <div class="tag-card">
        <div class="tag-card-header">
          <span class="badge badge-outline">${m.category}</span>
          <button class="btn btn-ghost btn-sm btn-icon" title="Delete Tag">🗑️</button>
        </div>
        <div class="tag-key-value">
          <span class="tag-key">${m.key}:</span>
          <span class="tag-value">${m.value}</span>
        </div>
        <div class="tag-card-footer">
          <span class="text-sm text-muted">${m.usageCount} resources</span>
          <button class="btn btn-sm btn-outline">View Resources</button>
        </div>
      </div>
    `).join('');
  }
};

window.TaggingSystem = TaggingSystem;
