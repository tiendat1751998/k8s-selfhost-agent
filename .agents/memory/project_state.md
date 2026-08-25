# K8sControl Project State — Session Update 2026-08-24T15:20

## Git State
Branch: `refactor/modularize-overview-components` (pushed to origin)
Latest: `98d5d52` feat(telemetry): implement smart top resource offender auto-detection and point-in-time log sync
Previous commits this session:
- `e1b8a74` feat(logs): implement server-side push-down filtering and zero-alloc stream scanner for container logs
- `f63cb23` docs: update project state memory with virtual scrolling optimization
- `6d3fbd5` perf(logs): implement high-performance virtual scrolling window and memoized parsing for 20k+ log lines
- `0cf69b1` feat(telemetry): add output TX peak rate to live network IO summary card
- `a33cfa0` feat(telemetry): dynamic 1h hourly bucket aggregation for 7d/30d queries and redesigned cyber log filter toolbar
- `8484c3e` fix(overview): fix timezone parsing flatline and log level count filter
- `12a6925` fix(overview): reset drawer scroll to top on tab switch
- `0558f00` fix(historical-chart): ensure KPI cards and header stay visible in custom range and prevent 0-interval flatlines
- `2431f0b` fix(drawer): implement custom date-time search and fix bottom scroll cutoff
- `88ea205` fix(ui): pin drawer mode tabs, restore header layout, and polish historical KPI cards
- `1bb06c9` fix(ui): correct OverviewHud and RequestFlowBar template paths
- `297418b` refactor(ui): modularize OverviewView monolithic component into 9 clean sub-components
- `272468f` docs: update project state memory with consolidated time toolbar
- `2d6ffb8` feat(telemetry): consolidate time presets into unified pill bar and streamline custom range picker
- `5d6feff` feat(telemetry): add custom date-time range selection and point-in-time log sync for node historical telemetry
- `2dccd81` fix(logs): support all-lines tail depth and accurate log level classification for errors, warnings, and search
- `1e3855a` docs: update project state with live log streaming feature
- `b1b989f` feat(drawer): add real-time live auto-tail log streaming with pause/resume and smart auto-scroll
- `90b916e` fix(drawer): dynamically stream realtime node telemetry into saturation curves and auto-sync history buffer
- `a6b79bf` docs: update project state with drawer KPI & chart calibration
- `4f2bb1a` fix(drawer): integrate realtime metrics into top KPI cards, calibrate SVG percentage scaling, and sync tooltip series colors
- `29d457c` feat(drawer): enrich Hardware Saturation Trends with live metric strip, series toggles, and peak KPIs
- `145fedf` docs: update project state with real log terminal feature
- `1817aa5` feat(drawer): add real workload failure log terminal viewer with ANSI parsing and level filter
- `f131860` feat(drawer): add 3-group process categorization (Workloads, Systemd Daemons, OS Kernel) with origin tags and SRE state indicators
- `1680b7b` docs: update project state with tab and search layout polish
- `520e069` feat(drawer): separate Docker/K8s vs Host/System categories, widen search bar, and fix mode tab switcher symmetry
- `1c35175` docs: update project state with host app classification
- `03c18cc` feat(drawer): intelligently classify host platform apps (k8s-agent, hermes, proxysql, dockerd) as apps & services vs core kernel OS daemons
- `28fbe8c` docs: update project state with category filter and compact network cards
- `dc5bce0` feat(drawer): add app vs system category filters, compact search input, and 4-column compact network interfaces
- `c8c5e2c` docs: update project state with unified process & workload table
- `74adfdb` feat(overview): unify workloads & processes into single high-density table with req/s, err %, bandwidth, state, and pagination
- `b11ebc4` style(overview): expand drawer width to 980px, relocate page size to bottom footer, and fit state column without horizontal scroll
- `ea4031b` style(drawer): polish page size pill, network interface margins, and top processes search/sort controls
- `335d385` fix(metrics): guard container network delta baseline to eliminate lifetime byte rate spikes
- `5b3ce8f` feat(drawer): align network I/O row, stack NIC in/out rates, and implement high-scale service search & pagination
- `9190deb` fix(overview): standardize drawer network I/O card, eliminate badge wrapping, and gap bandwidth rates
- `b35b541` feat(ui): overhaul overview css, node card 2-row layout, and typography
- `78d7110` test(metrics): add unit test for agent scrape normalization and top processes fallback
- `531ff6a` fix(overview): resolve drawer network wrapping, duplicate badges, and os formatting
- `285812e` fix(overview): optimize drawer network table, format os info, and enrich agent scrape
- `bcf081b` test: update docker handler unit tests with mock UpdateServiceResources
- `29f449a` feat: implement workload scale and hardware resource limits tuning (RAM/CPU)
- `62409be` feat: optimize HUD + live request flow animation bar

