/**
 * Enterprise Release Notes Module
 * Implements a dynamic changelog modal and version viewer.
 */
(function (global) {
  'use strict';

  var CURRENT_VERSION = 'v1.2.0';
  var STORAGE_KEY = 'k8s_last_read_version';

  var CHANGELOG = [
    {
      version: 'v1.2.0',
      date: '2026-06-27',
      items: [
        '<strong>Feature Flags Manager:</strong> Added toggles in global settings to enable/disable Swarm, AI Ops, Cost, and Compliance features dynamically.',
        '<strong>Release Notes Center:</strong> Integrated top-bar changelog timeline with unread version highlight badges.',
        '<strong>Security Impersonation:</strong> Enhanced Kubernetes API clients to apply secure impersonated credentials dynamically on a per-request thread-safe basis.'
      ]
    },
    {
      version: 'v1.1.0',
      date: '2026-06-26',
      items: [
        '<strong>Guided Onboarding Tour:</strong> Built an interactive onboarding guide highlighting the control plane features for new users.',
        '<strong>Multi-Tenancy Isolation:</strong> Restricted SQL data retrieval in PG fleet and runbook repositories by JWT tenant context claims.',
        '<strong>WebSocket Security:</strong> Enforced authentication checks on WebSocket connections using query string JWT fallbacks.'
      ]
    },
    {
      version: 'v1.0.0',
      date: '2026-06-25',
      items: [
        '<strong>Enterprise Control Plane:</strong> Initial release of the next-generation multi-cluster management control plane.',
        '<strong>AI Operations Copilot:</strong> Integrated AI Copilot and Root Cause Analysis engine to assist operators with diagnosis.',
        '<strong>Topology Map:</strong> Interactive visualization mapping deployment dependencies and node topologies.'
      ]
    }
  ];

  function init() {
    injectStyles();
    bindEvents();
    checkUnread();
  }

  function injectStyles() {
    if (document.getElementById('release-notes-styles')) return;

    var style = document.createElement('style');
    style.id = 'release-notes-styles';
    style.textContent = `
      .release-notes-modal-overlay {
        position: fixed;
        top: 0;
        left: 0;
        right: 0;
        bottom: 0;
        background: rgba(11, 14, 17, 0.85);
        backdrop-filter: blur(4px);
        z-index: 2000;
        display: flex;
        align-items: center;
        justify-content: center;
        animation: rn-fadeIn 0.2s ease;
      }
      .release-notes-modal {
        background: var(--color-surface-card);
        border: 1px solid var(--color-hairline);
        border-radius: var(--rounded-lg);
        width: 100%;
        max-width: 580px;
        max-height: 80vh;
        display: flex;
        flex-direction: column;
        box-shadow: 0 20px 40px rgba(0,0,0,0.5);
        animation: rn-slideUp 0.3s cubic-bezier(0.16, 1, 0.3, 1);
        overflow: hidden;
      }
      .release-notes-header {
        padding: var(--space-md) var(--space-lg);
        border-bottom: 1px solid var(--color-hairline);
        display: flex;
        justify-content: space-between;
        align-items: center;
        background: var(--color-surface-elevated);
      }
      .release-notes-title {
        font-size: var(--text-title-sm);
        font-weight: 700;
        color: var(--color-on-dark);
        display: flex;
        align-items: center;
        gap: var(--space-xs);
        margin: 0;
      }
      .release-notes-close {
        background: transparent;
        border: none;
        color: var(--color-muted);
        font-size: 20px;
        cursor: pointer;
        transition: color 0.2s;
        padding: 4px;
        line-height: 1;
      }
      .release-notes-close:hover {
        color: var(--color-on-dark);
      }
      .release-notes-body {
        padding: var(--space-lg);
        overflow-y: auto;
        flex: 1;
        display: flex;
        flex-direction: column;
        gap: var(--space-lg);
      }
      .release-version-section {
        display: flex;
        flex-direction: column;
        gap: var(--space-sm);
        border-bottom: 1px solid var(--color-hairline);
        padding-bottom: var(--space-md);
      }
      .release-version-section:last-child {
        border-bottom: none;
        padding-bottom: 0;
      }
      .release-version-header {
        display: flex;
        align-items: center;
        justify-content: space-between;
      }
      .release-version-tag {
        background: rgba(252, 213, 53, 0.1);
        color: var(--color-primary);
        border: 1px solid rgba(252, 213, 53, 0.2);
        padding: 2px 8px;
        border-radius: var(--rounded-md);
        font-size: var(--text-body-xs);
        font-weight: 700;
        font-family: var(--font-number);
      }
      .release-version-date {
        font-size: var(--text-body-xs);
        color: var(--color-muted);
      }
      .release-notes-list {
        margin: 0;
        padding-left: var(--space-lg);
        color: var(--color-muted-strong);
        font-size: var(--text-body-sm);
        display: flex;
        flex-direction: column;
        gap: var(--space-xs);
      }
      .release-note-item {
        line-height: 1.5;
      }
      .release-note-item strong {
        color: var(--color-on-dark);
      }
      @keyframes rn-fadeIn {
        from { opacity: 0; }
        to { opacity: 1; }
      }
      @keyframes rn-slideUp {
        from { transform: translateY(16px); opacity: 0; }
        to { transform: translateY(0); opacity: 1; }
      }
    `;
    document.head.appendChild(style);
  }

  function bindEvents() {
    var btn = document.getElementById('release-notes-btn');
    if (btn) {
      btn.addEventListener('click', showModal);
    }
  }

  function checkUnread() {
    var lastRead = localStorage.getItem(STORAGE_KEY);
    var badge = document.getElementById('release-notes-unread-badge');
    if (badge) {
      if (lastRead !== CURRENT_VERSION) {
        badge.style.display = 'block';
      } else {
        badge.style.display = 'none';
      }
    }
  }

  function showModal() {
    // Hide unread badge
    localStorage.setItem(STORAGE_KEY, CURRENT_VERSION);
    var badge = document.getElementById('release-notes-unread-badge');
    if (badge) badge.style.display = 'none';

    // Render modal markup
    var overlay = document.createElement('div');
    overlay.className = 'release-notes-modal-overlay';
    
    var modalHtml = `
      <div class="release-notes-modal">
        <div class="release-notes-header">
          <h3 class="release-notes-title">📣 Release Notes & Updates</h3>
          <button class="release-notes-close" id="release-notes-close-btn">&times;</button>
        </div>
        <div class="release-notes-body">
    `;

    CHANGELOG.forEach(function (rel) {
      var activeLabel = (rel.version === CURRENT_VERSION) ? ' <span style="font-size:9px;background:var(--color-primary);color:#000;padding:1px 4px;border-radius:3px;margin-left:5px;font-weight:600;">ACTIVE</span>' : '';
      modalHtml += `
        <div class="release-version-section">
          <div class="release-version-header">
            <div>
              <span class="release-version-tag">${rel.version}</span>
              ${activeLabel}
            </div>
            <span class="release-version-date">${rel.date}</span>
          </div>
          <ul class="release-notes-list">
      `;

      rel.items.forEach(function (item) {
        modalHtml += `<li class="release-note-item">${item}</li>`;
      });

      modalHtml += `
          </ul>
        </div>
      `;
    });

    modalHtml += `
        </div>
      </div>
    `;

    overlay.innerHTML = modalHtml;
    document.body.appendChild(overlay);

    // Bind Close events
    var closeBtn = overlay.querySelector('#release-notes-close-btn');
    if (closeBtn) {
      closeBtn.addEventListener('click', closeModal);
    }
    
    overlay.addEventListener('click', function (e) {
      if (e.target === overlay) {
        closeModal();
      }
    });

    function closeModal() {
      overlay.style.opacity = '0';
      overlay.style.transition = 'opacity 0.2s ease';
      setTimeout(function () {
        overlay.remove();
      }, 200);
    }
  }

  // Expose module
  global.ReleaseNotes = {
    init: init,
    showModal: showModal
  };

  // Run auto init
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }

})(window);
