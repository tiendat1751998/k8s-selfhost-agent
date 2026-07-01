---
name: DevOps
description: Instructions for container orchestration, Kubernetes/Swarm deployment setups, CI/CD pipelines, and infrastructure scaling.
---

# AGENTS.md — DevOps Engineer Workflow

## Session Startup (MANDATORY)

Before doing anything:

1. Read `.agents/context/deployment-topology.md` — know the infrastructure.
2. Read `.agents/context/architecture.md` — know the system design.
3. Read `.agents/context/security-policies.md` — know container security requirements.
4. Read `.agents/TASK_LOG.md` (if exists) — know current task state.

**NEVER start work without knowing the infrastructure topology.**

---

## Workflow Overview

All deployment tasks follow a 5-step workflow:

```
1. Read Topology → 2. Write Dockerfile/Compose → 3. Validate → 4. Write Helm Charts → 5. Deploy & Verify
```

---

## Step 1: Read Topology

Before modifying any deployment files:

1. Understand the service map — the system consists of **3 services**:
   - **Go Backend** (port 8080) — standalone binary serving REST API + WebSocket + Frontend SPA.
   - **ADK Playground** (port 8200) — Python uvicorn server, 10 specialist agents.
   - **Frontend** — static SPA served by the Go backend at `/*`.

2. Understand the infrastructure dependencies — **3 infrastructure components**:
   - **PostgreSQL 16** (port 5432) — 24 migration files, pgx/v5 driver, connection pool max 25.
   - **Redis 7** (port 6379) — go-redis/v9, cache DB0, maxmemory 256mb LRU.
   - **NATS JetStream** (port 4222) — stream `INCIDENTS`, subjects `incidents.>`.

3. Understand the network topology:
   ```
   Internet → Ingress → Service (ClusterIP:8080) → Go Backend Pod
                                                         │
                                    ┌────────────────────┼────────────────────┐
                                    ▼                    ▼                    ▼
                              PostgreSQL:5432       Redis:6379          NATS:4222
   ```

4. Check existing deployment files:
   - `deployments/docker/Dockerfile` — multi-stage (golang:1.23-alpine → distroless).
   - `deployments/docker/docker-compose.yml` — 4 services (app, postgres, redis, nats).
   - `deployments/helm/k8sselfhost/` — Helm chart (Chart v0.1.0, App v0.1.0).
   - `deployments/k8s/argocd-app.yaml` — ArgoCD Application (auto-sync, self-heal).
   - `.github/workflows/ci.yml` — CI pipeline (lint → test → build).

---

## Step 2: Write Dockerfile / Compose

When creating or updating container images and compose files:

### Dockerfile Rules (MANDATORY)

| Rule | Detail |
|------|--------|
| **Multi-stage builds** | Builder: `golang:1.23-alpine`. Runtime: `gcr.io/distroless/static-debian12:nonroot`. |
| **Non-root user** | `USER nonroot:nonroot` — NEVER run the app as root. |
| **Layer caching** | Copy `go.mod` + `go.sum` first → `go mod download` → copy source. |
| **.dockerignore** | Exclude `.git`, `*.exe`, `*.mp3`, `*.mp4`, `__pycache__`, `orchestrator-agent/.venv`. |
| **Pin versions** | DO NOT use `latest` — pin specifically: `golang:1.23-alpine`, `postgres:16-alpine`. |
| **Healthcheck** | `wget --spider -q http://localhost:8080/livez` for the app service. |
| **Static binary** | `CGO_ENABLED=0 GOOS=linux GOARCH=amd64` for cross-compilation. |
| **Strip debug** | `-ldflags="-s -w"` to reduce binary size. |

### Reference Dockerfile

```dockerfile
# Build stage
FROM golang:1.23-alpine AS builder
RUN apk add --no-cache git ca-certificates tzdata
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /app/bin/server ./cmd/server
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /app/bin/agent-runner ./cmd/agent-runner

# Runtime stage
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /app/bin/server /server
COPY --from=builder /app/bin/agent-runner /agent-runner
COPY --from=builder /app/migrations /migrations
COPY --from=builder /app/frontend /frontend
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/server"]
```

### Docker Compose Rules (k8sselfhost)

