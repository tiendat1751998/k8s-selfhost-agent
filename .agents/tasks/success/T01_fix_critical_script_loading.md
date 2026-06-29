# TASK: Fix Application-Breaking Script Loading

## Priority: 🔴 CRITICAL — App crashes on load
## Status: COMPLETED
## Estimated Effort: 15 minutes

---

## Problem Description

The application is completely broken on page load due to missing/misordered script tags in `index.html`.

## Sub-Tasks

### 1.1 — Restore `globalStore.js` as first script tag
- **File**: `frontend/index.html` (script section ~L2297)
- **What**: `<script src="/core/state/globalStore.js">` was removed during a refactoring pass.
- **Impact**: `AppState` is undefined → **every module** that calls `AppState.on()`, `AppState.emit()`, `AppState.setXxx()` crashes with `ReferenceError`.
- **Fix**: Add `<script src="/core/state/globalStore.js"></script>` as the **very first** `<script>` tag, before `auth.js` and everything else.
- **Verification**: Open browser console → no `ReferenceError: AppState is not defined`.

### 1.2 — Fix duplicate `window.fetch` override collision
- **File 1**: `frontend/modules/auth/auth.js` (L4-5)
- **File 2**: `frontend/core/services/api-client.js` (L10-15)
- **What**: Both files execute `const originalFetch = window.fetch; window.fetch = function(...)`. Whichever loads second overwrites the first's wrapper, meaning either:
  - Auth headers are never injected (if api-client loads second), OR
  - Caching/coalescing never works (if auth loads second)
- **Fix**: Rewrite `auth.js` to use a non-destructive wrapper pattern. Since `auth.js` loads first, `api-client.js` will capture auth's wrapper as its "original", creating a proper chain: `api-client → auth → native fetch`.
- **Key code change in auth.js**:
  ```js
  // BEFORE (broken):
  const originalFetch = window.fetch;
  window.fetch = async function() { ... }
  
  // AFTER (correct chain):
  const nativeFetch = window.fetch;
  window.fetch = function(input, init) {
      // inject auth headers
      init.headers['Authorization'] = 'Bearer ' + token;
      return nativeFetch.call(this, input, init).then(response => {
          if (response.status === 401) showLoginModal();
          return response;
      });
  };
  ```
- **Verification**: Network tab shows `Authorization: Bearer ...` header on API calls AND duplicate requests are coalesced.

### 1.3 — Fix undefined CSS variable `--color-text` in login modal
- **File**: `frontend/index.html` (L26)
- **What**: Login modal uses `color:var(--color-text)` but this variable is not defined in `styles.css`. The correct variable is `--color-on-dark`.
- **Fix**: Replace `var(--color-text)` → `var(--color-on-dark)` in the login modal inline style.
- **Verification**: Login modal heading text is visible (not invisible/transparent).

## Files Involved
- `frontend/index.html`
- `frontend/modules/auth/auth.js`
- `frontend/core/services/api-client.js`
- `frontend/core/state/globalStore.js` (exists but not loaded)
