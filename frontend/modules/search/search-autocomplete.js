/**
 * Topbar Global Search Autocomplete Controller
 */
(function (global) {
  'use strict';

  var activeDropdownIndex = -1;
  var queryCount = 0;
  var aiQueryCount = 0;

  function handleGlobalSearch() {
    var query = document.getElementById('global-search-input').value.trim();
    var dropdown = document.getElementById('global-search-dropdown');
    var resultsBox = document.getElementById('global-search-results');

    if (!query || !global.SearchIndex) {
      dropdown.style.display = 'none';
      return;
    }

    var catBtn = document.querySelector('.search-cat-tab.active');
    var category = catBtn ? catBtn.dataset.cat : 'all';

    queryCount++;
    if (query.toLowerCase().indexOf('ai') === 0 || query.toLowerCase().indexOf('rca') >= 0) {
      aiQueryCount++;
    }

    var results = global.SearchIndex.queryIndex(query, category);
    resultsBox.innerHTML = '';
    activeDropdownIndex = -1;

    if (results.length === 0) {
      resultsBox.innerHTML = '<div style="color:var(--color-muted);padding:var(--space-sm);text-align:center;font-size:13px;">No results found</div>';
    } else {
      results.slice(0, 5).forEach(function (res, idx) {
        var div = document.createElement('div');
        div.className = 'global-search-item';
        div.dataset.index = idx;
        div.style.cssText = 'padding:var(--space-xs) var(--space-sm);border-radius:var(--rounded-md);cursor:pointer;margin-bottom:2px;transition:background 0.2s;';

        var icon = getResourceIcon(res.type);
        div.innerHTML =
          '<div style="display:flex;justify-content:space-between;align-items:center;">' +
            '<span style="font-weight:600;font-size:13px;">' + icon + ' ' + highlightText(res.name, query) + '</span>' +
            '<span class="badge badge-pending" style="font-size:10px;">' + res.type.toUpperCase() + '</span>' +
          '</div>' +
          '<div style="font-size:11px;color:var(--color-muted);margin-top:2px;">' +
            'Cluster: <strong>' + esc(res.cluster) + '</strong> | Namespace: ' + esc(res.namespace) +
          '</div>';

        div.addEventListener('mouseenter', function () {
          clearDropdownHighlights();
          div.style.background = 'var(--color-surface-elevated)';
          activeDropdownIndex = idx;
        });

        div.addEventListener('click', function () {
          searchDropdown.style.display = 'none';
          Router.navigate('enterprise-search');
          if (global.EnterpriseSearchSection) {
            global.EnterpriseSearchSection.switchSearchTab('universal');
            document.getElementById('search-main-input').value = query;
            global.EnterpriseSearchSection.executeMainSearch();
          }
        });

        resultsBox.appendChild(div);
      });
    }
    dropdown.style.display = 'block';
  }

  function handleGlobalSearchKeys(e) {
    var dropdown = document.getElementById('global-search-dropdown');
    if (dropdown.style.display === 'none') return;

    var items = dropdown.querySelectorAll('.global-search-item');
    if (items.length === 0) return;

    if (e.key === 'ArrowDown') {
      e.preventDefault();
      activeDropdownIndex = (activeDropdownIndex + 1) % items.length;
      highlightDropdownItem(items);
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      activeDropdownIndex = (activeDropdownIndex - 1 + items.length) % items.length;
      highlightDropdownItem(items);
    } else if (e.key === 'Enter') {
      e.preventDefault();
      if (activeDropdownIndex >= 0 && items[activeDropdownIndex]) {
        items[activeDropdownIndex].click();
      }
    }
  }

  function highlightDropdownItem(items) {
    clearDropdownHighlights();
    var activeItem = items[activeDropdownIndex];
    if (activeItem) {
      activeItem.style.background = 'var(--color-surface-elevated)';
      activeItem.scrollIntoView({ block: 'nearest' });
    }
  }

  function clearDropdownHighlights() {
    var dropdown = document.getElementById('global-search-dropdown');
    dropdown.querySelectorAll('.global-search-item').forEach(function (el) {
      el.style.background = 'transparent';
    });
  }

  function getResourceIcon(type) {
    switch (type) {
      case 'cluster': return '☸️';
      case 'node': return '🖥️';
      case 'pod': return '📦';
      case 'deployment': return '🚀';
      case 'log': return '📋';
      case 'git': return '🔀';
      case 'incident': return '🔴';
      case 'ai': return '🧠';
      default: return '◈';
    }
  }

  function highlightText(text, term) {
    if (!term) return esc(text);
    var regex = new RegExp('(' + escapeRegExp(term) + ')', 'gi');
    return esc(text).replace(regex, '<span style="background:rgba(252,213,53,0.25);color:var(--color-primary);font-weight:700;border-radius:2px;padding:0 2px;">$1</span>');
  }

  function escapeRegExp(string) {
    return string.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
  }

  global.SearchAutocomplete = {
    handleGlobalSearch: handleGlobalSearch,
    handleGlobalSearchKeys: handleGlobalSearchKeys,
    getQueryCount: function () { return queryCount; },
    getAiQueryCount: function () { return aiQueryCount; }
  };

})(window);
