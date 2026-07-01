# API Contracts

## Base URL

```
http://localhost:8080/api/v1
```

## Authentication

All `/api/v1/*` routes require JWT Bearer token:

```
Authorization: Bearer <jwt-token>
```

JWT payload claims:
```json
{
  "sub": "user-id",
  "role": "platform_admin | tenant_admin | viewer",
  "tenant": "tenant-id"
}
```

Unauthenticated routes:
- `POST /api/v1/auth/login` — Login
- `GET /healthz` — Health check
- `GET /readyz` — Readiness check
- `GET /livez` — Liveness check
- `GET /metrics` — Prometheus metrics
- `GET /ping` — Heartbeat

---

## API Route Map

All routes below are prefixed with `/api/v1` and require JWT auth.

### Core Platform

| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| GET | `/` | inline | Service info (`{"service":"k8sselfhost","version":"0.1.0"}`) |
| POST | `/telemetry` | inline | Client telemetry ingestion |

### Incidents & RCA

| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| GET | `/incidents` | Dashboard | List incidents with filters |
| GET | `/incidents/{id}` | Dashboard | Get incident details |
| POST | `/incidents` | Dashboard | Create incident |
| PUT | `/incidents/{id}` | Dashboard | Update incident |
| GET | `/incidents/{id}/rca` | Dashboard | Get RCA report for incident |
| POST | `/incidents/{id}/rca` | Dashboard | Trigger AI RCA analysis |

### Fleet Management

| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| GET | `/fleet/clusters` | Fleet | List all registered clusters |
| POST | `/fleet/clusters` | Fleet | Register a new cluster |
| GET | `/fleet/clusters/{id}` | Fleet | Get cluster details |
| PUT | `/fleet/clusters/{id}` | Fleet | Update cluster config |
| DELETE | `/fleet/clusters/{id}` | Fleet | Remove cluster |
| GET | `/fleet/clusters/{id}/resources` | Fleet | List cluster resources |
| POST | `/fleet/clusters/{id}/test` | Fleet | Test cluster connectivity |

### Drift Detection

| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| GET | `/drift` | Drift | List drift detections |
| POST | `/drift/scan` | Drift | Trigger drift scan |
| POST | `/drift/{id}/reconcile` | Drift | Reconcile drift |

### Deployments

| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| GET | `/deployments` | Deployments | List deployments |
| POST | `/deployments` | Deployments | Create deployment |
| GET | `/deployments/{id}` | Deployments | Get deployment |
| PUT | `/deployments/{id}/rollback` | Deployments | Rollback deployment |

### Change Management

| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| GET | `/changes` | Changes | List change requests |
| POST | `/changes` | Changes | Create change request |
| PUT | `/changes/{id}/approve` | Changes | Approve change |
| PUT | `/changes/{id}/reject` | Changes | Reject change |

### Promotions

| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| GET | `/promotions` | Promotion | List promotions |
| POST | `/promotions` | Promotion | Create promotion |
| PUT | `/promotions/{id}/execute` | Promotion | Execute promotion |

### Runbooks

| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| GET | `/runbooks` | Runbook | List runbooks |
| POST | `/runbooks` | Runbook | Create runbook |
| GET | `/runbooks/{id}` | Runbook | Get runbook |
| POST | `/runbooks/{id}/execute` | Runbook | Execute runbook |

### Resource Explorer

| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| GET | `/explorer` | Explorer | List resources across clusters |

### Docker (Swarm)

| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| GET | `/docker/services` | Docker | List Docker services |
| GET | `/docker/containers` | Docker | List Docker containers |
| POST | `/docker/services/{id}/scale` | Docker | Scale Docker service |

### AI & Agents

| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| POST | `/ai/rca` | AI | Trigger AI root cause analysis |
| POST | `/ai/chat` | AI | AI chat completion |
| GET | `/agents` | Agents | List agent profiles |
| GET | `/agents/{name}` | Agents | Get agent profile |

### Notifications

| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| GET | `/notifications` | Notification | List notifications |
| POST | `/notifications` | Notification | Create notification |
| PUT | `/notifications/{id}/read` | Notification | Mark as read |

### Automation

| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| GET | `/automation` | Automation | List automation rules |
| POST | `/automation` | Automation | Create automation rule |
| PUT | `/automation/{id}` | Automation | Update rule |

### Observability

| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| GET | `/observability/slos` | Observability | List SLO definitions |

### Compliance

| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| GET | `/compliance` | Compliance | List compliance checks |

### Event Correlation

| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| GET | `/correlation` | Correlation | List correlated events |
| POST | `/correlation/analyze` | Correlation | Trigger correlation analysis |

### Other Modules

| Method | Path | Handler | Description |
|--------|------|---------|-------------|
| GET | `/capacity` | Capacity | Capacity planning data |
| GET | `/timeline` | Timeline | Event timeline |
| GET | `/tags` | Tagging | Resource tags |
| GET | `/reports-center` | Reporting | Generated reports |
| GET | `/health` | HealthCenter | Service health checks |
| GET | `/audit` | Audit | Audit log entries |
| GET | `/search` | Search | Global search |
| GET | `/cost` | Cost | Cost management data |
| GET | `/backup` | Backup | Backup status |

### WebSocket

| Path | Auth | Description |
|------|------|-------------|
| `GET /ws?token=<jwt>` | JWT via query param | Real-time event stream |

---

## Response Format

All API responses follow this structure:

```json
// Success
{
  "data": { ... },
  "meta": { "total": 100, "page": 1, "per_page": 20 }
}

// Error
{
  "error": "descriptive error message",
  "code": 400
}
```

## HTTP Status Codes

| Code | Usage |
|------|-------|
| `200 OK` | Successful read |
| `201 Created` | Resource created |
| `202 Accepted` | Async operation accepted |
| `400 Bad Request` | Invalid input |
| `401 Unauthorized` | Missing/invalid token |
| `403 Forbidden` | Insufficient role |
| `404 Not Found` | Resource not found |
| `500 Internal Server Error` | System failure |
