# Full-Platform Offensive Security, Functional & Remaining-Screens Audit (2026-10-27)

> **Audit Scope**: Complete end-to-end inspection of all remaining operational/governance screens, Go backend handlers/usecases/repositories, RBAC/Auth/SSRF/RCE boundaries, and live E2E browser execution (`127.0.0.1:3000` -> `127.0.0.1:8080` -> `10.10.10.60`, `10.10.10.133`, `10.10.10.80`).
> **Audit Triad**: `security-engineer` (STRIDE + OWASP API Top 10) + `qa-test-engineer` (Live Chrome DevTools MCP across all routes) + `research` (CodeGraph full-repository stub/contract forensics).
> **Linked Remediation WBS**: [`070-platform-security-and-functional-remediation.md`](../tasks/070-platform-security-and-functional-remediation.md)

---

## Part 1 — Offensive Security Findings (13 Proven Vulnerabilities: 3 CRITICAL, 5 HIGH, 3 MEDIUM, 2 LOW)

| ID | Severity | CWE / Category | Exact File & Line Evidence | Vulnerability & Exploit Vector | Required Remediation |
| :--- | :--- | :--- | :--- | :--- | :--- |
| **`SEC-CRIT-01`** | **CRITICAL** | CWE-918 (SSRF) | `internal/usecase/slo/collector_probes.go:291-332`, `internal/adapter/http/observability_handler.go:72-100` | `probeCustomSLO` dials arbitrary `http://` / `tcp://` URLs stored in `SLO.Query` (writable by `operator` / `tenant_admin`) without `netguard`, allowing probing of loopback (`127.0.0.1:5432`), RFC1918 LAN, and cloud metadata (`169.254.169.254`). | Validate custom `SLO.Query` URLs via `netguard.ValidateURL` on create/update AND enforce `netguard.New().NewTransport()` / safe dialer inside `probeCustomSLO` (keeping internal `compute_hosts` probes strictly bound to DB-registered node records). |
| **`SEC-CRIT-02`** | **CRITICAL** | CWE-78 / CWE-250 (RCE) | `internal/adapter/http/platform_routes.go:78`, `iac_handler.go:32-36`, `internal/infrastructure/iac/terraform.go:74`, `ansible.go:124-141` | `/api/v1/iac/*` allows `tenant_admin` to invoke host `terraform`/`ansible-playbook`. When `IAC_WORKSPACE_DIR` is unset (`baseDir == ""`), `normalizeWorkDir` bypasses path containment; `opts.Inventory` is passed to `-i` without leading-dash/path containment checks. | Restrict `/api/v1/iac/*` mutations strictly to `platform_admin`; enforce non-empty sandbox `baseDir` default (fail-closed if outside sandbox); validate `opts.Inventory` against flag injection (`-`) and path traversal. |
| **`SEC-CRIT-03`** | **CRITICAL** | CWE-639 (BOLA / IDOR) | `internal/adapter/http/deployment_handler.go:66-90,190-202,217-250`, `internal/usecase/deployment/usecase.go:164-169` | `if req.Namespace != ""` skips tenant namespace ownership checks when `namespace == ""` (`type: "docker"`), allowing Tenant A (`operator`) to delete, restart, or scale Tenant B's or host system Docker containers. | Require `platform_admin` when mutating global/unnamespaced (`namespace == ""`) Docker/Swarm workloads, or verify container `tenant_id` label ownership before execution. |
| **`SEC-HIGH-01`** | **HIGH** | CWE-285 (Cross-Tenant Recon) | `internal/adapter/http/explorer_handler.go:28-47`, `internal/infrastructure/kubernetes/explorer_k8s.go:38-40` | Non-`platform_admin` (`viewer`) calling `GET /api/v1/explorer` with empty or `"all"` namespace triggers `metav1.NamespaceAll`, leaking Pods, Services, and ConfigMaps across all tenants. | Enforce `isTenantNamespaceAllowed(tenantID, ns)` on `GET /api/v1/explorer` for all non-`platform_admin` callers; reject `"all"` / `""` unless `platform_admin` or scope strictly to allowed tenant namespaces. |
| **`SEC-HIGH-02`** | **HIGH** | CWE-307 / CWE-290 (Rate-Limit Spoofing) | `internal/adapter/http/router.go:93`, `internal/adapter/http/middleware/rate_limit.go:142-151` | Unconditional `chimw.RealIP` trusts spoofed `X-Forwarded-For` / `X-Real-IP` headers from any TCP client, completely bypassing the 5 req/min brute-force limiter on `POST /api/v1/auth/login`. | Remove global unconditional `chimw.RealIP`; implement trusted-proxy CIDR check (`TRUSTED_PROXIES`, defaulting to loopback `127.0.0.1/32, ::1/128`) before honoring forwarded IP headers. |
| **`SEC-HIGH-03`** | **HIGH** | CWE-287 (Auth Policy Bypass) | `migrations/036_mark_admin_requires_password_change.up.sql:7-9`, `internal/adapter/http/auth_handler.go:177-240` | `users.requires_password_change` is never queried or returned in `Login` / `/auth/me`, leaving forced-rotation accounts unaware/unenforced. | Select and return `requires_password_change` in user auth queries & login/session payload, and provide `POST /api/v1/auth/change-password` + non-destructive security banner/modal in UI. |
| **`SEC-HIGH-04`** | **HIGH** | CWE-704 (`SQLSTATE 22P02` Tenant Schema Collision) | `migrations/057_tenant_isolation_remaining_tables.up.sql:4-74`, `promotion_repo.go:125`, `reporting_repo.go:92`, `report_repo.go:38`, `capacity_repo.go:80`, `changes_repo.go:127`, `compliance_repo.go:65`, `correlation_repo.go:94`, `drift_repo.go:156`, `tagging_repo.go:48`, `tenancy_repo.go:183`, `timeline_repo.go:151` | Migration `057` created `tenant_id UUID` on 21 tables while auth middleware & 12 repositories pass string slugs (`"default-tenant"`), causing runtime `SQLSTATE 22P02: invalid input syntax for type uuid` on tenant-scoped queries/writes. | Inspect live `information_schema.columns` & FKs (`LES-007`), add migration `063` converting mismatched `tenant_id UUID` columns (without UUID FK constraints, or migrating FKs cleanly) to `VARCHAR(255) NOT NULL DEFAULT 'default-tenant'` consistent with `automation_rules`, `clusters`, `incidents`, and `compute_hosts`. |
| **`SEC-HIGH-05`** | **HIGH** | CWE-862 / CWE-522 (Agent Token Harvest & Exposure) | `internal/adapter/http/platform_routes.go:164`, `automation_executor.go:290-300,418-440`, `docker_host_handler.go` | `POST /api/v1/agents/register` lacks `RBACMiddleware("platform_admin")` — any `viewer` can register an attacker URL and harvest `Authorization: Bearer <AGENT_AUTH_TOKEN>` when central collectors/executors poll hosts. Host `labels["auth_token"]` must also never leak to browser GET responses. | Gate `POST /api/v1/agents/register` behind `platform_admin` (or valid `AGENT_AUTH_TOKEN` header), validate agent URLs, and strip/mask `auth_token` / secret labels on all `GET` host responses. |
| **`SEC-MED-01`** | **MEDIUM** | CWE-319 (Agent TLS Option) | `cmd/agent/main.go:356` | Node agent `:9100` only calls `srv.ListenAndServe()`, lacking optional TLS support (`AGENT_TLS_CERT` / `AGENT_TLS_KEY`) when deployed across untrusted VLANs. | Add optional `ListenAndServeTLS` support in `cmd/agent/main.go` when `AGENT_TLS_CERT` and `AGENT_TLS_KEY` env vars are configured. |
| **`SEC-MED-02`** | **MEDIUM** | CWE-922 (JWT in `localStorage`) | `frontend-vue/src/stores/authStore.ts:14,46` | Access JWT is stored in `localStorage.getItem('k8s_auth_token')` (XSS-exfiltrable) even though backend already sets an `HttpOnly` refresh token cookie (`k8s_refresh_token`) at `/api/v1/auth/refresh`. | Keep access token in memory (`ref<string | null>`) in `authStore.ts` and bootstrap/refresh silently via `/api/v1/auth/refresh` (`credentials: 'include'`), purging legacy `localStorage` token keys. |
| **`SEC-MED-03`** | **MEDIUM** | CWE-613 (Unrevoked Access JWT on Logout) | `internal/adapter/http/auth_handler.go:273`, `middleware/jwt_auth.go` | `Logout` clears refresh token but leaves issued 15-minute access JWTs valid until `exp`. | Add thread-safe bounded TTL token/JTI revocation denylist checked by `JWTAuthMiddleware` upon `POST /api/v1/auth/logout`. |
| **`SEC-LOW-01`** | **LOW** | CORS Header Gap | `internal/adapter/http/middleware.go:273` | `X-Tenant-ID` missing from `Access-Control-Allow-Headers`, breaking cross-origin preflight requests that pass explicit tenant headers. | Add `X-Tenant-ID` to `Access-Control-Allow-Headers`. |
| **`SEC-LOW-02`** | **LOW** | Dead / Unhashed `k8s_live_` API Keys | `internal/adapter/http/settings_handler.go:329-359`, `middleware/jwt_auth.go` | `/settings` generates `k8s_live_...` keys, immediately zeroes `Token = ""` before saving, stores no SHA-256 hash, and `JWTAuthMiddleware` rejects `Bearer k8s_live_...` as malformed JWT! | Persist `token_hash` (SHA-256) + masked prefix in PostgreSQL, return raw `k8s_live_` key **once** on creation, and authenticate `Bearer k8s_live_...` tokens in `JWTAuthMiddleware`. |

