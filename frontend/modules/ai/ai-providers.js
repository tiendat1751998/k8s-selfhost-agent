/**
 * AI Providers — Model registry, health check, latency metrics, test console.
 * Connects to backend /api/v1/ai/ endpoints for real provider management.
 */
(function (global) {
  'use strict';

  var API_BASE = '/api/v1/ai';
  var tableBody = document.getElementById('ai-table-body');
  var providerSelect = document.getElementById('ai-test-provider');
  var promptInput = document.getElementById('ai-test-prompt');
  var sendBtn = document.getElementById('ai-test-send');
  var responseEl = document.getElementById('ai-test-response');
  var latencyEl = document.getElementById('ai-test-latency');
  var tokensEl = document.getElementById('ai-test-tokens');

  var initialized = false;
  function init() {
    if (initialized) return;
    initialized = true;
    AppState.on('aiProviders', render);
    AppState.on('navigate', function (s) {
      if (s === 'ai-providers') fetchProviders();
    });

    if (sendBtn) sendBtn.addEventListener('click', sendTest);

    var addBtn = document.getElementById('add-ai-btn');
    if (addBtn) addBtn.addEventListener('click', showAddModal);

    // Sub-tab switching: Registry ↔ Copilot Console
    var tabRegistry = document.getElementById('ai-tab-btn-registry');
    var tabCopilot = document.getElementById('ai-tab-btn-copilot');
    var panelRegistry = document.getElementById('ai-panel-registry');
    var panelCopilot = document.getElementById('ai-panel-copilot');

    if (tabRegistry && tabCopilot && panelRegistry && panelCopilot) {
      tabRegistry.addEventListener('click', function () {
        tabRegistry.classList.add('active');
        tabCopilot.classList.remove('active');
        panelRegistry.style.display = '';
        panelRegistry.classList.add('active');
        panelCopilot.style.display = 'none';
        panelCopilot.classList.remove('active');
        if (addBtn) addBtn.style.display = '';
      });
      tabCopilot.addEventListener('click', function () {
        tabCopilot.classList.add('active');
        tabRegistry.classList.remove('active');
        panelCopilot.style.display = '';
        panelCopilot.classList.add('active');
        panelRegistry.style.display = 'none';
        panelRegistry.classList.remove('active');
        if (addBtn) addBtn.style.display = 'none';
      });
    }

    // Initial fetch
    fetchProviders();
  }

  /** Fetch providers from the backend API */
  async function fetchProviders() {
    try {
      var res = await fetch(API_BASE + '/providers');
      if (res.ok) {
        var providers = await res.json();
        AppState.setAiProviders(providers);
      } else {
        throw new Error('Response not OK');
      }
    } catch (err) {
      console.warn('Failed to fetch AI providers from API, using state fallback:', err);
      render(AppState.getState().aiProviders);
    }
  }

  function render(providers) {
    if (!tableBody) return;
    providers = providers || [];
    tableBody.innerHTML = '';

    // Update test console dropdown
    if (providerSelect) {
      providerSelect.innerHTML = '<option value="">Select provider…</option>';
      providers.forEach(function (p) {
        var opt = document.createElement('option');
        opt.value = p.name;
        opt.textContent = p.name + ' (' + p.model + ')';
        providerSelect.appendChild(opt);
      });
    }

    if (providers.length === 0) {
      var emptyRow = document.createElement('tr');
      emptyRow.innerHTML = '<td colspan="6" style="text-align:center;color:var(--color-muted);padding:24px">No AI providers registered. Click "+ Add Provider" to get started.</td>';
      tableBody.appendChild(emptyRow);
      return;
    }

    providers.forEach(function (p) {
      var tr = document.createElement('tr');
      tr.innerHTML =
        '<td><strong>' + esc(p.name) + '</strong>' + (p.default ? ' <span class="badge badge-info" style="font-size:10px">DEFAULT</span>' : '') + '</td>' +
        '<td style="font-family:var(--font-number)">' + esc(p.model) + '</td>' +
        '<td><code style="font-size:12px;color:var(--color-muted)">' + esc(p.endpoint) + '</code></td>' +
        '<td>' + healthBadge(p.status) + '</td>' +
        '<td style="font-family:var(--font-number)">' + (p.latency || '—') + '</td>' +
        '<td><div class="action-group">' +
          '<button class="action-btn" data-action="health">Health Check</button>' +
          '<button class="action-btn" data-action="detail">Details</button>' +
          '<button class="action-btn action-btn-danger" data-action="delete">Delete</button>' +
        '</div></td>';

      tr.querySelectorAll('.action-btn').forEach(function (btn) {
        btn.addEventListener('click', function () {
          if (this.dataset.action === 'health') {
            runHealthCheck(p.name);
          } else if (this.dataset.action === 'delete') {
            deleteProvider(p.name);
          } else {
            showDetailModal(p);
          }
        });
      });

      tableBody.appendChild(tr);
    });
  }

  /** Trigger a real health check via the API */
  async function runHealthCheck(name) {
    try {
      var res = await fetch(API_BASE + '/providers/' + encodeURIComponent(name) + '/health', { method: 'POST' });
      var result = await res.json();
      var icon = result.status === 'healthy' ? '✅' : '❌';
      alert('Provider "' + name + '" — ' + icon + ' ' + result.status +
        (result.error ? '\nError: ' + result.error : ''));
      AppState.addAuditLog({ action: 'health-check', target: 'ai/' + name, result: result.status });
      await fetchProviders(); // Refresh the table with updated status
    } catch (err) {
      alert('Health check failed: ' + err.message);
    }
  }

  /** Delete a provider via the API */
  async function deleteProvider(name) {
    if (!confirm('Remove provider "' + name + '"?')) return;

    try {
      var res = await fetch(API_BASE + '/providers/' + encodeURIComponent(name), { method: 'DELETE' });
      await res.json();
      AppState.addAuditLog({ action: 'delete', target: 'ai/' + name, result: 'success' });
      await fetchProviders();
    } catch (err) {
      alert('Delete failed: ' + err.message);
    }
  }

  /** Send a real test prompt to the backend API */
  async function sendTest() {
    if (!providerSelect || !promptInput || !providerSelect.value) {
      alert('Please select a provider and enter a prompt');
      return;
    }

    var prompt = promptInput.value.trim();
    if (!prompt) { alert('Enter a prompt first'); return; }

    if (sendBtn) { sendBtn.disabled = true; sendBtn.textContent = '⏳ Sending…'; }
    if (responseEl) responseEl.textContent = 'Processing… (this may take a moment)';

    try {
      var res = await fetch(API_BASE + '/test', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          provider: providerSelect.value,
          prompt: prompt
        })
      });
      if (!res.ok) {
        var errData = await res.json();
        throw new Error(errData.error || 'Request failed');
      }
      var data = await res.json();
      if (responseEl) responseEl.textContent = data.content;
      if (latencyEl) latencyEl.textContent = data.duration_ms + 'ms';
      if (tokensEl) tokensEl.textContent = (data.prompt_tokens + data.response_tokens);
      AppState.addAuditLog({ action: 'test', target: 'ai/test-console/' + providerSelect.value, result: 'success' });
    } catch (err) {
      if (responseEl) responseEl.textContent = '❌ Error: ' + err.message;
      AppState.addAuditLog({ action: 'test', target: 'ai/test-console', result: 'error' });
    } finally {
      if (sendBtn) { sendBtn.disabled = false; sendBtn.textContent = '▶ Send Test'; }
    }
  }

  function showDetailModal(p) {
    Modal.open({
      title: '🧠 ' + p.name,
      body:
        '<div class="pipeline-detail">' +
          '<div class="pipeline-detail-row"><span class="pipeline-detail-label">Provider Type</span><span class="pipeline-detail-value">' + esc(p.type) + '</span></div>' +
          '<div class="pipeline-detail-row"><span class="pipeline-detail-label">Model</span><span class="pipeline-detail-value">' + esc(p.model) + '</span></div>' +
          '<div class="pipeline-detail-row"><span class="pipeline-detail-label">Endpoint</span><span class="pipeline-detail-value" style="font-size:12px">' + esc(p.endpoint) + '</span></div>' +
          '<div class="pipeline-detail-row"><span class="pipeline-detail-label">Status</span><span class="pipeline-detail-value">' + healthBadge(p.status) + '</span></div>' +
          '<div class="pipeline-detail-row"><span class="pipeline-detail-label">Latency</span><span class="pipeline-detail-value">' + (p.latency || '—') + '</span></div>' +
          '<div class="pipeline-detail-row"><span class="pipeline-detail-label">Default</span><span class="pipeline-detail-value">' + (p.default ? 'Yes' : 'No') + '</span></div>' +
        '</div>',
      actions: [{ label: 'Close', primary: true }]
    });
  }

  function showAddModal() {
    Modal.open({
      title: '+ Add AI Provider',
      body:
        '<div class="form-group"><label class="form-label">Provider Type</label><select class="form-select" id="add-ai-type"><option value="ollama">Ollama</option><option value="openai">OpenAI-compatible</option><option value="vllm">vLLM</option></select></div>' +
        '<div class="form-group"><label class="form-label">Name</label><input class="form-select" id="add-ai-name" placeholder="ollama-local"></div>' +
        '<div class="form-group"><label class="form-label">Model</label><input class="form-select" id="add-ai-model" placeholder="llama3:8b"></div>' +
        '<div class="form-group"><label class="form-label">Endpoint</label><input class="form-select" id="add-ai-endpoint" placeholder="http://localhost:11434"></div>' +
        '<div class="form-group"><label class="form-label">API Key (optional)</label><input class="form-select" id="add-ai-apikey" type="password" placeholder="sk-…"></div>' +
        '<div class="form-group"><label class="form-label"><input type="checkbox" id="add-ai-default"> Set as default provider</label></div>',
      actions: [
        { label: 'Cancel' },
        { label: 'Add Provider', primary: true, onClick: async function () {
          var typeEl = document.getElementById('add-ai-type');
          var nameEl = document.getElementById('add-ai-name');
          var modelEl = document.getElementById('add-ai-model');
          var endpointEl = document.getElementById('add-ai-endpoint');
          var apikeyEl = document.getElementById('add-ai-apikey');
          var defaultEl = document.getElementById('add-ai-default');

          if (!nameEl || !nameEl.value || !modelEl || !modelEl.value || !endpointEl || !endpointEl.value) {
            alert('Please fill in all required fields');
            return;
          }

          var payload = {
            type: typeEl ? typeEl.value : 'ollama',
            name: nameEl.value.trim(),
            model: modelEl.value.trim(),
            endpoint: endpointEl.value.trim(),
            api_key: apikeyEl ? apikeyEl.value.trim() : '',
            default: defaultEl ? defaultEl.checked : false
          };

          try {
            var res = await fetch(API_BASE + '/providers', {
              method: 'POST',
              headers: { 'Content-Type': 'application/json' },
              body: JSON.stringify(payload)
            });
            if (!res.ok) {
              var errData = await res.json();
              throw new Error(errData.error || 'Failed to add provider');
            }
            await res.json();
            AppState.addAuditLog({ action: 'create', target: 'ai/' + payload.name, result: 'success' });
            await fetchProviders();
            Modal.close();
          } catch (err) {
            alert('Failed to add provider: ' + err.message);
          }
        }}
      ]
    });
  }

  function healthBadge(s) {
    if (s === 'healthy') return '<span class="badge badge-healthy">● Healthy</span>';
    if (s === 'degraded') return '<span class="badge badge-degraded">● Degraded</span>';
    if (s === 'down') return '<span class="badge badge-down">● Down</span>';
    return '<span class="badge badge-pending">' + esc(s || 'unknown') + '</span>';
  }

  

  global.AiProvidersSection = { init: init };
})(window);
