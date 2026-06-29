/**
 * Enterprise Logs Search UI Controller
 */
(function (global) {
  'use strict';

  var logLimit = 100;

  function populateLogsClusters(clusters) {
    var sel = document.getElementById('search-logs-cluster');
    if (!sel || !clusters) return;
    var cur = sel.value;
    sel.innerHTML = '<option value="">All Clusters</option>';
    clusters.forEach(function (c) {
      sel.innerHTML += '<option value="' + esc(c.name) + '">' + esc(c.name) + '</option>';
    });
    sel.value = cur;
  }

  function executeLogSearch(keepLimit, isLogsAdvActive) {
    if (keepLimit !== true) logLimit = 100;
    var streamEl = document.getElementById('search-logs-stream');
    if (!streamEl || !global.SearchIndex) return;
    streamEl.innerHTML = '';

    var logs = [];
    var query = '';
    var rawIndex = global.SearchIndex.getRawIndex();

    if (isLogsAdvActive && global.SearchAdvBuilder) {
      var compiled = global.SearchAdvBuilder.compileRules(document.getElementById('logs-adv-rules'));
      var matchType = document.getElementById('logs-adv-match-type').value;
      logs = rawIndex.filter(function (item) {
        if (item.type !== 'log') return false;
        return global.SearchAdvBuilder.evaluateCompiledRules(item, compiled, matchType);
      });
    } else {
      query = document.getElementById('search-logs-input').value.trim();
      var cluster = document.getElementById('search-logs-cluster').value;
      var ns = document.getElementById('search-logs-namespace').value;
      var severity = document.getElementById('search-logs-severity').value;

      logs = rawIndex.filter(function (item) {
        if (item.type !== 'log') return false;
        if (cluster && item.cluster !== cluster) return false;
        if (ns && item.namespace !== ns) return false;

        var msg = item.name;
        if (severity) {
          if (severity === 'INFO' && msg.indexOf('INFO') < 0 && msg.indexOf('kubectl') < 0 && msg.indexOf('watcher') < 0) return false;
          if (severity === 'WARN' && msg.indexOf('WARN') < 0) return false;
          if (severity === 'ERROR' && msg.indexOf('ERROR') < 0 && msg.indexOf('restarted') < 0 && msg.indexOf('NotReady') < 0) return false;
        }

        if (query && msg.toLowerCase().indexOf(query.toLowerCase()) < 0) return false;
        return true;
      });
    }

    if (logs.length === 0) {
      streamEl.innerHTML = '<div style="color:var(--color-muted);font-style:italic;">No matching log streams found.</div>';
      return;
    }

    var visibleLogs = logs.slice(0, logLimit);
    visibleLogs.forEach(function (log) {
      var div = document.createElement('div');
      div.style.cssText = 'padding:3px 0;border-bottom:1px solid rgba(255,255,255,0.02);line-height:1.4;';

      var formattedMsg = esc(log.name);
      if (isLogsAdvActive && global.SearchAdvBuilder) {
        var compiledRules = global.SearchAdvBuilder.compileRules(document.getElementById('logs-adv-rules'));
        function collectVals(rules) {
          var vals = [];
          rules.forEach(function (r) {
            if (r.type === 'rule' && r.field === 'message' && r.val) vals.push(r.val);
            else if (r.type === 'group' && r.rules) vals = vals.concat(collectVals(r.rules));
          });
          return vals;
        }
        var highlightVals = collectVals(compiledRules);
        if (highlightVals.length > 0 && global.EnterpriseSearchSection) {
          formattedMsg = global.EnterpriseSearchSection.highlightText(log.name, highlightVals[0]);
        }
      } else {
        if (query && global.EnterpriseSearchSection) {
          formattedMsg = global.EnterpriseSearchSection.highlightText(log.name, query);
        }
      }

      if (log.name.indexOf('ERROR') >= 0 || log.name.indexOf('restarted') >= 0 || log.name.indexOf('NotReady') >= 0) {
        div.innerHTML = '<span style="color:var(--color-trading-down)">[ERROR]</span> ' + formattedMsg;
      } else if (log.name.indexOf('WARN') >= 0) {
        div.innerHTML = '<span style="color:var(--color-primary)">[WARN]</span> ' + formattedMsg;
      } else {
        div.innerHTML = '<span style="color:var(--color-muted)">[INFO]</span> ' + formattedMsg;
      }

      streamEl.appendChild(div);
    });

    if (logs.length > logLimit) {
      var loadMoreDiv = document.createElement('div');
      loadMoreDiv.style.cssText = 'padding:12px;text-align:center;width:100%;';
      var loadMoreBtn = document.createElement('button');
      loadMoreBtn.className = 'btn btn-outline btn-sm';
      loadMoreBtn.textContent = 'Load More (' + (logs.length - logLimit) + ' remaining)';
      loadMoreBtn.addEventListener('click', function (e) {
        e.preventDefault();
        logLimit += 100;
        executeLogSearch(true, isLogsAdvActive);
      });
      loadMoreDiv.appendChild(loadMoreBtn);
      streamEl.appendChild(loadMoreDiv);
    }
    streamEl.scrollTop = streamEl.scrollHeight;
  }

  global.SearchLogsUI = {
    populateLogsClusters: populateLogsClusters,
    executeLogSearch: executeLogSearch
  };

})(window);