---

## Part 2 — Live Functional, Stub & UX Defects Across All Remaining Screens

> Visual evidence stored in `docs/qa-evidence/2026-10-27/remaining-screens-audit/01..10.png`.

### 1. `/helm` (`HelmCatalogView.vue`) — **CRITICAL BROKEN UI (120 Blank Cards)**
- **Live Defect (`05-helm.png`)**: Backend Helm v3 SDK (`internal/infrastructure/helm/release_operations.go`) is **100% real** and returns 120 real charts from configured repositories, **but** serializes Go structs (`Name`, `Chart.Metadata.Name`, `Chart.Metadata.Version`, `Chart.Metadata.Description`) while `HelmChartsGrid.vue` / `useHelm.ts` reads flat lowercase `item.name`, `item.version`, `item.description`. Result: **120 identical blank cards** showing `"v"`, fallback text, and a dead `Install Chart` button (`undefined` chart name).

### 2. `/clusters` (`/fleet`) & Top-Bar Context Selector — **HTTP 500 + FALSE GREEN HEALTHY**
- **Live Defect (`03-clusters.png`)**:
  1. `GET /api/v1/docker/swarm` returns **HTTP `500`** (`failed to get swarm info`) when Docker/Swarm is inactive on the host instead of graceful `200 { "active": false }`.
  2. `migrations/062:23-28` seeded `k8snode` with `status='connected', health_status='healthy'`, and `internal/usecase/cluster/health_checker.go:49-51` skips health probing whenever `c.ImportMethod != "kubeconfig"`. Meanwhile `GlobalContextSelector.vue:22-26` defaults unknown health to `'healthy'` (`dot-emerald` green dot).
  3. **Contradiction**: `/slo` honestly reports `kube-apiserver :6443` at `0.00%` RED, while `/clusters` and the global top bar proudly show `k8snode` as **`● HEALTHY` (green)** and cluster drawer shows `0 / 0 READY`!