- Use the `services:` format (Compose v2 — no `version` key needed).
- Defined services: `app`, `postgres`, `redis`, `nats`.
- Named volumes for persistent data: `postgres_data`, `redis_data`, `nats_data`.
- Network: `k8sselfhost` (bridge driver).
- Mandatory health checks for all services.
- `depends_on` with `condition: service_healthy` to ensure startup ordering.
- Environment variables prefix: `K8S_` (e.g., `K8S_POSTGRES_HOST`, `K8S_REDIS_PORT`).

### Docker Compose Environment Variables

| Variable | Description | Example |
|----------|-------------|---------|
| `K8S_SERVER_HOST` | Server bind address | `0.0.0.0` |
| `K8S_SERVER_PORT` | Server port | `8080` |
| `K8S_POSTGRES_HOST` | PostgreSQL hostname | `postgres` |
| `K8S_POSTGRES_PORT` | PostgreSQL port | `5432` |
| `K8S_POSTGRES_USER` | PostgreSQL user | `postgres` |
| `K8S_POSTGRES_PASSWORD` | PostgreSQL password | (from secret) |
| `K8S_POSTGRES_DBNAME` | Database name | `k8sselfhost` |
| `K8S_POSTGRES_SSLMODE` | SSL mode | `disable` |
| `K8S_REDIS_HOST` | Redis hostname | `redis` |
| `K8S_REDIS_PORT` | Redis port | `6379` |
| `K8S_NATS_URL` | NATS connection URL | `nats://nats:4222` |
| `K8S_LOG_LEVEL` | Log level | `info` / `debug` |
| `K8S_LLM_PROVIDER` | LLM provider | `ollama` / `gemini` |
| `K8S_LLM_ENDPOINT` | LLM API endpoint | `http://ollama:11434` |
| `K8S_LLM_MODEL` | LLM model name | `llama3` / `gemini-2.0-flash` |
| `ENCRYPTION_KEY` | 32-byte AES-256 key | (from secret) |
| `JWT_SECRET` | JWT signing key | (from secret) |

### Docker Swarm Constraints

When deploying to Docker Swarm:

- Place labels in `deploy.labels`, NOT at the service level.
- Named volumes persist data — anonymous volumes DO NOT.
- `docker stack deploy` REMOVES services not present in the YAML — always deploy the full stack.
- Resource limits: `deploy.resources.limits` for memory and CPU.
- Replicas: `deploy.replicas` — default 1, scale via `docker service scale`.
- Update strategy: `deploy.update_config` with `order: start-first` for zero-downtime updates.

---

## Step 3: Validate

Before deploying, run the validation pipeline:

```bash
# 1. Build Docker image
docker build -t k8sselfhost:latest -f deployments/docker/Dockerfile .

# 2. Lint Dockerfile
hadolint deployments/docker/Dockerfile

# 3. Validate docker-compose
docker compose -f deployments/docker/docker-compose.yml config --quiet

# 4. Helm lint
helm lint deployments/helm/k8sselfhost/

# 5. Helm template render (dry-run)
helm template k8sselfhost deployments/helm/k8sselfhost/ --debug

# 6. Kubernetes manifest validation
helm template k8sselfhost deployments/helm/k8sselfhost/ | kubeval --strict

# 7. Security scan container image
trivy image --severity CRITICAL,HIGH k8sselfhost:latest
```

### Validation Rules

- **All validations must pass** — if any step fails, fix it before deploying.
- Never skip validation because "it should be correct".
- If Helm template fails → fix the templates before packaging.
- If Trivy scan finds a CRITICAL CVE → fix or accept the risk before deploying.

---

## Step 4: Write Helm Charts

When creating or updating Helm charts for Kubernetes deployment:

### Chart Structure

```
deployments/helm/k8sselfhost/
├── Chart.yaml              # Chart metadata (v0.1.0)
├── values.yaml             # Default values
└── templates/
    ├── _helpers.tpl         # Template helpers (labels, selectors, fullname)
    ├── deployment.yaml      # Main app Deployment
    ├── service.yaml         # ClusterIP Service (port 8080)
    ├── serviceaccount.yaml  # ServiceAccount
    ├── ingress.yaml         # Ingress (disabled by default)
    ├── hpa.yaml             # HorizontalPodAutoscaler
    ├── pdb.yaml             # PodDisruptionBudget
    ├── networkpolicy.yaml   # NetworkPolicy
    ├── secrets.yaml         # Kubernetes Secret (postgres password)
    ├── configmap-grafana.yaml # Grafana dashboard ConfigMap
    └── prometheusrule.yaml  # PrometheusRule alerts
```

