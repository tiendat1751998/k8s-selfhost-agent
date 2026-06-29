/**
 * Enterprise RBAC Matrix Sub-Component
 */
(function (global) {
  'use strict';

  function initRBAC() {
    renderRBACMatrix();
    populateRBACUserSelector();
    renderAuditLogs();

    var saveMatrixBtn = document.getElementById('ent-rbac-save-matrix');
    if (saveMatrixBtn) {
      saveMatrixBtn.addEventListener('click', function () {
        var roles = ['Platform Admin', 'Org Admin', 'DevOps Team', 'Developer', 'Viewer'];
        Object.keys(global.EnterpriseState.rbacMatrix).forEach(function (perm) {
          roles.forEach(function (role) {
            var chk = document.getElementById('rbac-chk-' + perm.replace(/ /g, '-') + '-' + role.replace(/ /g, '-'));
            if (chk) {
              global.EnterpriseState.rbacMatrix[perm][role] = chk.checked;
            }
          });
        });
        global.EnterpriseState.addAuditLogEntry('admin', 'Platform Admin', 'Save Access Matrix', 'All Role Mapping Policies');
        alert('RBAC Permissions Policy Matrix persisted successfully to security context.');
      });
    }

    var assignBtn = document.getElementById('ent-rbac-assign-btn');
    if (assignBtn) {
      assignBtn.addEventListener('click', function () {
        var user = document.getElementById('ent-rbac-user-select').value;
        var role = document.getElementById('ent-rbac-role-select').value;
        
        var match = global.EnterpriseState.members.find(function (m) { 
          return m.user === user && m.orgId === global.EnterpriseState.activeOrgId; 
        });
        if (match) {
          match.role = role;
        } else {
          // New member
          var org = global.EnterpriseState.organizations.find(function (o) { 
            return o.id === global.EnterpriseState.activeOrgId; 
          });
          global.EnterpriseState.members.push({ 
            user: user, 
            role: role, 
            scope: org ? org.name : 'Organization', 
            orgId: global.EnterpriseState.activeOrgId 
          });
        }

        if (global.EnterpriseTenancy && global.EnterpriseTenancy.renderTables) {
          global.EnterpriseTenancy.renderTables();
          global.EnterpriseTenancy.updateMetrics();
        }
        global.EnterpriseState.addAuditLogEntry('admin', 'Platform Admin', 'Assign Role', user + ' → ' + role);
        alert('User: ' + user + ' assigned to role: ' + role);
      });
    }

    var createRoleBtn = document.getElementById('ent-rbac-create-role-btn');
    if (createRoleBtn) {
      createRoleBtn.addEventListener('click', function () {
        var newRole = document.getElementById('ent-rbac-custom-role-name').value.trim();
        var inherit = document.getElementById('ent-rbac-custom-role-inherit').value;
        if (!newRole) return alert('Role name is required');

        // Check if role exists
        var testSelect = document.getElementById('ent-rbac-role-select');
        var exists = Array.from(testSelect.options).some(function (opt) { return opt.value === newRole; });
        if (exists) return alert('Role already exists');

        // Register custom role in local matrix state
        Object.keys(global.EnterpriseState.rbacMatrix).forEach(function (perm) {
          global.EnterpriseState.rbacMatrix[perm][newRole] = global.EnterpriseState.rbacMatrix[perm][inherit];
        });

        // Add options to select dropdowns
        var opt = document.createElement('option');
        opt.value = newRole;
        opt.textContent = newRole;
        testSelect.appendChild(opt);

        document.getElementById('ent-rbac-custom-role-name').value = '';
        renderRBACMatrix();
        global.EnterpriseState.addAuditLogEntry('admin', 'Platform Admin', 'Create Custom Role', newRole + ' (Inherits ' + inherit + ')');
        alert('Custom role ' + newRole + ' initialized in permissions framework.');
      });
    }
  }

  function renderRBACMatrix() {
    var matrixBody = document.getElementById('ent-rbac-matrix-body');
    if (!matrixBody) return;
    matrixBody.innerHTML = '';

    var perms = Object.keys(global.EnterpriseState.rbacMatrix);
    perms.forEach(function (perm) {
      var tr = document.createElement('tr');
      var tdPerm = document.createElement('td');
      tdPerm.style.textAlign = 'left';
      tdPerm.innerHTML = '<code style="color:var(--color-primary);">' + perm + '</code>';
      tr.appendChild(tdPerm);

      // Get roles dynamically from first permission mapping
      var roles = Object.keys(global.EnterpriseState.rbacMatrix[perm]);
      roles.forEach(function (role) {
        var td = document.createElement('td');
        var checked = global.EnterpriseState.rbacMatrix[perm][role] ? 'checked' : '';
        var inputId = 'rbac-chk-' + perm.replace(/ /g, '-') + '-' + role.replace(/ /g, '-');
        td.innerHTML = '<input type="checkbox" id="' + inputId + '" ' + checked + ' style="transform:scale(1.1);cursor:pointer;">';
        tr.appendChild(td);
      });

      matrixBody.appendChild(tr);
    });

    // Update table headers to include new custom roles
    var firstPerm = perms[0];
    if (firstPerm) {
      var roles = Object.keys(global.EnterpriseState.rbacMatrix[firstPerm]);
      var matrixTable = matrixBody.closest('table');
      if (matrixTable) {
        var thead = matrixTable.querySelector('thead tr');
        if (thead) {
          thead.innerHTML = '<th style="text-align:left;">Resource Scope</th>';
          roles.forEach(function (role) {
            var th = document.createElement('th');
            th.style.textAlign = 'center';
            th.textContent = role;
            thead.appendChild(th);
          });
        }
      }
    }
  }

  function populateRBACUserSelector() {
    var select = document.getElementById('ent-rbac-user-select');
    if (!select) return;
    select.innerHTML = '';

    // List of system users
    var users = ['admin', 'sre-team', 'dev-lead', 'viewer', 'acme-operator', 'acme-dev', 'developer-bot', 'security-auditor'];
    users.forEach(function (u) {
      var opt = document.createElement('option');
      opt.value = u;
      opt.textContent = u;
      select.appendChild(opt);
    });
  }

  function renderAuditLogs() {
    var auditBody = document.getElementById('ent-rbac-audit-body');
    if (!auditBody) return;
    auditBody.innerHTML = '';

    global.EnterpriseState.auditLogs.forEach(function (log) {
      var tr = document.createElement('tr');
      tr.innerHTML =
        '<td><strong>' + esc(log.user) + '</strong></td>' +
        '<td><span class="badge badge-synced">' + esc(log.role) + '</span></td>' +
        '<td><strong style="color:var(--color-primary);">' + esc(log.action) + '</strong></td>' +
        '<td><code>' + esc(log.target) + '</code></td>' +
        '<td style="color:var(--color-muted);font-size:12px;">' + esc(log.time) + '</td>';
      auditBody.appendChild(tr);
    });
  }

  

  global.EnterpriseRBAC = {
    init: initRBAC,
    renderMatrix: renderRBACMatrix,
    renderAuditLogs: renderAuditLogs
  };

})(window);
