/**
 * Reusable Table Renderer — Supports column resizing, sorting, filtering, and pagination.
 */
(function (global) {
  'use strict';

  class TableRenderer {
    constructor(containerId, options) {
      this.container = document.getElementById(containerId);
      if (!this.container) {
        console.warn('Table container not found:', containerId);
        return;
      }
      this.columns = options.columns || []; // Array of { key, label, width, sortable, render }
      this.data = options.data || [];
      this.filteredData = [...this.data];
      this.sortKey = null;
      this.sortOrder = 'asc'; // 'asc' or 'desc'
      this.currentPage = 1;
      this.pageSize = options.pageSize || 10;
      this.onRowClick = options.onRowClick || null;

      this.init();
    }

    init() {
      this.container.innerHTML = `
        <div class="table-renderer-wrapper" style="width: 100%; overflow-x: auto; position: relative;">
          <table class="table" style="width: 100%; border-collapse: collapse; table-layout: fixed;">
            <thead>
              <tr class="table-header-row"></tr>
            </thead>
            <tbody class="table-body"></tbody>
          </table>
          <div class="table-pagination-bar" style="display: flex; justify-content: space-between; align-items: center; padding: 12px; border-top: 1px solid var(--color-hairline);">
            <div class="table-pagination-info" style="font-size: 13px; color: var(--color-muted);"></div>
            <div class="table-pagination-actions" style="display: flex; gap: 8px;">
              <button class="btn btn-outline btn-xs btn-prev" disabled>Previous</button>
              <button class="btn btn-outline btn-xs btn-next" disabled>Next</button>
            </div>
          </div>
        </div>
      `;
      this.wrapper = this.container.querySelector('.table-renderer-wrapper');
      this.table = this.container.querySelector('table');
      this.theadRow = this.container.querySelector('thead tr');
      this.tbody = this.container.querySelector('.table-body');
      this.infoEl = this.container.querySelector('.table-pagination-info');
      this.prevBtn = this.container.querySelector('.btn-prev');
      this.nextBtn = this.container.querySelector('.btn-next');

      this.prevBtn.addEventListener('click', () => this.goToPage(this.currentPage - 1));
      this.nextBtn.addEventListener('click', () => this.goToPage(this.currentPage + 1));

      this.renderHeaders();
      this.renderBody();
    }

    setData(newData) {
      this.data = newData || [];
      this.filteredData = [...this.data];
      this.currentPage = 1;
      if (this.sortKey) {
        this.sort(this.sortKey, this.sortOrder, true);
      } else {
        this.renderBody();
      }
    }

    showLoading(rowCount = 5) {
      this.tbody.innerHTML = '';
      for (let i = 0; i < rowCount; i++) {
        const tr = document.createElement('tr');
        this.columns.forEach(col => {
          const td = document.createElement('td');
          td.style.padding = '12px 16px';
          td.innerHTML = `
            <div class="skeleton">
              <div class="skeleton-line"></div>
              <div class="skeleton-line" style="width: 70%; margin-bottom: 0;"></div>
            </div>
          `;
          tr.appendChild(td);
        });
        this.tbody.appendChild(tr);
      }
      this.infoEl.textContent = 'Loading data...';
      this.prevBtn.disabled = true;
      this.nextBtn.disabled = true;
    }

    filter(keyword) {
      const q = (keyword || '').trim().toLowerCase();
      if (!q) {
        this.filteredData = [...this.data];
      } else {
        this.filteredData = this.data.filter(row => {
          return this.columns.some(col => {
            const val = row[col.key];
            if (val === undefined || val === null) return false;
            return String(val).toLowerCase().includes(q);
          });
        });
      }
      this.currentPage = 1;
      this.renderBody();
    }

    sort(key, order, force) {
      if (this.sortKey === key && !order && !force) {
        this.sortOrder = this.sortOrder === 'asc' ? 'desc' : 'asc';
      } else {
        this.sortKey = key;
        if (order) this.sortOrder = order;
      }

      this.filteredData.sort((a, b) => {
        let valA = a[key];
        let valB = b[key];
        if (typeof valA === 'string') valA = valA.toLowerCase();
        if (typeof valB === 'string') valB = valB.toLowerCase();

        if (valA < valB) return this.sortOrder === 'asc' ? -1 : 1;
        if (valA > valB) return this.sortOrder === 'asc' ? 1 : -1;
        return 0;
      });

      this.renderHeaders();
      this.renderBody();
    }

    goToPage(page) {
      const maxPage = Math.max(1, Math.ceil(this.filteredData.length / this.pageSize));
      if (page < 1 || page > maxPage) return;
      this.currentPage = page;
      this.renderBody();
    }

    renderHeaders() {
      this.theadRow.innerHTML = '';
      this.columns.forEach(col => {
        const th = document.createElement('th');
        th.style.position = 'relative';
        th.style.width = col.width || 'auto';
        th.style.cursor = col.sortable ? 'pointer' : 'default';
        
        let label = col.label;
        if (this.sortKey === col.key) {
          label += this.sortOrder === 'asc' ? ' ▴' : ' ▾';
        }
        th.textContent = label;

        if (col.sortable) {
          th.addEventListener('click', (e) => {
            if (e.target.classList.contains('table-resizer')) return;
            this.sort(col.key);
          });
        }

        // Resizer Handle
        const resizer = document.createElement('div');
        resizer.className = 'table-resizer';
        resizer.style.cssText = 'position:absolute;top:0;right:0;bottom:0;width:6px;cursor:col-resize;z-index:10;';
        
        // Resize mouse handling
        let startX, startWidth;
        const doDrag = (e) => {
          th.style.width = (startWidth + e.clientX - startX) + 'px';
        };
        const stopDrag = () => {
          document.removeEventListener('mousemove', doDrag);
          document.removeEventListener('mouseup', stopDrag);
        };
        resizer.addEventListener('mousedown', (e) => {
          startX = e.clientX;
          startWidth = th.offsetWidth;
          document.addEventListener('mousemove', doDrag);
          document.addEventListener('mouseup', stopDrag);
        });

        th.appendChild(resizer);
        this.theadRow.appendChild(th);
      });
    }

    renderBody() {
      this.tbody.innerHTML = '';
      
      const totalItems = this.filteredData.length;
      const totalPages = Math.max(1, Math.ceil(totalItems / this.pageSize));
      const startIdx = (this.currentPage - 1) * this.pageSize;
      const endIdx = Math.min(startIdx + this.pageSize, totalItems);
      const pageData = this.filteredData.slice(startIdx, endIdx);

      if (pageData.length === 0) {
        const tr = document.createElement('tr');
        tr.innerHTML = `<td colspan="${this.columns.length}" style="text-align: center; color: var(--color-muted); padding: var(--space-md);">No entries found</td>`;
        this.tbody.appendChild(tr);
      } else {
        pageData.forEach(row => {
          const tr = document.createElement('tr');
          if (this.onRowClick) {
            tr.style.cursor = 'pointer';
            tr.addEventListener('click', () => this.onRowClick(row));
          }

          this.columns.forEach(col => {
            const td = document.createElement('td');
            td.style.overflow = 'hidden';
            td.style.textOverflow = 'ellipsis';
            td.style.whiteSpace = 'nowrap';
            
            if (col.render) {
              const customContent = col.render(row[col.key], row);
              if (customContent instanceof HTMLElement) {
                td.appendChild(customContent);
              } else {
                td.innerHTML = customContent;
              }
            } else {
              td.textContent = row[col.key] !== undefined ? row[col.key] : '';
            }
            tr.appendChild(td);
          });
          this.tbody.appendChild(tr);
        });
      }

      // Update pagination bar status
      this.infoEl.textContent = totalItems > 0 
        ? `Showing ${startIdx + 1}-${endIdx} of ${totalItems} entries`
        : `Showing 0-0 of 0 entries`;

      this.prevBtn.disabled = this.currentPage === 1;
      this.nextBtn.disabled = this.currentPage === totalPages;
    }
  }

  global.TableRenderer = TableRenderer;

})(window);
