/**
 * API Client — REST fallback for loading initial data with caching, cancellation, and coalescing.
 */
(function (global) {
  'use strict';

  const BASE_URL = window.location.origin + '/api/v1';

  // ── FETCH INTERCEPTOR FOR CACHING, COALESCING, AND CANCELLATION ──
  const originalFetch = window.fetch;
  const fetchCache = new Map();
  const activeFetches = new Map();
  const CACHE_DURATION = 3000; // 3 seconds

  window.fetch = function (input, init) {
    const method = (init && init.method || 'GET').toUpperCase();
    let url = typeof input === 'string' ? input : (input && input.url || '');

    // Pass through if not a GET request, not targeting api/v1, or contains nocache
    if (method !== 'GET' || !url.includes('/api/v1/') || url.includes('nocache=true')) {
      return originalFetch.apply(this, arguments);
    }

    const cacheKey = url;

    // 1. Coalesce identical pending requests
    if (activeFetches.has(cacheKey)) {
      return activeFetches.get(cacheKey).promise.then(res => res.clone());
    }

    // 2. Return client-side cache if valid
    if (fetchCache.has(cacheKey)) {
      const cached = fetchCache.get(cacheKey);
      if (Date.now() - cached.timestamp < CACHE_DURATION) {
        return Promise.resolve(cached.response.clone());
      } else {
        fetchCache.delete(cacheKey);
      }
    }

    // Create abort controller for request cancellation on navigation
    const controller = new AbortController();
    const signal = controller.signal;
    const fetchInit = Object.assign({}, init, { signal });

    // 3. Dispatch original fetch
    const promise = originalFetch(input, fetchInit)
      .then(async response => {
        activeFetches.delete(cacheKey);
        if (response.ok) {
          const cachedResponse = response.clone();
          fetchCache.set(cacheKey, {
            response: cachedResponse,
            timestamp: Date.now()
          });
        }
        return response;
      })
      .catch(err => {
        activeFetches.delete(cacheKey);
        throw err;
      });

    activeFetches.set(cacheKey, { promise, controller });
    return promise.then(res => res.clone());
  };

  // Explicit request cancellation helper on navigation transitions
  function abortAll() {
    activeFetches.forEach(function (active) {
      try {
        active.controller.abort();
      } catch (e) {
        // Silence abort exceptions
      }
    });
    activeFetches.clear();
  }

  // ── API CLIENT METHODS ──

  async function fetchJSON(path) {
    try {
      const resp = await fetch(BASE_URL + path);
      if (!resp.ok) throw new Error('HTTP ' + resp.status);
      return await resp.json();
    } catch (e) {
      if (e.name === 'AbortError') {
        console.log('[API] Fetch aborted:', path);
      } else {
        console.warn('[API] Fetch failed:', path, e.message);
      }
      return null;
    }
  }

  async function loadIncidents(limit, offset) {
    limit = limit || 50;
    offset = offset || 0;
    return fetchJSON('/incidents?limit=' + limit + '&offset=' + offset);
  }

  async function loadReports(limit, offset) {
    limit = limit || 50;
    offset = offset || 0;
    return fetchJSON('/reports?limit=' + limit + '&offset=' + offset);
  }

  async function loadPRs(limit, offset) {
    limit = limit || 50;
    offset = offset || 0;
    return fetchJSON('/prs?limit=' + limit + '&offset=' + offset);
  }

  async function loadMetrics() {
    return fetchJSON('/metrics');
  }

  global.APIClient = { loadIncidents, loadReports, loadPRs, loadMetrics, abortAll };

})(window);