### Helm Rules (MANDATORY)

| Rule | Detail |
|------|--------|
| **Templating** | Use `_helpers.tpl` for common labels, selectors, and fullname. |
| **Values-driven** | All configurable values must be in `values.yaml` — no hardcoding. |
| **Resource limits** | requests: CPU 100m, Memory 128Mi / limits: CPU 500m, Memory 256Mi. |
| **HPA** | minReplicas: 1, maxReplicas: 5, target CPU: 80%. |
| **PDB** | minAvailable: 1 for production. |
| **Secrets** | Sensitive values (passwords, keys) in `secrets.yaml` — referencing via `secretKeyRef`. |
| **Probes** | Liveness: `/livez` (delay 10s, period 30s), Readiness: `/readyz` (delay 5s, period 10s). |
| **Security** | `runAsNonRoot: true`, `readOnlyRootFilesystem: true`, drop ALL capabilities. |
| **Annotations** | Prometheus scrape: `prometheus.io/scrape: "true"`, port: `"8080"`, path: `"/metrics"`. |
| **NetworkPolicy** | Restrict ingress/egress for least-privilege access. |

### Values.yaml Key Sections

```yaml
# Infrastructure connections
config:
  postgres:
    host: postgresql    # In-cluster PostgreSQL service name
    port: 5432
    maxConns: 25
    minConns: 5
  redis:
    host: redis-master  # In-cluster Redis service name
    port: 6379
  nats:
    url: nats://nats:4222
  llm:
    provider: ollama    # Or "gemini" for cloud
    endpoint: http://ollama:11434
    model: llama3

# Subchart dependencies
postgresql:
  enabled: true         # Deploy PostgreSQL as subchart
redis:
  enabled: true         # Deploy Redis as subchart
  architecture: standalone
nats:
  enabled: true         # Deploy NATS as subchart
  jetstream:
    enabled: true
```

---

## Step 5: Deploy & Verify

### Docker Compose (Local Development)

```bash
# Start all services
make docker-up

# Verify all services healthy
docker compose -f deployments/docker/docker-compose.yml ps

# Check logs
make docker-logs

# Run migrations
make migrate

# Verify API responds
curl -s http://localhost:8080/healthz
curl -s http://localhost:8080/livez

# Stop services
make docker-down
```

### Kubernetes (Helm)

```bash
# Install / upgrade
helm upgrade --install k8sselfhost deployments/helm/k8sselfhost/ \
  --namespace k8sselfhost --create-namespace \
  --values deployments/helm/k8sselfhost/values.yaml

# Verify deployment
kubectl -n k8sselfhost get pods
kubectl -n k8sselfhost get svc
kubectl -n k8sselfhost logs -l app.kubernetes.io/name=k8sselfhost

# Verify health
kubectl -n k8sselfhost exec deploy/k8sselfhost -- wget -qO- http://localhost:8080/healthz

# Rollback if needed
helm rollback k8sselfhost -n k8sselfhost
```

### ArgoCD (GitOps)

ArgoCD Application configured at `deployments/k8s/argocd-app.yaml`:

```yaml
syncPolicy:
  automated:
    prune: true       # Remove resources not in Git
    selfHeal: true    # Auto-reconcile drift
  retry:
    limit: 5
    backoff:
      duration: 5s
      factor: 2
      maxDuration: 3m
```

---

## CI/CD Pipeline (GitHub Actions)

Pipeline: `.github/workflows/ci.yml`

```
Push/PR → Lint (golangci-lint) → Test (go test -race) → Build (binary + Docker image)
```

### Pipeline Stages

| Stage | Tool | Purpose |
|-------|------|---------|
| **Lint** | `golangci-lint` | Static analysis, code quality |
| **Test** | `go test -race -coverprofile` | Unit tests with race detection |
| **Build** | `go build` + `docker build` | Compile binaries, build container |

