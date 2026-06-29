/**
 * State Manager — Extended for Enterprise Configuration Control Plane.
 * Covers: incidents, agents, logs, metrics + kubernetes, gitProviders, cicd, aiProviders,
 * connectionHealth, auditLogs.
 */
(function (global) {
  'use strict';

  var LIMITS = { incidents: 50, logs: 200, auditLogs: 200 };

  var state = {
    // Operational
    incidents: [],
    agents: [],
    logs: [],
    metrics: { cpu: [], memory: [], incidentRate: [], podCount: 0 },
    stats: { critical: 0, warning: 0, resolved: 0, agentRuns: 0 },
    pipeline: { steps: {}, currentStep: null, status: 'idle' },
    connection: 'connecting',

    // Enterprise Config
    kubernetes: [],
    gitProviders: [],
    cicd: [],
    aiProviders: [],
    connectionHealth: {},
    auditLogs: [],
    settings: {}
  };

  var listeners = {};

  // Deep comparison helper for state comparison
  function isEqual(a, b) {
    if (a === b) return true;
    if (typeof a !== typeof b) return false;
    if (a && b && typeof a === 'object') {
      try {
        return JSON.stringify(a) === JSON.stringify(b);
      } catch (e) {
        return false;
      }
    }
    return false;
  }

  function on(event, callback) {
    if (!listeners[event]) listeners[event] = [];
    listeners[event].push(callback);
    return function unsubscribe() {
      off(event, callback);
    };
  }

  function off(event, callback) {
    if (listeners[event]) {
      listeners[event] = listeners[event].filter(function (cb) {
        return cb !== callback;
      });
    }
  }

  function emit(event, data) {
    if (listeners[event]) {
      listeners[event].forEach(function (cb) {
        try { cb(data); } catch (e) { console.error('State listener error:', e); }
      });
    }
  }

  // ── Operational state ──

  function addIncident(incident) {
    // Avoid duplicate incident triggers if duplicate incident id & status matches
    var isDuplicate = state.incidents.some(function (inc) {
      return inc.id === incident.id && inc.status === incident.status;
    });
    if (isDuplicate) return;

    state.incidents.unshift(incident);
    if (state.incidents.length > LIMITS.incidents) state.incidents.pop();
    if (incident.severity === 'critical') state.stats.critical++;
    else if (incident.severity === 'warning') state.stats.warning++;
    emit('incident', incident);
    emit('stats', state.stats);
  }

  function addLog(log) {
    // De-duplicate immediately repeated logs to prevent visual flashing/lags
    if (state.logs.length > 0) {
      var last = state.logs[state.logs.length - 1];
      if (last.message === log.message && last.pod === log.pod && last.stream === log.stream) {
        return;
      }
    }
    state.logs.push(log);
    if (state.logs.length > LIMITS.logs) state.logs.shift();
    emit('log', log);
  }

  function updateAgent(agentEvent) {
    if (state.agents.length > 0) {
      var lastAgent = state.agents[state.agents.length - 1];
      if (isEqual(lastAgent, agentEvent)) return;
    }
    state.pipeline.currentStep = agentEvent.step;
    state.pipeline.status = agentEvent.status;
    state.pipeline.steps[agentEvent.step] = { status: agentEvent.status, duration: agentEvent.duration || 0 };
    if (agentEvent.status === 'success' && agentEvent.step === 'PR') {
      state.stats.agentRuns++;
      state.stats.resolved++;
      emit('stats', state.stats);
    }
    state.agents.push(agentEvent);
    emit('agent', agentEvent);
  }

  function updateMetrics(metricsData) {
    var changed = false;
    if (metricsData.cpu !== undefined) {
      state.metrics.cpu.push({ value: metricsData.cpu, time: Date.now() });
      if (state.metrics.cpu.length > 30) state.metrics.cpu.shift();
      changed = true;
    }
    if (metricsData.memory !== undefined) {
      state.metrics.memory.push({ value: metricsData.memory, time: Date.now() });
      if (state.metrics.memory.length > 30) state.metrics.memory.shift();
      changed = true;
    }
    if (metricsData.podCount !== undefined && state.metrics.podCount !== metricsData.podCount) {
      state.metrics.podCount = metricsData.podCount;
      changed = true;
    }
    if (changed) {
      emit('metrics', state.metrics);
    }
  }

  // ── Connection status ──

  function setConnection(status) {
    if (state.connection === status) return;
    state.connection = status;
    emit('connection', status);
  }

  function resetPipeline() {
    var defaultPipeline = { steps: {}, currentStep: null, status: 'idle' };
    if (isEqual(state.pipeline, defaultPipeline)) return;
    state.pipeline = defaultPipeline;
    emit('agent', { step: null, status: 'idle' });
  }

  // Helper to securely sanitize state data before storing it
  function sanitize(val) {
    if (global.Security && typeof global.Security.sanitizeObject === 'function') {
      return global.Security.sanitizeObject(val);
    }
    return val;
  }

  // ── Enterprise Config state ──

  function setKubernetes(clusters) {
    var val = sanitize(clusters || []);
    if (isEqual(state.kubernetes, val)) return;
    state.kubernetes = val;
    emit('kubernetes', state.kubernetes);
  }

  // Set git providers
  function setGitProviders(providers) {
    var val = sanitize(providers || []);
    if (isEqual(state.gitProviders, val)) return;
    state.gitProviders = val;
    emit('gitProviders', state.gitProviders);
  }

  // Set CI/CD pipelines
  function setCicd(pipelines) {
    var val = sanitize(pipelines || []);
    if (isEqual(state.cicd, val)) return;
    state.cicd = val;
    emit('cicd', state.cicd);
  }

  // Set AI providers
  function setAiProviders(providers) {
    var val = sanitize(providers || []);
    if (isEqual(state.aiProviders, val)) return;
    state.aiProviders = val;
    emit('aiProviders', state.aiProviders);
  }

  function setConnectionHealth(health) {
    var val = sanitize(health || {});
    if (isEqual(state.connectionHealth, val)) return;
    state.connectionHealth = val;
    emit('connectionHealth', state.connectionHealth);
  }

  function addAuditLog(entry) {
    var cleanEntry = sanitize(entry || {});
    cleanEntry.timestamp = cleanEntry.timestamp || new Date().toISOString();
    cleanEntry.actor = cleanEntry.actor || 'admin';
    state.auditLogs.unshift(cleanEntry);
    if (state.auditLogs.length > LIMITS.auditLogs) state.auditLogs.pop();
    emit('auditLog', cleanEntry);
  }

  function getState() { return state; }

  global.AppState = {
    on: on, off: off, emit: emit, getState: getState,
    addIncident: addIncident, addLog: addLog, updateAgent: updateAgent, updateMetrics: updateMetrics,
    setConnection: setConnection, resetPipeline: resetPipeline,
    setKubernetes: setKubernetes, setGitProviders: setGitProviders,
    setCicd: setCicd, setAiProviders: setAiProviders,
    setConnectionHealth: setConnectionHealth, addAuditLog: addAuditLog,
    setSettings: function(s) {
      var val = sanitize(s || {});
      if (isEqual(state.settings, val)) return;
      state.settings = val;
      emit('settings', state.settings);
    }
  };

})(window);
