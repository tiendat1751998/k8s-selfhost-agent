---
name: DevOps
description: Instructions for container orchestration, Kubernetes/Swarm deployment setups, CI/CD pipelines, and infrastructure scaling.
---

# AGENTS.md — DevOps Engineer Workflow

## Session Startup (MANDATORY)

Before doing anything:

1. Read `.agents/context/deployment-topology.md` — know the infrastructure
2. Read `.agents/context/architecture.md` — know the system design
3. Read `.agents/context/security-policies.md` — know container security requirements
4. Read `.agents/TASK_LOG.md` (if exists) — know current task state

**NEVER start work without knowing the infrastructure topology.**

---

## Workflow Overview

Mọi task deployment đều tuân theo workflow 5 bước:

```
1. Read Topology → 2. Write Dockerfile/Compose → 3. Validate → 4. Write Helm Charts → 5. Deploy & Verify
```

---

## Bước 1: Read Topology

Trước khi chỉnh sửa bất kỳ file deployment nào:

1. Hiểu service map — hệ thống có **3 services**:
   - **Go Backend** (port 8080) — standalone binary serving REST API + WebSocket + Frontend SPA
   - **ADK Playground** (port 8200) — Python uvicorn server, 10 specialist agents
   - **Frontend** — static SPA served by Go backend tại `/*`

2. Hiểu infrastructure dependencies — **3 infrastructure components**:
   - **PostgreSQL 16** (port 5432) — 24 migration files, pgx/v5 driver, pool max 25
   - **Redis 7** (port 6379) — go-redis/v9, cache DB0, maxmemory 256mb LRU
   - **NATS JetStream** (port 4222) — stream `INCIDENTS`, subjects `incidents.>`

3. Hiểu network topology:
   ```
   Internet → Ingress → Service (ClusterIP:8080) → Go Backend Pod
                                                         │
                                    ┌────────────────────┼────────────────────┐
                                    ▼                    ▼                    ▼
                              PostgreSQL:5432       Redis:6379          NATS:4222
   ```

4. Check deployment files hiện có:
   - `deployments/docker/Dockerfile` — multi-stage (golang:1.23-alpine → distroless)
   - `deployments/docker/docker-compose.yml` — 4 services (app, postgres, redis, nats)
   - `deployments/helm/k8sselfhost/` — Helm chart (Chart v0.1.0, App v0.1.0)
   - `deployments/k8s/argocd-app.yaml` — ArgoCD Application (auto-sync, self-heal)
   - `.github/workflows/ci.yml` — CI pipeline (lint → test → build)

---

## Bước 2: Write Dockerfile / Compose

Khi tạo hoặc cập nhật container images và compose files:

### Dockerfile Rules (bắt buộc)

| Quy tắc | Chi tiết |
|---------|----------|
| **Multi-stage builds** | Builder: `golang:1.23-alpine`. Runtime: `gcr.io/distroless/static-debian12:nonroot` |
| **Non-root user** | `USER nonroot:nonroot` — KHÔNG BAO GIỜ chạy app với root |
| **Layer caching** | Copy `go.mod` + `go.sum` trước → `go mod download` → copy source |
| **.dockerignore** | Loại bỏ `.git`, `*.exe`, `*.mp3`, `*.mp4`, `__pycache__`, `orchestrator-agent/.venv` |
| **Pin versions** | KHÔNG dùng `latest` — pin cụ thể: `golang:1.23-alpine`, `postgres:16-alpine` |
| **Healthcheck** | `wget --spider -q http://localhost:8080/livez` cho app service |
| **Static binary** | `CGO_ENABLED=0 GOOS=linux GOARCH=amd64` cho cross-compilation |
| **Strip debug** | `-ldflags="-s -w"` để giảm binary size |

### Dockerfile hiện tại (reference)

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

- Dùng `services:` format (Compose v2 — không cần `version` key)
- Defined services: `app`, `postgres`, `redis`, `nats`
- Named volumes cho persistent data: `postgres_data`, `redis_data`, `nats_data`
- Network: `k8sselfhost` (bridge driver)
- Health checks bắt buộc cho tất cả services
- `depends_on` với `condition: service_healthy` để đảm bảo startup order
- Environment variables prefix: `K8S_` (ví dụ: `K8S_POSTGRES_HOST`, `K8S_REDIS_PORT`)

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

Khi deploy lên Docker Swarm:

- Đặt labels trong `deploy.labels`, KHÔNG phải ở service level
- Named volumes persist data — anonymous volumes KHÔNG persist
- `docker stack deploy` REMOVES services không có trong YAML — luôn deploy full stack
- Resource limits: `deploy.resources.limits` cho memory và CPU
- Replicas: `deploy.replicas` — default 1, scale qua `docker service scale`
- Update strategy: `deploy.update_config` với `order: start-first` cho zero-downtime

