/**
 * Enterprise Search Console UI Controller
 */
(function (global) {
  'use strict';

  var currentSearchTab = 'universal';
  var currentFilterCat = 'all';
  var mainLimit = 25;
  var savedSearches = [];
  var isLogsAdvActive = false;

  function init() {
    if (global.SearchIndex) {
      global.SearchIndex.buildStaticIndex();
    }

    var searchInput = document.getElementById('global-search-input');
    var searchDropdown = document.getElementById('global-search-dropdown');

    if (searchInput) {
      searchInput.addEventListener('input', debounce(function () {
        if (global.SearchAutocomplete) global.SearchAutocomplete.handleGlobalSearch();
      }, 300));
      searchInput.addEventListener('focus', function () {
        if (this.value.trim() && searchDropdown) searchDropdown.style.display = 'block';
      });
      searchInput.addEventListener('keydown', function (e) {
        if (global.SearchAutocomplete) global.SearchAutocomplete.handleGlobalSearchKeys(e);
      });
    }

    window.addEventListener('keydown', function (e) {
      if ((e.ctrlKey || e.metaKey) && e.key === 'k') {
        e.preventDefault();
        if (searchInput) {
          searchInput.focus();
          searchInput.select();
        }
      }
    });

    document.addEventListener('click', function (e) {
      if (searchDropdown && !e.target.closest('#global-search-input') && !e.target.closest('#global-search-dropdown')) {
        searchDropdown.style.display = 'none';
      }
    });

    document.querySelectorAll('.search-cat-tab').forEach(function (tab) {
      tab.addEventListener('click', function (e) {
        e.stopPropagation();
        document.querySelectorAll('.search-cat-tab').forEach(function (t) { t.classList.remove('active'); });
        this.classList.add('active');
        if (global.SearchAutocomplete) global.SearchAutocomplete.handleGlobalSearch();
      });
    });

    document.querySelectorAll('.search-main-tab').forEach(function (tab) {
      tab.addEventListener('click', function () {
        switchSearchTab(this.dataset.tab);
      });
    });

    document.querySelectorAll('.search-filter-cat').forEach(function (btn) {
      btn.addEventListener('click', function () {
        document.querySelectorAll('.search-filter-cat').forEach(function (b) { b.classList.remove('active'); });
        this.classList.add('active');
        currentFilterCat = this.dataset.filter;
        executeMainSearch();
      });
    });

    var mainSearchBtn = document.getElementById('search-main-btn');
    var mainSearchInput = document.getElementById('search-main-input');
    if (mainSearchBtn) mainSearchBtn.addEventListener('click', executeMainSearch);
    if (mainSearchInput) mainSearchInput.addEventListener('keydown', function (e) {
      if (e.key === 'Enter') executeMainSearch();
    });

    var logSearchBtn = document.getElementById('search-logs-btn');
    var logSearchInput = document.getElementById('search-logs-input');
    if (logSearchBtn) logSearchBtn.addEventListener('click', executeLogSearch);
    if (logSearchInput) logSearchInput.addEventListener('keydown', function (e) {
      if (e.key === 'Enter') executeLogSearch();
    });

    var gitSearchBtn = document.getElementById('search-git-btn');
    if (gitSearchBtn) {
      gitSearchBtn.addEventListener('click', function () {
        if (global.SearchGitTrace) global.SearchGitTrace.executeGitTrace();
      });
    }

    var graphSearchBtn = document.getElementById('search-graph-btn');
    if (graphSearchBtn) {
      graphSearchBtn.addEventListener('click', function () {
        if (global.SearchGraphVisualizer) global.SearchGraphVisualizer.drawDeploymentGraph();
      });
    }

    // Dynamic AppState Sync
    if (global.AppState) {
      global.AppState.on('kubernetes', populateLogsClusters);
      global.AppState.on('navigate', function (s) {
        if (s === 'enterprise-search') {
          populateLogsClusters(global.AppState.getState().kubernetes);
          triggerRenderAnalytics();
        }
      });

      if (global.SearchIndex) {
        global.AppState.on('kubernetes', global.SearchIndex.indexClusters);
        global.AppState.on('logs', global.SearchIndex.indexLogs);
        global.AppState.on('incidents', global.SearchIndex.indexIncidents);
      }
    }

    try {
      savedSearches = JSON.parse(localStorage.getItem('k8s_saved_searches') || '[]');
    } catch (e) {
      savedSearches = [];
    }
    renderSavedSearches();

    var mainSaveBtn = document.getElementById('search-main-save-btn');
    if (mainSaveBtn) {
      mainSaveBtn.addEventListener('click', function () {
        var query = document.getElementById('search-main-input').value.trim();
        if (!query) {
          alert('Please enter a search query first.');
          return;
        }
        var name = prompt('Enter a name for this saved search:', query);
        if (name) {
          savedSearches.push({ name: name, type: 'universal', config: { query: query, category: currentFilterCat } });
          localStorage.setItem('k8s_saved_searches', JSON.stringify(savedSearches));
          renderSavedSearches();
        }
      });
    }

    var toggleLogsAdvBtn = document.getElementById('toggle-logs-adv-builder');
    if (toggleLogsAdvBtn) {
      toggleLogsAdvBtn.addEventListener('click', function () {
        isLogsAdvActive = !isLogsAdvActive;
        var advContainer = document.getElementById('logs-adv-builder-container');
        var simpleFilters = document.getElementById('logs-simple-filters');
        if (isLogsAdvActive) {
          if (advContainer) advContainer.style.display = 'block';
          if (simpleFilters) simpleFilters.style.display = 'none';
          this.textContent = '⚙️ Switch to Simple Filters';
          var rulesContainer = document.getElementById('logs-adv-rules');
          if (rulesContainer && rulesContainer.children.length === 0 && global.SearchAdvBuilder) {
            rulesContainer.appendChild(global.SearchAdvBuilder.createRuleElement());
          }
        } else {
          if (advContainer) advContainer.style.display = 'none';
          if (simpleFilters) simpleFilters.style.display = 'grid';
          this.textContent = '⚙️ Switch to Advanced Filter Builder';
        }
      });
    }

    var logsAdvAddRuleBtn = document.getElementById('logs-adv-add-rule-btn');
    if (logsAdvAddRuleBtn) {
      logsAdvAddRuleBtn.addEventListener('click', function () {
        var container = document.getElementById('logs-adv-rules');
        if (container && global.SearchAdvBuilder) container.appendChild(global.SearchAdvBuilder.createRuleElement());
      });
    }

    var logsAdvAddGroupBtn = document.getElementById('logs-adv-add-group-btn');
    if (logsAdvAddGroupBtn) {
      logsAdvAddGroupBtn.addEventListener('click', function () {
        var container = document.getElementById('logs-adv-rules');
        if (container && global.SearchAdvBuilder) container.appendChild(global.SearchAdvBuilder.createGroupElement());
      });
    }

    var logsAdvApplyBtn = document.getElementById('logs-adv-apply-btn');
    if (logsAdvApplyBtn) logsAdvApplyBtn.addEventListener('click', function () { executeLogSearch(); });

    var logsAdvResetBtn = document.getElementById('logs-adv-reset-btn');
    if (logsAdvResetBtn) {
      logsAdvResetBtn.addEventListener('click', function () {
        var container = document.getElementById('logs-adv-rules');
        if (container && global.SearchAdvBuilder) {
          container.innerHTML = '';
          container.appendChild(global.SearchAdvBuilder.createRuleElement());
        }
        executeLogSearch();
      });
    }

    var logsSaveBtn = document.getElementById('search-logs-save-btn');
    if (logsSaveBtn) logsSaveBtn.addEventListener('click', handleLogsSave);

    var logsAdvSaveBtn = document.getElementById('logs-adv-save-btn');
    if (logsAdvSaveBtn) logsAdvSaveBtn.addEventListener('click', handleLogsSave);
  }

  function handleLogsSave() {
    var namePrompt = isLogsAdvActive ? 'Advanced Log Search' : (document.getElementById('search-logs-input').value.trim() || 'Log Search');
    var name = prompt('Enter a name for this saved log search:', namePrompt);
    if (!name) return;

    var config = {};
    if (isLogsAdvActive && global.SearchAdvBuilder) {
      config = {
        isLogsAdvActive: true,
        matchType: document.getElementById('logs-adv-match-type').value,
        rules: global.SearchAdvBuilder.compileRules(document.getElementById('logs-adv-rules'))
      };
    } else {
      config = {
        isLogsAdvActive: false,
        query: document.getElementById('search-logs-input').value.trim(),
        cluster: document.getElementById('search-logs-cluster').value,
        namespace: document.getElementById('search-logs-namespace').value,
        severity: document.getElementById('search-logs-severity').value
      };
    }

    savedSearches.push({ name: name, type: 'logs', config: config });
    localStorage.setItem('k8s_saved_searches', JSON.stringify(savedSearches));
    renderSavedSearches();
  }

  function switchSearchTab(tabId) {
    currentSearchTab = tabId;
    document.querySelectorAll('.search-main-tab').forEach(function (btn) {
      btn.classList.toggle('active', btn.dataset.tab === tabId);
    });
    document.querySelectorAll('.search-tab-content').forEach(function (c) {
      c.classList.toggle('active', c.id === 'search-tab-' + tabId);
    });

    if (tabId === 'universal') executeMainSearch();
    if (tabId === 'logs') executeLogSearch();
    if (tabId === 'analytics') triggerRenderAnalytics();
  }

  function executeMainSearch(keepLimit) {
    if (keepLimit !== true) mainLimit = 25;
    var query = document.getElementById('search-main-input').value.trim();
    var list = document.getElementById('search-main-results-list');
    var countLabel = document.getElementById('search-results-count');
    if (!list || !global.SearchIndex) return;

    if (!query) {
      list.innerHTML = '<div class="empty-state"><div class="empty-state-icon">🔍</div><div class="empty-state-text">Enter query to search infrastructure index</div></div>';
      countLabel.textContent = 'Showing 0 matches';
      return;
    }

    var results = global.SearchIndex.queryIndex(query, currentFilterCat);
    countLabel.textContent = 'Showing ' + results.length + ' matches';

    list.innerHTML = '';
    if (results.length === 0) {
      list.innerHTML = '<div class="empty-state"><div class="empty-state-icon">🗑️</div><div class="empty-state-text">No matches found for "' + esc(query) + '"</div></div>';
      return;
    }

    var visibleResults = results.slice(0, mainLimit);
    visibleResults.forEach(function (res) {
      var card = document.createElement('div');
      card.style.cssText = 'background:var(--color-surface-card);border:1px solid var(--color-hairline);padding:var(--space-md);border-radius:var(--rounded-lg);display:flex;justify-content:space-between;align-items:center;margin-bottom:var(--space-xs);';

      var icon = getResourceIcon(res.type);
      card.innerHTML =
        '<div>' +
          '<div style="font-weight:600;font-size:14px;color:var(--color-on-dark);">' + icon + ' ' + highlightText(res.name, query) + '</div>' +
          '<div style="font-size:12px;color:var(--color-muted);margin-top:var(--space-xxs);">' +
            'Cluster: <strong style="color:var(--color-body)">' + esc(res.cluster) + '</strong> | Namespace: ' + esc(res.namespace) + ' | Match relevance score: ' + res.score +
          '</div>' +
        '</div>' +
        '<div>' +
          '<span class="badge badge-synced">' + res.type.toUpperCase() + '</span>' +
        '</div>';

      list.appendChild(card);
    });

    if (results.length > mainLimit) {
      var loadMoreDiv = document.createElement('div');
      loadMoreDiv.style.cssText = 'padding:12px;text-align:center;width:100%;';
      var loadMoreBtn = document.createElement('button');
      loadMoreBtn.className = 'btn btn-outline btn-sm';
      loadMoreBtn.textContent = 'Load More (' + (results.length - mainLimit) + ' remaining)';
      loadMoreBtn.addEventListener('click', function (e) {
        e.preventDefault();
        mainLimit += 25;
        executeMainSearch(true);
      });
      loadMoreDiv.appendChild(loadMoreBtn);
      list.appendChild(loadMoreDiv);
    }
  }

  function populateLogsClusters(clusters) {
    if (global.SearchLogsUI) global.SearchLogsUI.populateLogsClusters(clusters);
  }

  function executeLogSearch(keepLimit) {
    if (global.SearchLogsUI) global.SearchLogsUI.executeLogSearch(keepLimit, isLogsAdvActive);
  }

  function triggerRenderAnalytics() {
    if (global.SearchAnalytics && global.SearchAutocomplete) {
      global.SearchAnalytics.renderAnalytics(
        global.SearchAutocomplete.getQueryCount(),
        global.SearchAutocomplete.getAiQueryCount()
      );
    }
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

  function debounce(func, wait) {
    var timeout;
    return function () {
      var context = this, args = arguments;
      clearTimeout(timeout);
      timeout = setTimeout(function () { func.apply(context, args); }, wait);
    };
  }

  function renderSavedSearches() {
    var list = document.getElementById('saved-searches-list');
    if (!list) return;
    list.innerHTML = '';
    if (savedSearches.length === 0) {
      list.innerHTML = '<div style="font-size:11px;color:var(--color-muted);font-style:italic;">No saved searches</div>';
      return;
    }
    savedSearches.forEach(function (item, idx) {
      var div = document.createElement('div');
      div.className = 'saved-search-item';
      div.style.cssText = 'display:flex; justify-content:space-between; align-items:center; background:var(--color-surface); padding:6px 8px; border-radius:var(--rounded-md); font-size:12px; margin-bottom:4px; border:1px solid var(--color-hairline); cursor:pointer;';

      var textSpan = document.createElement('span');
      textSpan.style.cssText = 'overflow:hidden; text-overflow:ellipsis; white-space:nowrap; flex:1; font-weight:500;';
      var prefix = item.type === 'logs' ? '📋 ' : (item.type === 'deploy' ? '🚀 ' : '🔍 ');
      textSpan.textContent = prefix + item.name;

      var deleteBtn = document.createElement('button');
      deleteBtn.className = 'btn btn-ghost btn-xs';
      deleteBtn.innerHTML = '🗑️';
      deleteBtn.style.padding = '2px 4px';
      deleteBtn.addEventListener('click', function (e) {
        e.stopPropagation();
        savedSearches.splice(idx, 1);
        localStorage.setItem('k8s_saved_searches', JSON.stringify(savedSearches));
        renderSavedSearches();
        if (global.DeploymentCatalog && global.DeploymentCatalog.renderSavedSearches) {
          global.DeploymentCatalog.renderSavedSearches();
        }
      });

      div.addEventListener('click', function () { loadSavedSearch(item); });
      div.appendChild(textSpan);
      div.appendChild(deleteBtn);
      list.appendChild(div);
    });
  }

  function loadSavedSearch(item) {
    if (item.type === 'universal') {
      Router.navigate('enterprise-search');
      switchSearchTab('universal');
      document.getElementById('search-main-input').value = item.config.query;
      currentFilterCat = item.config.category || 'all';
      document.querySelectorAll('.search-filter-cat').forEach(function (btn) {
        btn.classList.toggle('active', btn.dataset.filter === currentFilterCat);
      });
      executeMainSearch();
    } else if (item.type === 'logs') {
      Router.navigate('enterprise-search');
      switchSearchTab('logs');
      isLogsAdvActive = !!item.config.isLogsAdvActive;

      var advContainer = document.getElementById('logs-adv-builder-container');
      var simpleFilters = document.getElementById('logs-simple-filters');
      var toggleBtn = document.getElementById('toggle-logs-adv-builder');

      if (isLogsAdvActive && global.SearchAdvBuilder) {
        if (advContainer) advContainer.style.display = 'block';
        if (simpleFilters) simpleFilters.style.display = 'none';
        if (toggleBtn) toggleBtn.textContent = '⚙️ Switch to Simple Filters';
        document.getElementById('logs-adv-match-type').value = item.config.matchType;
        global.SearchAdvBuilder.loadRulesIntoDOM(item.config.rules, document.getElementById('logs-adv-rules'));
      } else {
        if (advContainer) advContainer.style.display = 'none';
        if (simpleFilters) simpleFilters.style.display = 'grid';
        if (toggleBtn) toggleBtn.textContent = '⚙️ Switch to Advanced Filter Builder';
        document.getElementById('search-logs-input').value = item.config.query;
        document.getElementById('search-logs-cluster').value = item.config.cluster;
        document.getElementById('search-logs-namespace').value = item.config.namespace;
        document.getElementById('search-logs-severity').value = item.config.severity;
      }
      executeLogSearch();
    } else if (item.type === 'deploy' && global.DeploymentCatalog && global.DeploymentCatalog.loadSavedSearch) {
      Router.navigate('deployment-center');
      global.DeploymentCatalog.loadSavedSearch(item);
    }
  }

  global.EnterpriseSearchSection = {
    init: init,
    renderSavedSearches: renderSavedSearches,
    loadSavedSearch: loadSavedSearch,
    getSavedSearches: function () { return savedSearches; },
    saveSearch: function (name, type, config) {
      savedSearches.push({ name: name, type: type, config: config });
      localStorage.setItem('k8s_saved_searches', JSON.stringify(savedSearches));
      renderSavedSearches();
    },
    switchSearchTab: switchSearchTab,
    executeMainSearch: executeMainSearch,
    highlightText: UIComponents.highlightText
  };
})(window);
