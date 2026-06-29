// command-palette.js - Global Ctrl+K Command Palette
(function() {
    let searchInput;
    let searchDropdown;
    let searchResults;
    let currentCategory = 'all';

    document.addEventListener("DOMContentLoaded", () => {
        searchInput = document.getElementById('global-search-input');
        searchDropdown = document.getElementById('global-search-dropdown');
        searchResults = document.getElementById('global-search-results');
        
        if (!searchInput || !searchDropdown || !searchResults) return;

        // Global hotkey
        document.addEventListener('keydown', (e) => {
            if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'k') {
                e.preventDefault();
                searchInput.focus();
                searchDropdown.style.display = 'block';
            }
            if (e.key === 'Escape') {
                searchDropdown.style.display = 'none';
                searchInput.blur();
            }
        });

        // Search Input handler
        let debounceTimer;
        searchInput.addEventListener('input', (e) => {
            clearTimeout(debounceTimer);
            const query = e.target.value.trim();
            
            if (query.length === 0) {
                searchDropdown.style.display = 'none';
                return;
            }

            searchDropdown.style.display = 'block';
            debounceTimer = setTimeout(() => {
                performSearch(query, currentCategory);
            }, 300); // 300ms debounce
        });

        // Category Tabs
        const tabs = document.querySelectorAll('.search-cat-tab');
        tabs.forEach(tab => {
            tab.addEventListener('click', (e) => {
                tabs.forEach(t => t.classList.remove('active'));
                e.target.classList.add('active');
                currentCategory = e.target.getAttribute('data-cat');
                performSearch(searchInput.value.trim(), currentCategory);
            });
        });

        // Close on click outside
        document.addEventListener('click', (e) => {
            if (!searchInput.contains(e.target) && !searchDropdown.contains(e.target)) {
                searchDropdown.style.display = 'none';
            }
        });
    });

    async function performSearch(query, category) {
        if (!query) return;

        searchResults.innerHTML = '<div style="padding:10px;text-align:center;color:var(--color-muted);">Searching...</div>';

        try {
            const res = await fetch(`/api/v1/search?q=${encodeURIComponent(query)}&type=${category}`);
            if (res.ok) {
                const data = await res.json();
                renderResults(data);
            } else {
                searchResults.innerHTML = '<div style="padding:10px;text-align:center;color:var(--color-trading-down);">Search requires API connection</div>';
            }
        } catch (e) {
            searchResults.innerHTML = '<div style="padding:10px;text-align:center;color:var(--color-trading-down);">Search requires API connection</div>';
        }
    }

    function renderResults(results) {
        if (!results || results.length === 0) {
            searchResults.innerHTML = '<div style="padding:10px;text-align:center;color:var(--color-muted);">No results found</div>';
            return;
        }

        let html = '';
        results.forEach(r => {
            const icon = r.type === 'kubernetes' ? '📦' : r.type === 'log' ? '📋' : '🔴';
            html += `
                <div class="search-result-item" style="padding:8px; border-bottom:1px solid var(--color-hairline); cursor:pointer; display:flex; align-items:center; gap:8px;" onmouseover="this.style.background='var(--color-surface-hover)'" onmouseout="this.style.background='transparent'">
                    <span style="font-size:16px;">${icon}</span>
                    <div style="flex:1;">
                        <div style="font-weight:600; font-size:13px; color:var(--color-text);">${r.title}</div>
                        <div style="font-size:11px; color:var(--color-muted);">${r.desc}</div>
                    </div>
                </div>
            `;
        });
        searchResults.innerHTML = html;
    }
})();
