/**
 * AI Copilot — Natural Language Command Interface
 * Floating command bar accessible via Ctrl+Shift+K from any page.
 * Parses natural language intents and executes platform operations.
 */
(function (global) {
  'use strict';

  var copilotEl = null;
  var inputEl = null;
  var outputEl = null;
  var historyEl = null;
  var isOpen = false;
  var commandHistory = [];
  var historyIndex = -1;
  var pageInputEl = null;
  var pageOutputEl = null;
  var pageSuggestionsEl = null;

  /* ─── Intent Parser ─── */
  var intentPatterns = [
    { pattern: /deploy\s+(\S+)\s+to\s+(\S+)/i, intent: 'deploy', extract: function(m){ return { image: m[1], target: m[2] }; } },
    { pattern: /scale\s+(\S+)\s+to\s+(\d+)\s*replicas?/i, intent: 'scale', extract: function(m){ return { deployment: m[1], replicas: parseInt(m[2]) }; } },
    { pattern: /restart\s+(pod|deployment|service)\s+(\S+)/i, intent: 'restart', extract: function(m){ return { resourceType: m[1], name: m[2] }; } },
    { pattern: /show\s+failed\s+deployments?\s*(today|this week)?/i, intent: 'query_failed_deployments', extract: function(m){ return { timeframe: m[1] || 'today' }; } },
    { pattern: /find\s+(all\s+)?oomkilled\s+pods?/i, intent: 'query_oomkilled', extract: function(){ return {}; } },
    { pattern: /health\s+(of\s+)?(\S+)/i, intent: 'cluster_health', extract: function(m){ return { cluster: m[2] }; } },
    { pattern: /what\s+is\s+the\s+health/i, intent: 'cluster_health', extract: function(){ return { cluster: 'all' }; } },
    { pattern: /rollback\s+(\S+)/i, intent: 'rollback', extract: function(m){ return { deployment: m[1] }; } },
    { pattern: /delete\s+(pod|deployment)\s+(\S+)/i, intent: 'delete', extract: function(m){ return { resourceType: m[1], name: m[2] }; } },
    { pattern: /logs?\s+(for\s+)?(\S+)/i, intent: 'view_logs', extract: function(m){ return { resource: m[2] }; } },
    { pattern: /status/i, intent: 'system_status', extract: function(){ return {}; } },
    { pattern: /help/i, intent: 'help', extract: function(){ return {}; } }
  ];

  function parseIntent(command) {
    for (var i = 0; i < intentPatterns.length; i++) {
      var match = command.match(intentPatterns[i].pattern);
      if (match) {
        return { intent: intentPatterns[i].intent, params: intentPatterns[i].extract(match), raw: command };
      }
    }
    return { intent: 'unknown', params: {}, raw: command };
  }

  /* ─── Command Suggestions ─── */
  var suggestions = [
    'deploy nginx to production',
    'scale payment-api to 20 replicas',
    'show failed deployments today',
    'find all OOMKilled pods',
    'health of prod-cluster-01',
    'restart deployment order-service',
    'rollback payment-api',
    'logs for payment-api-7f8a',
    'status',
    'help'
  ];

  /* ─── Execution Engine ─── */
  function executeIntent(parsed) {
    switch (parsed.intent) {
      case 'deploy':
        return simulateExecution('🚀 Deploying <strong>' + parsed.params.image + '</strong> to <strong>' + parsed.params.target + '</strong>...', [
          { msg: 'Pulling image ' + parsed.params.image + ':latest...', delay: 600 },
          { msg: 'Creating deployment manifest...', delay: 400 },
          { msg: 'Applying to namespace ' + parsed.params.target + '...', delay: 800 },
          { msg: 'Waiting for rollout...', delay: 1200 },
          { msg: '✅ Deployment successful. 3/3 pods running.', delay: 0 }
        ]);

      case 'scale':
        return simulateExecution('📈 Scaling <strong>' + parsed.params.deployment + '</strong> to <strong>' + parsed.params.replicas + '</strong> replicas...', [
          { msg: 'Updating replica count...', delay: 400 },
          { msg: 'Scheduling ' + parsed.params.replicas + ' pods...', delay: 800 },
          { msg: '✅ Scale complete. ' + parsed.params.replicas + '/' + parsed.params.replicas + ' pods ready.', delay: 0 }
        ]);

      case 'restart':
        return simulateExecution('🔄 Restarting ' + parsed.params.resourceType + ' <strong>' + parsed.params.name + '</strong>...', [
          { msg: 'Sending restart signal...', delay: 500 },
          { msg: 'Terminating existing pods...', delay: 700 },
          { msg: 'Scheduling new pods...', delay: 900 },
          { msg: '✅ Restart complete. All pods healthy.', delay: 0 }
        ]);

      case 'query_failed_deployments':
        return showResult('📋 <strong>Failed Deployments (' + parsed.params.timeframe + ')</strong>', [
          '<tr><td>payment-api</td><td>production</td><td style="color:#ef4444;">ImagePullBackOff</td><td>2h ago</td></tr>',
          '<tr><td>analytics-worker</td><td>staging</td><td style="color:#ef4444;">CrashLoopBackOff</td><td>5h ago</td></tr>'
        ].join(''), true);

      case 'query_oomkilled':
        return showResult('📋 <strong>OOMKilled Pods</strong>', [
          '<tr><td>ml-worker-4c5d</td><td>ml-jobs</td><td>512Mi limit exceeded</td><td>30m ago</td></tr>',
          '<tr><td>cache-warmer-2a3b</td><td>staging</td><td>256Mi limit exceeded</td><td>2h ago</td></tr>',
          '<tr><td>data-proc-8e9f</td><td>production</td><td>1Gi limit exceeded</td><td>6h ago</td></tr>'
        ].join(''), true);

      case 'cluster_health':
        return showResult('💓 <strong>Cluster Health: ' + parsed.params.cluster + '</strong>',
          '<div style="display:grid;grid-template-columns:1fr 1fr;gap:8px;margin-top:8px;">'
          + '<div style="padding:8px;background:var(--color-surface);border-radius:6px;border-left:3px solid #10b981;"><strong>prod-cluster-01</strong><br><span style="color:#10b981;">● Healthy</span> — 12 nodes, 89 pods</div>'
          + '<div style="padding:8px;background:var(--color-surface);border-radius:6px;border-left:3px solid #eab308;"><strong>staging-cluster</strong><br><span style="color:#eab308;">● Warning</span> — 4 nodes, 23 pods</div>'
          + '<div style="padding:8px;background:var(--color-surface);border-radius:6px;border-left:3px solid #10b981;"><strong>dev-cluster</strong><br><span style="color:#10b981;">● Healthy</span> — 2 nodes, 8 pods</div>'
          + '</div>', false);

      case 'rollback':
        return simulateExecution('⏪ Rolling back <strong>' + parsed.params.deployment + '</strong>...', [
          { msg: 'Fetching previous revision...', delay: 400 },
          { msg: 'Applying revision 3 → 2...', delay: 800 },
          { msg: 'Waiting for rollout...', delay: 1000 },
          { msg: '✅ Rollback complete. Running on previous stable version.', delay: 0 }
        ]);

      case 'delete':
        return showResult('⚠️ <strong>Confirm Deletion</strong>',
          '<p>Are you sure you want to delete ' + parsed.params.resourceType + ' <code>' + parsed.params.name + '</code>?</p>'
          + '<div style="display:flex;gap:8px;margin-top:8px;">'
          + '<button class="btn btn-primary btn-sm" style="background:#ef4444;" onclick="AICopilot.appendOutput(\'✅ ' + parsed.params.resourceType + ' ' + parsed.params.name + ' deleted.\')">Confirm Delete</button>'
          + '<button class="btn btn-ghost btn-sm" onclick="AICopilot.appendOutput(\'❌ Deletion cancelled.\')">Cancel</button>'
          + '</div>', false);

      case 'view_logs':
        return showResult('📋 <strong>Logs: ' + parsed.params.resource + '</strong>',
          '<pre style="background:#0d1117;color:#39eb0a;padding:8px;border-radius:4px;font-size:11px;max-height:150px;overflow-y:auto;">'
          + '[2026-06-25 12:00:01] INFO  Starting application server...\n'
          + '[2026-06-25 12:00:02] INFO  Connected to database at postgres:5432\n'
          + '[2026-06-25 12:00:02] INFO  Redis cache initialized\n'
          + '[2026-06-25 12:00:03] INFO  HTTP server listening on :8080\n'
          + '[2026-06-25 12:00:15] WARN  High memory usage detected: 85%\n'
          + '[2026-06-25 12:01:22] INFO  Health check passed\n'
          + '</pre>', false);

      case 'system_status':
        return showResult('📊 <strong>System Status</strong>',
          '<div style="display:grid;grid-template-columns:repeat(4,1fr);gap:8px;margin-top:8px;">'
          + '<div style="text-align:center;padding:8px;background:var(--color-surface);border-radius:6px;"><div style="font-size:20px;font-weight:700;color:#10b981;">3</div><div style="font-size:11px;color:var(--color-muted);">Clusters</div></div>'
          + '<div style="text-align:center;padding:8px;background:var(--color-surface);border-radius:6px;"><div style="font-size:20px;font-weight:700;color:#06b6d4;">89</div><div style="font-size:11px;color:var(--color-muted);">Pods</div></div>'
          + '<div style="text-align:center;padding:8px;background:var(--color-surface);border-radius:6px;"><div style="font-size:20px;font-weight:700;color:#eab308;">2</div><div style="font-size:11px;color:var(--color-muted);">Warnings</div></div>'
          + '<div style="text-align:center;padding:8px;background:var(--color-surface);border-radius:6px;"><div style="font-size:20px;font-weight:700;color:#ef4444;">0</div><div style="font-size:11px;color:var(--color-muted);">Critical</div></div>'
          + '</div>', false);

      case 'help':
        return showResult('💡 <strong>AI Copilot Commands</strong>',
          '<div style="font-size:12px;line-height:1.8;margin-top:4px;">'
          + suggestions.map(function(s){ return '<code style="background:var(--color-surface);padding:2px 6px;border-radius:3px;cursor:pointer;" onclick="AICopilot.runCommand(\'' + s + '\')">' + s + '</code>'; }).join('<br>')
          + '</div>', false);

      default:
        return showResult('🤔 <strong>Unknown Command</strong>',
          '<p style="font-size:12px;">I didn\'t understand: <code>' + parsed.raw + '</code></p>'
          + '<p style="font-size:12px;color:var(--color-muted);">Try <code onclick="AICopilot.runCommand(\'help\')" style="cursor:pointer;color:var(--color-primary);">help</code> to see available commands.</p>', false);
    }
  }

  function simulateExecution(header, steps) {
    appendOutput(header);
    var totalDelay = 0;
    steps.forEach(function(step) {
      totalDelay += step.delay;
      setTimeout(function() {
        appendOutput('<span style="color:var(--color-muted);font-size:12px;">  › ' + step.msg + '</span>');
      }, totalDelay);
    });
  }

  function showResult(header, bodyHtml, isTable) {
    var html = '<div style="margin-top:4px;">' + header + '</div>';
    if (isTable) {
      html += '<table class="data-table" style="font-size:12px;margin-top:8px;"><tbody>' + bodyHtml + '</tbody></table>';
    } else {
      html += bodyHtml;
    }
    appendOutput(html);
  }

  function appendOutput(html) {
    if (outputEl) {
      var div = document.createElement('div');
      div.className = 'copilot-output-entry';
      div.innerHTML = html;
      outputEl.appendChild(div);
      outputEl.scrollTop = outputEl.scrollHeight;
    }
    if (pageOutputEl) {
      var div = document.createElement('div');
      div.className = 'copilot-output-entry';
      div.innerHTML = html;
      pageOutputEl.appendChild(div);
      pageOutputEl.scrollTop = pageOutputEl.scrollHeight;
    }
  }

  function renderPageSuggestions() {
    if (!pageSuggestionsEl) return;
    pageSuggestionsEl.innerHTML = suggestions.map(function(s) {
      return '<div class="copilot-page-suggestion-item" data-cmd="' + s + '">' + s + '</div>';
    }).join('');

    pageSuggestionsEl.querySelectorAll('.copilot-page-suggestion-item').forEach(function(item) {
      item.addEventListener('click', function() {
        var cmd = this.dataset.cmd;
        AICopilot.runCommand(cmd);
      });
    });
  }

  /* ─── UI Rendering ─── */
  function createCopilotUI() {
    copilotEl = document.createElement('div');
    copilotEl.id = 'ai-copilot-overlay';
    copilotEl.className = 'copilot-overlay';
    copilotEl.style.display = 'none';
    copilotEl.innerHTML = ''
      + '<div class="copilot-panel">'
      + '  <div class="copilot-header">'
      + '    <span class="copilot-title">🤖 AI Copilot</span>'
      + '    <span class="copilot-shortcut">Ctrl+Shift+K</span>'
      + '    <button class="btn btn-ghost btn-sm copilot-close" onclick="AICopilot.close()">✕</button>'
      + '  </div>'
      + '  <div class="copilot-output" id="copilot-output">'
      + '    <div class="copilot-output-entry" style="color:var(--color-muted);font-size:12px;">Welcome to AI Copilot. Type a command or <code style="cursor:pointer;color:var(--color-primary);" onclick="AICopilot.runCommand(\'help\')">help</code> to get started.</div>'
      + '  </div>'
      + '  <div class="copilot-input-area">'
      + '    <span class="copilot-prompt">›</span>'
      + '    <input type="text" class="copilot-input" id="copilot-input" placeholder="Ask anything... e.g. deploy nginx to production" autocomplete="off">'
      + '  </div>'
      + '  <div class="copilot-suggestions" id="copilot-suggestions"></div>'
      + '</div>';

    document.body.appendChild(copilotEl);

    inputEl = document.getElementById('copilot-input');
    outputEl = document.getElementById('copilot-output');

    inputEl.addEventListener('keydown', function(e) {
      if (e.key === 'Enter' && inputEl.value.trim()) {
        var cmd = inputEl.value.trim();
        commandHistory.unshift(cmd);
        historyIndex = -1;
        appendOutput('<div class="copilot-cmd-echo"><span class="copilot-prompt">›</span> ' + cmd + '</div>');
        var parsed = parseIntent(cmd);
        executeIntent(parsed);
        inputEl.value = '';
        updateSuggestions('');
      } else if (e.key === 'Escape') {
        AICopilot.close();
      } else if (e.key === 'ArrowUp') {
        e.preventDefault();
        if (historyIndex < commandHistory.length - 1) {
          historyIndex++;
          inputEl.value = commandHistory[historyIndex];
        }
      } else if (e.key === 'ArrowDown') {
        e.preventDefault();
        if (historyIndex > 0) {
          historyIndex--;
          inputEl.value = commandHistory[historyIndex];
        } else {
          historyIndex = -1;
          inputEl.value = '';
        }
      }
    });

    inputEl.addEventListener('input', function() {
      updateSuggestions(inputEl.value);
    });

    copilotEl.addEventListener('click', function(e) {
      if (e.target === copilotEl) AICopilot.close();
    });

    // Cache page-specific elements
    pageInputEl = document.getElementById('page-copilot-input');
    pageOutputEl = document.getElementById('page-copilot-output');
    pageSuggestionsEl = document.getElementById('page-copilot-suggestions');

    if (pageInputEl) {
      pageInputEl.addEventListener('keydown', function(e) {
        if (e.key === 'Enter' && pageInputEl.value.trim()) {
          var cmd = pageInputEl.value.trim();
          commandHistory.unshift(cmd);
          historyIndex = -1;
          appendOutput('<div class="copilot-cmd-echo"><span class="copilot-prompt">›</span> ' + cmd + '</div>');
          var parsed = parseIntent(cmd);
          executeIntent(parsed);
          pageInputEl.value = '';
        } else if (e.key === 'ArrowUp') {
          e.preventDefault();
          if (historyIndex < commandHistory.length - 1) {
            historyIndex++;
            pageInputEl.value = commandHistory[historyIndex];
          }
        } else if (e.key === 'ArrowDown') {
          e.preventDefault();
          if (historyIndex > 0) {
            historyIndex--;
            pageInputEl.value = commandHistory[historyIndex];
          } else {
            historyIndex = -1;
            pageInputEl.value = '';
          }
        }
      });
    }

    if (pageSuggestionsEl) {
      renderPageSuggestions();
    }
  }

  function updateSuggestions(query) {
    var sugEl = document.getElementById('copilot-suggestions');
    if (!sugEl) return;
    if (!query) { sugEl.innerHTML = ''; return; }
    var q = query.toLowerCase();
    var matches = suggestions.filter(function(s) { return s.toLowerCase().indexOf(q) !== -1; }).slice(0, 5);
    sugEl.innerHTML = matches.map(function(s) {
      return '<div class="copilot-suggestion" onclick="AICopilot.runCommand(\'' + s + '\')">' + s + '</div>';
    }).join('');
  }

  /* ─── Public API ─── */
  var AICopilot = {
    init: function() {
      createCopilotUI();
      document.addEventListener('keydown', function(e) {
        if (e.ctrlKey && e.shiftKey && e.key === 'K') {
          e.preventDefault();
          AICopilot.toggle();
        }
      });
    },
    toggle: function() {
      if (isOpen) this.close(); else this.open();
    },
    open: function() {
      if (!copilotEl) return;
      copilotEl.style.display = 'flex';
      isOpen = true;
      setTimeout(function() { inputEl && inputEl.focus(); }, 100);
    },
    close: function() {
      if (!copilotEl) return;
      copilotEl.style.display = 'none';
      isOpen = false;
    },
    runCommand: function(cmd) {
      if (inputEl) inputEl.value = cmd;
      commandHistory.unshift(cmd);
      historyIndex = -1;
      appendOutput('<div class="copilot-cmd-echo"><span class="copilot-prompt">›</span> ' + cmd + '</div>');
      var parsed = parseIntent(cmd);
      executeIntent(parsed);
      if (inputEl) inputEl.value = '';
      updateSuggestions('');
    },
    appendOutput: appendOutput
  };

  global.AICopilot = AICopilot;
})(window);
