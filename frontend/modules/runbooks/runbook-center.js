/**
 * Runbook Center Module
 * Operational runbooks library with categories, search, and CRUD.
 */
(function (global) {
  'use strict';

  var runbooks = [
    { id: 'rb-001', title: 'Pod CrashLoopBackOff Recovery', category: 'Application', tags: ['pod', 'restart', 'debugging'], steps: 5, lastUsed: '2d ago', author: 'SRE Team' },
    { id: 'rb-002', title: 'Node NotReady Troubleshooting', category: 'Infrastructure', tags: ['node', 'kubelet', 'drain'], steps: 8, lastUsed: '5d ago', author: 'Platform Team' },
    { id: 'rb-003', title: 'Database Failover Procedure', category: 'Database', tags: ['postgres', 'failover', 'ha'], steps: 12, lastUsed: '1w ago', author: 'DBA Team' },
    { id: 'rb-004', title: 'Certificate Rotation', category: 'Security', tags: ['tls', 'cert', 'rotation'], steps: 6, lastUsed: '2w ago', author: 'Security Team' },
    { id: 'rb-005', title: 'Network Policy Debugging', category: 'Network', tags: ['network', 'calico', 'policy'], steps: 7, lastUsed: '3d ago', author: 'Platform Team' },
    { id: 'rb-006', title: 'OOMKilled Pod Investigation', category: 'Application', tags: ['oom', 'memory', 'limits'], steps: 6, lastUsed: '1d ago', author: 'SRE Team' },
    { id: 'rb-007', title: 'Cluster Upgrade Playbook', category: 'Infrastructure', tags: ['upgrade', 'k8s', 'rolling'], steps: 15, lastUsed: '1mo ago', author: 'Platform Team' },
    { id: 'rb-008', title: 'Incident Response Playbook', category: 'Security', tags: ['incident', 'response', 'escalation'], steps: 10, lastUsed: '4d ago', author: 'SRE Team' },
    { id: 'rb-009', title: 'Disaster Recovery - Full Restore', category: 'Infrastructure', tags: ['dr', 'backup', 'restore'], steps: 18, lastUsed: '2mo ago', author: 'Platform Team' }
  ];

  var catColors = { Application: '#6366f1', Infrastructure: '#06b6d4', Database: '#f97316', Security: '#ef4444', Network: '#10b981' };

  function renderRunbooks() {
    var container = document.getElementById('runbook-grid');
    if (!container) return;

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
      var col = catColors[rb.category] || '#6b7280';
      return '<div class="panel" style="padding:var(--space-md);cursor:pointer;transition:transform 0.2s;" onmouseover="this.style.transform=\'translateY(-2px)\'" onmouseout="this.style.transform=\'none\'" onclick="RunbookCenter.viewRunbook(\'' + rb.id + '\')">'
        + '<div style="display:flex;align-items:center;gap:8px;margin-bottom:var(--space-sm);">'
        + '<span style="font-size:10px;background:' + col + ';color:#fff;padding:2px 8px;border-radius:4px;font-weight:600;">' + rb.category + '</span>'
        + '<span style="font-size:11px;color:var(--color-muted);margin-left:auto;">' + rb.steps + ' steps</span>'
        + '</div>'
        + '<h4 style="margin:0 0 8px;font-size:14px;">' + rb.title + '</h4>'
        + '<div style="display:flex;flex-wrap:wrap;gap:4px;margin-bottom:8px;">'
        + rb.tags.map(function(t){ return '<span style="font-size:10px;background:var(--color-surface);border:1px solid var(--color-hairline);padding:1px 6px;border-radius:3px;">' + t + '</span>'; }).join('')
        + '</div>'
        + '<div style="font-size:11px;color:var(--color-muted);display:flex;justify-content:space-between;">'
        + '<span>By ' + rb.author + '</span><span>Used ' + rb.lastUsed + '</span>'
        + '</div></div>';
    }).join('');
  }

  global.RunbookCenter = {
    init: function() { this.refresh(); },
    refresh: function() { renderRunbooks(); },
    viewRunbook: function(id) {
      var rb = runbooks.find(function(r){ return r.id === id; });
      if (!rb || !global.Modal) return;
      global.Modal.open({
        title: '📓 ' + rb.title,
        body: '<div style="padding:var(--space-xs);"><p style="color:var(--color-muted);font-size:12px;">Category: ' + rb.category + ' · ' + rb.steps + ' steps · Author: ' + rb.author + '</p>'
          + '<div style="margin-top:var(--space-sm);">'
          + '<div style="padding:8px;background:var(--color-surface);border-radius:6px;margin-bottom:6px;font-size:13px;"><input type="checkbox"> <strong>Step 1:</strong> Identify affected resources</div>'
          + '<div style="padding:8px;background:var(--color-surface);border-radius:6px;margin-bottom:6px;font-size:13px;"><input type="checkbox"> <strong>Step 2:</strong> Collect logs and metrics</div>'
          + '<div style="padding:8px;background:var(--color-surface);border-radius:6px;margin-bottom:6px;font-size:13px;"><input type="checkbox"> <strong>Step 3:</strong> Apply remediation</div>'
          + '<div style="padding:8px;background:var(--color-surface);border-radius:6px;margin-bottom:6px;font-size:13px;"><input type="checkbox"> <strong>Step 4:</strong> Verify recovery</div>'
          + '<div style="padding:8px;background:var(--color-surface);border-radius:6px;font-size:13px;"><input type="checkbox"> <strong>Step 5:</strong> Document findings</div>'
          + '</div></div>'
      });
    },
    showCreateModal: function() {
      if (!global.Modal) return;
      global.Modal.open({
        title: '➕ Create Runbook',
        body: '<div style="padding:var(--space-xs);">'
          + '<div class="form-group"><label class="form-label">Title</label><input type="text" class="form-select" placeholder="e.g. Pod Recovery Procedure"></div>'
          + '<div class="form-group"><label class="form-label">Category</label><select class="form-select"><option>Application</option><option>Infrastructure</option><option>Database</option><option>Security</option><option>Network</option></select></div>'
          + '<div class="form-group"><label class="form-label">Content (Markdown)</label><textarea class="form-select" rows="6" placeholder="## Step 1\nDescribe the first step..."></textarea></div>'
          + '<button class="btn btn-primary btn-sm" onclick="alert(\'Runbook created\');Modal.close();">Create</button>'
          + '</div>'
      });
    }
  };
})(window);
