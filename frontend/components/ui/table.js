/**
 * TableComponent — Reusable enterprise table system.
 * Eliminates copy-pasted enterprise-table boilerplate across modules.
 */
(function (global) {
  'use strict';

  class TableComponent {
    constructor({ containerId, columns, emptyMessage }) {
      this.containerId = containerId;
      this.columns = columns;
      this.emptyMessage = emptyMessage || 'No data available';
      this.container = document.getElementById(this.containerId);
      this.tbody = null;
      
      if (this.container) {
        this.render();
      }
    }

    render() {
      if (!this.container) return;
      
      const wrap = document.createElement('div');
      wrap.className = 'enterprise-table-wrap';
      
      const table = document.createElement('table');
      table.className = 'enterprise-table';
      
      const thead = document.createElement('thead');
      const tr = document.createElement('tr');
      
      this.columns.forEach(col => {
        const th = document.createElement('th');
        th.textContent = col.label || col.key;
        tr.appendChild(th);
      });
      thead.appendChild(tr);
      
      this.tbody = document.createElement('tbody');
      
      table.appendChild(thead);
      table.appendChild(this.tbody);
      wrap.appendChild(table);
      
      this.container.innerHTML = '';
      this.container.appendChild(wrap);
      
      this.clear();
    }

    setData(rows) {
      if (!this.tbody) return;
      
      this.tbody.innerHTML = '';
      
      if (!rows || rows.length === 0) {
        const tr = document.createElement('tr');
        const td = document.createElement('td');
        td.colSpan = this.columns.length;
        const div = document.createElement('div');
        div.className = 'empty-state';
        div.style.cssText = 'border: none; padding: 24px;';
        const text = document.createElement('div');
        text.className = 'empty-state-text';
        text.textContent = this.emptyMessage;
        div.appendChild(text);
        td.appendChild(div);
        td.style.textAlign = 'center';
        tr.appendChild(td);
        this.tbody.appendChild(tr);
        return;
      }
      
      rows.forEach(row => {
        const tr = document.createElement('tr');
        this.columns.forEach(col => {
          const td = document.createElement('td');
          if (col.render) {
            const htmlOrNode = col.render(row[col.key], row);
            if (htmlOrNode instanceof HTMLElement) {
              td.appendChild(htmlOrNode);
            } else {
              td.innerHTML = htmlOrNode;
            }
          } else {
            td.textContent = row[col.key] || '';
          }
          tr.appendChild(td);
        });
        this.tbody.appendChild(tr);
      });
    }

    clear() {
      this.setData([]);
    }

    static create(config) {
      return new TableComponent(config);
    }
  }

  global.TableComponent = TableComponent;

})(window);
