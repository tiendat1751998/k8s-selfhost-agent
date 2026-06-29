---
title: "Phase 3: Observability & Monitoring Integration"
status: "completed"
priority: "medium"
assignee: "backend-agent"
---

# Objective
Connect the system to real observability backends (Prometheus, Grafana, Loki, Elasticsearch) to provide live metrics and logs to the dashboard.

# Scope
Refactor the Observability domain:
1. **Observability (`ObservabilityRepo`)**: Replace mock metric generators with PromQL queries.

# Tasks
- [x] Implement a Prometheus client wrapper in `internal/infrastructure/provider/prometheus`.
- [x] Implement `prometheus.ObservabilityRepo` to execute actual PromQL queries for CPU/Memory/Network over time.
- [x] Implement log fetching from Elasticsearch or Loki for container logs.
- [x] Map the real Prometheus data structures to the internal `domain.Observability` entities.
- [x] Update `cmd/server/main.go` to wire the Observability repository.

# Acceptance Criteria
- Dashboard metric charts accurately plot data retrieved from Prometheus.
- The Logs viewer streams real container logs instead of mock strings.
