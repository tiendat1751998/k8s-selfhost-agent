# Performance Budgets

## API Response Time Targets

| Endpoint Type | p50 | p99 | Max |
|--------------|-----|-----|-----|
| REST GET (list) | <5ms | <20ms | 100ms |
| REST GET (single) | <2ms | <10ms | 50ms |
| REST POST/PUT | <5ms | <25ms | 100ms |
| WebSocket message | <1ms | <5ms | 20ms |
| AI RCA analysis | <5s | <15s | 30s |

---

## Database Performance

| Metric | Target |
|--------|--------|
| Simple SELECT queries | <2ms |
| JOIN queries | <5ms |
| INSERT/UPDATE | <5ms |
| Connection pool max | 25 connections |
| Connection pool min | 5 connections |
| Slow query threshold | >100ms (log warning) |
| N+1 prevention | Batch load relations |

---

## Cache Performance (Redis)

| Metric | Target |
|--------|--------|
| Cache hit ratio | >= 95% |
| GET latency | <0.5ms p99 |
| SET latency | <1ms p99 |
| TTL default | 5 minutes |

---

## Infrastructure Targets

| Component | Metric | Target |
|-----------|--------|--------|
| Go Backend | Memory | <256Mi (pod limit) |
| Go Backend | CPU | <500m (pod limit) |
| Go Backend | Startup time | <3s |
| PostgreSQL | Max connections | 25 |
| NATS | Message delivery | <10ms |
| Container | Image size | Minimal (distroless/alpine) |

---

## Kubernetes Resource Limits

```yaml
resources:
  requests:
    cpu: 100m
    memory: 128Mi
  limits:
    cpu: 500m
    memory: 256Mi
```

## Autoscaling

| Metric | Value |
|--------|-------|
| Min replicas | 1 |
| Max replicas | 5 |
| Scale trigger | CPU > 80% |

---

## SLO/SLI Definitions

| SLI | SLO Target |
|-----|------------|
| API Availability | >= 99.9% |
| Incident Detection Time | <30s |
| RCA Generation Time | <15s |
| Drift Scan Interval | Every 5 minutes |
| WebSocket Connection Stability | 99.5% uptime |

---

## Telemetry & Observability

| Component | Technology |
|-----------|-----------|
| Metrics | Prometheus (`/metrics` endpoint) |
| Tracing | OpenTelemetry (OTLP gRPC export) |
| Logging | zap structured JSON logs |
| Health checks | `/healthz`, `/readyz`, `/livez` |
| Profiling | pprof (gated by `K8S_ENABLE_PPROF=true`) |
