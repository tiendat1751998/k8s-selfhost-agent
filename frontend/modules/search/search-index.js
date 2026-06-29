/**
 * Enterprise Search Indexing & Query Service
 */
(function (global) {
  'use strict';

  var searchIndex = [];
  var searchCache = new Map();

  function buildStaticIndex() {
    searchIndex = [];

    // Git Commits
    indexResource('git', { id: 'c7f82b1', name: 'PR #142 (Merged): increase memory limit to 512Mi', author: 'admin', branch: 'main' });
    indexResource('git', { id: 'a3d2e14', name: 'PR #140 (Merged): fix authentication token expiration', author: 'dev-team', branch: 'main' });
    indexResource('git', { id: 'b8f921d', name: 'PR #138 (Merged): setup initial CI/CD pipeline', author: 'sre-team', branch: 'main' });
    indexResource('git', { id: 'd9c3e41', name: 'PR #145 (Open): update liveness probes in deployment config', author: 'oncall', branch: 'dev' });

    // AI RCA Reports
    indexResource('ai', { id: 'rca-oom', name: 'AI RCA: Pod api-server OOMKilled Analysis Report', cluster: 'prod-us-east', result: 'exceeded 512Mi limits' });
    indexResource('ai', { id: 'rca-probe', name: 'AI RCA: Startup Probe Failure Analysis for payment-svc', cluster: 'staging-1', result: 'container crashed in startup' });

    // Populate live state changes
    if (global.AppState) {
      indexClusters(global.AppState.getState().kubernetes);
      indexLogs(global.AppState.getState().logs);
      indexIncidents(global.AppState.getState().incidents);
    }
  }

  function indexResource(type, obj) {
    searchCache.clear();

    var existingIdx = searchIndex.findIndex(function (item) {
      return item.type === type && item.id === obj.id;
    });

    var indexItem = {
      type: type,
      id: obj.id || obj.name,
      name: obj.name,
      cluster: obj.cluster || obj.clusterName || '—',
      namespace: obj.namespace || 'default',
      metadata: obj,
      score: 0.0
    };

    if (existingIdx >= 0) {
      searchIndex[existingIdx] = indexItem;
    } else {
      searchIndex.push(indexItem);
    }
  }

  function indexClusters(clusters) {
    if (!clusters) return;
    clusters.forEach(function (c) {
      indexResource('cluster', { id: c.name, name: c.name, endpoint: c.endpoint, provider: c.provider });
    });
  }

  function indexLogs(logs) {
    if (!logs) return;
    logs.forEach(function (log, idx) {
      indexResource('log', { id: 'log-' + idx, name: log, cluster: 'prod-us-east', namespace: 'production' });
    });
  }

  function indexIncidents(incidents) {
    if (!incidents) return;
    incidents.forEach(function (inc, idx) {
      indexResource('incident', { id: inc.id || 'inc-' + idx, name: inc.type + ': ' + inc.message, cluster: inc.cluster || inc.clusterName, namespace: inc.namespace });
    });
  }

  function queryIndex(query, category) {
    if (!query) return [];
    var q = query.toLowerCase();
    var cacheKey = q + '_' + (category || 'all');

    if (searchCache.has(cacheKey)) {
      return searchCache.get(cacheKey);
    }

    var results = searchIndex.filter(function (item) {
      if (category && category !== 'all') {
        if (category === 'kubernetes' && ['cluster', 'node', 'pod', 'deployment'].indexOf(item.type) < 0) return false;
        if (category !== 'kubernetes' && item.type !== category) return false;
      }

      var score = 0;
      var name = item.name.toLowerCase();
      var metadataStr = JSON.stringify(item.metadata).toLowerCase();

      if (name === q) score += 20;
      else if (name.indexOf(q) === 0) score += 10;
      else if (name.indexOf(q) > 0) score += 5;

      var words = q.split(' ');
      words.forEach(function (word) {
        if (name.indexOf(word) >= 0) score += 2;
        if (metadataStr.indexOf(word) >= 0) score += 1;
      });

      item.score = score;
      return score > 0;
    });

    var sortedResults = results.sort(function (a, b) { return b.score - a.score; });
    searchCache.set(cacheKey, sortedResults);
    return sortedResults;
  }

  global.SearchIndex = {
    buildStaticIndex: buildStaticIndex,
    indexResource: indexResource,
    indexClusters: indexClusters,
    indexLogs: indexLogs,
    indexIncidents: indexIncidents,
    queryIndex: queryIndex,
    getRawIndex: function () { return searchIndex; }
  };

})(window);
