---
title: "Phase 10: Security, Auth & RBAC (Production Hardening)"
status: "pending"
priority: "critical"
assignee: "backend-agent"
---

# Objective
A production Kubernetes control plane cannot operate without strict Authentication and Role-Based Access Control (RBAC). The current system allows unauthenticated API calls.

# Scope
HTTP Router (`internal/adapter/http`), Database schemas, Kubernetes Impersonation.

# Tasks
- [ ] **Authentication Middleware**: Implement JWT/OIDC based authentication. Protect all `/api/v1/` and `/ws` endpoints.
- [ ] **Multi-Tenancy & RBAC**: Introduce Users, Roles, and Tenants in the database. Ensure users can only see the Clusters and Runbooks they have permission to access.
- [ ] **Secret Management**: Do NOT store Kubeconfig tokens or Docker TLS certs in plaintext in PostgreSQL. Integrate with HashiCorp Vault or encrypt them at rest using AES-GCM.
- [ ] **Audit Logging**: Ensure the `postgres.AuditRepo` (from Phase 1) records the exact `UserID` making the change, along with their IP address and action.
- [ ] **Kubernetes Impersonation**: When executing commands on the actual K8s cluster, impersonate the logged-in user via `Impersonate-User` headers in `client-go` instead of using the global Admin token.

# Acceptance Criteria
- `curl` requests without a valid `Authorization` header receive HTTP 401.
- Sensitive cluster credentials are encrypted at rest.
- Actions performed via the UI strictly respect the RBAC permissions of the logged-in user.