## Repo
GitHub: https://github.com/tiendat1751998/k8s-selfhost-agent (master)

## Credentials & Config
- Postgres: 10.10.10.133:5432 myuser/mysecretpassword mydatabase
- Docker: tcp://10.10.10.133:2375
- Admin: admin@k8s.local / admin123
- JWT: k8s-selfhost-enterprise-jwt-secret-2026
- Encryption: 0123456789abcdef0123456789abcdef

## Infrastructure Topology (10.10.10.*)
- k8smater: 10.10.10.133 (Swarm Manager + Docker Registry 10.10.10.133:5000) — Ubuntu 24.04.1 LTS, kernel 6.8.0-137-generic
- worker1: 10.10.10.150 — Ubuntu 24.04.1 LTS, kernel 6.8.0-137-generic
- worker2: 10.10.10.151 — Ubuntu 24.04.1 LTS, kernel 6.8.0-124-generic
- k8sworker3: 10.10.10.152 (currently DOWN)
- masterdb: 10.10.10.200 — Ubuntu 24.04.1 LTS, kernel 6.8.0-117-generic
- workerdb1: 10.10.10.201 — Ubuntu 24.04.1 LTS, kernel 6.8.0-124-generic
- SSH username: datdt
- Agent deploy path: ~/k8s-agent, systemd user service (systemctl --user)

## Architecture
- Backend: Go + chi, port 8080, Clean Architecture
- Frontend: Vue3 + Vite + Pinia, port 3000
- PlatformHandlers in router.go holds ALL handlers
- WebSocket: WSHub + WSBridge.Broadcast (real-time metrics)
- Metrics collector: goroutine poll Docker + agent hosts 5s
- RBAC: platform_admin, tenant_admin, operator, viewer
- JWT: access 15min + partial 5min (MFA) + refresh 7d httpOnly
- TOTP: pquerna/otp, AES-GCM encrypted, bcrypt recovery codes

### TPS Architecture
- **TPSCollector** (`internal/usecase/metrics/tps_collector.go`, ~1120 lines): 5s background loop
  - Network I/O: k8s-agent aggregation
  - HTTP Gateway: Traefik via LoadBalancerProvider
  - Database: PostgreSQL pg_stat_database via pgxpool
  - NATS Messaging: /varz endpoint
  - Container Network: Docker stats aggregation
- **LoadBalancerProvider** interface (`internal/domain/loadbalancer/provider.go`):
  - `GetServiceStats()` → per-service request stats
  - `HealthCheck()`, `Name()`
  - Implementations: TraefikProvider (`internal/infrastructure/loadbalancer/traefik.go`)
  - Designed to swap Traefik/Nginx/HAProxy without code changes
- **TraefikProvider**: Parses `/api/http/services` + `/metrics` (Prometheus format)
  - `traefik_service_requests_total` per service per status code
  - `traefik_entrypoint_requests_total` for aggregate RPS
  - `traefik_entrypoint_open_connections` for active connections
  - Delta RPS calculation with previous sample tracking

### Agent Architecture
- **Agent Collector** (`cmd/agent/collector.go`, ~1238 lines):
  - `readTopProcesses()` scans /proc/[pid]/{stat,cmdline,status,io}
  - `readOSDistro()` tries /etc/os-release, /usr/lib/os-release, /etc/lsb-release
  - `readKernelVersion()` reads /proc/sys/kernel/osrelease
  - Disk filtering: physical FS whitelist + mount blacklist + device dedup
  - Per-interface network stats (ens33, docker0, docker_gwbridge, etc.)

### Overview Page Architecture
- **Summary HUD**: Compact single-row flexbox, color-coded (green/amber/rose)
  - NODES ONLINE, CONTAINERS, AVG CPU, AVG MEMORY, THROUGHPUT
- **Request Flow Animation Bar**: Animated particles proportional to req/s
  - Active connections, queued requests, error rate, health coloring
- **5-Min Saturation Trends**: SVG chart moved to below HUD
- **Node Cards**: Drag-and-drop, stable sort, localStorage persistence
- **Node Inspect Drawer** (click node card):
  - OS Distro, Kernel Version, Architecture, Uptime, Load Avg
  - Hardware Saturation Telemetry (CPU, RAM, Disk, Network)
  - 📡 Network Interface Throughput (per interface: ens33, docker0...)
  - 📦 Apps & Services on this Node (5 services on k8smater, filtered by Swarm node mapping)
    - Columns: Service, CPU, Memory, ↓ Rx, ↑ Tx, Req/s, Err %, Status
  - 🔥 Top Processes (10 active, sortable, filterable)
