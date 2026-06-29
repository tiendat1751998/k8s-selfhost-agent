---
title: "Phase 6: Frontend JavaScript Refactoring & Optimization"
status: "pending"
priority: "high"
assignee: "frontend-agent"
---

# Objective
Clean up the "trash" UI code. Enforce proper error boundaries, modern async/await patterns, and eliminate boilerplate and duplicate utility functions.

# Scope
All JavaScript files in `frontend/modules/` and `frontend/app/`.

# Tasks
- [ ] **Async/Await Chains**: Audit all `fetch()` calls. Convert messy `.then().catch()` chains into clean `async/await` blocks with proper `try/catch` error boundaries.
- [ ] **Event-Driven Loops**: Prevent memory leaks by ensuring event listeners are properly managed (e.g., in `AppState.on`). Remove zombie event listeners when DOM elements are destroyed.
- [ ] **Deduplicate Utilities**: Centralize repeated functions (like `esc()`, `timeAgo()`, `badgeClass()`) into a shared `utils.js` or `helpers.js`.
- [ ] **State Management**: Consolidate scattered state updates. Ensure `AppState` is the single source of truth and mutations are predictable.
- [ ] **Error Boundaries**: Add global error handling for network failures so the UI doesn't silently break if the backend is down.

# Acceptance Criteria
- No duplicate helper functions across module files.
- All network calls are wrapped in `try/catch`.
- Console is completely clean of warnings and unhandled promise rejections.
