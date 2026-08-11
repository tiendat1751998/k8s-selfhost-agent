# Task: Eliminate Hardcoded State, Simulated Latencies, & Mock Data

Refactor the frontend modules to query live API endpoints, replacing simulated variables and random maths.

## Scope of Work

### 1. Remove Hardcoded Enterprise State
- **File**: `frontend/modules/platform/enterprise.js` (lines 10–91)
- **Fix**: Expose REST endpoints under `/api/v1/tenancy/` to retrieve organizations, projects, members, and RBAC values. Update `enterprise.js` to fetch this state dynamically from PostgreSQL.

### 2. Live Cost Trends & Optimization Recommendations
- **File**: `frontend/modules/cost/cost-management.js` (lines 204–213, 237–245)
- **Fix**: Query live statistical trends and advice from the `/api/v1/cost/summary` REST API.

### 3. Rename confusing cluster variables
- **File**: `frontend/modules/fleet/fleet-view.js` (lines 88, 107)
- **Fix**: Rename the property storing live API data from `this.mocks` to `this.clusters` or `this.fleetData`.

### 4. Fetch Live connection Latencies
- **File**: `frontend/modules/topology/topology.js` (lines 306–307)
- **Fix**: Replace random math simulations (`Math.random()`) with live telemetry/Prometheus measurements queried from the backend metrics endpoints.
