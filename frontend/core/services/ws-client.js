/**
 * WebSocket Client — Connection manager with exponential backoff reconnect,
 * message validation, watchdog heartbeat check, and offline indicator.
 */
(function (global) {
  'use strict';

  const MAX_RECONNECT = 15;
  const BASE_DELAY = 1000;
  const WATCHDOG_INTERVAL = 45000; // 45 seconds

  const STATE = {
    CLOSED: 0,
    CONNECTING: 1,
    OPEN: 2
  };

  class WebSocketClient {
    constructor() {
      this.ws = null;
      this.state = STATE.CLOSED;
      this.reconnectAttempts = 0;
      this.url = '';
      this.reconnectTimer = null;
      this.watchdogTimer = null;
    }

    connect(wsUrl) {
      this.url = wsUrl || this.url;
      if (!this.url) return;

      if (this.state === STATE.CONNECTING || this.state === STATE.OPEN) {
        return; // Prevent overlapping connections
      }

      this.cleanup();
      this.state = STATE.CONNECTING;
      if (global.AppState) global.AppState.setConnection('connecting');

      try {
        this.ws = new WebSocket(this.url);
      } catch (e) {
        console.error('WebSocket creation failed:', e);
        this.scheduleReconnect();
        return;
      }

      this.ws.onopen = () => {
        console.log('[WS] Connected to', this.url);
        this.state = STATE.OPEN;
        this.reconnectAttempts = 0;
        if (global.AppState) global.AppState.setConnection('online');
        this.resetWatchdog();
      };

      this.ws.onmessage = (event) => {
        this.resetWatchdog();
        try {
          const msg = JSON.parse(event.data);
          if (!msg || !msg.type) return;
          this.handleMessage(msg);
        } catch (e) {
          console.warn('[WS] Invalid message:', e);
        }
      };

      this.ws.onerror = (event) => {
        console.error('[WS] Error:', event);
      };

      this.ws.onclose = (event) => {
        console.log('[WS] Closed:', event.code, event.reason);
        this.state = STATE.CLOSED;
        if (global.AppState) global.AppState.setConnection('offline');
        this.scheduleReconnect();
      };
    }

    resetWatchdog() {
      if (this.watchdogTimer) {
        clearTimeout(this.watchdogTimer);
      }
      this.watchdogTimer = setTimeout(() => {
        console.warn('[WS] Heartbeat watchdog timeout. No message received for 45s. Reconnecting...');
        if (this.ws) {
          try {
            this.ws.close();
          } catch (e) {}
        }
      }, WATCHDOG_INTERVAL);
    }

    handleMessage(msg) {
      const event = new CustomEvent('ws-message', { detail: msg });
      window.dispatchEvent(event);

      if (!global.AppState) return;

      switch (msg.type) {
        case 'incident':
          if (msg.data && typeof msg.data === 'object') {
            msg.data.timestamp = msg.data.timestamp || new Date().toISOString();
            global.AppState.addIncident(msg.data);
          }
          break;

        case 'agent':
          if (msg.data && msg.data.step) {
            global.AppState.updateAgent(msg.data);
          }
          break;

        case 'log':
          if (msg.data !== undefined) {
            const logText = typeof msg.data === 'string' ? msg.data : JSON.stringify(msg.data);
            const cleanText = global.Security && global.Security.redactSecrets 
              ? global.Security.redactSecrets(logText) 
              : logText;
            
            global.AppState.addLog({
              text: cleanText,
              timestamp: new Date().toISOString()
            });
          }
          break;

        case 'metrics':
          if (msg.data && typeof msg.data === 'object') {
            global.AppState.updateMetrics(msg.data);
          }
          break;

        case 'config':
          if (msg.data && typeof msg.data === 'object') {
            if (msg.data.kubernetes) global.AppState.setKubernetes(msg.data.kubernetes);
            if (msg.data.gitProviders) global.AppState.setGitProviders(msg.data.gitProviders);
            if (msg.data.cicd) global.AppState.setCicd(msg.data.cicd);
            if (msg.data.aiProviders) global.AppState.setAiProviders(msg.data.aiProviders);
            if (msg.data.connectionHealth) global.AppState.setConnectionHealth(msg.data.connectionHealth);
          }
          break;

        case 'connection.status':
          if (msg.data) global.AppState.setConnectionHealth(msg.data);
          break;

        case 'cluster.health':
          var health = global.AppState.getState().connectionHealth || {};
          health.k8s = msg.data;
          global.AppState.setConnectionHealth(health);
          break;

        case 'git.sync':
          if (msg.data) global.AppState.setGitProviders(msg.data);
          break;

        case 'ai.latency':
          if (msg.data) global.AppState.setAiProviders(msg.data);
          break;

        case 'ai_provider_status':
          if (msg.data && msg.data.name) {
            var providers = global.AppState.getState().aiProviders || [];
            var updated = false;
            providers.forEach(function (p) {
              if (p.name === msg.data.name) {
                p.status = msg.data.status || 'unknown';
                p.latency = msg.data.latency || '—';
                updated = true;
              }
            });
            if (updated) {
              global.AppState.setAiProviders([].concat(providers));
            }
          }
          break;

        default:
          console.warn('[WS] Unknown message type:', msg.type);
      }
    }

    scheduleReconnect() {
      if (this.state === STATE.CONNECTING) return;
      this.state = STATE.CLOSED;

      if (this.reconnectAttempts >= MAX_RECONNECT) {
        console.error('[WS] Max reconnect attempts reached');
        if (global.AppState) global.AppState.setConnection('offline');
        return;
      }

      this.reconnectAttempts++;
      const delay = Math.min(BASE_DELAY * Math.pow(2, this.reconnectAttempts - 1), 30000);
      const jitter = delay * (0.5 + Math.random() * 0.5);

      console.log('[WS] Reconnecting in', Math.round(jitter), 'ms (attempt', this.reconnectAttempts + ')');
      if (global.AppState) global.AppState.setConnection('connecting');

      this.reconnectTimer = setTimeout(() => this.connect(), jitter);
    }

    cleanup() {
      if (this.reconnectTimer) {
        clearTimeout(this.reconnectTimer);
        this.reconnectTimer = null;
      }
      if (this.watchdogTimer) {
        clearTimeout(this.watchdogTimer);
        this.watchdogTimer = null;
      }
      if (this.ws) {
        this.ws.onopen = this.ws.onmessage = this.ws.onerror = this.ws.onclose = null;
        if (this.ws.readyState === WebSocket.OPEN || this.ws.readyState === WebSocket.CONNECTING) {
          try {
            this.ws.close();
          } catch (e) {}
        }
        this.ws = null;
      }
    }

    send(data) {
      if (this.ws && this.state === STATE.OPEN) {
        this.ws.send(JSON.stringify(data));
      }
    }

    disconnect() {
      this.reconnectAttempts = MAX_RECONNECT;
      this.cleanup();
      this.state = STATE.CLOSED;
      if (global.AppState) global.AppState.setConnection('offline');
    }
  }

  // Export as singleton
  global.WSClient = new WebSocketClient();

})(window);
