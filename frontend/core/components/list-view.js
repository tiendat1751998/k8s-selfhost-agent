(function (global) {
  'use strict';

  class ListView {
    constructor({ sectionId, title, description, containerId, columns, endpoint, renderRow, emptyMessage }) {
      this.sectionId = sectionId;
      this.title = title;
      this.description = description;
      this.containerId = containerId;
      this.columns = columns;
      this.endpoint = endpoint;
      this.renderRow = renderRow;
      this.emptyMessage = emptyMessage || 'No items found.';
    }

    init() {
      this.container = document.getElementById(this.containerId);
      if (!this.container) return;
      this.render();
      
      AppState.on('navigate', (section) => {
        if (section === this.sectionId) {
          this.loadData();
        }
      });
    }

    render() {
      const colHeaders = this.columns.map(c => `<th style="padding: 12px 8px;">${c}</th>`).join('');
      const colCount = this.columns.length;
      
      this.container.innerHTML = `
        <div class="panel fade-in" style="margin: var(--space-md); border: 1px solid var(--color-hairline); box-shadow: 0 4px 12px rgba(0,0,0,0.15);">
          <div class="panel-header" style="padding: var(--space-md); border-bottom: 1px solid var(--color-hairline);">
            <h3 style="margin: 0; font-size: 18px; color: var(--color-on-dark);">${this.title}</h3>
            <p style="margin: var(--space-xs) 0 0 0; font-size: 13px; color: var(--color-muted);">${this.description}</p>
          </div>
          <div class="panel-body" style="padding: var(--space-md);">
            <div class="table-container" style="overflow-x: auto;">
              <table class="table" style="width: 100%; border-collapse: collapse; text-align: left;">
                <thead>
                  <tr style="border-bottom: 2px solid var(--color-hairline); color: var(--color-on-dark); font-size: 13px;">
                    ${colHeaders}
                  </tr>
                </thead>
                <tbody id="${this.sectionId}-tbody" style="font-size: 13px;">
                  <tr><td colspan="${colCount}" class="text-center" style="padding: 24px; text-align: center; color: var(--color-muted);">Loading...</td></tr>
                </tbody>
              </table>
            </div>
          </div>
        </div>
      `;
    }

    async loadData() {
      const tbody = document.getElementById(`${this.sectionId}-tbody`);
      if (!tbody) return;

      try {
        let json;
        if (this.endpoint.startsWith('/api/v1/')) {
          const res = await fetch(this.endpoint);
          if (!res.ok) throw new Error('Failed to fetch data');
          json = await res.json();
        } else {
          json = await APIClient.get(this.endpoint);
        }
        const items = json.data || [];
        this.renderTable(items);
      } catch (e) {
        const errorMsg = global.Security && global.Security.escapeHTML ? global.Security.escapeHTML(e.message) : e.message;
        tbody.innerHTML = `<tr><td colspan="${this.columns.length}" class="text-center text-danger" style="padding: 24px; text-align: center; color: var(--color-trading-down);">Error loading data: ${errorMsg}</td></tr>`;
      }
    }

    renderTable(data) {
      const tbody = document.getElementById(`${this.sectionId}-tbody`);
      if (!tbody) return;

      if (!data || data.length === 0) {
        tbody.innerHTML = `<tr><td colspan="${this.columns.length}" class="text-center text-muted" style="padding: 24px; text-align: center; color: var(--color-muted);">${Security.escapeHTML(this.emptyMessage)}</td></tr>`;
        return;
      }

      tbody.innerHTML = data.map(this.renderRow).join('');
    }
  }

  global.ListView = ListView;

})(window);
