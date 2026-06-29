/**
 * Agent Pipeline — Visualizes execution steps in real time.
 * Steps: TASK → LLM → CODE → TEST → FIX → COMMIT → PR
 */
(function (global) {
  'use strict';

  const STEPS = ['TASK', 'LLM', 'CODE', 'TEST', 'FIX', 'COMMIT', 'PR'];
  const stepEls = {};
  let runCount = 0;

  function init() {
    STEPS.forEach(step => {
      stepEls[step] = document.querySelector('[data-step="' + step + '"]');
    });

    AppState.on('agent', handleAgentEvent);
  }

  function handleAgentEvent(evt) {
    if (!evt || !evt.step) return;

    const step = evt.step.toUpperCase();
    const status = evt.status || 'running';
    const duration = evt.duration || 0;

    // Reset all steps if we're starting a new pipeline at TASK
    if (step === 'TASK' && status === 'running') {
      resetSteps();
      runCount++;
      updateDetail('detail-runs', runCount);
      updateEl('agent-run-count', runCount);
    }

    // Mark preceding steps as completed
    const stepIndex = STEPS.indexOf(step);
    if (stepIndex > 0) {
      for (let i = 0; i < stepIndex; i++) {
        setStepState(STEPS[i], 'completed');
      }
    }

    // Set current step state
    setStepState(step, status);

    // Update duration
    if (duration > 0) {
      const durEl = document.getElementById('dur-' + step);
      if (durEl) durEl.textContent = formatDuration(duration);
    }

    // Update detail panel
    updateDetail('detail-step', step);
    updateDetail('detail-status', capitalise(status));
    updateDetail('detail-duration', duration > 0 ? formatDuration(duration) : '—');

    // Update status label
    const statusLabel = document.getElementById('agent-status-label');
    if (statusLabel) {
      statusLabel.textContent = step + ' — ' + capitalise(status);
      statusLabel.style.color = statusColor(status);
    }
  }

  function setStepState(step, status) {
    const el = stepEls[step];
    if (!el) return;
    el.className = 'pipeline-step ' + status;
  }

  function resetSteps() {
    STEPS.forEach(step => {
      setStepState(step, '');
      const durEl = document.getElementById('dur-' + step);
      if (durEl) durEl.textContent = '—';
    });
  }

  function formatDuration(ms) {
    if (ms < 1000) return ms + 'ms';
    if (ms < 60000) return (ms / 1000).toFixed(1) + 's';
    return (ms / 60000).toFixed(1) + 'm';
  }

  function capitalise(str) {
    return str.charAt(0).toUpperCase() + str.slice(1);
  }

  function statusColor(status) {
    switch (status) {
      case 'running': return 'var(--color-primary)';
      case 'success': return 'var(--color-trading-up)';
      case 'failed': return 'var(--color-trading-down)';
      default: return 'var(--color-muted)';
    }
  }

  function updateDetail(id, value) {
    const el = document.getElementById(id);
    if (el) el.textContent = value;
  }

  function updateEl(id, value) {
    const el = document.getElementById(id);
    if (el) el.textContent = value;
  }

  global.AgentsPanel = { init };

})(window);
