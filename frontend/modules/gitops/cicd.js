/**
 * CI/CD Integrations — Pipeline registry, webhook monitoring, trigger history.
 */
(function (global) {
  'use strict';

  var tableBody = document.getElementById('cicd-table-body');

  function init() {
    AppState.on('cicd', render);
    AppState.on('navigate', function (s) { if (s === 'cicd') render(AppState.getState().cicd); });

    var addBtn = document.getElementById('add-cicd-btn');
    if (addBtn) addBtn.addEventListener('click', showAddModal);
  }

  function render(pipelines) {
    if (!tableBody) return;
    pipelines = pipelines || [];
    tableBody.innerHTML = '';

    pipelines.forEach(function (p) {
      var tr = document.createElement('tr');
      tr.innerHTML =
        '<td><strong>' + esc(p.name) + '</strong></td>' +
        '<td>' + esc(p.provider) + '</td>' +
        '<td>' + statusBadge(p.status) + '</td>' +
        '<td style="font-family:var(--font-number)">' + (p.successRate || 0) + '%</td>' +
        '<td style="font-family:var(--font-number);font-size:12px;color:var(--color-muted)">' + timeAgo(p.lastRun) + '</td>' +
        '<td><div class="action-group">' +
          '<button class="action-btn" data-action="detail">Logs</button>' +
          '<button class="action-btn" data-action="trigger">Trigger</button>' +
        '</div></td>';

      tr.querySelectorAll('.action-btn').forEach(function (btn) {
        btn.addEventListener('click', function () {
          if (this.dataset.action === 'detail') {
            Modal.open({
              title: '📋 Pipeline: ' + p.name,
              body: '<pre class="ai-test-response">' + esc(p.lastLog || 'No logs available') + '</pre>',
              actions: [{ label: 'Close', primary: true }]
            });
          } else {
            AppState.addAuditLog({ action: 'trigger', target: 'cicd/' + p.name, result: 'success' });
            alert('Pipeline "' + p.name + '" triggered ✅');
          }
        });
      });

      tableBody.appendChild(tr);
    });
  }

  function showAddModal() {
    Modal.open({
      title: '+ Add CI/CD Integration',
      body:
        '<div class="form-group"><label class="form-label">Pipeline Name</label><input class="form-select" placeholder="deploy-prod"></div>' +
        '<div class="form-group"><label class="form-label">Provider</label><select class="form-select"><option>GitHub Actions</option><option>GitLab CI</option><option>Jenkins</option><option>ArgoCD</option></select></div>' +
        '<div class="form-group"><label class="form-label">Webhook URL</label><input class="form-select" placeholder="https://…"></div>',
      actions: [{ label: 'Cancel' }, { label: 'Add', primary: true }]
    });
  }

  function statusBadge(s) {
    if (s === 'active') return '<span class="badge badge-healthy">Active</span>';
    if (s === 'failing') return '<span class="badge badge-down">Failing</span>';
    return '<span class="badge badge-pending">' + esc(s || 'inactive') + '</span>';
  }
  

  global.CicdSection = { init: init };
})(window);