---

## Bước 3: Validate

Trước khi deploy, chạy validation pipeline:

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

# 6. Kubernetes manifest validation (if kubeval installed)
helm template k8sselfhost deployments/helm/k8sselfhost/ | kubeval --strict

# 7. Security scan container image
trivy image --severity CRITICAL,HIGH k8sselfhost:latest
```

### Validation Rules

- **Tất cả validation phải pass** — nếu có bất kỳ step nào fail, fix trước khi deploy
- Không bao giờ skip validation vì "chắc chắn đúng"
- Nếu Helm template fail → fix templates trước khi package
- Nếu trivy scan tìm CRITICAL CVE → fix hoặc accept risk trước khi deploy
- Nếu hadolint warning → evaluate và fix nếu có thể

---

## Bước 4: Write Helm Charts

Khi tạo hoặc cập nhật Helm charts cho Kubernetes deployment:

### Chart Structure (hiện tại)

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

### Helm Rules (bắt buộc)

| Quy tắc | Chi tiết |
|---------|----------|
| **Templating** | Dùng `_helpers.tpl` cho common labels, selectors, fullname |
| **Values-driven** | Mọi configurable value đều trong `values.yaml` — không hardcode |
| **Resource limits** | requests: CPU 100m, Memory 128Mi / limits: CPU 500m, Memory 256Mi |
| **HPA** | minReplicas: 1, maxReplicas: 5, target CPU: 80% |
| **PDB** | minAvailable: 1 cho production |
| **Secrets** | Sensitive values (passwords, keys) trong `secrets.yaml` — referencing via `secretKeyRef` |
| **Probes** | Liveness: `/livez` (delay 10s, period 30s), Readiness: `/readyz` (delay 5s, period 10s) |
| **Security** | `runAsNonRoot: true`, `readOnlyRootFilesystem: true`, drop ALL capabilities |
| **Annotations** | Prometheus scrape: `prometheus.io/scrape: "true"`, port: `"8080"`, path: `"/metrics"` |
| **NetworkPolicy** | Restrict ingress/egress cho least-privilege access |

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

## Bước 5: Deploy & Verify

### Docker Compose (Local Development)

```bash
# Start all services
make docker-up
# hoặc: docker compose -f deployments/docker/docker-compose.yml up -d

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

- Source: `deployments/helm/k8sselfhost/` from repo `main` branch
- Destination: `k8sselfhost` namespace
- Auto-sync enabled — push to `main` triggers deploy

### Verification Checklist

Sau khi deploy, verify:

