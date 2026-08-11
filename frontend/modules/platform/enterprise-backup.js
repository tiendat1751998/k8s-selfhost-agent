/**
 * Enterprise Backup & DR Sub-Component
 */
(function (global) {
  'use strict';

  function initBackup() {
    renderBackupPolicies();
    loadBackupHistory();
    renderBackupBackends();

    var createPolicyBtn = document.getElementById('ent-backup-create-policy');
    if (createPolicyBtn) {
      createPolicyBtn.addEventListener('click', function () {
        var name = document.getElementById('ent-backup-policy-name').value.trim();
        var cron = document.getElementById('ent-backup-policy-cron').value.trim();
        var target = document.getElementById('ent-backup-policy-target').value;
        var backend = document.getElementById('ent-backup-policy-backend').value;

        if (!name || !cron) return alert('Name and Cron expression are required');

        global.EnterpriseState.backupPolicies.push({ name: name, target: target, cron: cron, backend: backend });
        renderBackupPolicies();
        document.getElementById('ent-backup-policy-name').value = '';
        document.getElementById('ent-backup-policy-cron').value = '';
        global.EnterpriseState.addAuditLogEntry('admin', 'Platform Admin', 'Create Backup Policy', name);
        alert('Backup schedule Policy ' + name + ' created.');
      });
    }

    var backupNowBtn = document.getElementById('ent-backup-now-btn');
    if (backupNowBtn) {
      backupNowBtn.addEventListener('click', function () {
        backupNowBtn.disabled = true;
        backupNowBtn.textContent = '⏳ Backing up...';

        APIClient.post('/backup/recover', { target: 'manual-backup-' + new Date().toISOString().slice(0, 10) })
        .then(function () {
          alert('Backup task created successfully.');
          loadBackupHistory();
          backupNowBtn.disabled = false;
          backupNowBtn.textContent = '⚡ Back Up Now';
        }).catch(function (e) {
          console.error(e);
          alert('Failed to trigger manual backup.');
          backupNowBtn.disabled = false;
          backupNowBtn.textContent = '⚡ Back Up Now';
        });
      });
    }

    var simulateDRBtn = document.getElementById('ent-dr-simulate-btn');
    if (simulateDRBtn) {
      simulateDRBtn.addEventListener('click', function () {
        var snapId = document.getElementById('ent-dr-snap-select').value;
        if (!snapId || snapId === 'No snapshots available') return alert('No backup snapshot selected');

        simulateDRBtn.disabled = true;
        var consoleEl = document.getElementById('ent-dr-log-console');
        if (consoleEl) {
          consoleEl.textContent = '[DR Engine] Locating backup configurations...\n';
        }

        APIClient.post('/backup/recover', { target: snapId })
        .catch(function (e) {
          console.error(e);
          alert('Failed to trigger disaster recovery simulation.');
          simulateDRBtn.disabled = false;
        });
      });
    }

    window.addEventListener('ws-message', function (e) {
      var msg = e.detail;
      if (!msg) return;

      if (msg.type === 'log') {
        var text = msg.data || '';
        if (text.indexOf('backup:') === 0) {
          var consoleEl = document.getElementById('ent-dr-log-console');
          if (consoleEl) {
            consoleEl.textContent += '[DR Engine] ' + text.substring(7).trim() + '\n';
            consoleEl.scrollTop = consoleEl.scrollHeight;
          }
        }
      } else if (msg.type === 'backup_status') {
        var simulateDRBtn = document.getElementById('ent-dr-simulate-btn');
        if (simulateDRBtn) {
          simulateDRBtn.disabled = false;
        }
        alert('Disaster recovery simulation completed successfully.');
        loadBackupHistory();
      }
    });
  }

  function loadBackupHistory() {
    APIClient.get('/backup/history')
      .then(function (json) {
        var data = json.data || [];
        renderBackupSnapshots(data);
        populateDRSnapshotSelector(data);
      })
      .catch(function (e) {
        console.error('Failed to load backup history:', e);
      });
  }

  function renderBackupPolicies() {
    var tbody = document.getElementById('ent-backup-policies-body');
    if (!tbody) return;
    tbody.innerHTML = '';

    global.EnterpriseState.backupPolicies.forEach(function (p) {
      var tr = document.createElement('tr');
      tr.innerHTML =
        '<td><strong>' + esc(p.name) + '</strong></td>' +
        '<td><span class="badge badge-synced">' + esc(p.target) + '</span></td>' +
        '<td><code>' + esc(p.cron) + '</code></td>' +
        '<td><span class="badge badge-healthy">' + esc(p.backend) + '</span></td>';
      tbody.appendChild(tr);
    });
  }

  function renderBackupSnapshots(data) {
    var tbody = document.getElementById('ent-backup-snapshots-body');
    if (!tbody) return;
    tbody.innerHTML = '';

    if (!data || data.length === 0) {
      tbody.innerHTML = '<tr><td colspan="5" style="text-align:center;color:var(--color-muted);">No snapshots available</td></tr>';
      return;
    }

    data.forEach(function (s) {
      var tr = document.createElement('tr');
      var badgeCls = s.status === 'success' ? 'badge-healthy' : 'badge-pending';
      var restoreBtn = s.status === 'success' ? '<button class="action-btn ent-snap-restore-btn" data-snap="' + esc(s.id) + '" data-target="' + esc(s.target) + '">Restore</button>' : '—';
      
      tr.innerHTML =
        '<td><code>' + esc(s.id.substring(0, 8)) + '</code></td>' +
        '<td>' + esc(s.target) + '</td>' +
        '<td style="font-family:var(--font-number);">' + esc(s.size) + '</td>' +
        '<td><span class="badge ' + badgeCls + '">' + esc(s.status) + '</span></td>' +
        '<td><div class="action-group">' + restoreBtn + '</div></td>';
      tbody.appendChild(tr);
    });

    tbody.querySelectorAll('.ent-snap-restore-btn').forEach(function (btn) {
      btn.addEventListener('click', function () {
        var snapId = this.dataset.snap;
        var target = this.dataset.target;
        if (confirm('Confirm restoration of namespace from backup ' + snapId + '?')) {
          APIClient.post('/backup/recover', { target: target })
          .then(function () {
            alert('Snapshot ' + snapId + ' restore triggered. Progress logs are written to system output.');
            loadBackupHistory();
          })
          .catch(function (e) {
            console.error(e);
            alert('Failed to trigger restore.');
          });
        }
      });
    });
  }

  function renderBackupBackends() {
    var tbody = document.getElementById('ent-backup-backends-body');
    if (!tbody) return;
    tbody.innerHTML = '';

    global.EnterpriseState.storageBackends.forEach(function (b) {
      var tr = document.createElement('tr');
      tr.innerHTML =
        '<td><strong>' + esc(b.name) + '</strong></td>' +
        '<td><span class="badge badge-synced">' + esc(b.type) + '</span></td>' +
        '<td><code>' + esc(b.endpoint) + '</code></td>' +
        '<td><span class="badge badge-healthy">● Online</span></td>';
      tbody.appendChild(tr);
    });
  }

  function populateDRSnapshotSelector(data) {
    var select = document.getElementById('ent-dr-snap-select');
    if (!select) return;
    select.innerHTML = '';

    if (!data || data.length === 0) {
      var opt = document.createElement('option');
      opt.textContent = 'No snapshots available';
      select.appendChild(opt);
      return;
    }

    data.filter(function (s) { return s.status === 'success'; }).forEach(function (s) {
      var opt = document.createElement('option');
      opt.value = s.target;
      opt.textContent = s.target + ' (' + s.size + ' - ' + s.id.substring(0, 8) + ')';
      select.appendChild(opt);
    });
  }

  global.EnterpriseBackup = {
    init: initBackup,
    renderPolicies: renderBackupPolicies,
    renderSnapshots: renderBackupSnapshots,
    renderBackends: renderBackupBackends
  };

})(window);
