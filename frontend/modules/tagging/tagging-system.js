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
      btnSave.addEventListener('click', async () => {
        const keyEl = document.getElementById('tag-key');
        const valEl = document.getElementById('tag-value');
        const catEl = document.getElementById('tag-category');
        
        const key = keyEl ? keyEl.value.trim() : '';
        const value = valEl ? valEl.value.trim() : '';
        const category = catEl ? catEl.value : 'custom';
        
        if (!key || !value) {
          alert('Key and Value are required');
          return;
        }
        
        btnSave.disabled = true;
        btnSave.textContent = 'Saving...';
        
        try {
          const res = await fetch('/api/v1/tags', {
            method: 'POST',
            headers: {
              'Content-Type': 'application/json'
            },
            body: JSON.stringify({ key, value, category })
          });
          
          if (!res.ok) throw new Error('Failed to save tag');
          
          alert('Tag created successfully');
          modal.style.display = 'none';
          
          if (keyEl) keyEl.value = '';
          if (valEl) valEl.value = '';
          
          this.loadData();
        } catch (e) {
          alert('Error saving tag: ' + e.message);
        } finally {
          btnSave.disabled = false;
          btnSave.textContent = 'Save Tag';
        }
      });
    }
  },

  async loadData() {
    try {
      const res = await fetch('/api/v1/tags');
      if (res.ok) {
        const json = await res.json();
        const items = json.data || [];
        this.renderTags(items);
      } else {
        throw new Error('Failed to fetch tags');
      }
    } catch (e) {
      console.warn('Tags API unavailable:', e);
      const grid = document.getElementById('tagging-grid');
      if (grid) {
        grid.innerHTML = `<div class="text-center" style="grid-column:span 3;padding:24px;color:var(--color-muted);">Unable to load tags. Detail: ${e.message}</div>`;
      }
    }
  },

  renderTags(data) {
    const grid = document.getElementById('tagging-grid');
    if (!grid) return;

    if (data.length === 0) {
      grid.innerHTML = '<div class="text-center" style="grid-column:span 3;padding:48px 24px;color:var(--color-muted);">No tags defined yet. Click "Create Tag" to add one.</div>';
      return;
    }

    grid.innerHTML = data.map(m => `
      <div class="tag-card">
        <div class="tag-card-header">
          <span class="badge badge-outline">${esc(m.category || 'custom')}</span>
          <button class="btn btn-ghost btn-sm btn-icon" title="Delete Tag" onclick="TaggingSystem.deleteTag('${m.id || ''}')">🗑️</button>
        </div>
        <div class="tag-key-value">
          <span class="tag-key">${esc(m.key || '')}:</span>
          <span class="tag-value">${esc(m.value || '')}</span>
        </div>
        <div class="tag-card-footer">
          <span class="text-sm text-muted">${m.usage_count || 0} resources</span>
          <button class="btn btn-sm btn-outline">View Resources</button>
        </div>
      </div>
    `).join('');
  },

  async deleteTag(id) {
    if (!id) return;
    if (!confirm('Are you sure you want to delete this tag?')) return;
    
    try {
      const res = await fetch(`/api/v1/tags/${id}`, {
        method: 'DELETE'
      });
      if (!res.ok) throw new Error('Failed to delete tag');
      
      alert('Tag deleted successfully');
      this.loadData();
    } catch (e) {
      alert('Error deleting tag: ' + e.message);
    }
  }
};

window.TaggingSystem = TaggingSystem;
