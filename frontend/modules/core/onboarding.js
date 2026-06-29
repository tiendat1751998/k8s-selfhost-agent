/**
 * Onboarding Guided Tour Module
 * Renders step-by-step tooltips pointing to critical platform features.
 */
(function (global) {
  'use strict';

  const COMPLETED_KEY = 'k8s_onboarding_completed';

  const steps = [
    {
      title: '👋 Welcome to K8S Control Plane!',
      content: 'Let\'s take a quick 1-minute guided tour of the key features of the Enterprise Edition.',
      target: null // Centered modal
    },
    {
      title: '🔍 Universal Command Palette',
      content: 'Press <strong>Ctrl + K</strong> from anywhere or click here to open the Command Palette. Search for clusters, resources, or trigger AI operations.',
      target: '#global-search-input',
      placement: 'bottom'
    },
    {
      title: '🔔 Notification Center',
      content: 'Real-time alerts, warning indicators, and AI-driven recommendations are queued here. Click the bell to view the notifications inbox.',
      target: '#notif-bell',
      placement: 'bottom'
    },
    {
      title: '⚙️ Customize Dashboard',
      content: 'Tailor your overview dashboard widgets and layout. Hide panels or save favorite views to match your workflow.',
      target: '#overview-customize-btn',
      placement: 'bottom'
    },
    {
      title: '🎉 Tour Complete!',
      content: 'You\'re ready to manage your self-healing Kubernetes fleet. You can restart this tour anytime from the top bar.',
      target: null // Centered modal
    }
  ];

  let currentStep = 0;
  let overlayEl = null;
  let tooltipEl = null;
  let activeTarget = null;
  let originalStyles = {};

  function init() {
    createOverlayAndTooltip();

    // Check if user has completed onboarding
    const completed = localStorage.getItem(COMPLETED_KEY);
    if (!completed) {
      // Delay slightly to allow other modules to finish layout rendering
      setTimeout(() => {
        startTour();
      }, 1000);
    }
  }

  function createOverlayAndTooltip() {
    if (document.getElementById('tour-overlay')) return;

    // 1. Create Overlay
    overlayEl = document.createElement('div');
    overlayEl.id = 'tour-overlay';
    overlayEl.style.cssText = 'position:fixed;top:0;left:0;width:100%;height:100%;background:rgba(0,0,0,0.6);z-index:999990;display:none;transition:opacity 0.3s;';
    document.body.appendChild(overlayEl);

    // 2. Create Tooltip Container
    tooltipEl = document.createElement('div');
    tooltipEl.id = 'tour-tooltip';
    tooltipEl.style.cssText = 'position:absolute;background:var(--color-bg);border:1px solid var(--color-hairline);border-radius:12px;box-shadow:0 20px 50px rgba(0,0,0,0.6);z-index:999995;width:320px;padding:var(--space-md);display:none;flex-direction:column;gap:12px;color:var(--color-text);font-family:inherit;';
    document.body.appendChild(tooltipEl);
  }

  function startTour() {
    currentStep = 0;
    overlayEl.style.display = 'block';
    showStep(currentStep);
  }

  function showStep(index) {
    if (index < 0 || index >= steps.length) {
      endTour();
      return;
    }

    currentStep = index;
    const step = steps[index];

    // Reset previous target styling
    resetActiveTarget();

    // Set tooltip content
    const isFirst = index === 0;
    const isLast = index === steps.length - 1;
    const nextText = isLast ? 'Done' : 'Next';

    tooltipEl.innerHTML = `
      <div style="font-weight:700;font-size:16px;color:var(--color-on-dark);display:flex;align-items:center;justify-content:between;">
        <span>${step.title}</span>
        <span style="font-size:11px;color:var(--color-muted);margin-left:auto;">${index + 1}/${steps.length}</span>
      </div>
      <div style="font-size:13.5px;line-height:1.5;color:var(--color-text-secondary);">${step.content}</div>
      <div style="display:flex;justify-content:space-between;align-items:center;margin-top:4px;">
        <button class="btn btn-ghost btn-sm" id="tour-btn-skip" style="font-size:12px;padding:4px 8px;">Skip</button>
        <div style="display:flex;gap:6px;margin-left:auto;">
          ${!isFirst ? '<button class="btn btn-ghost btn-sm" id="tour-btn-back" style="font-size:12px;padding:4px 8px;">Back</button>' : ''}
          <button class="btn btn-primary btn-sm" id="tour-btn-next" style="font-size:12px;padding:4px 12px;font-weight:600;">${nextText}</button>
        </div>
      </div>
    `;

    // Wire buttons
    document.getElementById('tour-btn-skip').addEventListener('click', endTour);
    if (!isFirst) {
      document.getElementById('tour-btn-back').addEventListener('click', () => showStep(currentStep - 1));
    }
    document.getElementById('tour-btn-next').addEventListener('click', () => {
      if (isLast) {
        endTour();
      } else {
        showStep(currentStep + 1);
      }
    });

    // Handle target highlight and placement
    if (step.target) {
      const target = document.querySelector(step.target);
      if (target && target.offsetWidth > 0 && target.offsetHeight > 0) {
        activeTarget = target;
        highlightTarget(target);
        positionTooltip(target, step.placement || 'bottom');
        return;
      }
    }

    // Default centered placement (modal style)
    positionCenter();
  }

  function highlightTarget(target) {
    // Save original styles
    originalStyles = {
      position: target.style.position,
      zIndex: target.style.zIndex,
      outline: target.style.outline,
      boxShadow: target.style.boxShadow,
      borderRadius: target.style.borderRadius
    };

    // Apply highlight styles
    target.style.position = 'relative';
    target.style.zIndex = '999992';
    target.style.outline = '4px solid var(--color-primary)';
    target.style.boxShadow = '0 0 25px var(--color-primary)';
    target.style.borderRadius = '4px';
  }

  function resetActiveTarget() {
    if (activeTarget && originalStyles) {
      activeTarget.style.position = originalStyles.position;
      activeTarget.style.zIndex = originalStyles.zIndex;
      activeTarget.style.outline = originalStyles.outline;
      activeTarget.style.boxShadow = originalStyles.boxShadow;
      activeTarget.style.borderRadius = originalStyles.borderRadius;
    }
    activeTarget = null;
    originalStyles = {};
  }

  function positionTooltip(target, placement) {
    tooltipEl.style.display = 'flex';
    const rect = target.getBoundingClientRect();
    const tooltipWidth = tooltipEl.offsetWidth;
    const tooltipHeight = tooltipEl.offsetHeight;

    let top = 0;
    let left = 0;

    switch (placement) {
      case 'bottom':
        top = rect.bottom + window.scrollY + 12;
        left = rect.left + window.scrollX + (rect.width - tooltipWidth) / 2;
        break;
      case 'top':
        top = rect.top + window.scrollY - tooltipHeight - 12;
        left = rect.left + window.scrollX + (rect.width - tooltipWidth) / 2;
        break;
      case 'left':
        top = rect.top + window.scrollY + (rect.height - tooltipHeight) / 2;
        left = rect.left + window.scrollX - tooltipWidth - 12;
        break;
      case 'right':
        top = rect.top + window.scrollY + (rect.height - tooltipHeight) / 2;
        left = rect.right + window.scrollX + 12;
        break;
    }

    // Boundary check
    if (left < 10) left = 10;
    if (left + tooltipWidth > window.innerWidth - 10) {
      left = window.innerWidth - tooltipWidth - 10;
    }

    tooltipEl.style.top = top + 'px';
    tooltipEl.style.left = left + 'px';
  }

  function positionCenter() {
    tooltipEl.style.display = 'flex';
    const tooltipWidth = tooltipEl.offsetWidth;
    const tooltipHeight = tooltipEl.offsetHeight;

    const top = (window.innerHeight - tooltipHeight) / 2 + window.scrollY;
    const left = (window.innerWidth - tooltipWidth) / 2 + window.scrollX;

    tooltipEl.style.top = top + 'px';
    tooltipEl.style.left = left + 'px';
  }

  function endTour() {
    resetActiveTarget();
    if (overlayEl) overlayEl.style.display = 'none';
    if (tooltipEl) tooltipEl.style.display = 'none';
    localStorage.setItem(COMPLETED_KEY, 'true');
  }

  // Register load
  document.addEventListener('DOMContentLoaded', init);

  // Expose Global API
  global.Onboarding = {
    start: startTour,
    reset: () => {
      localStorage.removeItem(COMPLETED_KEY);
      startTour();
    }
  };
})(window);
