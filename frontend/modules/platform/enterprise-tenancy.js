/**
 * Enterprise Multi-Tenancy Sub-Component
 */
(function (global) {
  'use strict';

  var initialized = false;
  function initTenancy() {
    if (initialized) return;
    initialized = true;
    var orgSelect = document.getElementById('ent-org-select');
    var projSelect = document.getElementById('ent-project-select');

    if (!orgSelect || !projSelect) return;

    // Populate Orgs Select
    orgSelect.innerHTML = '';
    global.EnterpriseState.organizations.forEach(function (org) {
      var opt = document.createElement('option');
      opt.value = org.id;
      opt.textContent = org.name;
      orgSelect.appendChild(opt);
    });

    orgSelect.value = global.EnterpriseState.activeOrgId;

    // Populate Projects Select
    updateProjectsDropdown();

    orgSelect.addEventListener('change', function () {
      global.EnterpriseState.activeOrgId = this.value;
      updateProjectsDropdown();
      renderTenancyTables();
      updateTenancyMetrics();
      if (global.EnterpriseRBAC && global.EnterpriseRBAC.renderMatrix) {
        global.EnterpriseRBAC.renderMatrix();
      }
    });

    projSelect.addEventListener('change', function () {
      global.EnterpriseState.activeProjId = this.value;
      renderTenancyTables();
      updateTenancyMetrics();
    });

    // Event Bindings
    var createOrgBtn = document.getElementById('ent-btn-create-org');
    if (createOrgBtn) createOrgBtn.addEventListener('click', openCreateOrgModal);

    var createProjBtn = document.getElementById('ent-btn-create-proj');
    if (createProjBtn) createProjBtn.addEventListener('click', openCreateProjectModal);

    renderTenancyTables();
    updateTenancyMetrics();
  }

  function updateProjectsDropdown() {
    var projSelect = document.getElementById('ent-project-select');
    if (!projSelect) return;
    projSelect.innerHTML = '';

    var filtered = global.EnterpriseState.projects.filter(function (p) { 
      return p.orgId === global.EnterpriseState.activeOrgId; 
    });
    filtered.forEach(function (p) {
      var opt = document.createElement('option');
      opt.value = p.id;
      opt.textContent = p.name;
      projSelect.appendChild(opt);
    });

    if (filtered.length > 0) {
      global.EnterpriseState.activeProjId = filtered[0].id;
      projSelect.value = global.EnterpriseState.activeProjId;
    } else {
      global.EnterpriseState.activeProjId = '';
    }
  }

  function updateTenancyMetrics() {
    var nsEl = document.getElementById('ent-metric-namespaces');
    var wlEl = document.getElementById('ent-metric-workloads');
    var usEl = document.getElementById('ent-metric-users');
    var tierEl = document.getElementById('ent-metric-tier');

    var currentOrg = global.EnterpriseState.organizations.find(function (org) { 
      return org.id === global.EnterpriseState.activeOrgId; 
    });
    var orgProjList = global.EnterpriseState.projects.filter(function (p) { 
      return p.orgId === global.EnterpriseState.activeOrgId; 
    });
    var orgWorkloadsCount = orgProjList.reduce(function (acc, val) { return acc + val.workloads; }, 0);
    var orgMembers = global.EnterpriseState.members.filter(function (m) { 
      return m.orgId === global.EnterpriseState.activeOrgId; 
    });

    if (nsEl) nsEl.textContent = orgProjList.length * 2 + 1; // Simulated
    if (wlEl) wlEl.textContent = orgWorkloadsCount;
    if (usEl) usEl.textContent = orgMembers.length;
    if (tierEl) tierEl.textContent = currentOrg ? currentOrg.tier : 'Standard';
  }

  function renderTenancyTables() {
    var projBody = document.getElementById('ent-projects-body');
    var memBody = document.getElementById('ent-members-body');

    if (projBody) {
      projBody.innerHTML = '';
      var orgProjList = global.EnterpriseState.projects.filter(function (p) { 
        return p.orgId === global.EnterpriseState.activeOrgId; 
      });
      if (orgProjList.length === 0) {
        projBody.innerHTML = '<tr><td colspan="3" style="text-align:center;color:var(--color-muted);">No projects created.</td></tr>';
      } else {
        orgProjList.forEach(function (p) {
          var tr = document.createElement('tr');
          var envBadges = p.envs.map(function (env) {
            var cls = env === 'prod' ? 'badge-down' : env === 'staging' ? 'badge-degraded' : 'badge-synced';
            return '<span class="badge ' + cls + '">' + env + '</span>';
          }).join(' ');

          tr.innerHTML =
            '<td><strong>' + esc(p.name) + '</strong></td>' +
            '<td>' + envBadges + '</td>' +
            '<td><strong style="font-family:var(--font-number);">' + p.workloads + ' pod replicas</strong></td>';
          projBody.appendChild(tr);
        });
      }
    }

    if (memBody) {
      memBody.innerHTML = '';
      var orgMembers = global.EnterpriseState.members.filter(function (m) { 
        return m.orgId === global.EnterpriseState.activeOrgId; 
      });
      if (orgMembers.length === 0) {
        memBody.innerHTML = '<tr><td colspan="3" style="text-align:center;color:var(--color-muted);">No users assigned.</td></tr>';
      } else {
        orgMembers.forEach(function (m) {
          var tr = document.createElement('tr');
          tr.innerHTML =
            '<td><strong>' + esc(m.user) + '</strong></td>' +
            '<td><span class="badge badge-healthy">' + esc(m.role) + '</span></td>' +
            '<td style="color:var(--color-muted);font-size:12px;">' + esc(m.scope) + '</td>';
          memBody.appendChild(tr);
        });
      }
    }
  }

  function openCreateOrgModal() {
    Modal.open({
      title: '🏢 Create SaaS Organization Space',
      body: '<div class="form-group"><label class="form-label">Organization Name</label><input type="text" class="form-select" id="new-org-name" placeholder="Acme Enterprise Ltd."></div>' +
            '<div class="form-group"><label class="form-label">Subscription Tier</label><select class="form-select" id="new-org-tier"><option value="Standard">Standard Tier</option><option value="Enterprise" selected>Enterprise Core Plan</option></select></div>' +
            '<div class="form-group"><label class="form-label">Default Region</label><select class="form-select"><option>us-east-1</option><option>ap-southeast-1</option></select></div>',
      actions: [
        { label: 'Cancel' },
        { label: 'Create Workspace', primary: true, onClick: async function () {
          var name = document.getElementById('new-org-name').value.trim();
          var tier = document.getElementById('new-org-tier').value;
          if (!name) return alert('Name is required');

          var newId = 'org-' + name.toLowerCase().replace(/[^a-z0-9]/g, '-');
          var payload = { id: newId, name: name, tier: tier };

          try {
            await APIClient.post('/organizations', payload);
            global.EnterpriseState.organizations.push(payload);
            global.EnterpriseState.activeOrgId = newId;
            initialized = false;
            initTenancy();
            Modal.close();
            global.EnterpriseState.addAuditLogEntry('admin', 'Platform Admin', 'Create Organization', name);
          } catch (e) {
            alert('Failed to create organization: ' + e.message);
          }
        }}
      ]
    });
  }

  function openCreateProjectModal() {
    Modal.open({
      title: '📁 Create Tenant Isolated Project',
      body: '<div class="form-group"><label class="form-label">Project Name</label><input type="text" class="form-select" id="new-proj-name" placeholder="API Service Pipeline"></div>' +
            '<div class="form-group"><label class="form-label">Environments to Provision</label>' +
              '<div style="display:flex;gap:12px;margin-top:4px;">' +
                '<label style="display:flex;align-items:center;gap:4px;"><input type="checkbox" id="env-dev" checked> dev</label>' +
                '<label style="display:flex;align-items:center;gap:4px;"><input type="checkbox" id="env-stag" checked> staging</label>' +
                '<label style="display:flex;align-items:center;gap:4px;"><input type="checkbox" id="env-prod"> production</label>' +
              '</div>' +
            '</div>' +
            '<div class="form-group"><label class="form-label">Estimated Workload Replicas</label><input type="number" class="form-select" id="new-proj-workloads" value="4"></div>',
      actions: [
        { label: 'Cancel' },
        { label: 'Provision project', primary: true, onClick: async function () {
          var name = document.getElementById('new-proj-name').value.trim();
          var wl = parseInt(document.getElementById('new-proj-workloads').value) || 2;
          if (!name) return alert('Name is required');

          var envs = [];
          if (document.getElementById('env-dev').checked) envs.push('dev');
          if (document.getElementById('env-stag').checked) envs.push('staging');
          if (document.getElementById('env-prod').checked) envs.push('prod');

          var newId = 'proj-' + name.toLowerCase().replace(/[^a-z0-9]/g, '-');
          var payload = { id: newId, orgId: global.EnterpriseState.activeOrgId, name: name, envs: envs, workloads: wl };

          try {
            await APIClient.post('/projects', payload);
            global.EnterpriseState.projects.push(payload);
            
            updateProjectsDropdown();
            var projSelect = document.getElementById('ent-project-select');
            if (projSelect) projSelect.value = newId;
            global.EnterpriseState.activeProjId = newId;

            renderTenancyTables();
            updateTenancyMetrics();
            Modal.close();
            global.EnterpriseState.addAuditLogEntry('admin', 'Platform Admin', 'Create Project', name);
          } catch (e) {
            alert('Failed to create project: ' + e.message);
          }
        }}
      ]
    });
  }

  

  global.EnterpriseTenancy = {
    init: initTenancy,
    renderTables: renderTenancyTables,
    updateMetrics: updateTenancyMetrics
  };

})(window);
