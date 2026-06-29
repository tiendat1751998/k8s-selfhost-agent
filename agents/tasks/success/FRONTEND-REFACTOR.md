# FRONTEND REFACTOR & ENTERPRISE STRUCTURE OPTIMIZATION

## ROLE

You are a Senior Frontend Architect responsible for restructuring a chaotic legacy frontend into a scalable enterprise-grade UI system.

Current problems:

- All files are placed inside `/js` and `/js/sections`
- No clear module boundaries
- Navigation menu is cluttered and unstructured
- UI features are mixed with logic
- No scalable architecture for large system (100+ pages/modules)

---

# OBJECTIVE

Refactor the entire frontend into a **modular, scalable, enterprise-grade architecture**.

The system must support:

- Kubernetes Control Plane UI
- GitOps Dashboard
- AI Operations Center
- Multi-tenant SaaS UI
- Action Center + Monitoring + Search system

---

# RULES (CRITICAL)

- DO NOT break existing functionality
- DO NOT remove features
- ONLY reorganize structure and improve architecture
- MUST maintain backward compatibility
- MUST keep UI working during refactor

---

# TARGET ARCHITECTURE

## NEW FOLDER STRUCTURE

Refactor `/js` into:

```
/frontend
 ├── /core
 │    ├── state/
 │    ├── router/
 │    ├── services/
 │    ├── utils/
 │    └── constants/
 │
 ├── /modules
 │    ├── /dashboard
 │    ├── /clusters
 │    ├── /deployments
 │    ├── /actions
 │    ├── /incidents
 │    ├── /ai
 │    ├── /gitops
 │    ├── /search
 │    ├── /catalog
 │    ├── /rbac
 │    └── /settings
 │
 ├── /components
 │    ├── ui/
 │    ├── layout/
 │    ├── charts/
 │    └── common/
 │
 ├── /services
 │    ├── api/
 │    ├── websocket/
 │    ├── cache/
 │    └── integrations/
 │
 ├── /hooks
 ├── /styles
 └── /app
```

---

# TASK 1 — MENU RESTRUCTURE (CRITICAL)

## Problem

Current menu is:

- flat
- ungrouped
- not scalable
- mixes actions + views

---

## Solution

Create **enterprise sidebar navigation system**

### NEW MENU STRUCTURE

```
Overview
│
├── Infrastructure
│   ├── Clusters
│   ├── Nodes
│   ├── Pods
│
├── Deployments
│   ├── Applications
│   ├── Rollouts
│   ├── Scaling
│
├── Operations
│   ├── Action Center
│   ├── Incidents
│   ├── Logs
│
├── GitOps
│   ├── Repositories
│   ├── Pull Requests
│   ├── Deploy History
│
├── AI Ops
│   ├── RCA Engine
│   ├── AI Chat
│   ├── Predictions
│
├── Search
│   ├── Global Search
│   ├── Logs Search
│   ├── Trace Search
│
├── Platform
│   ├── Catalog
│   ├── RBAC
│   ├── Settings
```

---

## Requirements

- collapsible sidebar groups
- icons per module
- active route highlighting
- role-based menu filtering (RBAC ready)
- lazy loading of modules

---

# TASK 2 — MODULE-BASED ARCHITECTURE

## REQUIREMENTS

Each module must be:

- self-contained
- have its own:
  - UI components
  - state logic
  - API layer
  - hooks

Example:

```
/modules/clusters/
   index.js
   components/
   services/
   hooks/
   styles/
```

---

# TASK 3 — ROUTER REFACTOR

## REQUIREMENTS

Replace flat routing with:

- module-based routing
- lazy loading
- code splitting

Example:

- /clusters → clusters module
- /actions → actions module

---

# TASK 4 — STATE MANAGEMENT CLEANUP

## REQUIREMENTS

Centralize state into:

- global state store
- module-local state
- API cache layer

Remove:

- scattered global variables
- duplicated state logic

---

# TASK 5 — UI DESIGN SYSTEM CLEANUP

## REQUIREMENTS

Standardize:

- buttons
- cards
- tables
- modals
- forms

Create reusable:

- BaseButton
- BaseCard
- BaseTable
- BaseModal

---

# TASK 6 — SERVICE LAYER ISOLATION

## REQUIREMENTS

Move all API calls into:

/services/api

Create separation:

- clusters API
- deployments API
- ai API
- gitops API

---

# TASK 7 — PERFORMANCE OPTIMIZATION

## REQUIREMENTS

- lazy load modules
- virtualized tables
- memoized components
- debounce search inputs
- avoid full page reloads

---

# TASK 8 — BACKWARD COMPATIBILITY

## REQUIREMENTS

- old URLs must still work
- old UI flow must not break
- gradual migration allowed

---

# FINAL GOAL

After refactor:

System becomes:

- scalable frontend architecture
- enterprise control plane UI
- module-based system like:
  - Rancher UI
  - Datadog UI
  - OpenShift Console

---

# EXECUTION RULE

- refactor incrementally
- one module at a time
- test after each change
- never break UI flow