### 3. `/runbooks` (`RunbooksView.vue`) — **HTTP 404 STEP RUN + FAKE PLAYBOOK ANIMATION**
- **Live Defect (`06-runbooks-execution.png`)**:
  1. Clicking `Run Step` calls `POST /api/v1/runbooks/{id}/steps/0/execute` -> **HTTP `404`** (route does not exist in `runbook_handler.go:25-39`).
  2. Clicking `Execute Playbook` -> `Start Procedure` runs a fake client-side step timer making `0` real step RPCs, while `POST /api/v1/runbooks/{id}/execute` (`runbook_handler.go:219-260`) is a hollow stub that only updates `last_used_at = NOW()`!
- **Required Fix**: Implement real safe diagnostic/remediation step execution against registered `:9100` node agents (`/health`, `/k8s/health`, `/metrics`, or whitelisted `/remediate` actions) with real step output logs, exit status, and latency.

### 4. `/audit` Secondary Controls (`AuditComplianceView.vue`) — **HTTP 404 REMEDIATE + EMPTY CONTAINER CVEs**
- **Live Defect (`07-audit-remediation-failure.png`)**:
  1. Clicking `Remediate` on any live compliance finding calls `POST /api/v1/compliance/violations/{id}/remediate` -> **HTTP `404`** (red banner `Remediation failed: 404 page not found`).
  2. `Container CVEs (0)` tab (`audit_findings`) has zero backend scanner population despite live container/daemon telemetry on `.60`, `.133`, `.80`.

