---
title: "Phase 2: Live Kubernetes Integration"
status: "completed"
priority: "high"
assignee: "backend-agent"
---

# Objective
Replace mock cluster data with real, live data fetched directly from Kubernetes API servers.

# Scope
Refactor the following domains to use Kubernetes `client-go`:
1. **Resource Explorer (`ExplorerRepo`)**: Fetch actual Pods, Deployments, Services, ConfigMaps, etc.
2. **Capacity Management (`CapacityRepo`)**: Fetch live CPU/Memory usage metrics from `metrics.k8s.io`.
3. **Health Center (`HealthCenterRepo`)**: Ping actual cluster health endpoints and node statuses.

# Tasks
- [x] Review `internal/infrastructure/kubernetes/client.go` to ensure it supports multi-cluster dynamic client generation (using credentials stored in `fleet_clusters` DB table).
- [x] Implement `kubernetes.ExplorerRepo` that takes a `cluster_id`, looks up the Kubeconfig/Token from Postgres, and queries the K8s API.
- [x] Implement `kubernetes.CapacityRepo` to integrate with the Kubernetes Metrics API.
- [x] Implement `kubernetes.HealthCenterRepo` to accurately reflect node readiness and API server availability.
- [x] Update `cmd/server/main.go` to wire these new repositories.

# Acceptance Criteria
- Dashboard reflects real-time Kubernetes objects.
- Creating a deployment in K8s immediately shows up in the Explorer view.
- Capacity graphs show real resource utilization, not static mock numbers.
