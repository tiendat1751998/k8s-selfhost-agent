---
title: "Phase 11a: File Size Refactoring (Enterprise Polish)"
status: "pending"
priority: "high"
assignee: "frontend-agent"
---

# Objective
Auditing file line counts showed that `frontend/modules/search/enterprise-search.js` exceeds the 500-line hard limit significantly (1,126 lines). We must refactor and split it into smaller, decoupled modules.

# Tasks
- [ ] **Split Search Module**: Split `frontend/modules/search/enterprise-search.js` into three separate JS files under `frontend/modules/search/`:
  - `search-index.js`: Handles indexing data, static mappings, and updating live state indices.
  - `search-adv-builder.js`: Handles compiling advanced log query builder rules, expression tree rendering, and query match evaluation.
  - `enterprise-search.js` (entry point): Restrict to UI tab switching, query forms submission, rendering result list structures, and integrating with `SearchCenter`.
- [ ] **Prune Swarm Module**: Review and split/prune `frontend/modules/provider/docker-swarm.js` (512 lines) to keep it under 500 lines.
- [ ] **Prune Actions Module**: Review and split/prune `frontend/modules/actions/action-center.js` (507 lines) to keep it under 500 lines.
- [ ] **HTML/CSS Consolidation**: Verify that all stylesheets and HTML files are organized cleanly.

# Acceptance Criteria
- No Javascript module exceeds 500 lines of code.
- Decoupled modules are dynamically registered and correctly loaded in the application.
- All search functionality (Global Search autocomplete, main logs query builder, and Git tracer) behaves identically.