### 5. `/explorer` (`ClusterExplorerView.vue`) — **SILENT `:6443` OFFLINE & SWARM MASQUERADE**
- **Live Defect (`04-explorer.png`)**:
  1. Backend returns `"k8s_degraded": true`, but UI completely ignores the flag and renders generic `"No Kubernetes resources discovered matching current filters."` instead of an explicit SRE diagnostic banner explaining `kube-apiserver (:6443)` is unreachable.
  2. `internal/adapter/http/explorer_swarm.go:13-98` masquerades Docker Swarm tasks as fake Kubernetes `Pod` / `Deployment` objects.

### 6. `/overview` (`FleetOverviewView.vue`) & Sidebar Header — **MISSING NODE IPs, `0/0` WORKLOADS & SYNTHETIC RPS**
- **Live Defect (`01-overview-desktop.png`)**:
  1. Host table `IP ADDRESS & OS` column shows `-- Ubuntu` / `-- Debian` because `OverviewNodeRow` reads an unpopulated IP field instead of extracting the host IP from `host.address` (`http://10.10.10.60:9100`).
  2. Top KPI `CONTAINERS` card displays `0/0 STOPPED` while 8 live system/runtime daemons are running across the 3 nodes.
  3. `tps_collector.go:324-328` fabricates `estimatedRPS = traefikRx / 1200.0`, `QDeep-Dive` button lacks icon spacing, and sidebar collapse `<` button overlaps `K8S CONTROL` wordmark on wide desktop.

### 7. `/incidents` (`IncidentsLifecycleView.vue`) — **404 POLLING SPAM & DUMMY GITOPS PR**
- **Live Defect (`02-incidents.png`)**:
  1. Selecting an incident spams `GET /api/v1/incidents/{id}/report` and `/pr` returning HTTP `404` in network logs when RCA/PR records don't exist yet (should return `200 { exists: false }` or `204` cleanly).
  2. `ensureRemediationPR` (`incident_remediation.go:100-145`) fabricates hardcoded dummy YAML strings when no Git provider is configured instead of reporting honest GitOps integration state, and dev-only `Simulate` button sits in production header.

### 8. `/tenancy` (`TenancyManagementView.vue`) & `/settings` (`SystemSettingsView.vue`) — **EPHEMERAL RAM MAPS & KMS THEATRE**
- **Live Defect (`08-tenancy.png`, `10-settings.png`)**:
  1. `tenancy_handler.go:24-40` stores org deletions, member CRUD, RBAC matrix overrides, and resource quotas in **in-memory Go maps** (`deletedOrgs`, `orgMembers`, `customRoles`, `quotas`) because `postgres/tenancy_repo.go` doesn't implement `extendedTenancyRepo` — every change vanishes on server restart!
  2. `crypto_handler.go:148-225` uses an in-memory map while `ZeroTrustDrawer.vue:106,176` falsely claims `"Hardware Security Module (HSM PKCS#11) / Vault KMS Provider"`. Must persist key metadata in PostgreSQL and label honestly as **Local AES-256-GCM Envelope Encryption** unless external Vault/HSM is actually configured.
