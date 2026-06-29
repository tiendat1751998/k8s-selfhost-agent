/**
 * Deployment Center — Deployment Wizard Component
 */
(function (global) {
  'use strict';

  function resetWizard() {
    global.DeploymentState.wizardStep = 1;
    document.getElementById('wiz-btn-back').disabled = true;
    document.getElementById('wiz-btn-next').textContent = 'Next →';
    document.getElementById('wiz-btn-next').disabled = false;
    document.getElementById('wiz-image').value = '';
    document.getElementById('wiz-image-tag').value = 'latest';
    document.getElementById('wiz-replicas').value = '2';
    document.getElementById('wiz-namespace').value = 'default';
    document.getElementById('wiz-cluster-select').value = '';
    document.getElementById('wiz-cpu-req').value = '100m';
    document.getElementById('wiz-cpu-limit').value = '500m';
    document.getElementById('wiz-mem-req').value = '128Mi';
    document.getElementById('wiz-mem-limit').value = '512Mi';
    document.getElementById('wiz-container-port').value = '80';
    document.getElementById('wiz-service-port').value = '80';
    document.getElementById('wiz-network-type').value = 'ClusterIP';
    document.getElementById('wiz-domain-group').style.display = 'none';
    document.getElementById('wiz-volume-type').value = 'none';
    document.getElementById('wiz-vol-details').style.display = 'none';
    document.getElementById('wiz-env-list').innerHTML = '';
    addEnvRow('APP_ENV', 'production');
    showStepPanel(1);
  }

  function addEnvRow(k, v) {
    var div = document.createElement('div');
    div.style.cssText = 'display:flex;gap:var(--space-xs);margin-bottom:var(--space-xs);';
    div.innerHTML =
      '<input type="text" class="form-select form-select-sm wiz-env-key" placeholder="KEY" value="' + esc(k) + '" style="flex:1;">' +
      '<input type="text" class="form-select form-select-sm wiz-env-val" placeholder="VALUE" value="' + esc(v) + '" style="flex:1;">' +
      '<button class="btn btn-ghost btn-sm wiz-env-del" style="border-color:var(--color-trading-down);color:var(--color-trading-down);padding:0 8px;">✕</button>';
    div.querySelector('.wiz-env-del').addEventListener('click', function () {
      div.remove();
    });
    document.getElementById('wiz-env-list').appendChild(div);
  }

  function showStepPanel(step) {
    global.DeploymentState.wizardStep = step;
    document.querySelectorAll('.wizard-step-panel').forEach(function (p) {
      p.classList.toggle('active', parseInt(p.dataset.step) === step);
      p.style.display = parseInt(p.dataset.step) === step ? '' : 'none';
    });

    // Update Titles
    var titles = {
      1: 'Step 1: Select Target',
      2: 'Step 2: Source & Image',
      3: 'Step 3: Configure Resources',
      4: 'Step 4: Configure Network',
      5: 'Step 5: Configure Storage',
      6: 'Step 6: Configure Environment',
      7: 'Step 7: Configure Security',
      8: 'Step 8: Review & Cost Estimation',
      9: 'Step 9: Rolling Out Deployment'
    };
    document.getElementById('wizard-step-title').textContent = titles[step] || ('Step ' + step);
    document.getElementById('wizard-steps-indicator').textContent = 'Step ' + step + ' of 9';

    // Buttons configuration
    var btnBack = document.getElementById('wiz-btn-back');
    var btnNext = document.getElementById('wiz-btn-next');
    if (btnBack) btnBack.disabled = step === 1 || step === 9;

    if (step === 8) {
      if (btnNext) btnNext.textContent = '🚀 Deploy';
      calculateCostsAndReview();
    } else if (step === 9) {
      if (btnNext) {
        btnNext.textContent = 'Finish';
        btnNext.disabled = true;
      }
      triggerWizardDeploy();
    } else {
      if (btnNext) btnNext.textContent = 'Next →';
      if (btnNext) btnNext.disabled = false;
    }
  }

  function validateStep(step) {
    if (step === 1) {
      var cluster = document.getElementById('wiz-cluster-select').value;
      if (!cluster) {
        document.getElementById('wiz-cluster-select').style.borderColor = 'var(--color-trading-down)';
        return false;
      }
      document.getElementById('wiz-cluster-select').style.borderColor = '';
    }
    if (step === 2) {
      var img = document.getElementById('wiz-image').value.trim();
      if (!img) {
        document.getElementById('wiz-image').style.borderColor = 'var(--color-trading-down)';
        return false;
      }
      document.getElementById('wiz-image').style.borderColor = '';
    }
    return true;
  }

  function navigateStep(direction) {
    if (direction === 1) {
      if (global.DeploymentState.wizardStep === 9) {
        // Finished deploy, back to catalog
        if (global.DeploymentCenter) {
          global.DeploymentCenter.switchTab('catalog');
        }
        return;
      }
      if (!validateStep(global.DeploymentState.wizardStep)) return;
      showStepPanel(global.DeploymentState.wizardStep + 1);
    } else {
      showStepPanel(global.DeploymentState.wizardStep - 1);
    }
  }

  function calculateCostsAndReview() {
    var type = document.getElementById('wiz-target-type').value;
    var cluster = document.getElementById('wiz-cluster-select').value;
    var ns = document.getElementById('wiz-namespace').value.trim();
    var placement = document.getElementById('wiz-node-placement').value;
    var imgType = document.getElementById('wiz-source-type').value;
    var img = document.getElementById('wiz-image').value.trim();
    var tag = document.getElementById('wiz-image-tag').value.trim();
    var cpu = document.getElementById('wiz-cpu-req').value.trim();
    var mem = document.getElementById('wiz-mem-req').value.trim();
    var replicas = parseInt(document.getElementById('wiz-replicas').value) || 1;
    var port = document.getElementById('wiz-service-port').value;

    // Costs Estimator
    var cpuCores = parseCPU(cpu);
    var memGiB = parseMemory(mem);
    var volType = document.getElementById('wiz-volume-type').value;
    var volSize = volType !== 'none' ? 20 : 0; // Default 20GB size

    var costCpu = cpuCores * 15;
    var costMem = memGiB * 4;
    var costStorage = volSize * 0.10;
    var costPerReplica = costCpu + costMem + costStorage;
    var totalCost = costPerReplica * replicas;

    document.getElementById('cost-cpu-val').textContent = cpu + ' ($' + costCpu.toFixed(2) + '/mo)';
    document.getElementById('cost-mem-val').textContent = mem + ' ($' + costMem.toFixed(2) + '/mo)';
    document.getElementById('cost-replicas-val').textContent = replicas + 'x';
    document.getElementById('cost-total-val').textContent = '$' + totalCost.toFixed(2) + '/mo';

    var appName = img.split('/').pop().split(':')[0];
    document.getElementById('rev-name').textContent = appName;
    document.getElementById('rev-target').textContent = type + ' / ' + cluster + (type === 'kubernetes' ? ' / ' + ns : ' / placement=' + placement);
    document.getElementById('rev-source').textContent = imgType + ' / ' + img + ':' + tag;
    document.getElementById('rev-port').textContent = port;

    // YAML manifest generation
    global.DeploymentState.wizManifest = '';
    if (type === 'kubernetes') {
      global.DeploymentState.wizManifest =
        'apiVersion: apps/v1\n' +
        'kind: Deployment\n' +
        'metadata:\n' +
        '  name: ' + appName + '\n' +
        '  namespace: ' + ns + '\n' +
        'spec:\n' +
        '  replicas: ' + replicas + '\n' +
        '  selector:\n' +
        '    matchLabels:\n' +
        '      app: ' + appName + '\n' +
        '  template:\n' +
        '    metadata:\n' +
        '      labels:\n' +
        '        app: ' + appName + '\n' +
        '    spec:\n' +
        '      containers:\n' +
        '      - name: app\n' +
        '        image: ' + img + ':' + tag + '\n' +
        '        resources:\n' +
        '          requests:\n' +
        '            cpu: "' + cpu + '"\n' +
        '            memory: "' + mem + '"\n' +
        '          limits:\n' +
        '            cpu: "' + document.getElementById('wiz-cpu-limit').value + '"\n' +
        '            memory: "' + document.getElementById('wiz-mem-limit').value + '"\n' +
        (port ? '        ports:\n        - containerPort: ' + port + '\n' : '');
    } else {
      global.DeploymentState.wizManifest =
        'version: "3.8"\n' +
        'services:\n' +
        '  ' + appName + ':\n' +
        '    image: ' + img + ':' + tag + '\n' +
        '    deploy:\n' +
        '      replicas: ' + replicas + '\n' +
        '      resources:\n' +
        '        limits:\n' +
        '          cpus: "' + cpu + '"\n' +
        '          memory: ' + mem + '\n' +
        '      placement:\n' +
        '        constraints:\n' +
        '          - node.role == ' + (placement === 'manager-only' ? 'manager' : 'worker') + '\n';
    }
    document.getElementById('wiz-manifest-preview').textContent = global.DeploymentState.wizManifest;
  }

  function parseCPU(c) {
    if (!c) return 0.1;
    if (c.indexOf('m') >= 0) return parseInt(c) / 1000;
    return parseFloat(c);
  }

  function parseMemory(m) {
    if (!m) return 0.25;
    var val = parseFloat(m);
    if (m.indexOf('Ki') >= 0) return val / (1024 * 1024);
    if (m.indexOf('Mi') >= 0) return val / 1024;
    if (m.indexOf('Gi') >= 0) return val;
    return val;
  }

  function triggerWizardDeploy() {
    var progressEl = document.getElementById('wiz-deploy-progress');
    var logEl = document.getElementById('wiz-deploy-log');
    var statusEl = document.getElementById('wiz-deploy-status');
    var durEl = document.getElementById('wiz-deploy-duration');
    var btnNext = document.getElementById('wiz-btn-next');

    progressEl.style.width = '0%';
    statusEl.textContent = 'Deploying';
    statusEl.className = 'badge badge-degraded';

    var logs = [
      '→ Validating target cluster connection...',
      '→ Creating configuration bundle and credentials...',
      '→ Creating Pull Request in Git repository (GitOps Flow)...',
      '→ PR #148 created and auto-approved.',
      '→ Triggering ArgoCD sync / Swarm Service deploy...',
      '→ Registering Service endpoints & DNS records...',
      '→ Pulling container image...',
      '→ Pods rolling out (0/' + document.getElementById('wiz-replicas').value + ')...',
      '→ Probes checking health...',
      '✓ Rollout complete. Service is healthy.'
    ];

    var step = 0;
    var startTime = Date.now();

    function runLogStep() {
      if (step >= logs.length) {
        statusEl.textContent = 'Healthy';
        statusEl.className = 'badge badge-healthy';
        progressEl.style.width = '100%';
        btnNext.disabled = false;

        // Add application to catalog
        var type = document.getElementById('wiz-target-type').value;
        var cluster = document.getElementById('wiz-cluster-select').value;
        var ns = document.getElementById('wiz-namespace').value.trim();
        var img = document.getElementById('wiz-image').value.trim();
        var appName = img.split('/').pop().split(':')[0];
        global.DeploymentState.apps.unshift({
          name: appName,
          team: 'SRE',
          env: 'production',
          image: img + ':' + document.getElementById('wiz-image-tag').value,
          target: cluster,
          namespace: type === 'kubernetes' ? ns : '',
          type: type,
          replicas: parseInt(document.getElementById('wiz-replicas').value) || 2,
          status: 'healthy',
          cpu: document.getElementById('wiz-cpu-req').value,
          memory: document.getElementById('wiz-mem-req').value,
          port: parseInt(document.getElementById('wiz-service-port').value) || 80,
          netType: document.getElementById('wiz-network-type').value,
          volume: document.getElementById('wiz-volume-type').value
        });

        AppState.addAuditLog({ action: 'deploy', target: 'app/' + appName, result: 'success' });
        return;
      }

      logEl.textContent += '\n[' + new Date().toLocaleTimeString() + '] ' + logs[step];
      logEl.scrollTop = logEl.scrollHeight;

      var progress = Math.round(((step + 1) / logs.length) * 100);
      progressEl.style.width = progress + '%';
      durEl.textContent = ((Date.now() - startTime) / 1000).toFixed(1) + 's';

      step++;
      setTimeout(runLogStep, 400 + Math.random() * 400);
    }

    logEl.textContent = '[' + new Date().toLocaleTimeString() + '] Initializing deployment session...';
    setTimeout(runLogStep, 500);
  }

  function generateWithAI() {
    var promptEl = document.getElementById('wiz-ai-prompt');
    var query = promptEl.value.trim();
    if (!query) {
      promptEl.style.borderColor = 'var(--color-trading-down)';
      return;
    }
    promptEl.style.borderColor = '';

    // Simulate AI thinking
    var btn = document.getElementById('wiz-ai-generate');
    btn.textContent = '⏳ Thinking...';
    btn.disabled = true;

    setTimeout(function () {
      btn.textContent = '▶ Generate Configuration';
      btn.disabled = false;

      // Basic extraction regexes
      var q = query.toLowerCase();
      var image = 'nginx';
      var tag = 'latest';
      var replicas = 2;
      var cpu = '100m';
      var mem = '128Mi';
      var net = 'ClusterIP';

      // Parse image & tag
      var imgMatch = query.match(/(?:deploy|image|source)\s+([a-zA-Z0-9_\-\.\/]+)(?::([a-zA-Z0-9_\-\.]+))?/i);
      if (imgMatch) {
        image = imgMatch[1];
        if (imgMatch[2]) tag = imgMatch[2];
      } else {
        // search tokens
        var tokens = ['nginx', 'redis', 'postgres', 'mysql', 'node', 'python', 'django', 'tomcat', 'ghost'];
        for (var i = 0; i < tokens.length; i++) {
          if (q.indexOf(tokens[i]) >= 0) {
            image = tokens[i];
            break;
          }
        }
      }

      // Parse replicas
      var repMatch = query.match(/(\d+)\s*replicas/i);
      if (repMatch) replicas = parseInt(repMatch[1]);

      // Parse CPU
      var cpuMatch = query.match(/(\d+(?:m|core|vcpus?)?)\s*(?:cpu|cores?)/i);
      if (cpuMatch) {
        var c = cpuMatch[1];
        if (c.indexOf('m') < 0 && c.indexOf('core') < 0 && c.indexOf('vcpu') < 0) {
          cpu = c; // cores
        } else if (c.indexOf('m') >= 0) {
          cpu = c;
        } else {
          cpu = parseInt(c).toString();
        }
      }

      // Parse Memory
      var memMatch = query.match(/(\d+)\s*(?:gb|mb|mi|gi|g|m)\s*mem/i) || query.match(/(\d+)\s*(?:gb|mb|mi|gi|g|m)\b/i);
      if (memMatch) {
        var size = memMatch[1];
        var text = memMatch[0].toLowerCase();
        if (text.indexOf('gb') >= 0 || text.indexOf('gi') >= 0 || text.indexOf('g') >= 0) {
          mem = size + 'Gi';
        } else {
          mem = size + 'Mi';
        }
      }

      // Parse Network Type
      if (q.indexOf('loadbalancer') >= 0 || q.indexOf('public') >= 0) {
        net = 'LoadBalancer';
      } else if (q.indexOf('ingress') >= 0 || q.indexOf('domain') >= 0) {
        net = 'Ingress';
      } else if (q.indexOf('nodeport') >= 0) {
        net = 'NodePort';
      }

      // Prefill forms
      document.getElementById('wiz-image').value = image;
      document.getElementById('wiz-image-tag').value = tag;
      document.getElementById('wiz-replicas').value = replicas;
      document.getElementById('wiz-cpu-req').value = cpu;
      document.getElementById('wiz-mem-req').value = mem;
      document.getElementById('wiz-network-type').value = net;
      if (net === 'Ingress') {
        document.getElementById('wiz-domain-group').style.display = '';
        document.getElementById('wiz-domain').value = image.split('/').pop() + '.corp.internal';
      }

      // Try setting target cluster
      var clusterSel = document.getElementById('wiz-cluster-select');
      if (clusterSel && clusterSel.options.length > 1) {
        clusterSel.selectedIndex = 1; // default to first option
      }

      // Transition to Review Step
      showStepPanel(8);
      alert('AI Configuration Generated Successfully! Switched to Review Panel. ✅');
    }, 1200);
  }

  

  global.DeploymentWizard = { reset: resetWizard, addEnvRow, showStepPanel, navigateStep, generateWithAI };

})(window);
