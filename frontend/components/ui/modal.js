/**
 * Modal — Reusable modal/drawer system for editing configs, viewing details.
 */
(function (global) {
  'use strict';

  var overlay = null;
  var modalEl = null;
  var titleEl = null;
  var bodyEl = null;
  var footerEl = null;
  var closeBtn = null;

  function init() {
    overlay = document.getElementById('modal-overlay');
    modalEl = document.getElementById('modal');
    titleEl = document.getElementById('modal-title');
    bodyEl = document.getElementById('modal-body');
    footerEl = document.getElementById('modal-footer');
    closeBtn = document.getElementById('modal-close');

    if (closeBtn) closeBtn.addEventListener('click', close);
    if (overlay) overlay.addEventListener('click', function (e) {
      if (e.target === overlay) close();
    });

    document.addEventListener('keydown', function (e) {
      if (e.key === 'Escape') close();
    });
  }

  var triggerElement = null;

  function open(options) {
    if (!overlay) return;
    triggerElement = document.activeElement; // Track the element that triggered the modal

    titleEl.textContent = options.title || 'Details';
    bodyEl.innerHTML = options.body || '';
    footerEl.innerHTML = '';

    if (options.footer) {
      footerEl.innerHTML = options.footer;
    } else if (options.actions) {
      options.actions.forEach(function (action) {
        var btn = document.createElement('button');
        btn.className = 'btn ' + (action.primary ? 'btn-primary' : 'btn-ghost') + ' btn-sm';
        btn.textContent = action.label;
        btn.addEventListener('click', function () {
          if (action.onClick) action.onClick();
          if (action.closeOnClick !== false) close();
        });
        footerEl.appendChild(btn);
      });
    }

    overlay.style.display = 'flex';
    document.body.style.overflow = 'hidden';

    // Focus close button or primary button inside the modal for keyboard accessibility
    setTimeout(function () {
      var btn = footerEl.querySelector('.btn-primary') || closeBtn;
      if (btn) btn.focus();
    }, 50);
  }

  function close() {
    if (!overlay) return;
    overlay.style.display = 'none';
    bodyEl.innerHTML = '';
    footerEl.innerHTML = '';
    document.body.style.overflow = '';

    // Restore focus to the trigger element
    if (triggerElement) {
      triggerElement.focus();
      triggerElement = null;
    }
  }

  global.Modal = { init: init, open: open, close: close };

})(window);
