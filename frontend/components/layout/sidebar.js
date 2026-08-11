/**
 * Enterprise Sidebar Navigation Component
 * Replaces the static index.html sidebar with a dynamically generated,
 * collapsible, module-driven enterprise menu.
 */

class EnterpriseSidebar {
  constructor(containerId, router) {
    this.container = document.getElementById(containerId);
    this.router = router;
    
    // Define the new modular enterprise menu structure
    this.menuGroups = [
      {
        title: null, // Top level
        items: [
          { id: 'overview', label: 'Overview', icon: '📊' }
        ]
      },
      {
        title: 'Infrastructure',
        items: [
          { id: 'kubernetes', label: 'Clusters', icon: '☸️' },
          { id: 'explorer', label: 'Resource Explorer', icon: '🧭' }
        ]
      },
      {
        title: 'Operations',
        items: [
          { id: 'deployment-center', label: 'Applications', icon: '🚀' },
          { id: 'incidents', label: 'Incidents', icon: '🚨' },
          { id: 'observability', label: 'Observability', icon: '📊' },
          { id: 'agents', label: 'Agent Pipeline', icon: '🤖' },
          { id: 'connection-health', label: 'Health', icon: '💓' }
        ]
      },
      {
        title: 'GitOps',
        items: [
          { id: 'gitops', label: 'Repositories', icon: '🔀' },
          { id: 'drift', label: 'Drift Detection', icon: '⚖️' }
        ]
      },
      {
        title: null, // Bottom
        items: [
          { id: 'settings', label: 'Settings', icon: '⚙️' }
        ]
      }
    ];

    this.render();
    this.bindEvents();
  }

  render() {
    if (!this.container) return;
    
    let html = '';
    
    this.menuGroups.forEach((group, index) => {
      // Filter items based on feature flags
      const visibleItems = group.items.filter(item => {
        if (window.FeatureFlags && typeof window.FeatureFlags.isEnabled === 'function') {
          return window.FeatureFlags.isEnabled(item.id);
        }
        return true;
      });

      if (visibleItems.length === 0) {
        return; // Skip rendering this group if no items are visible
      }

      if (group.title) {
        // Collapsible Group Header with focusability and ARIA accessibility roles
        html += `
          <div class="sidebar-group">
            <div class="sidebar-group-header" data-group-index="${index}" tabindex="0" role="button" aria-expanded="true" style="padding: 10px 20px; color: var(--color-muted); font-size: 11px; text-transform: uppercase; letter-spacing: 0.5px; display: flex; justify-content: space-between; align-items: center; cursor: pointer; border-top: 1px solid var(--color-hairline); margin-top: 5px; outline: none;">
              <span class="group-title" style="font-weight: 600;">${group.title}</span>
              <span class="group-toggle" style="font-size: 8px;">▼</span>
            </div>
            <div class="sidebar-group-items" id="sidebar-group-${index}">
        `;
      }
      
      // Group Items
      visibleItems.forEach(item => {
        html += `
          <a href="#${item.id}" class="sidebar-link" data-section="${item.id}">
            <span class="sidebar-icon">${item.icon}</span>
            <span class="sidebar-text">${item.label}</span>
          </a>
        `;
      });
      
      if (group.title) {
        html += `</div></div>`;
      }
    });

    this.container.innerHTML = html;
  }

  bindEvents() {
    if (!this.container) return;

    // Toggle expand/collapse group helper
    const toggleGroup = (header) => {
      const index = header.getAttribute('data-group-index');
      const itemsContainer = document.getElementById(`sidebar-group-${index}`);
      const toggle = header.querySelector('.group-toggle');
      
      if (itemsContainer.style.display === 'none') {
        itemsContainer.style.display = 'block';
        toggle.textContent = '▼';
        header.setAttribute('aria-expanded', 'true');
      } else {
        itemsContainer.style.display = 'none';
        toggle.textContent = '▶';
        header.setAttribute('aria-expanded', 'false');
      }
    };

    // Handle group collapse/expand
    const headers = this.container.querySelectorAll('.sidebar-group-header');
    headers.forEach(header => {
      header.addEventListener('click', () => toggleGroup(header));
      // Keyboard support: Space/Enter keys
      header.addEventListener('keydown', (e) => {
        if (e.key === 'Enter' || e.key === ' ') {
          e.preventDefault();
          toggleGroup(header);
        }
      });
    });

    // Handle active route highlighting
    window.addEventListener('hashchange', () => this.updateActiveLink());
    this.updateActiveLink(); // Initial check

    // Mobile toggle support
    const toggleBtn = document.getElementById('sidebar-toggle-btn');
    const sidebar = document.getElementById('sidebar');
    if (toggleBtn && sidebar) {
      toggleBtn.addEventListener('click', (e) => {
        e.stopPropagation();
        sidebar.classList.toggle('open');
      });

      // Close sidebar when clicking outside of it (on mobile)
      document.addEventListener('click', (e) => {
        if (window.innerWidth <= 768 && sidebar.classList.contains('open')) {
          if (!sidebar.contains(e.target) && !toggleBtn.contains(e.target)) {
            sidebar.classList.remove('open');
          }
        }
      });
    }

    // Close sidebar on link click (for mobile viewports)
    const links = this.container.querySelectorAll('.sidebar-link');
    links.forEach(link => {
      link.addEventListener('click', () => {
        if (window.innerWidth <= 768 && sidebar) {
          sidebar.classList.remove('open');
        }
      });
    });
  }

  updateActiveLink() {
    if (!this.container) return;
    let hash = window.location.hash.replace('#', '') || 'overview';
    
    // Support module routing like #clusters/123 -> matches 'clusters'
    const routeParts = hash.split('/');
    const basePath = routeParts[0];

    const links = this.container.querySelectorAll('.sidebar-link');
    links.forEach(link => {
      if (link.getAttribute('data-section') === basePath) {
        link.classList.add('active');
      } else {
        link.classList.remove('active');
      }
    });
  }
}