```bash
# 1. Pods running
kubectl -n k8sselfhost get pods -o wide

# 2. Services accessible
kubectl -n k8sselfhost get svc

# 3. Health check
curl -s http://<endpoint>/healthz | jq .

# 4. API responds
curl -s -H "Authorization: Bearer <token>" http://<endpoint>/api/v1/ | jq .

# 5. WebSocket connection
wscat -c ws://<endpoint>/ws?token=<token>

# 6. Metrics available
curl -s http://<endpoint>/metrics | head -20

# 7. Logs clean (no ERROR entries)
kubectl -n k8sselfhost logs deploy/k8sselfhost --tail=50 | grep -i error
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

### Makefile Targets

| Target | Command | Purpose |
|--------|---------|---------|
| `make build` | `go build` | Build server + agent-runner binaries |
| `make test` | `go test ./... -race` | Run all tests |
| `make lint` | `golangci-lint run` | Lint code |
| `make fmt` | `go fmt` + `goimports` | Format code |
| `make run` | `go run ./cmd/server` | Run server locally |
| `make docker-build` | `docker build` | Build Docker image |
| `make docker-up` | `docker compose up -d` | Start Docker Compose |
| `make docker-down` | `docker compose down` | Stop Docker Compose |
| `make migrate` | `psql -f migrations/*.sql` | Apply DB migrations |
| `make tidy` | `go mod tidy` | Tidy Go modules |

---

## Infrastructure Constraints (k8sseflhost)

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

### Resource Limits (Kubernetes)

```yaml
resources:
  requests:
    cpu: 100m
    memory: 128Mi
  limits:
    cpu: 500m
    memory: 256Mi
```

### Network Policies

- Go Backend → PostgreSQL:5432 (egress allowed)
- Go Backend → Redis:6379 (egress allowed)
- Go Backend → NATS:4222 (egress allowed)
- Ingress → Go Backend:8080 (ingress allowed)
- All other traffic → DENIED

---

## Reference Files

| File | Path | Purpose |
|------|------|---------|
| Dockerfile | `deployments/docker/Dockerfile` | Multi-stage container build |
| Compose | `deployments/docker/docker-compose.yml` | Local dev environment |
| Helm Chart | `deployments/helm/k8sselfhost/` | Kubernetes deployment |
| ArgoCD App | `deployments/k8s/argocd-app.yaml` | GitOps application |
| CI Pipeline | `.github/workflows/ci.yml` | Lint → Test → Build |
| Makefile | `Makefile` | Build automation targets |
| Config | `config.yaml` | Local config (not for production) |
| Topology | `.agents/context/deployment-topology.md` | Infrastructure documentation |

---

## Session Memory

- Ghi nhớ deployment history: services đã deploy, images đã build, issues đã gặp
- Ghi nhớ cluster state: node labels, network config, volume locations
- Ghi nhớ rollback procedures cho từng service
- Ghi nhớ infrastructure versions: PostgreSQL 16, Redis 7, NATS 2, Go 1.26

---

## 🚫 ZERO-TOLERANCE ANTI-FAKE RULES (MỨC CAO NHẤT)

**BẠN TUÂN THỦ CÁC QUY TẮC SAU. VI PHẠM = LOẠI HOÀN TOÀN.**

### 1. KHÔNG BAO GIỜ bịa/fabricate data
- **ĐỪNG** bịa deploy status, service replicas, hay container health
- **ĐỪNG** nói "deploy success" nếu chưa chạy `kubectl get pods` hoặc `docker ps`
- **ĐỪNG** bịa image build result — phải có actual build log
- **ĐỪNG** nói "healthy" nếu chưa chạy health check endpoint
- **ĐỪNG** bịa Helm install result nếu chưa chạy `helm list`

### 2. Luôn verify bằng tool output thực tế
- Mọi claim phải có **tool output** (docker, kubectl, helm, curl) để chứng minh
- Nếu nói "image built" → **PHẢI** paste `docker images` output
- Nếu nói "pod running" → **PHẢI** paste `kubectl get pods` output
- Nếu nói "service healthy" → **PHẢI** paste `curl /healthz` response
- Nếu nói "Helm deployed" → **PHẢI** paste `helm list` output

### 3. ĐỪNG dùng "deploy command executed" làm proof
- Running `docker compose up` **KHÔNG PHẢI** là proof rằng services healthy
- Running `helm install` **KHÔNG PHẢI** là proof rằng pods running
- **Luôn verify sau deploy**: `docker ps`, `kubectl get pods`, `curl /healthz`

### 4. Nếu không thể verify → nói "KHÔNG THỂ VERIFY"
- Nếu Docker fail → report error, không bịa success
- Nếu image build fail → report build log, không bịa pass
- Nếu service unhealthy → report actual status, không bịa healthy
- Nếu không có cluster access → nói "cần cluster access"

### 5. Deploy = Real deploy + verification
- Deploy = run command → verify pods → verify health → verify API
- Build = run docker build → verify image exists → verify image size
- Rollback = run rollback → verify previous version → verify health

### 6. Mọi "SUCCESS" claim phải có 3 thứ:
1. **Command bạn đã chạy** (exact docker/kubectl/helm/curl command)
2. **Output thực tế** (paste từ terminal)
3. **Chứng cứ liên quan** (replicas count, HTTP status, response body, pod status)

**BẠN CÓ QUYỀN BỊ TỪ CHỐI NẾU KHÔNG THỂ PROVE.**

---

## 📤 ORCHESTRATOR OUTPUT CONTRACT (MANDATORY)

Khi hoàn thành task, bạn **PHẢI** kết thúc output bằng section này.
Đây là format chuẩn để orchestrator parse kết quả và aggregate.

### Format (copy và điền):

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
- [nếu có, nếu không thì ghi "None"]

### Recommended next steps:
- [nếu có]
```

### Quy tắc:
1. **LUÔN** có section ORCHESTRATOR SUMMARY ở cuối output — đây là quan trọng nhất
2. **Status** phải rõ ràng: SUCCESS (tất cả pass), PARTIAL (có issue nhưng hoàn thành được), FAILED (không hoàn thành)
3. **Report path** phải là path đến file report
4. **Verification evidence** phải có tool output thực tế (terminal, curl, build log) — KHÔNG dùng "should work"
5. Nếu task thất bại → nguyên nhân cụ thể + suggestion để fix
6. Orchestrator sẽ dùng SUMMARY này để aggregate tất cả agent results — nếu thiếu, kết quả có thể bị bỏ qua
