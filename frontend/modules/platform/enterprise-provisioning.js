/**
 * Enterprise Cluster Provisioning Sub-Component
 */
(function (global) {
  'use strict';

  function initProvisioning() {
    renderNodePoolsTable();

    // Provider selection cards click handlers
    ['eks', 'gke', 'aks', 'bare'].forEach(function (prov) {
      var el = document.getElementById('prov-card-' + prov);
      if (el) {
        el.addEventListener('click', function () {
          document.querySelectorAll('#ent-prov-panels .health-card').forEach(function (card) {
            card.style.borderColor = 'var(--color-hairline)';
          });
          el.style.borderColor = 'var(--color-primary)';
          global.EnterpriseState.selectedProvider = prov;
        });
      }
    });

    var addPoolBtn = document.getElementById('ent-prov-add-pool-btn');
    if (addPoolBtn) {
      addPoolBtn.addEventListener('click', function () {
        var name = 'pool-' + (global.EnterpriseState.nodePools.length + 1);
        global.EnterpriseState.nodePools.push({
          name: name,
          spec: 't3.xlarge (4 CPU / 16GB)',
          limit: 10,
          autoscale: 'Enabled'
        });
        renderNodePoolsTable();
      });
    }

    // Step Nav Button Handlers
    var backBtn = document.getElementById('ent-prov-btn-back');
    var nextBtn = document.getElementById('ent-prov-btn-next');

    if (backBtn && nextBtn) {
      backBtn.addEventListener('click', function () {
        if (global.EnterpriseState.activeWizardStep > 1) {
          global.EnterpriseState.activeWizardStep--;
          switchWizardStep(global.EnterpriseState.activeWizardStep);
        }
      });

      nextBtn.addEventListener('click', function () {
        if (global.EnterpriseState.activeWizardStep < 7) {
          global.EnterpriseState.activeWizardStep++;
          switchWizardStep(global.EnterpriseState.activeWizardStep);
        }
      });
    }

    var launchBtn = document.getElementById('ent-prov-launch-btn');
    if (launchBtn) {
      launchBtn.addEventListener('click', function () {
        launchBtn.disabled = true;
        var statusEl = document.getElementById('ent-prov-status-badge');
        var durationEl = document.getElementById('ent-prov-duration');
        var fillEl = document.getElementById('ent-prov-progress');
        var consoleEl = document.getElementById('ent-prov-console');

        if (statusEl) {
          statusEl.textContent = 'Provisioning';
          statusEl.className = 'badge badge-degraded';
        }

        var logs = [
          'Initializing Terraform providers plugins...',
          'Loading workspace state context backend AWS S3...',
          'Applying terraform infrastructure definition files...',
          'Deploying control plane AWS EKS cluster resources...',
          'Control Plane endpoint online. Verifying communication credentials...',
          'Configuring Node Group system-pool: scaling target [2-3] nodes...',
          'System instance nodes registered successfully with API server daemon.',
          'Configuring Node Group workload-pool: scaling target [3-5] nodes...',
          'Bootstrapping Kubernetes cluster network routes...',
          'Installing Cilium CNI security engine eBPF controls hooks...',
          'Cilium network plugin is active. Isolation network policies applied.',
          'Creating gp3 default StorageClass volume attachments rules...',
          'Bootstrapping cluster monitoring system namespace observ-metrics...',
          'Installing Prometheus Operator metrics monitors...',
          'Grafana dashboard analytics console is online.',
          'Installing Loki Log aggregation agent containers daemonsets...',
          'GitOps bootstrapping: Installing ArgoCD deployment controller engine...',
          'Registering EKS cluster token with GitOps Central Control Management...',
          'Synchronizing cluster basic resources blueprints repository...',
          'Base cluster synchronization finalized. Ingress controller routing healthy.',
          'EKS Cluster registered in central dashboard. Provisioning completed!'
        ];

        consoleEl.textContent = '';
        var logIdx = 0;
        var start = Date.now();
        var progressTimer = setInterval(function () {
          var elapsed = ((Date.now() - start) / 1000).toFixed(1);
          if (durationEl) durationEl.textContent = elapsed + 's';
          if (fillEl) fillEl.style.width = Math.min(100, (logIdx / logs.length) * 100) + '%';

          if (logIdx < logs.length) {
            consoleEl.textContent += '[Bootstrap] ' + logs[logIdx] + '\n';
            consoleEl.scrollTop = consoleEl.scrollHeight;
            logIdx++;
          } else {
            clearInterval(progressTimer);
            launchBtn.disabled = false;
            if (statusEl) {
              statusEl.textContent = 'Active';
              statusEl.className = 'badge badge-healthy';
            }

            // Push cluster to application state
            var currentClusters = AppState.getState().kubernetes || [];
            var providerMap = {
              'eks': 'aws_eks',
              'gke': 'gcp_gke',
              'aks': 'azure_aks',
              'bare': 'bare_metal'
            };
            var newCluster = {
              name: 'eks-' + Math.floor(Math.random() * 900 + 100) + '-prod',
              provider: providerMap[global.EnterpriseState.selectedProvider] || 'aws_eks',
              status: 'healthy',
              nodes: global.EnterpriseState.nodePools.reduce(function (acc, val) { return acc + val.limit; }, 0),
              cpu: '16 Cores',
              memory: '64 GB'
            };
            currentClusters.push(newCluster);
            
            // FIX: updateState was a bug, correct call is setKubernetes
            AppState.setKubernetes(currentClusters);

            global.EnterpriseState.addAuditLogEntry('admin', 'Platform Admin', 'Provision Cluster', newCluster.name);
            alert('Kubernetes cluster ' + newCluster.name + ' successfully provisioned and linked to console.');
          }
        }, 350);
      });
    }
  }

  function switchWizardStep(step) {
    // Toggle active panel
    document.querySelectorAll('.ent-prov-step-panel').forEach(function (panel) {
      panel.style.display = 'none';
      if (parseInt(panel.dataset.wstep) === step) panel.style.display = 'block';
    });

    // Update active index indicator list
    document.querySelectorAll('#ent-prov-wizard-steps .sidebar-link').forEach(function (el) {
      var itemStep = parseInt(el.dataset.wstep);
      el.classList.remove('active');
      if (itemStep === step) el.classList.add('active');
    });

    // Update title
    var titleEl = document.getElementById('ent-prov-step-title');
    var titles = {
      1: 'Step 1: Select Infrastructure Provider',
      2: 'Step 2: Configure Region / Infra',
      3: 'Step 3: Define Node Pools',
      4: 'Step 4: Networking Setup (CNI)',
      5: 'Step 5: Storage Configuration',
      6: 'Step 6: Security & GitOps auto-bootstrapping',
      7: 'Step 7: Bootstrap Rollout'
    };
    if (titleEl) titleEl.textContent = titles[step];

    // Buttons control
    var backBtn = document.getElementById('ent-prov-btn-back');
    var nextBtn = document.getElementById('ent-prov-btn-next');
    if (backBtn) backBtn.disabled = step === 1;
    if (nextBtn) nextBtn.disabled = step === 7;
  }

  function renderNodePoolsTable() {
    var tbody = document.getElementById('ent-prov-nodepools-body');
    if (!tbody) return;
    tbody.innerHTML = '';

    global.EnterpriseState.nodePools.forEach(function (p, idx) {
      var tr = document.createElement('tr');
      tr.innerHTML =
        '<td><strong>' + esc(p.name) + '</strong></td>' +
        '<td>' + esc(p.spec) + '</td>' +
        '<td style="font-family:var(--font-number);">' + p.limit + ' max nodes</td>' +
        '<td><span class="badge badge-healthy">' + esc(p.autoscale) + '</span></td>' +
        '<td><button class="action-btn danger ent-prov-del-pool" data-idx="' + idx + '">Remove</button></td>';
      tbody.appendChild(tr);
    });

    tbody.querySelectorAll('.ent-prov-del-pool').forEach(function (btn) {
      btn.addEventListener('click', function () {
        var index = parseInt(this.dataset.idx);
        global.EnterpriseState.nodePools.splice(index, 1);
        renderNodePoolsTable();
      });
    });
  }

  

  global.EnterpriseProvisioning = {
    init: initProvisioning,
    renderNodePools: renderNodePoolsTable
  };

})(window);
