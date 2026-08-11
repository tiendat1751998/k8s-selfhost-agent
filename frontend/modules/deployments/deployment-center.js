/**
 * Deployment Center Orchestrator — Catalog, multi-step deployment wizard, AI assistant, and details drawer.
 */
(function (global) {
  'use strict';

  // ── CENTRALIZED STATE ──
  global.DeploymentState = {
    currentTab: 'catalog',
    wizardStep: 1,
    maxSteps: 8,
    apps: [],
    drawerEl: null,
    drawerOverlay: null,
    currentApp: null,
    wizManifest: ''
  };

  function init() {
    if (global.DeploymentDrawer) global.DeploymentDrawer.create();

    // Tab switcher
    document.querySelectorAll('.deploy-tab').forEach(function (tab) {
      tab.addEventListener('click', function () {
        switchTab(this.dataset.tab);
      });
    });

    // Populate cluster dropdown
    AppState.on('kubernetes', populateClusters);
    AppState.on('navigate', async function (s) {
      if (s === 'deployment-center') {
        try {
          var data = await APIClient.get('/fleet');
            if (data && data.data) AppState.setKubernetes(data.data);
        } catch (e) {
          console.error('Failed to load fleet clusters:', e);
        }
        populateClusters(AppState.getState().kubernetes);
        if (global.DeploymentCatalog) global.DeploymentCatalog.loadInitialApps();
      }
    });

    // Target Platform type changes
    var targetType = document.getElementById('wiz-target-type');
    if (targetType) {
      targetType.addEventListener('change', function () {
        var isK8s = this.value === 'kubernetes';
        document.getElementById('wiz-ns-group').style.display = isK8s ? '' : 'none';
        document.getElementById('wiz-node-group').style.display = isK8s ? 'none' : '';
        // Toggle storage options
        var volType = document.getElementById('wiz-volume-type');
        if (volType) {
          volType.innerHTML = isK8s ? 
            '<option value="none">No Persistent Storage</option><option value="pvc">Persistent Volume Claim (PVC)</option><option value="hostpath">Host Path Mount</option>' :
            '<option value="none">No Persistent Storage</option><option value="hostpath">Host Path Mount</option><option value="named">Named Swarm Volume</option>';
        }
        populateClusters(AppState.getState().kubernetes);
      });
    }

    // Network Service Type changes
    var netType = document.getElementById('wiz-network-type');
    if (netType) {
      netType.addEventListener('change', function () {
        var isIngress = this.value === 'Ingress';
        document.getElementById('wiz-domain-group').style.display = isIngress ? '' : 'none';
      });
    }

    // Volume Type changes
    var volType = document.getElementById('wiz-volume-type');
    if (volType) {
      volType.addEventListener('change', function () {
        var hasVol = this.value !== 'none';
        document.getElementById('wiz-vol-details').style.display = hasVol ? '' : 'none';
        var scGroup = document.getElementById('wiz-sc-group');
        if (scGroup) {
          scGroup.style.display = this.value === 'pvc' ? '' : 'none';
        }
      });
    }

    // Environment variables UI
    var envAdd = document.getElementById('wiz-env-add');
    if (envAdd) {
      envAdd.addEventListener('click', function () {
        if (global.DeploymentWizard) global.DeploymentWizard.addEnvRow('', '');
      });
    }

    var envImport = document.getElementById('wiz-env-import');
    if (envImport) {
      envImport.addEventListener('click', function () {
        var input = prompt('Paste environment variables in KEY=VALUE format (one per line):');
        if (input) {
          var lines = input.split('\n');
          lines.forEach(function (line) {
            var parts = line.split('=');
            if (parts.length >= 2) {
              if (global.DeploymentWizard) global.DeploymentWizard.addEnvRow(parts[0].trim(), parts.slice(1).join('=').trim());
            }
          });
        }
      });
    }

    var envExport = document.getElementById('wiz-env-export');
    if (envExport) {
      envExport.addEventListener('click', function () {
        var rows = [];
        document.querySelectorAll('#wiz-env-list > div').forEach(function (row) {
          var k = row.querySelector('.wiz-env-key').value;
          var v = row.querySelector('.wiz-env-val').value;
          if (k) rows.push(k + '=' + v);
        });
        alert('Environment Variables:\n\n' + (rows.join('\n') || 'None'));
      });
    }

    // Wizard navigation buttons
    var btnBack = document.getElementById('wiz-btn-back');
    var btnNext = document.getElementById('wiz-btn-next');

    if (btnBack) btnBack.addEventListener('click', function () { if (global.DeploymentWizard) global.DeploymentWizard.navigateStep(-1); });
    if (btnNext) btnNext.addEventListener('click', function () { if (global.DeploymentWizard) global.DeploymentWizard.navigateStep(1); });

    // AI Assisted generator
    var btnAiGen = document.getElementById('wiz-ai-generate');
    if (btnAiGen) btnAiGen.addEventListener('click', function () { if (global.DeploymentWizard) global.DeploymentWizard.generateWithAI(); });

    // Initial catalog apps
    if (global.DeploymentCatalog) {
      global.DeploymentCatalog.init();
      global.DeploymentCatalog.loadInitialApps();
      var body = document.getElementById('deploy-catalog-body');
      if (body) {
        body.innerHTML = '<tr><td colspan="7"><div class="skeleton" style="height:120px;border-radius:var(--rounded-lg);"></div></td></tr>';
      }
      global.DeploymentCatalog.renderCatalog();
    }
  }

  function switchTab(tabId) {
    global.DeploymentState.currentTab = tabId;
    document.querySelectorAll('.deploy-tab').forEach(function (btn) {
      btn.classList.toggle('active', btn.dataset.tab === tabId);
    });
    document.querySelectorAll('.deploy-tab-content').forEach(function (c) {
      c.classList.toggle('active', c.id === 'deploy-tab-' + tabId);
    });

    if (tabId === 'wizard') {
      if (global.DeploymentWizard) global.DeploymentWizard.reset();
    } else {
      if (global.DeploymentCatalog) global.DeploymentCatalog.loadInitialApps();
    }
  }

  function populateClusters(clusters) {
    var sel = document.getElementById('wiz-cluster-select');
    if (!sel || !clusters) return;
    var cur = sel.value;
    sel.innerHTML = '<option value="">Select a cluster...</option>';
    
    var targetType = document.getElementById('wiz-target-type');
    var selectedType = targetType ? targetType.value : 'kubernetes';

    clusters.forEach(function (c) {
      var provider = (c.provider || '').toLowerCase();
      var isSwarmCluster = provider === 'docker' || provider === 'docker_swarm';
      if (selectedType === 'kubernetes' && isSwarmCluster) return;
      if (selectedType === 'swarm' && !isSwarmCluster) return;

      sel.innerHTML += '<option value="' + esc(c.name) + '">' + esc(c.name) + ' (' + esc(c.provider) + ')</option>';
    });
    sel.value = cur;
  }

  

  global.DeploymentCenter = { init: init, switchTab: switchTab };

})(window);
