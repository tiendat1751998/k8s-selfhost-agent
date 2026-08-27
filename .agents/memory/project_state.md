# K8sControl Project State — Session Update 2026-08-27T10:25

## Git State
Branch: `feat/disk-io-realtime-telemetry` (pushed to origin)
Latest: `006c460` fix(cluster): support onprem, k3s, baremetal, and all kubeconfig imported clusters in dynamic client manager
Previous commits this session:
- `e5c4596` fix(ui): de-clutter slo tracking mobile view with 2x2 kpis, compact burn rate bar, and responsive header
- `bd6adc5` fix(ui): overhaul mobile number visibility with workload cards, 2x2 historical kpis, and compact toggles
- `ab42656` fix(ui): de-clutter deep-dive modal with 2x2 kpi grid, compact header, and streamlined series toggles
- `b556dce` fix(hmr): purge service worker dev cache to prevent vite hmr websocket 400 handshake error
- `736b8e1` fix(ui): de-clutter multi-cluster fleet mobile view with 2x2 grid and compact actions
- `e4d4826` fix(ui): de-clutter mobile web ui with concise typography, compact data chips, and streamlined hud cards
- `d383226` feat(ui): revamp node diagnostics drawer with 2x2 gauge grid, compact tabs, and safe mobile scroll clearance
- `ee6d3b5` fix(ui): polish overview storage hud card, streamline request flow bar, and compact header action buttons
- `601cd44` fix(ui): enhance mobile drawer exit with pinned top-right close button and thumb-friendly footer close button
- `3efaebe` docs(memory): record DEC-068 for platform-wide interactive functional QA and PWA quality gate
- `8c71879` fix(ui): upgrade overview hud to strict css grid across all breakpoints
- `5f42d3c` docs(memory): record DEC-066 for platform-wide 2x2 grid and smart button wrapping
- `5be91ab` feat(ui): enforce 2x2 metric grid and smart button wrapping across all views
- `ee38d7d` docs(memory): record DEC-065 for comprehensive multi-view mobile QA audit
- `851906f` fix(ui): fix overview hud 1-column mobile override and clean top hud navbar cramping
- `e3982d3` docs(memory): record DEC-064 for overview hud 2x2 grid and top navbar ergonomics
- `9513f13` feat(ui): implement 2x2 high-density metric grid and side-by-side header actions across all views
- `222e43e` docs(memory): record DEC-063 2x2 high-density metric grid & mobile actions
- `a7cb934` feat(ui): implement fluid mobile typography and streamline overview telemetry
- `f421f11` docs(memory): record DEC-062 for fluid mobile typography & streamlined overview
- `3e62058` docs(memory): record DEC-061 for 33-view mobile RWD overhaul
- `0a66362` feat(frontend): comprehensive RWD and mobile PWA overhaul across all 33 views
- `e8aaebd` feat(pwa): add progressive web app manifest, service worker, mobile bottom nav, and agents view rwd
- `9a614b0` fix(frontend): overhaul mobile responsive layout for trend header, chart x-axis, and top hud
- `013aa9a` fix(frontend): add hierarchical tie-breaking for process sorting and replace container unshift with push
- `9942ce6` feat(frontend): add disk io column and compact state badge in node process table
- `20a1570` feat(frontend): consolidate ingress metrics into 5-min trend header and streamline request flow bar
- `f9503bb` docs(memory): sync DEC-052 to DEC-055 and update deployment script invariants in memory
- `f64355a` fix(agent): enhance disk IO collection with sub-device fallback and robust await averaging
- `73cdeb0` feat(ui): refine NodeCard with balanced 2x2 telemetry matrix, compact tabular streams, and header container chip
- `bc1a2a0` feat(tps): harden TPS calculations with counter reset protection, delta cache hit ratio, and max latency tracking
- `561936c` feat(telemetry): add high-precision Linux disk I/O telemetry, IOPS, await latency, and live dashboard visualization
- `df9eee3` fix(chart): remove clumsy peak badge box and streamline apex halo beacon
- `8c59e49` docs(memory): record DEC-050 smooth cubic spline chart upgrade
- `fb91758` feat(chart): upgrade telemetry saturation curves to smooth monotone cubic splines with neon glow and peak beacons
- `8f6c28c` feat(header): integrate alert bell, dropdown toast, and telemetry health into top hud
- `acd1cdb` feat(overview): top-right floating cyber alert toast and alert center modal
- `7820b2a` feat(overview): persistent alert snooze and mute system with one-click suppression

## Agent Deployment Script Invariant
- **Single-Node Deployment**: `./deploy-agent.sh user@<ip>`
  - Automatically compiles Linux binary `k8s-agent` from `cmd/agent/`
  - Deploys to target host over SCP
  - Configures non-sudo `systemd --user` service with `loginctl enable-linger`
  - Restarts `k8s-agent` on port 9100
