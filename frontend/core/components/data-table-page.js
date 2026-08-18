(function (global) {
  'use strict';

  function createDataTablePage(config) {
    return {
      init() {
        this.container = document.getElementById(config.containerId);
        if (!this.container) return;
        this.render();
        this.bindEvents();
        if (config.onInit) config.onInit(this);
        if (config.autoLoad !== false) this.loadData();
      },

      render() {
        const statsHtml = config.stats ? `
          <div class="${config.idPrefix}-stats" style="display:flex; gap:16px; margin-bottom:24px;">
            ${config.stats.map(s => `
              <div class="stat-card" style="flex:1; background:var(--color-surface); padding:16px; border-radius:8px; border:1px solid var(--color-hairline);">
                <div class="stat-title" style="color:var(--color-muted); font-size:12px; text-transform:uppercase; margin-bottom:8px;">${s.label}</div>
                <div class="stat-value" style="font-size:24px; font-weight:600; color:${s.color || 'var(--color-on-dark)'};">${s.value}</div>
              </div>
            `).join('')}
          </div>
        ` : '';

        const actionsHtml = config.actions ? config.actions.map(a => `
          <button class="btn ${a.primary ? 'btn-primary' : 'btn-outline'}" id="${a.id}">
            ${a.icon ? `<span class="icon">${a.icon}</span> ` : ''}${a.label}
          </button>
        `).join('') : '';
        
        const filtersHtml = config.filtersHtml ? `<div class="${config.idPrefix}-filters">${config.filtersHtml}</div>` : '';
        const customTabs = config.tabsHtml ? `<div class="${config.idPrefix}-tabs">${config.tabsHtml}</div>` : '';
        
        let contentHtml = '';
        if (config.viewType === 'grid') {
          contentHtml = `
            <div class="${config.idPrefix}-grid" id="${config.idPrefix}-content-area" style="display:grid; grid-template-columns:repeat(auto-fill, minmax(300px, 1fr)); gap:16px;">
              <div class="skeleton" style="height:200px; grid-column: span 3; border-radius:var(--rounded-lg);"></div>
            </div>
          `;
        } else {
          contentHtml = `
            <div class="${config.idPrefix}-list">
              <div class="table-container">
                <table class="table">
                  <thead>
                    <tr>${(config.columns || []).map(c => `<th>${c}</th>`).join('')}</tr>
                  </thead>
                  <tbody id="${config.idPrefix}-content-area">
                    <tr><td colspan="${(config.columns || []).length}" class="text-center">Loading...</td></tr>
                  </tbody>
                </table>
              </div>
            </div>
          `;
        }

        this.container.innerHTML = `
          <div class="${config.idPrefix}-container fade-in">
            <div class="${config.idPrefix}-header" style="display:flex; justify-content:space-between; align-items:center; margin-bottom:24px;">
              <div>
                <h2>${config.title}</h2>
                <p class="text-muted">${config.description || ''}</p>
              </div>
              <div class="${config.idPrefix}-actions" style="display:flex; gap:8px;">
                ${actionsHtml}
              </div>
            </div>
            ${statsHtml}
            ${filtersHtml}
            ${customTabs}
            ${contentHtml}
            ${config.modalsHtml || ''}
          </div>
        `;
      },

      bindEvents() {
        if (config.bindEvents) {
          config.bindEvents.call(this);
        }
      },

      async loadData() {
        if (!config.endpoint) return;
        try {
          const endpoint = typeof config.endpoint === 'function' ? config.endpoint.call(this) : config.endpoint;
          const json = await window.APIClient.get(endpoint);
          const items = json?.data || [];
          this.data = items;
          this.renderContent(items);
          if (config.onDataLoaded) config.onDataLoaded.call(this, items);
        } catch (e) {
          console.error(config.title + ' API request failed:', e);
          const area = document.getElementById(config.idPrefix + '-content-area');
          if (area) {
             if (config.viewType === 'grid') {
               area.innerHTML = `<div class="text-center text-danger w-100 py-10" style="grid-column:1/-1;">Failed to load data: ${Security.escapeHTML(e.message)}</div>`;
             } else {
               area.innerHTML = `<tr><td colspan="${(config.columns || []).length}" class="text-center text-danger">Failed to load data: ${Security.escapeHTML(e.message)}</td></tr>`;
             }
          }
        }
      },

      renderContent(data) {
        const area = document.getElementById(config.idPrefix + '-content-area');
        if (!area) return;

        if (!data || data.length === 0) {
          if (config.viewType === 'grid') {
             area.innerHTML = `<div class="text-center text-muted w-100 py-10" style="grid-column:1/-1;">${Security.escapeHTML(config.emptyMessage || 'No items found.')}</div>`;
          } else {
             area.innerHTML = `
              <tr>
                <td colspan="${(config.columns || []).length}" style="text-align:center;color:var(--color-muted);padding:var(--space-lg);">
                  <div style="font-size:36px;margin-bottom:var(--space-sm);">${Security.escapeHTML(config.emptyIcon || '')}</div>
                  <h4 style="margin:0;font-weight:600;">${Security.escapeHTML(config.emptyMessage || 'No items available')}</h4>
                </td>
              </tr>
            `;
          }
          return;
        }

        if (config.renderItems) {
           area.innerHTML = config.renderItems.call(this, data);
        } else if (config.renderRow) {
           area.innerHTML = data.map(m => config.renderRow.call(this, m)).join('');
        }
      }
    };
  }

  global.createDataTablePage = createDataTablePage;
})(window);