### CI Service Containers

Tests run against real services (not mocks):
- **PostgreSQL 16-alpine** on port 5432 (test DB: `k8sselfhost_test`)
- **Redis 7-alpine** on port 6379

---

## Infrastructure Constraints (k8sselfhost)

### Service Ports

| Service | Port | Protocol |
|---------|------|----------|
| Go Backend | 8080 | HTTP/WS |
| ADK Playground | 8200 | HTTP |
| PostgreSQL | 5432 | TCP |
| Redis | 6379 | TCP |
| NATS Client | 4222 | TCP |
| NATS Monitoring | 8222 | HTTP |
| Prometheus Metrics | 8080 (`/metrics`) | HTTP |

### Container Security

| Setting | Value |
|---------|-------|
| Base image | `gcr.io/distroless/static-debian12:nonroot` |
| User | `nonroot:nonroot` (UID 65534) |
| Root filesystem | Read-only |
| Privilege escalation | Disabled |
| Capabilities | ALL dropped |

---

## 🚫 ZERO-TOLERANCE ANTI-FAKE RULES (HIGHEST LEVEL)

**YOU MUST COMPLY WITH THE FOLLOWING RULES. VIOLATION = IMMEDIATE TERMINATION.**

### 1. NEVER fabricate data
- **DO NOT** fabricate deploy statuses, service replicas, or container health.
- **DO NOT** state "deploy success" unless you have run `kubectl get pods` or `docker ps` and pasted the output.
- **DO NOT** fabricate image build results — you must provide actual build logs.
- **DO NOT** state "healthy" unless you have queried the health check endpoint.

### 2. ALWAYS verify using actual tool outputs
- Every claim must be backed by **real tool output** (docker, kubectl, helm, curl).
- If you state "image built" → you **MUST** paste the `docker images` output.
- If you state "pod running" → you **MUST** paste the `kubectl get pods` output.
- If you state "service healthy" → you **MUST** paste the `curl /healthz` response.

### 3. DO NOT use "deploy command executed" as proof
- Running `docker compose up` **IS NOT** proof that services are healthy.
- Running `helm install` **IS NOT** proof that pods are running.
- **Always verify after deploy**: `docker ps`, `kubectl get pods`, `curl /healthz`.

### 4. If you cannot verify → state "CANNOT VERIFY"
- If Docker fails → report the error; do not fabricate success.
- If the image build fails → report the build logs; do not fabricate a pass.

### 5. Deploy = Real deploy + verification
- Deploy = run command → verify pods → verify health → verify API.
- Build = run docker build → verify image exists → verify image size.

### 6. Every "SUCCESS" claim must include 3 things:
1. **Command you ran** (exact docker/kubectl/helm/curl command)
2. **Actual output** (pasted from the terminal)
3. **Relevant evidence** (replicas count, HTTP status, response body, pod status)

**YOU WILL BE REJECTED IF YOU CANNOT PROVE.**

---

## 📤 ORCHESTRATOR OUTPUT CONTRACT (MANDATORY)

When completing a task, you **MUST** end the output with this section.
This is the standard format for the orchestrator to parse and aggregate results.

### Format (copy and fill):

```markdown
## ORCHESTRATOR SUMMARY

**Status**: SUCCESS | PARTIAL | FAILED
**Report**: .agents/reports/<filename.md>

### What was done:
- [bullet point 1]
- [bullet point 2]

### Verification evidence:
```
[paste tool output proof here — kubectl, docker, curl, helm]
```

### Issues/Blockers:
- [if any, otherwise write "None"]

### Recommended next steps:
- [if any]
```

### Rules:
1. **ALWAYS** include the ORCHESTRATOR SUMMARY section at the end of the output — this is critical.
2. **Status** must be clear: SUCCESS (all passed), PARTIAL (completed with minor issues), FAILED (not completed).
3. **Report path** must be the path to the report file.
4. **Verification evidence** must include actual tool output (terminal, curl, build log) — DO NOT use "should work".
5. If the task failed → specify the cause + suggest a fix.
6. The orchestrator will use this SUMMARY to aggregate all agent results — if missing, the results may be ignored.
