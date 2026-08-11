# Task: Eliminate Simulated Client-side Operations

Replace fake frontend delays and simulated log output sequences with real backend API interactions.

## Scope of Work

### 1. Integrate Real Kubernetes/Swarm Actions
- **File**: `frontend/modules/actions/action-center.js` (lines 266–358)
- **Fix**: Replace client-side simulation delay (`setTimeout` kubectl stubs) with active AJAX requests to `/api/v1/deployments/scale`, `/api/v1/deployments/restart`, `/api/v1/deployments/delete`, etc., displaying real rollout statuses.

### 2. Connect App Marketplace Deployment
- **File**: `frontend/modules/platform/enterprise-marketplace.js` (lines 121–157)
- **Fix**: Connect the "Deploy Application" action to trigger active application configuration creations via `POST /api/v1/deployments`.

### 3. Connect Cluster Provisioning Wizard
- **File**: `frontend/modules/platform/enterprise-provisioning.js` (lines 72–140)
- **Fix**: Wire the Terraform provisioning launch step to submit a cluster registration call to `POST /api/v1/fleet`.
