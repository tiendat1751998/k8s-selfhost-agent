---
title: "Phase 8b: Advanced Filtering & Saved Searches"
status: "success"
priority: "medium"
assignee: "frontend-agent"
---

# Objective
Implement advanced query filtering builder for Logs and Deployments, and support saving search filters.

# Scope
Frontend SPA (HTML/JS/CSS).

# Tasks
- [x] **Saved Searches**: Allow users to save current search term and filters in the Search Center.
- [x] **Advanced Filter Builder**: Support constructing query conditions (AND/OR, nested conditions) for Logs and Deployments.

# Acceptance Criteria
- Users can save log search queries, view them in a "Saved Searches" list, and load them with one click.
- Filter builder supports combining multiple fields (e.g. cluster, namespace, text query) with AND/OR logic.
