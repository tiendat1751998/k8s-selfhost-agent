# TASK: Refactor frontend/core/services/ws-client.js
## Target File Path: frontend/core/services/ws-client.js
## Current Status: SUCCESS

## Critical Issues Found
- **Memory/Event Listener Leak**: Reconnection logic creates a new WebSocket instance but `cleanup()` uses `ws.onopen = null` etc. However, if multiple reconnects overlap, stale timers might fire.
- **Duplicate Utility Logic**: `sanitize` function is implemented redundantly across multiple files instead of utilizing a shared utility.
- **Global Pollution**: Attaches directly to `window` instead of using proper module exports or an ES6 class structure.

## Action Plan & Refactoring Rules
1. Implement a clear state machine for WebSocket connections (CONNECTING, OPEN, CLOSED) to prevent overlapping connection attempts.
2. Extract the `sanitize` utility to a common core file.
3. Wrap the logic in an IIFE cleanly or migrate to standard ES6 classes/modules to prevent global scope pollution (while maintaining the required integration).
