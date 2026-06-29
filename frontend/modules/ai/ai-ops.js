/**
 * AI Operations — RCA Panel, Remediation View, Chat Console, Incident Correlation.
 * P3-001 through P3-004 combined.
 */
(function (global) {
  'use strict';

  var chatMessages = [];

  function init() {
    var rcaBtn = document.getElementById('ai-rca-run');
    var chatSend = document.getElementById('ai-chat-send');
    var chatInput = document.getElementById('ai-chat-input');

    if (rcaBtn) rcaBtn.addEventListener('click', runRCA);
    if (chatSend) chatSend.addEventListener('click', sendChat);
    if (chatInput) chatInput.addEventListener('keydown', function (e) {
      if (e.key === 'Enter') sendChat();
    });

    // Populate cluster dropdown
    AppState.on('kubernetes', populateClusters);
    AppState.on('navigate', function (s) {
      if (s === 'ai-ops') populateClusters(AppState.getState().kubernetes);
    });

    // Initial welcome message in chat
    addChatMessage('system', 'Welcome to AI Operations Console. Ask me anything about your clusters, pods, or incidents.');
  }

  function populateClusters(clusters) {
    var sel = document.getElementById('ai-ops-cluster');
    if (!sel || !clusters) return;
    var cur = sel.value;
    sel.innerHTML = '<option value="">Select Cluster</option>';
    clusters.forEach(function (c) {
      sel.innerHTML += '<option value="' + esc(c.name) + '">' + esc(c.name) + '</option>';
    });
    sel.value = cur;
  }

  function runRCA() {
    var target = document.getElementById('ai-rca-target');
    var nameInput = document.getElementById('ai-rca-name');
    var btn = document.getElementById('ai-rca-run');
    var type = target ? target.value : 'pod';
    var name = nameInput ? nameInput.value.trim() : '';

    if (!name) { nameInput.style.borderColor = 'var(--color-trading-down)'; return; }
    nameInput.style.borderColor = '';

    btn.textContent = '⏳ Analyzing...';
    btn.disabled = true;

    // Simulate AI analysis
    setTimeout(function () {
      var result = generateRCAResult(type, name);
      renderRemediation(result);
      renderCorrelation(result);
      btn.textContent = '🔍 Run RCA';
      btn.disabled = false;
      AppState.addAuditLog({ action: 'ai-rca', target: type + '/' + name, result: 'success' });
    }, 2000);
  }

  function generateRCAResult(type, name) {
    var rcaData = {
      pod: {
        rootCause: 'Out of Memory (OOMKilled). Container exceeded memory limit of 512Mi. Peak usage was 687Mi during traffic spike at 08:55 UTC.',
        confidence: 87,
        risk: 'High',
        fix: 'Increase memory limit to 1Gi in deployment spec. Consider adding HPA with memory-based scaling.',
        patch: 'resources:\n  limits:\n    memory: "1Gi"\n  requests:\n    memory: "512Mi"'
      },
      deployment: {
        rootCause: 'Rolling update failed. New image tag v2.4.1 has startup probe failure. Container crashes within 30s of startup.',
        confidence: 92,
        risk: 'Critical',
        fix: 'Rollback to v2.3.9. Investigate startup probe configuration and application initialization.',
        patch: 'image: registry.internal/' + name + ':v2.3.9'
      },
      namespace: {
        rootCause: 'ResourceQuota exceeded. Namespace has reached pod limit (50/50). New deployments cannot schedule.',
        confidence: 95,
        risk: 'Medium',
        fix: 'Increase ResourceQuota pod limit or clean up unused pods.',
        patch: 'spec:\n  hard:\n    pods: "100"'
      },
      cluster: {
        rootCause: 'Node pressure detected on 2 of 4 nodes. Memory pressure on ip-10-0-1-78 (91% usage). Disk pressure on ip-10-0-2-11.',
        confidence: 78,
        risk: 'High',
        fix: 'Drain affected nodes and add capacity. Consider cluster autoscaler configuration.',
        patch: 'replicas: 6  # increase node count'
      }
    };
    return rcaData[type] || rcaData.pod;
  }

  function renderRemediation(result) {
    var body = document.getElementById('ai-remediation-body');
    if (!body) return;
    body.innerHTML =
      '<div class="pipeline-detail" style="margin-bottom:var(--space-sm);">' +
        '<div class="pipeline-detail-row"><span class="pipeline-detail-label">Root Cause</span><span class="pipeline-detail-value" style="max-width:250px;word-break:break-word;">' + esc(result.rootCause) + '</span></div>' +
        '<div class="pipeline-detail-row"><span class="pipeline-detail-label">Confidence</span><span class="pipeline-detail-value">' + confidenceBadge(result.confidence) + '</span></div>' +
        '<div class="pipeline-detail-row"><span class="pipeline-detail-label">Risk Score</span><span class="pipeline-detail-value"><span class="badge badge-' + riskClass(result.risk) + '">' + esc(result.risk) + '</span></span></div>' +
        '<div class="pipeline-detail-row"><span class="pipeline-detail-label">Suggested Fix</span><span class="pipeline-detail-value" style="max-width:250px;word-break:break-word;">' + esc(result.fix) + '</span></div>' +
      '</div>' +
      '<div style="margin-top:var(--space-sm);">' +
        '<label class="form-label">Patch Preview</label>' +
        '<pre class="ai-test-response" style="font-size:12px;">' + esc(result.patch) + '</pre>' +
      '</div>' +
      '<div style="display:flex;gap:var(--space-xs);margin-top:var(--space-sm);">' +
        '<button class="btn btn-primary btn-sm" onclick="alert(\'GitOps patch created!\')">📝 Apply Patch</button>' +
        '<button class="btn btn-ghost btn-sm" onclick="alert(\'PR created!\')">🔀 Create PR</button>' +
      '</div>';
  }

  function renderCorrelation(result) {
    var body = document.getElementById('ai-correlation-body');
    if (!body) return;
    body.innerHTML =
      '<div style="margin-bottom:var(--space-sm);">' +
        '<h4 style="margin:0 0 var(--space-sm);font-size:14px;color:var(--color-text-secondary);">Similar Incidents</h4>' +
        correlationCard('OOMKilled', '3 days ago', 'payment-svc', 'Fixed by increasing memory limit') +
        correlationCard('CrashLoopBackOff', '1 week ago', 'auth-proxy', 'Root cause: missing env var') +
        correlationCard('ImagePullBackOff', '2 weeks ago', 'frontend', 'Fixed by updating image registry credentials') +
      '</div>' +
      '<div>' +
        '<h4 style="margin:0 0 var(--space-sm);font-size:14px;color:var(--color-text-secondary);">Learned Solutions</h4>' +
        '<div style="background:rgba(252,213,53,0.06);border:1px solid rgba(252,213,53,0.15);border-radius:6px;padding:var(--space-sm);font-size:13px;">' +
          '<strong>Pattern detected:</strong> OOMKilled events cluster around traffic spikes (08:00-10:00 UTC). ' +
          'Consider implementing predictive scaling based on traffic patterns.' +
        '</div>' +
      '</div>';
  }

  function correlationCard(type, when, resource, resolution) {
    return '<div style="background:var(--color-bg);border-radius:6px;padding:var(--space-sm);margin-bottom:var(--space-xs);">' +
      '<div style="display:flex;justify-content:space-between;align-items:center;">' +
        '<span class="badge badge-down">' + esc(type) + '</span>' +
        '<span style="font-size:11px;color:var(--color-muted);">' + esc(when) + '</span>' +
      '</div>' +
      '<div style="font-size:12px;margin-top:4px;"><strong>' + esc(resource) + '</strong> — ' + esc(resolution) + '</div>' +
    '</div>';
  }

  // ── AI CHAT ──
  function sendChat() {
    var input = document.getElementById('ai-chat-input');
    if (!input) return;
    var text = input.value.trim();
    if (!text) return;
    input.value = '';

    addChatMessage('user', text);

    // Simulate AI response
    setTimeout(function () {
      var response = generateChatResponse(text);
      addChatMessage('ai', response);
    }, 1500);
  }

  function addChatMessage(role, text) {
    chatMessages.push({ role: role, text: text, time: new Date().toISOString() });
    renderChat();
  }

  function renderChat() {
    var container = document.getElementById('ai-chat-messages');
    if (!container) return;
    container.innerHTML = chatMessages.map(function (m) {
      var isUser = m.role === 'user';
      var isSystem = m.role === 'system';
      return '<div style="display:flex;justify-content:' + (isUser ? 'flex-end' : 'flex-start') + ';margin-bottom:var(--space-sm);">' +
        '<div style="max-width:80%;padding:var(--space-sm);border-radius:8px;font-size:13px;' +
          (isUser ? 'background:var(--color-primary);color:#0b0e11;' :
           isSystem ? 'background:var(--color-hairline);color:var(--color-muted);font-style:italic;' :
           'background:var(--color-surface);border:1px solid var(--color-hairline);') + '">' +
          '<div style="white-space:pre-wrap;">' + esc(m.text) + '</div>' +
          '<div style="font-size:10px;margin-top:4px;opacity:0.6;">' + timeShort(m.time) + '</div>' +
        '</div>' +
      '</div>';
    }).join('');
    container.scrollTop = container.scrollHeight;
  }

  function generateChatResponse(query) {
    var q = query.toLowerCase();
    if (q.indexOf('restart') >= 0)
      return 'Pod restarts are typically caused by:\n1. OOMKilled — container exceeds memory limit\n2. Liveness probe failure — health check timing out\n3. Application crash — unhandled exception\n\nCheck `kubectl describe pod <name>` for the "Last State" reason. I recommend running an RCA analysis for detailed root cause identification.';
    if (q.indexOf('oomkilled') >= 0 || q.indexOf('oom') >= 0)
      return 'OOMKilled indicates the container was terminated because it exceeded its memory limit.\n\nCommon fixes:\n• Increase memory limits in the deployment spec\n• Optimize application memory usage\n• Add memory-based HPA for auto-scaling\n• Check for memory leaks in the application';
    if (q.indexOf('scale') >= 0)
      return 'To scale a deployment:\n$ kubectl scale deployment/<name> --replicas=<count>\n\nFor auto-scaling:\n$ kubectl autoscale deployment/<name> --min=2 --max=10 --cpu-percent=80\n\nRecommendation: Use HPA with both CPU and memory metrics for production workloads.';
    if (q.indexOf('node') >= 0 && q.indexOf('pressure') >= 0)
      return 'Node pressure conditions:\n• MemoryPressure: Available memory < eviction threshold\n• DiskPressure: Available disk space < threshold\n• PIDPressure: Too many processes\n\nResolution:\n1. Drain the affected node\n2. Investigate and clean up resources\n3. Consider adding more nodes to the cluster';
    return 'I analyzed your question: "' + query + '"\n\nBased on the current cluster state:\n• 3 clusters healthy, 1 degraded\n• 6 active pods, 2 with issues\n• Resource utilization: CPU 67%, Memory 72%\n\nI recommend checking the Action Center for available operations, or run a detailed RCA analysis from the panel above.';
  }

  function confidenceBadge(conf) {
    var cls = conf >= 80 ? 'healthy' : conf >= 60 ? 'degraded' : 'down';
    return '<span class="badge badge-' + cls + '">' + conf + '%</span>';
  }

  function riskClass(risk) {
    if (risk === 'Critical') return 'down';
    if (risk === 'High') return 'degraded';
    return 'healthy';
  }

  function timeShort(ts) { try { return new Date(ts).toLocaleTimeString(); } catch (e) { return ts; } }
  

  global.AiOpsSection = { init: init };
})(window);
