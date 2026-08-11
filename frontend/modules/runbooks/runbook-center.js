/**
 * Runbook Center Module
 * Operational runbooks library with categories, search, and CRUD.
 */
(function (global) {
  'use strict';

  var runbooks = [];
  var loadingRunbooks = false;

  var catColors = { Application: '#6366f1', Infrastructure: '#06b6d4', Database: '#f97316', Security: '#ef4444', Network: '#10b981' };

  function countSteps(content) {
    if (!content) return 0;
    var lines = content.split('\n');
    var count = 0;
    lines.forEach(function (line) {
      var trimmed = line.trim().toLowerCase();
      if (trimmed.startsWith('## step') || trimmed.startsWith('### step') || trimmed.startsWith('step') || /^\d+\.\s/.test(trimmed)) {
        count++;
      }
    });
    return count || 1; // Default to at least 1 step if content exists
  }

  function renderRunbooks() {
    var container = document.getElementById('runbook-grid');
    if (!container) return;

    if (loadingRunbooks) {
      container.innerHTML = '<div style="grid-column:1/-1;text-align:center;padding:var(--space-md);"><span class="loading loading-spinner"></span> Loading runbooks...</div>';
      return;
    }

    if (runbooks.length === 0) {
      container.innerHTML = UIComponents.emptyState({
        title: 'No Runbooks Registered',
        description: 'Create operational runbooks playbooks to document debugging procedures.',
        icon: '📓',
        actionText: 'Create Runbook',
        actionId: 'create-first-runbook-btn'
      });
      setTimeout(() => {
        const btn = document.getElementById('create-first-runbook-btn');
        if (btn) btn.addEventListener('click', () => global.RunbookCenter.showCreateModal());
      }, 50);
      return;
    }

    container.innerHTML = runbooks.map(function(rb) {
      var category = Security.escapeHTML(rb.category || 'Application');
      var col = catColors[category] || '#6b7280';
      var steps = rb.steps_count !== undefined ? rb.steps_count : (rb.steps || 0);
      
      var lastUsedVal = 'Never';
      if (rb.last_used_at) {
        lastUsedVal = new Date(rb.last_used_at).toLocaleDateString();
      } else if (rb.lastUsed) {
        lastUsedVal = rb.lastUsed;
      }

      var author = Security.escapeHTML(rb.author || 'SRE Team');
      var title = Security.escapeHTML(rb.title || '');
      var tags = rb.tags || [];

      return '<div class="panel" style="padding:var(--space-md);cursor:pointer;transition:transform 0.2s;" onmouseover="this.style.transform=\'translateY(-2px)\'" onmouseout="this.style.transform=\'none\'" onclick="RunbookCenter.viewRunbook(\'' + rb.id + '\')">'
        + '<div style="display:flex;align-items:center;gap:8px;margin-bottom:var(--space-sm);">'
        + '<span style="font-size:10px;background:' + col + ';color:#fff;padding:2px 8px;border-radius:4px;font-weight:600;">' + category + '</span>'
        + '<span style="font-size:11px;color:var(--color-muted);margin-left:auto;">' + steps + ' steps</span>'
        + '</div>'
        + '<h4 style="margin:0 0 8px;font-size:14px;">' + title + '</h4>'
        + '<div style="display:flex;flex-wrap:wrap;gap:4px;margin-bottom:8px;">'
        + tags.map(function(t){ return '<span style="font-size:10px;background:var(--color-surface);border:1px solid var(--color-hairline);padding:1px 6px;border-radius:3px;">' + Security.escapeHTML(t) + '</span>'; }).join('')
        + '</div>'
        + '<div style="font-size:11px;color:var(--color-muted);display:flex;justify-content:space-between;">'
        + '<span>By ' + author + '</span><span>Used ' + Security.escapeHTML(lastUsedVal) + '</span>'
        + '</div></div>';
    }).join('');
  }

  global.RunbookCenter = {
    init: function() { this.refresh(); },
    refresh: function() {
      var self = this;
      loadingRunbooks = true;
      renderRunbooks();

      APIClient.get('/runbooks')
        .then(function(res) {
          loadingRunbooks = false;
          if (res && res.data) {
            runbooks = res.data;
          } else {
            console.error('Failed to parse runbooks response.');
          }
          renderRunbooks();
        })
        .catch(function(err) {
          loadingRunbooks = false;
          console.error('Error fetching runbooks:', err);
          renderRunbooks();
        });
    },
    viewRunbook: function(id) {
      var rb = runbooks.find(function(r){ return r.id === id; });
      if (!rb || !global.Modal) return;
      
      var steps = rb.steps_count !== undefined ? rb.steps_count : (rb.steps || 0);
      var category = Security.escapeHTML(rb.category || 'Application');
      var author = Security.escapeHTML(rb.author || 'SRE Team');
      var title = Security.escapeHTML(rb.title || '');
      
      var checklistHtml = '';
      var numSteps = steps || 5;
      for (var i = 1; i <= numSteps; i++) {
        checklistHtml += '<div style="padding:8px;background:var(--color-surface);border-radius:6px;margin-bottom:6px;font-size:13px;"><input type="checkbox"> <strong>Step ' + i + ':</strong> ' + (i === 1 ? 'Identify affected resources' : (i === 2 ? 'Collect logs and metrics' : (i === 3 ? 'Apply remediation' : (i === 4 ? 'Verify recovery' : 'Document findings')))) + '</div>';
      }

      global.Modal.open({
        title: '📓 ' + title,
        body: '<div style="padding:var(--space-xs);"><p style="color:var(--color-muted);font-size:12px;">Category: ' + category + ' · ' + steps + ' steps · Author: ' + author + '</p>'
          + '<div style="margin-top:var(--space-sm);">'
          + checklistHtml
          + '</div>'
          + '<div style="margin-top:var(--space-md);display:flex;gap:8px;">'
          + '<button id="rb-delete-btn" class="btn btn-ghost btn-sm" style="color:#ef4444;margin-right:auto;" onclick="RunbookCenter.deleteRunbook(\'' + rb.id + '\')">Delete Runbook</button>'
          + '<button class="btn btn-ghost btn-sm" onclick="Modal.close()">Close</button>'
          + '</div>'
          + '</div>'
      });
    },
    showCreateModal: function() {
      if (!global.Modal) return;
      global.Modal.open({
        title: '➕ Create Runbook',
        body: '<div style="padding:var(--space-xs);">'
          + '<div class="form-group"><label class="form-label">Title</label><input type="text" class="form-select" id="rb-new-title" placeholder="e.g. Pod Recovery Procedure"></div>'
          + '<div class="form-group"><label class="form-label">Category</label><select class="form-select" id="rb-new-category"><option>Application</option><option>Infrastructure</option><option>Database</option><option>Security</option><option>Network</option></select></div>'
          + '<div class="form-group"><label class="form-label">Tags (comma-separated)</label><input type="text" class="form-select" id="rb-new-tags" placeholder="e.g. pod, restart, debugging"></div>'
          + '<div class="form-group"><label class="form-label">Content (Markdown)</label><textarea class="form-select" rows="6" id="rb-new-content" placeholder="## Step 1\nDescribe the first step..."></textarea></div>'
          + '<div style="display:flex;gap:8px;margin-top:var(--space-sm);">'
          + '<button id="rb-create-submit-btn" class="btn btn-primary btn-sm" onclick="RunbookCenter.createRunbook()">Create</button>'
          + '<button class="btn btn-ghost btn-sm" onclick="Modal.close()">Cancel</button>'
          + '</div>'
          + '</div>'
      });
    },
    createRunbook: function() {
      var titleInput = document.getElementById('rb-new-title');
      var categorySelect = document.getElementById('rb-new-category');
      var tagsInput = document.getElementById('rb-new-tags');
      var contentArea = document.getElementById('rb-new-content');
      var submitBtn = document.getElementById('rb-create-submit-btn');

      if (!titleInput || !titleInput.value.trim()) { alert('Please enter a title.'); return; }
      if (!contentArea || !contentArea.value.trim()) { alert('Please enter the content.'); return; }

      if (submitBtn) {
        submitBtn.disabled = true;
        submitBtn.textContent = 'Creating...';
      }

      var tags = [];
      if (tagsInput && tagsInput.value.trim()) {
        tags = tagsInput.value.split(',').map(function(t) { return t.trim(); }).filter(Boolean);
      }

      var content = contentArea.value.trim();
      var stepsCount = countSteps(content);

      var payload = {
        title: titleInput.value.trim(),
        category: categorySelect ? categorySelect.value : 'Application',
        content: content,
        tags: tags,
        author: 'SRE Team',
        steps_count: stepsCount
      };

      var self = this;
      APIClient.post('/runbooks', payload)
        .then(function(res) {
          if (res) {
            if (global.Modal) global.Modal.close();
            self.refresh();
          } else {
            alert('Failed to create runbook.');
            if (submitBtn) {
              submitBtn.disabled = false;
              submitBtn.textContent = 'Create';
            }
          }
        })
        .catch(function(err) {
          alert('Error creating runbook: ' + err.message);
          if (submitBtn) {
            submitBtn.disabled = false;
            submitBtn.textContent = 'Create';
          }
        });
    },
    deleteRunbook: function(id) {
      if (!confirm('Are you sure you want to delete this runbook?')) return;
      
      var deleteBtn = document.getElementById('rb-delete-btn');
      if (deleteBtn) {
        deleteBtn.disabled = true;
        deleteBtn.textContent = 'Deleting...';
      }

      var self = this;
      APIClient.delete('/runbooks/' + id)
        .then(function(res) {
          if (res && res.status === 'deleted') {
            if (global.Modal) global.Modal.close();
            self.refresh();
          } else {
            alert('Failed to delete runbook.');
            if (deleteBtn) {
              deleteBtn.disabled = false;
              deleteBtn.textContent = 'Delete Runbook';
            }
          }
        })
        .catch(function(err) {
          alert('Error deleting runbook: ' + err.message);
          if (deleteBtn) {
            deleteBtn.disabled = false;
            deleteBtn.textContent = 'Delete Runbook';
          }
        });
    }
  };
})(window);