- **Node IP Mapping**:
  - `k8smater`: `10.10.10.133` (Central Control Plane & Swarm Manager)
  - `masterdb`: `10.10.10.200`
  - `worker1`: `10.10.10.150`
  - `worker2`: `10.10.10.151`
  - `workerdb1`: `10.10.10.201`
  - `k8sworker3`: `10.10.10.152` (offline)

## Scalability Roadmap Master Architecture (1,000 - 10,000 Nodes)
- **Master Plan File**: [`docs/plans/fleet-scalability-roadmap-10k-nodes.md`](file:///d:/project/k8sseflhost/docs/plans/fleet-scalability-roadmap-10k-nodes.md)
- **Evolutionary Scaling Phases**:
  - Phase 1 (100 - 500 Nodes): `@tanstack/vue-virtual` Virtual Scroller in `OverviewView.vue`, Standalone VictoriaMetrics single-binary adapter, Binary WebSocket delta streams.
  - Phase 2 (500 - 2,000 Nodes): `k8s-agent` gRPC Push over mTLS, NATS JetStream buffer cluster, WebGL Hex-Grid Cluster Density Map.
  - Phase 3 (2,000 - 5,000 Nodes): Regional Cellular Supernode Relays (eBPF FastPath + WireGuard), Edge Sentinel local autonomous self-healing.
  - Phase 4 (5,000 - 10,000+ Nodes): 3-Tier TSDB Lifecycle (Hot VM $\rightarrow$ Warm 1m $\rightarrow$ Cold Snappy Parquet on S3/MinIO), Multi-Region Control Plane Federation.

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
- **Cluster Health Investigation**:
  - Scrape engine successfully polls 5/6 real nodes on port 9100.
  - Zero mock data; offline node alerts correctly surfaced for `k8sworker3` and manageable via Top HUD Alert Bell / Alert Center Modal.
  - Summary telemetry accurately aggregates active nodes, container count, average CPU/RAM utilization, and cluster-wide throughput.

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
- **Summary HUD**: Strict CSS Grid across all breakpoints (Desktop >1200px: `repeat(5, minmax(0, 1fr))`, Tablet 769px-1200px: `repeat(3, minmax(0, 1fr))`, Mobile <=768px: `repeat(2, minmax(0, 1fr))` with Card 5 Cluster Storage spanning 2 cols in clean vertical layout with 3-4px progress bar and full unclipped byte metrics `💽 88.6 GB / 196.2 GB`). Concise labels (`Nodes`, `Containers`, `CPU`, `Memory`, `Storage`), hidden redundant 'NOMINAL' badges on mobile, and compact footers (`1 up · 5 down`, `2 / 3.8 GB`, `88.6 / 196.2 GB`) eliminate all text ellipsis truncation.
- **Request Flow Animation Bar**: Streamlined 32-38px laser particle simulator with dynamic glowing speed, live state pill, pulsating glyphs (`● ● ● →`), removing redundant text badges on mobile for sleek uncluttered screen space.
- **5-Min Saturation Trends**: SVG chart directly below HUD with consolidated Gateway Ingress metrics in header (`active`, `queued`, `latency ms`, `error %`), concise title (`📈 Trends`), unified series legend toggles (`CPU`, `RAM`, `Throughput`), and a single `🔍 Deep-Dive` action trigger.
- **Header Actions & Title**: Concise single-line title (`⚡ Cluster Overview`), compact live badge (`● LIVE`), shortened action buttons ('🔄 Refresh', '🚀 Apps', '🖥️ Hosts', '☸️ Fleet') with 11.5px/12px font size in balanced 2x2 grid on mobile and clean 1-row layout on tablet.
- **Node Cards**: Drag-and-drop, stable sort, localStorage persistence, 2x2 resource matrix (RAM/Disk allocation & Live Net/Disk rates).
- **Node Inspect Drawer** (click node card):
  - OS Distro, Kernel Version, Architecture, Uptime, Load Avg
  - Hardware Saturation Telemetry (CPU, RAM, Disk, Network) with 2x2 hardware gauge grid on mobile/tablet
  - 📡 Network Interface Throughput (per interface: ens33, docker0...)
  - 📦 Apps & Services on this Node (5 services on k8smater, filtered by Swarm node mapping)
    - Columns: Service, CPU, Memory, ↓ Rx, ↑ Tx, Req/s, Err %, Status
  - 🔥 Top Processes (10 active, sortable by CPU/RAM/Disk I/O, filterable, dual-rate 📖 Read / ✍️ Write Disk I/O telemetry, compact 68px mini state badge with glowing status dot)
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

## Completed Features (Latest Session: 2026-08-27)
- **Kubernetes Dynamic Client Manager Multi-Provider & On-Prem Support** (DEC-078)
- **Service Level Objectives (SLO) Mobile Web Typography & Layout De-Cluttering** (DEC-077)
- **Mobile Web Ergonomics, Workload Card Visibility & 2x2 Historical KPI Overhaul** (DEC-076)
- **Cluster Telemetry & Ingress Saturation Deep-Dive Modal Mobile Ergonomics Overhaul** (DEC-075)
- **Service Worker Dev Mode Purge & Vite HMR WebSocket 400 Resolution** (DEC-074)
- **Multi-Cluster Fleet Mobile Web & PWA Typography De-Cluttering — Screen 1.1** (DEC-073)
- **Mobile Web UI De-Cluttering, Concise Typography & Streamlined HUD Cards** (DEC-072)
- **Node Diagnostics Drawer Mobile/PWA Ergonomics & 2x2 Gauge Grid Overhaul** (DEC-071)
- **Overview HUD Card 5 Ergonomics, Streamlined Flow Bar & Compact Header Actions** (DEC-070)
- **Mobile Inspect Drawer Exit Ergonomics & Pinned Close Controls** (DEC-069)
- **Platform-Wide E2E Interactive Functional Testing & Multi-Viewport PWA Quality Gate** (DEC-068)
- **Overview HUD Strict CSS Grid Layout & Resizing Resilience** (DEC-067)
- **Platform-Wide 2x2 Metric Grid Enforcement & Smart Button Wrapping** (DEC-066)
- **Multi-View Mobile Web & RWD Comprehensive QA Verification** (DEC-065)
- **Overview HUD 2x2 Grid Bug Fix & Top HUD Navbar Ergonomics Polish** (DEC-064)
- **2x2 High-Density Metric Grid & Side-by-Side Mobile Header Actions** (DEC-063)
- **Fluid Mobile Typography & Streamlined Overview Telemetry** (DEC-062)
- **Project-Wide Responsive Web Design (RWD) & Mobile PWA Overhaul Across All 33 Views** (DEC-061)
- **Progressive Web App (PWA), Service Worker Caching, Mobile Bottom Bar & Agents RWD** (DEC-060)
- **Mobile Responsive UI Overhaul for Telemetry & Navigation** (DEC-059)
- **Hierarchical Tie-Breaking for Process Sorting & Container Push** (DEC-058)
- **Process Table Real-Time Disk I/O & Compact State Badge Polish** (DEC-057)
- **Consolidated Ingress Telemetry Header & Streamlined Request Flow Simulator** (DEC-056)
- **High-Precision Linux Disk I/O Telemetry, IOPS & Await Latency** (DEC-052)
- **Hardened TPS Engine & Monotonic Counter Reset Protection** (DEC-053)
- **Symmetrical 2x2 NodeCard Resource Matrix & Tabular-Nums Anti-Jitter** (DEC-054)
- **Sub-Device Fallback for Block Device I/O** (DEC-055)
- Smooth Monotone Cubic Bézier Splines, Neon Glow Area Gradients & Apex Halo Beacons (DEC-050)
- Top HUD Navigation Alert Bell, Dropdown Toast & Real-time Telemetry Health Indicators (DEC-049)
- Persistent Alert Snooze & Mute System with One-Click Global Suppression (DEC-047, DEC-048)
- Dynamic Ceiling Headroom & Clamped Spline Telemetry Visualizer (DEC-051)

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

## Kubernetes Cluster State (UPDATED: Live Cluster Connected)
- **Cluster**: `k8snode` on `https://10.10.10.60:6443` (v1.35.8, 2 nodes: k8smasterdeb + k8smaster, Cilium 1.20.0 eBPF CNI, Hubble Relay/UI, 14 Pods).
- **K8s Deep Features (Pod Terminal, Live Pod Logs, YAML Editor, Helm Catalog)**: Deferred to dedicated phase after completing screen-by-screen mobile/PWA ergonomics audit.
- **Current Active Phase**: Step-by-step screen-by-screen mobile web UI & button-by-button ergonomics optimization:
  - Screen 1.1: Multi-Cluster Fleet (`/fleet`) — ✅ COMPLETED & VERIFIED (DEC-073)
  - Screen 1.2: SLO Tracking (`/slo`) — ✅ COMPLETED & VERIFIED (DEC-077)
  - Deep-Dive Traffic Modal & Node Historical Chart Numbers — ✅ COMPLETED & VERIFIED (DEC-075, DEC-076)
  - Screen 1.3: Deployments & Apps (`/deployments`) — ⏳ READY FOR NEXT AUDIT
  - Screen 1.4: Real-Time Logs (`/logs`)
  - Screen 1.5: Security & Compliance (`/security` / `/compliance`)
  - Screen 1.6: Infrastructure Hosts (`/hosts`)
  - Screen 1.7: Incidents & RCA (`/incidents`)
