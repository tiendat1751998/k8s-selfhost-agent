/**
 * Deployment Center — Catalog Component
 */
(function (global) {
  'use strict';

  var isDeployAdvActive = false;
  var currentSearchQuery = '';

  async function loadInitialApps() {
    try {
      var body = document.getElementById('deploy-catalog-body');
      if (body && (!global.DeploymentState.apps || global.DeploymentState.apps.length === 0)) {
        body.innerHTML = '<tr><td colspan="7"><div class="skeleton" style="height:120px;border-radius:var(--rounded-lg);"></div></td></tr>';
      }
      var res = await fetch('/api/v1/deployments');
      if (!res.ok) throw new Error('API request failed');
      var json = await res.json();
      global.DeploymentState.apps = json.data || [];
      renderCatalog();
    } catch (e) {
      console.error('Failed to load apps:', e);
      global.DeploymentState.apps = [];
      renderCatalog();
    }
  }

  function renderCatalog() {
    var body = document.getElementById('deploy-catalog-body');
    var empty = document.getElementById('deploy-catalog-empty');
    if (!body) return;
    body.innerHTML = '';

    var filteredApps = global.DeploymentState.apps;
    if (isDeployAdvActive) {
      var compiled = compileDeployRules(document.getElementById('deploy-adv-rules'));
      var matchType = document.getElementById('deploy-adv-match-type').value;
      filteredApps = global.DeploymentState.apps.filter(function(app) {
        return evaluateDeployRules(app, compiled, matchType);
      });
    } else if (currentSearchQuery) {
      filteredApps = global.DeploymentState.apps.filter(function(app) {
        return app.name.toLowerCase().indexOf(currentSearchQuery) >= 0 ||
               app.team.toLowerCase().indexOf(currentSearchQuery) >= 0 ||
               app.env.toLowerCase().indexOf(currentSearchQuery) >= 0 ||
               app.image.toLowerCase().indexOf(currentSearchQuery) >= 0 ||
               app.target.toLowerCase().indexOf(currentSearchQuery) >= 0;
      });
    }

    if (filteredApps.length === 0) {
      if (empty) empty.style.display = '';
      return;
    }
    if (empty) empty.style.display = 'none';

    filteredApps.forEach(function (app) {
      var originalIdx = global.DeploymentState.apps.indexOf(app);
      var tr = document.createElement('tr');
      var targetLabel = (app.type === 'kubernetes' ? '☸️ ' : '🦊 ') + app.target + (app.namespace ? '/' + app.namespace : '');
      tr.innerHTML =
        '<td><strong>' + esc(app.name) + '</strong></td>' +
        '<td><span class="badge badge-synced">' + esc(app.team) + '</span></td>' +
        '<td><span style="font-size:12px;font-weight:600;text-transform:capitalize;">' + esc(app.env) + '</span></td>' +
        '<td><code style="font-size:12px">' + esc(app.image) + '</code></td>' +
        '<td><span style="font-size:12px;color:var(--color-muted)">' + esc(targetLabel) + '</span></td>' +
        '<td>' + statusBadge(app.status) + '</td>' +
        '<td><div class="action-group">' +
          '<button class="action-btn" data-action="view" data-index="' + originalIdx + '">Details</button>' +
          '<button class="action-btn" data-action="scale" data-index="' + originalIdx + '">Scale</button>' +
          '<button class="action-btn" data-action="restart" data-index="' + originalIdx + '">Restart</button>' +
          '<button class="action-btn danger" data-action="delete" data-index="' + originalIdx + '">Delete</button>' +
        '</div></td>';

      tr.querySelectorAll('.action-btn').forEach(function (btn) {
        btn.addEventListener('click', function (e) {
          e.stopPropagation();
          var action = this.dataset.action;
          var index = parseInt(this.dataset.index);
          handleCatalogAction(action, index);
        });
      });

      body.appendChild(tr);
    });
  }

  function handleCatalogAction(action, idx) {
    var app = global.DeploymentState.apps[idx];
    if (!app) return;

    if (action === 'view') {
      if (global.DeploymentDrawer && global.DeploymentDrawer.open) {
        global.DeploymentDrawer.open(app, idx);
      }
      return;
    }

    if (action === 'scale') {
      Modal.open({
        title: '📐 Scale Application: ' + app.name,
        body: '<div class="form-group"><label class="form-label">Desired Replicas</label><input type="number" class="form-select" id="cat-scale-replicas" value="' + app.replicas + '" min="0" max="100"></div>',
        actions: [
          { label: 'Cancel' },
          { label: 'Scale', primary: true, onClick: async function () {
            var val = parseInt(document.getElementById('cat-scale-replicas').value);
            try {
              var response = await fetch('/api/v1/deployments/scale', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                  type: app.type,
                  cluster: app.target,
                  namespace: app.namespace || '',
                  name: app.name,
                  replicas: val
                })
              });
              if (!response.ok) throw new Error('API request failed');
              app.replicas = val;
              app.status = val === 0 ? 'down' : 'healthy';
              AppState.addAuditLog({ action: 'scale', target: 'app/' + app.name, result: 'replicas=' + val });
              renderCatalog();
              alert('Scale request sent for ' + app.name + ' ✅');
            } catch (e) {
              alert('Scale request failed: ' + e.message);
            }
          }}
        ]
      });
      return;
    }

    if (action === 'restart') {
      Modal.open({
        title: '🔄 Restart Application: ' + app.name,
        body: '<p style="color:var(--color-text-secondary)">Are you sure you want to trigger a rolling restart of <strong>' + esc(app.name) + '</strong>?</p>',
        actions: [
          { label: 'Cancel' },
          { label: 'Restart', primary: true, onClick: async function () {
            try {
              app.status = 'degraded';
              renderCatalog();
              var response = await fetch('/api/v1/deployments/restart', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                  type: app.type,
                  cluster: app.target,
                  namespace: app.namespace || '',
                  name: app.name
                })
              });
              if (!response.ok) throw new Error('API request failed');
              AppState.addAuditLog({ action: 'restart', target: 'app/' + app.name, result: 'triggered' });
              setTimeout(async function () {
                await loadInitialApps();
              }, 3000);
            } catch (e) {
              alert('Restart request failed: ' + e.message);
              loadInitialApps();
            }
          }}
        ]
      });
      return;
    }

    if (action === 'delete') {
      Modal.open({
        title: '⚠️ Delete Application: ' + app.name,
        body: '<p style="color:var(--color-text-secondary)">Are you sure you want to completely delete the deployment <strong>' + esc(app.name) + '</strong>?</p>' +
              '<p style="color:var(--color-trading-down);font-size:13px;">This will remove all replica containers and network endpoints.</p>',
        actions: [
          { label: 'Cancel' },
          { label: 'Delete', primary: true, onClick: async function () {
            try {
              var response = await fetch('/api/v1/deployments/delete', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                  type: app.type,
                  cluster: app.target,
                  namespace: app.namespace || '',
                  name: app.name
                })
              });
              if (!response.ok) throw new Error('API request failed');
              global.DeploymentState.apps.splice(idx, 1);
              AppState.addAuditLog({ action: 'delete', target: 'app/' + app.name, result: 'success' });
              renderCatalog();
            } catch (e) {
              alert('Delete request failed: ' + e.message);
            }
          }}
        ]
      });
      return;
    }
  }

  function statusBadge(status) {
    var cls = 'badge-healthy';
    if (status === 'degraded') cls = 'badge-degraded';
    if (status === 'down') cls = 'badge-down';
    return '<span class="badge ' + cls + '">' + esc(status) + '</span>';
  }

  function init() {
    var searchInput = document.getElementById('deploy-catalog-search');
    if (searchInput) {
      searchInput.addEventListener('input', function() {
        currentSearchQuery = this.value.trim().toLowerCase();
        renderCatalog();
      });
    }

    var toggleBtn = document.getElementById('toggle-deploy-adv-builder');
    if (toggleBtn) {
      toggleBtn.addEventListener('click', function() {
        isDeployAdvActive = !isDeployAdvActive;
        var advContainer = document.getElementById('deploy-adv-builder-container');
        if (isDeployAdvActive) {
          if (advContainer) advContainer.style.display = 'block';
          this.textContent = '⚙️ Switch to Simple Filter';
          var container = document.getElementById('deploy-adv-rules');
          if (container && container.children.length === 0) {
            container.appendChild(createDeployRuleElement());
          }
        } else {
          if (advContainer) advContainer.style.display = 'none';
          this.textContent = '⚙️ Advanced Filter';
        }
        renderCatalog();
      });
    }

    var addRuleBtn = document.getElementById('deploy-adv-add-rule-btn');
    if (addRuleBtn) {
      addRuleBtn.addEventListener('click', function() {
        var container = document.getElementById('deploy-adv-rules');
        if (container) container.appendChild(createDeployRuleElement());
      });
    }

    var addGroupBtn = document.getElementById('deploy-adv-add-group-btn');
    if (addGroupBtn) {
      addGroupBtn.addEventListener('click', function() {
        var container = document.getElementById('deploy-adv-rules');
        if (container) container.appendChild(createDeployGroupElement());
      });
    }

    var applyBtn = document.getElementById('deploy-adv-apply-btn');
    if (applyBtn) {
      applyBtn.addEventListener('click', function() {
        renderCatalog();
      });
    }

    var resetBtn = document.getElementById('deploy-adv-reset-btn');
    if (resetBtn) {
      resetBtn.addEventListener('click', function() {
        var container = document.getElementById('deploy-adv-rules');
        if (container) {
          container.innerHTML = '';
          container.appendChild(createDeployRuleElement());
        }
        renderCatalog();
      });
    }

    var saveBtn = document.getElementById('deploy-catalog-save-btn');
    if (saveBtn) {
      saveBtn.addEventListener('click', function() {
        var namePrompt = isDeployAdvActive ? 'Advanced Deployments Search' : (currentSearchQuery || 'Deployments Search');
        var name = prompt('Enter a name for this saved deployments search:', namePrompt);
        if (!name) return;

        var config = {};
        if (isDeployAdvActive) {
          config = {
            isDeployAdvActive: true,
            matchType: document.getElementById('deploy-adv-match-type').value,
            rules: compileDeployRules(document.getElementById('deploy-adv-rules'))
          };
        } else {
          config = {
            isDeployAdvActive: false,
            query: currentSearchQuery
          };
        }

        if (global.EnterpriseSearchSection && global.EnterpriseSearchSection.saveSearch) {
          global.EnterpriseSearchSection.saveSearch(name, 'deploy', config);
        }
      });
    }
  }

  function loadSavedSearch(item) {
    if (item.type !== 'deploy') return;
    isDeployAdvActive = !!item.config.isDeployAdvActive;
    
    var advContainer = document.getElementById('deploy-adv-builder-container');
    var toggleBtn = document.getElementById('toggle-deploy-adv-builder');
    var searchInput = document.getElementById('deploy-catalog-search');
    
    if (isDeployAdvActive) {
      if (advContainer) advContainer.style.display = 'block';
      if (toggleBtn) toggleBtn.textContent = '⚙️ Switch to Simple Filter';
      document.getElementById('deploy-adv-match-type').value = item.config.matchType;
      loadDeployRulesIntoDOM(item.config.rules, document.getElementById('deploy-adv-rules'));
      if (searchInput) searchInput.value = '';
      currentSearchQuery = '';
    } else {
      if (advContainer) advContainer.style.display = 'none';
      if (toggleBtn) toggleBtn.textContent = '⚙️ Advanced Filter';
      if (searchInput) searchInput.value = item.config.query || '';
      currentSearchQuery = (item.config.query || '').toLowerCase();
    }
    renderCatalog();
  }

  // ── DEPLOY ADVANCED FILTER HELPERS ──
  function createDeployRuleElement() {
    var div = document.createElement('div');
    div.className = 'adv-rule-row';
    div.style.cssText = 'display:flex; gap:var(--space-xs); align-items:center; margin-bottom:4px;';
    
    var fieldSelect = document.createElement('select');
    fieldSelect.className = 'form-select adv-rule-field';
    fieldSelect.style.cssText = 'width:150px; padding:4px 8px;';
    fieldSelect.innerHTML = 
      '<option value="name">Name</option>' +
      '<option value="team">Team</option>' +
      '<option value="env">Env</option>' +
      '<option value="image">Image</option>' +
      '<option value="target">Target</option>' +
      '<option value="status">Status</option>' +
      '<option value="replicas">Replicas</option>';
      
    var opSelect = document.createElement('select');
    opSelect.className = 'form-select adv-rule-op';
    opSelect.style.cssText = 'width:120px; padding:4px 8px;';
    opSelect.innerHTML = 
      '<option value="contains">Contains</option>' +
      '<option value="not_contains">Does Not Contain</option>' +
      '<option value="equals">Equals</option>' +
      '<option value="not_equals">Does Not Equal</option>' +
      '<option value="gt">Greater Than</option>' +
      '<option value="lt">Less Than</option>';
      
    var valInput = document.createElement('input');
    valInput.type = 'text';
    valInput.className = 'form-select adv-rule-val';
    valInput.placeholder = 'Value...';
    valInput.style.cssText = 'flex:1; padding:4px 8px;';
    
    var removeBtn = document.createElement('button');
    removeBtn.className = 'btn btn-ghost btn-xs';
    removeBtn.innerHTML = '❌';
    removeBtn.style.padding = '4px 8px';
    removeBtn.addEventListener('click', function() {
      div.remove();
    });
    
    div.appendChild(fieldSelect);
    div.appendChild(opSelect);
    div.appendChild(valInput);
    div.appendChild(removeBtn);
    return div;
  }

  function createDeployGroupElement() {
    var div = document.createElement('div');
    div.className = 'adv-group-box';
    div.style.cssText = 'border:1px dashed var(--color-hairline); border-radius:var(--rounded-md); padding:var(--space-sm); margin-bottom:8px; background:rgba(255,255,255,0.01);';
    
    var header = document.createElement('div');
    header.style.cssText = 'display:flex; align-items:center; gap:var(--space-sm); margin-bottom:8px;';
    header.innerHTML = 
      '<span style="font-size:11px; font-weight:600;">Match</span>' +
      '<select class="form-select adv-group-match" style="width:80px; padding:2px 4px; font-size:11px;">' +
        '<option value="AND">ALL (AND)</option>' +
        '<option value="OR">ANY (OR)</option>' +
      '</select>' +
      '<span style="font-size:11px; font-weight:600;">of sub-conditions:</span>';
      
    var removeBtn = document.createElement('button');
    removeBtn.className = 'btn btn-ghost btn-xs';
    removeBtn.innerHTML = 'Remove Group ❌';
    removeBtn.style.cssText = 'margin-left:auto; padding:2px 6px; font-size:11px;';
    removeBtn.addEventListener('click', function() {
      div.remove();
    });
    header.appendChild(removeBtn);
    
    var rulesContainer = document.createElement('div');
    rulesContainer.className = 'adv-group-rules-container';
    rulesContainer.style.cssText = 'padding-left:12px; display:flex; flex-direction:column; gap:4px;';
    
    var actions = document.createElement('div');
    actions.style.cssText = 'margin-top:8px; display:flex; gap:var(--space-xs);';
    
    var addRuleBtn = document.createElement('button');
    addRuleBtn.className = 'btn btn-ghost btn-xs';
    addRuleBtn.textContent = '+ Add Sub-Rule';
    addRuleBtn.style.padding = '2px 6px';
    addRuleBtn.addEventListener('click', function() {
      rulesContainer.appendChild(createDeployRuleElement());
    });
    
    actions.appendChild(addRuleBtn);
    
    div.appendChild(header);
    div.appendChild(rulesContainer);
    div.appendChild(actions);
    
    rulesContainer.appendChild(createDeployRuleElement());
    return div;
  }

  function compileDeployRules(container) {
    if (!container) return [];
    var rules = [];
    container.childNodes.forEach(function(node) {
      if (node.classList && node.classList.contains('adv-rule-row')) {
        var field = node.querySelector('.adv-rule-field').value;
        var op = node.querySelector('.adv-rule-op').value;
        var val = node.querySelector('.adv-rule-val').value.trim();
        rules.push({
          type: 'rule',
          field: field,
          op: op,
          val: val
        });
      } else if (node.classList && node.classList.contains('adv-group-box')) {
        var match = node.querySelector('.adv-group-match').value;
        var subContainer = node.querySelector('.adv-group-rules-container');
        var subRules = compileDeployRules(subContainer);
        rules.push({
          type: 'group',
          match: match,
          rules: subRules
        });
      }
    });
    return rules;
  }

  function evaluateDeployRules(app, compiledRules, matchType) {
    if (compiledRules.length === 0) return true;
    var isAnd = matchType === 'AND';
    var results = [];
    
    compiledRules.forEach(function(cond) {
      if (cond.type === 'rule') {
        var fieldVal = app[cond.field] || '';
        var matches = false;
        
        if (cond.field === 'replicas') {
          var fNum = parseInt(fieldVal) || 0;
          var qNum = parseInt(cond.val) || 0;
          switch (cond.op) {
            case 'equals': matches = fNum === qNum; break;
            case 'not_equals': matches = fNum !== qNum; break;
            case 'gt': matches = fNum > qNum; break;
            case 'lt': matches = fNum < qNum; break;
            default: matches = false;
          }
        } else {
          var fVal = String(fieldVal).toLowerCase();
          var qVal = String(cond.val).toLowerCase();
          switch (cond.op) {
            case 'contains': matches = fVal.indexOf(qVal) >= 0; break;
            case 'not_contains': matches = fVal.indexOf(qVal) < 0; break;
            case 'equals': matches = fVal === qVal; break;
            case 'not_equals': matches = fVal !== qVal; break;
            default: matches = false;
          }
        }
        results.push(matches);
      } else if (cond.type === 'group') {
        var groupMatches = evaluateDeployRules(app, cond.rules, cond.match);
        results.push(groupMatches);
      }
    });
    
    if (isAnd) {
      return results.every(function(r) { return r; });
    } else {
      return results.some(function(r) { return r; });
    }
  }

  function loadDeployRulesIntoDOM(rules, container) {
    if (!container || !rules) return;
    container.innerHTML = '';
    rules.forEach(function(cond) {
      if (cond.type === 'rule') {
        var el = createDeployRuleElement();
        el.querySelector('.adv-rule-field').value = cond.field;
        el.querySelector('.adv-rule-op').value = cond.op;
        el.querySelector('.adv-rule-val').value = cond.val;
        container.appendChild(el);
      } else if (cond.type === 'group') {
        var el = createDeployGroupElement();
        el.querySelector('.adv-group-match').value = cond.match;
        var subContainer = el.querySelector('.adv-group-rules-container');
        loadDeployRulesIntoDOM(cond.rules, subContainer);
        container.appendChild(el);
      }
    });
  }

  global.DeploymentCatalog = { 
    loadInitialApps, 
    renderCatalog, 
    handleCatalogAction,
    init,
    loadSavedSearch
  };
})(window);