- **Removed sections**: Service Throughput table, Per-Node Network table, Active Workload Containers, TPS category cards — all moved to inspect drawer
- **Smooth CSS transitions**: `.smooth-value`, `.smooth-bar`, `.smooth-opacity` prevent jarring number jumps

## Build & Deploy Commands
- Frontend: `cmd.exe /c "npm run dev"` in frontend-vue → port 3000
- Backend build: `go build -o standalone.exe ./cmd/standalone/...`
- Backend start: `Start-Process -FilePath '.\standalone.exe' -WorkingDirectory 'd:\project\k8sseflhost' -WindowStyle Hidden`
- Agent binary (Linux): `cmd.exe /c "set GOOS=linux&& set GOARCH=amd64&& go build -o k8s-agent-linux ./cmd/agent/..."`
- Deploy agents: Push to git → on k8smater: `cd ~/k8s-selfhost-agent && git pull && sh deploy-agent.sh datdt@<IP>`

## Config Files
- `config.yaml`: load_balancer.provider=traefik, load_balancer.url (from Docker host)
- Traefik: Docker Swarm service `tiki_traefik` with `--metrics.prometheus=true --metrics.prometheus.entryPoint=traefik`
- NATS: Container with `-m 8222` for HTTP monitoring

## Migrations 033-051
033:cloud_accounts 034:platform_settings 035:service_catalog 036:admin_pw
037:plugins 038:scaffold_templates 039:ecosystem_tools 040:compute_hosts
041:user_mfa+refresh_tokens 042-049:various 050:slo_health_samples 051:remove_all_seed_data

## Completed Features (this session 2026-08-22)
- OS Distro + Kernel Version on all nodes (agent + backend + frontend)
- Top Processes fix for worker1/worker2 (agent redeploy)
- Per-node network interface throughput in inspect drawer
- Per-node apps & services with Docker Swarm node mapping
- Per-service request metrics (Req/s, Err %) via LoadBalancerProvider
- LoadBalancerProvider abstraction (Traefik first, swap to Nginx/HAProxy)
- Overview cleanup: removed Service Throughput, Per-Node Network, Active Containers, TPS cards
- 5-Min Saturation Trends moved to top
- HUD optimization: compact layout, color-coded, animated request flow bar
- Smooth CSS transitions for real-time data
- Traefik Prometheus metrics enabled (--metrics.prometheus=true)

## Active/In-Progress Work

### IN-PROGRESS: Fix Traefik RPS calculation (subagent 9c7ef747)
- **Problem**: HTTP RPS shows 0.4 req/s even during load test (wrk generating thousands of requests)
- **Root cause**: `collectHTTPTPS()` uses `mw.GetRequestCount()` = backend API requests, NOT Traefik proxy traffic
- **Fix**: Parse `traefik_entrypoint_requests_total` from Prometheus `/metrics` for real aggregate RPS
- **Subagent**: `9c7ef747-f744-4070-81b7-c635c2cd26f6` (backend-coder, "Traefik RPS Fix Engineer")
- **Status**: Dispatched, running. User running `wrk -t2 -c20 -d180s http://10.10.10.133/` to test
- **After fix**: Rebuild server, verify RPS shows hundreds during load test

### ALSO IN-PROGRESS: HUD animation bar (subagent ade214ae) — COMPLETED
- Request flow animation with RPS-proportional particle speed
- Active connections, queued requests, error rate display

## Key Failures & Constraints
- Agent deployment: Windows → git push → k8smater git pull → deploy-agent.sh (no direct SCP)
- systemctl --user for agent service
- NATS needs `-m 8222` for HTTP monitoring
- Traefik API (/api/overview, /api/http/services) = config only, NO request counts
- Traefik Prometheus /metrics = real request counts (enabled this session)
- PostgreSQL cache_hit_ratio: backend 0.0-1.0, frontend ×100 for %

## User Rules (BINDING)
- Orchestrator-only: main thread NEVER writes code, ALL via subagents
- Zero tolerance for fake/mock data
- Vietnamese language, casual tone
- Always start BOTH backend (8080) AND frontend (3000)
- Use Chrome DevTools MCP for QA verification

## Next Roadmap Candidates
1. Complete Traefik RPS fix (in-progress subagent)
2. Per-app latency metrics (avg response time from Traefik service_request_duration)
3. AI SRE / RCA Engine + Ecosystem Tools Integration
4. Edge Agent Command & Remote Diagnostics
5. Cost model (deferred by user)

## BLOCKED (waiting K8s cluster hardware)
Pod Terminal, Helm Catalog, GitOps Visual, Real-time Logs,
Image CVE Scanning, Policy Dashboard, Service Mesh, Network Policy
