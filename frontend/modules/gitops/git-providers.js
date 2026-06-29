/**
 * Git Providers — Multi-provider management with health badges.
 */
(function (global) {
  'use strict';

  var tableBody = document.getElementById('git-table-body');

  function init() {
    AppState.on('gitProviders', render);
    AppState.on('navigate', function (s) { if (s === 'git-providers') render(AppState.getState().gitProviders); });

    var addBtn = document.getElementById('add-git-btn');
    if (addBtn) addBtn.addEventListener('click', showAddModal);
  }

  function render(providers) {
    if (!tableBody) return;
    providers = providers || [];
    tableBody.innerHTML = '';

    providers.forEach(function (p) {
      var tr = document.createElement('tr');
      var providerIcon = p.type === 'github' ? '🐙' : p.type === 'gitlab' ? '🦊' : '🍵';
      tr.innerHTML =
        '<td>' + providerIcon + ' <strong>' + esc(p.type) + '</strong></td>' +
        '<td>' + esc(p.organization) + '</td>' +
        '<td style="font-family:var(--font-number)">' + (p.repoCount || 0) + '</td>' +
        '<td>' + syncBadge(p.syncStatus) + '</td>' +
        '<td style="font-family:var(--font-number);font-size:12px;color:var(--color-muted)">' + timeAgo(p.lastWebhook) + '</td>' +
        '<td><div class="action-group">' +
          '<button class="action-btn" data-action="webhook">Test Webhook</button>' +
          '<button class="action-btn" data-action="sync">Sync</button>' +
          '<button class="action-btn" data-action="rotate">Rotate Token</button>' +
        '</div></td>';

      tr.querySelectorAll('.action-btn').forEach(function (btn) {
        btn.addEventListener('click', function () {
          AppState.addAuditLog({ action: this.dataset.action, target: 'git/' + p.type + '/' + p.organization, result: 'success' });
          alert(this.dataset.action + ' completed for ' + p.organization + ' ✅');
        });
      });

      tableBody.appendChild(tr);
    });
  }

  function showAddModal() {
    Modal.open({
      title: '+ Add Git Provider',
      body:
        '<div class="form-group"><label class="form-label">Provider</label><select class="form-select" id="new-git-type"><option>github</option><option>gitlab</option><option>gitea</option></select></div>' +
        '<div class="form-group"><label class="form-label">Organization</label><input class="form-select" placeholder="my-org"></div>' +
        '<div class="form-group"><label class="form-label">Access Token</label><input class="form-select" type="password" placeholder="ghp_xxxxx"></div>',
      actions: [{ label: 'Cancel' }, { label: 'Add Provider', primary: true, onClick: function () {
        AppState.addAuditLog({ action: 'create', target: 'git/new-provider', result: 'success' });
      }}]
    });
  }

  function syncBadge(status) {
    if (status === 'synced') return '<span class="badge badge-synced">✓ Synced</span>';
    if (status === 'syncing') return '<span class="badge badge-degraded">⟳ Syncing</span>';
    return '<span class="badge badge-pending">' + esc(status || 'pending') + '</span>';
  }

  

  global.GitProvidersSection = { init: init };
})(window);
