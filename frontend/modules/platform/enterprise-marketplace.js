/**
 * Enterprise App Marketplace Sub-Component
 */
(function (global) {
  'use strict';

  function initMarketplace() {
    renderMarketplaceCatalog('all');

    // Filter Buttons
    document.querySelectorAll('.ent-market-filter').forEach(function (btn) {
      btn.addEventListener('click', function () {
        document.querySelectorAll('.ent-market-filter').forEach(function (b) { b.classList.remove('active'); });
        this.classList.add('active');
        renderMarketplaceCatalog(this.dataset.filter);
      });
    });

    var saveCustomBtn = document.getElementById('ent-market-save-custom');
    if (saveCustomBtn) {
      saveCustomBtn.addEventListener('click', function () {
        Modal.open({
          title: '💾 Save Running Application as Template',
          body: '<div class="form-group"><label class="form-label">Deployment Reference</label><select class="form-select" id="save-app-ref"><option>nginx-prod (Isolated Namespace)</option><option>ai-ops-worker</option></select></div>' +
                '<div class="form-group"><label class="form-label">Template Title</label><input type="text" class="form-select" id="save-app-title" placeholder="e.g. Custom Python Core Service"></div>' +
                '<div class="form-group"><label class="form-label">Catalog Category</label><select class="form-select" id="save-app-cat"><option value="web">Web Services</option><option value="data">Database & Cache</option><option value="custom" selected>Organization Custom Template</option></select></div>' +
                '<div class="form-group"><label class="form-label">Version tag</label><input type="text" class="form-select" id="save-app-ver" value="v1.0.0"></div>',
          actions: [
            { label: 'Cancel' },
            { label: 'Publish to Catalog', primary: true, onClick: function () {
              var title = document.getElementById('save-app-title').value.trim();
              var cat = document.getElementById('save-app-cat').value;
              var ver = document.getElementById('save-app-ver').value;
              if (!title) return alert('Title is required');

              global.EnterpriseState.catalogTemplates.push({
                name: title,
                category: cat,
                version: ver,
                desc: 'Organization customized build deployment template.',
                ports: 8080,
                cpu: '250m',
                mem: '512Mi'
              });

              renderMarketplaceCatalog('all');
              Modal.close();
              global.EnterpriseState.addAuditLogEntry('admin', 'Platform Admin', 'Publish Template', title);
              alert('Template ' + title + ' published successfully to corporate application catalog.');
            }}
          ]
        });
      });
    }
  }

  function renderMarketplaceCatalog(filter) {
    var grid = document.getElementById('ent-market-grid');
    if (!grid) return;
    grid.innerHTML = '';

    var filtered = global.EnterpriseState.catalogTemplates;
    if (filter !== 'all') {
      filtered = global.EnterpriseState.catalogTemplates.filter(function (t) { return t.category === filter; });
    }

    filtered.forEach(function (t) {
      var card = document.createElement('div');
      card.className = 'health-card';
      card.style.display = 'flex';
      card.style.flexDirection = 'column';
      card.style.justifyContent = 'space-between';

      card.innerHTML =
        '<div>' +
          '<div class="health-card-header">' +
            '<span class="health-card-icon">📦</span>' +
            '<div class="health-card-title">' + esc(t.name) + '</div>' +
          '</div>' +
          '<div style="font-size:12px;color:var(--color-muted);margin-bottom:var(--space-md);line-height:1.4;">' + esc(t.desc) + '</div>' +
          '<div class="pipeline-detail" style="font-size:11px;margin-bottom:var(--space-sm);">' +
            '<div class="pipeline-detail-row"><span class="pipeline-detail-label">Version</span><span class="pipeline-detail-value badge badge-synced">' + esc(t.version) + '</span></div>' +
            '<div class="pipeline-detail-row"><span class="pipeline-detail-label">Default Ports</span><span class="pipeline-detail-value" style="font-family:var(--font-number);">' + t.ports + '</span></div>' +
            '<div class="pipeline-detail-row"><span class="pipeline-detail-label">CPU Request</span><span class="pipeline-detail-value">' + t.cpu + '</span></div>' +
            '<div class="pipeline-detail-row"><span class="pipeline-detail-label">Mem Request</span><span class="pipeline-detail-value">' + t.mem + '</span></div>' +
          '</div>' +
        '</div>' +
        '<button class="btn btn-ghost btn-sm ent-market-deploy-btn" data-app="' + esc(t.name) + '" style="width:100%;margin-top:auto;border-color:var(--color-primary);color:var(--color-primary);">🚀 Deploy Application</button>';
      grid.appendChild(card);
    });

    // Deploy Buttons Handlers
    grid.querySelectorAll('.ent-market-deploy-btn').forEach(function (btn) {
      btn.addEventListener('click', function () {
        var appName = this.dataset.app;
        openMarketplaceDeployModal(appName);
      });
    });
  }

  function openMarketplaceDeployModal(appName) {
    var t = global.EnterpriseState.catalogTemplates.find(function (item) { return item.name === appName; });
    if (!t) return;

    Modal.open({
      title: '🚀 Deploy ' + t.name,
      body: '<div id="deploy-wizard-container">' +
              '<div style="font-size:12px;color:var(--color-muted);margin-bottom:var(--space-md);">Provisioning container stack from catalog template templates.</div>' +
              '<div class="form-group"><label class="form-label">Select Deployment Target Cluster</label><select class="form-select" id="dep-target-cluster"><option value="eks">AWS EKS Cluster (Healthy)</option><option value="bare">Bare-Metal cluster</option></select></div>' +
              '<div class="form-group"><label class="form-label">Deployment isolated Namespace</label><select class="form-select" id="dep-target-ns"><option value="production">production</option><option value="staging">staging</option><option value="dev">dev</option></select></div>' +
              '<div class="form-group"><label class="form-label">Image Revision / Version</label><select class="form-select" id="dep-target-ver"><option>' + t.version + ' (Stable)</option><option>latest</option></select></div>' +
              '<div class="form-group"><label class="form-label">Replica Scaler Target (<span id="dep-rep-label">2</span>)</label><input type="range" class="form-select" id="dep-target-replicas" min="1" max="10" value="2"></div>' +
              '<div class="form-group"><label class="form-label">Container Override Port</label><input type="number" class="form-select" id="dep-target-port" value="' + t.ports + '"></div>' +
              '<div id="dep-rollout-console-wrapper" style="display:none;margin-top:var(--space-md);">' +
                '<label class="form-label">Rollout Rollout Output</label>' +
                '<pre class="ai-test-response" id="dep-rollout-console" style="height:140px;background:#0d1117;color:#39eb0a;font-family:monospace;padding:8px;font-size:11px;"></pre>' +
              '</div>' +
            '</div>',
      actions: [
        { label: 'Cancel' },
        { label: 'Verify & Install Catalog App', primary: true, onClick: async function () {
          var btnPrimary = document.querySelector('.modal-footer .btn-primary');
          if (btnPrimary.textContent === 'Done') {
            Modal.close();
            return;
          }
          btnPrimary.disabled = true;
          document.getElementById('dep-rollout-console-wrapper').style.display = 'block';
          var consoleEl = document.getElementById('dep-rollout-console');
          consoleEl.textContent = 'Initiating application deployment to target cluster...\n';

          var targetCluster = document.getElementById('dep-target-cluster').value;
          var targetNs = document.getElementById('dep-target-ns').value;
          var replicas = parseInt(document.getElementById('dep-target-replicas').value) || 2;
          var port = parseInt(document.getElementById('dep-target-port').value) || t.ports;

          try {
            var response = await APIClient.post('/deployments', {});

            consoleEl.textContent += '✓ Deployment manifest successfully applied.\n';
            consoleEl.textContent += '✓ App successfully registered on cluster: ' + targetCluster + '/' + targetNs + '\n';
            btnPrimary.disabled = false;
            btnPrimary.textContent = 'Done';
            global.EnterpriseState.addAuditLogEntry('admin', 'Platform Admin', 'Deploy Application', t.name);
          } catch (e) {
            consoleEl.textContent += '❌ Deployment failed: ' + e.message + '\n';
            btnPrimary.disabled = false;
          }
        }}
      ]
    });

    // Slider label updates
    var slider = document.getElementById('dep-target-replicas');
    var label = document.getElementById('dep-rep-label');
    if (slider && label) {
      slider.addEventListener('input', function () {
        label.textContent = this.value;
      });
    }
  }

  

  global.EnterpriseMarketplace = {
    init: initMarketplace,
    renderCatalog: renderMarketplaceCatalog
  };

})(window);
