/**
 * Security Sanitization Layer — Bulletproof HTML escaping to prevent XSS.
 */
(function (global) {
  'use strict';

  const SECRET_PATTERN = /(?:token|secret|password|api_key|apikey|auth)\s*[:=]\s*\S+/gi;

  function escapeHTML(str) {
    if (str === null || str === undefined) return '';
    return String(str)
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;')
      .replace(/'/g, '&#x27;')
      .replace(/\//g, '&#x2F;')
      .replace(/`/g, '&#x60;');
  }

  // Sanitize object recursively to clean any user imported JSON configs
  function sanitizeObject(obj) {
    if (!obj || typeof obj !== 'object') {
      if (typeof obj === 'string') {
        return escapeHTML(obj);
      }
      return obj;
    }
    if (Array.isArray(obj)) {
      return obj.map(sanitizeObject);
    }
    const clean = {};
    for (const key in obj) {
      if (Object.prototype.hasOwnProperty.call(obj, key)) {
        clean[key] = sanitizeObject(obj[key]);
      }
    }
    return clean;
  }

  function redactSecrets(text) {
    if (typeof text !== 'string') return text;
    return text.replace(SECRET_PATTERN, '[REDACTED]');
  }

  global.Security = {
    escapeHTML: escapeHTML,
    sanitizeObject: sanitizeObject,
    redactSecrets: redactSecrets
  };

})(window);
