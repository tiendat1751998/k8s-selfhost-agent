# Task 017 — Master Enterprise DevOps & Platform Engineering Specification
# Checkpoint: 2026-10-12T10:30:00+07:00

> **Project**: `k8sseflhost`  
> **Status**: MASTER_SPECIFICATION_LOCKED (Production Realities, Deep Edge-Cases & Enterprise Standard)  
> **Target Persona**: Principal DevOps Engineer / SRE Lead / Platform Architect / DevSecOps Specialist  
> **Target Environment**: Hybrid Cloud (AWS/GCP/Azure) & On-Premises Air-Gapped (Zero Internet)  
> **Design Philosophy**: Infrastructure-as-Code (IaC), GitOps-First, Zero-Trust DevSecOps, Production Resilience, Fail-Safe Guardrails  

---

## 1. TỔNG QUAN KIẾN TRÚC VẬN HÀNH TOÀN DIỆN (DEVOPS OPERATIONAL MATRIX)

Nền tảng `k8s-selfhost` là một **Hệ Điều Hành Trung Tâm Chỉ Huy Hợp Nhất (Unified Hybrid DevOps & SRE Control Plane)**, hoạt động độc lập (Self-Hosted), hỗ trợ 100% môi trường nội bộ cách ly mạng (Air-Gapped). Toàn bộ nghiệp vụ kỹ thuật được chia thành **7 Core Subsystems** không thể tách rời:

```
┌─────────────────────────────────────────────────────────────────────────────────────────────────┐
│                           HYBRID DEVOPS & SRE UNIFIED CONTROL PLANE                             │
├───────────────────────────────┬─────────────────────────────────┬───────────────────────────────┤
│ [A] DATABASE LIFECYCLE & DR   │ [B] GITOPS & GIT PROVIDERS      │ [C] DEVSECOPS & SECRETS       │
│ • Zero-Downtime Migrations    │ • GitHub, GitLab, Gitea REST    │ • HashiCorp Vault / ESO Sync  │
│ • Dual-Sync (Local + S3/R2)   │ • Incident Auto-Remediation PR  │ • Trivy & Grype CVE Scanner   │
│ • Multi-Engine (PG,Redis,SQL) │ • Webhook HMAC Verification     │ • Checkov IaC Policy Enforcer │
│ • Sanitized Staging Clones    │ • Git Drift Detection & Reconcile│ • Cosign & Syft SBOM Guard    │
├───────────────────────────────┼─────────────────────────────────┼───────────────────────────────┤
│ [D] K8S FLEET & TRAFFIC       │ [E] IAC & BARE-METAL PROVISION  │ [F] SRE TELEGRAM COPILOT      │
│ • Client-Go Fleet Manager     │ • Terraform/OpenTofu (Day-0)    │ • Deduplicated Alert FSM      │
│ • MetalLB L4 & Traefik L7     │ • Ansible OS Hardening (Day-1/2)│ • Interactive Inline Buttons  │
│ • Helm v3 SDK Release Engine  │ • K3s Node Air-Gapped Bootstrap │ • Automated LLM RCA Pipeline  │
├───────────────────────────────┴─────────────────────────────────┴───────────────────────────────┤
│ [G] DISTRIBUTED EDGE TELEMETRY & LOG RETRIEVAL                                                  │
│ • k8s-agent Columnar ZSTD Engine (Zero-ClickHouse/Loki Overhead)                                │
│ • In-Stream Secret Redaction Regex Engine (Triệt tiêu rò rỉ credential trong log)               │
│ • Scatter-Gather Cluster Search, Live Tail SSE, Resilient Ringbuffer Fallback                   │
└─────────────────────────────────────────────────────────────────────────────────────────────────┘
```

---

## 2. DANH MỤC KỊCH BẢN THỰC CHIẾN CHUYÊN SÂU (PRODUCTION SCENARIOS & DEEP EDGE-CASES)

### NHÓM 1: TẦNG MẠNG & LƯU LƯỢNG KUBERNETES (DEEP NETWORKING REALITIES)

1. **CNI IPAM Exhaustion (Hết Sạch IP PodCIDR do Rò Rỉ Lease)**:
   - *Căn nguyên*: Batch job và CronJob khởi tạo / hủy liên tục làm CNI (Calico/Flannel) không thu hồi IP lease kịp, hoặc daemonset chết giữa chừng gây leak IP.
   - *Hậu quả*: Worker node còn thừa CPU/RAM nhưng pod mới kẹt vĩnh viễn ở `ContainerCreating` với lỗi: `failed to allocate for range 0: no IP addresses available in range`.
   - *Guardrail*: k8s-agent theo dõi tỷ lệ cấp phát IPAM trên từng node; phát hiện leak khi IP allocation > 80% mà số pod thực tế < 50%; cơ chế tự động trigger CNI IPAM garbage collection mà không cần khởi động lại node.

2. **CoreDNS Cascading Failure (DNS Thundering Herd)**:
   - *Căn nguyên*: Cấu hình mặc định K8s `ndots: 5`. Mỗi khi app lookup domain ngoài (`api.stripe.com`), nó thử 4 lần domain nội bộ. Khi DB downstream chậm, hàng trăm microservices retry gửi bão DNS queries.
   - *Hậu quả*: CoreDNS nghẽn CPU, latency nhảy vọt từ 1ms lên 5000ms, kéo sập toàn bộ các service khác trong cụm.
   - *Guardrail*: Tự động triển khai `NodeLocal DNSCache` qua DaemonSet; tự động inject `ndots: 2` vào pod template; rate-limit DNS query từ client.

3. **HTTP/2 Rapid Reset & Ingress Proxy Buffer Spill to Disk**:
   - *Căn nguyên*: Client gửi stream dữ liệu lớn làm tràn bộ nhớ đệm RAM của Ingress (Traefik/Nginx), proxy tự động ghi đệm xuống file tạm trên đĩa (`/tmp/proxy_temp`).
   - *Hậu quả*: Ổ cứng node Ingress bị nghẽn I/O, toàn bộ traffic đi qua Ingress bị treo đồng loạt.
   - *Guardrail*: Tối ưu buffer sizes trong RAM; gắn disk riêng cho ingress proxy temp; cảnh báo sớm I/O latency trên Ingress nodes.

4. **Ingress TLS Certificate Hết Hạn & Rate Limit Let's Encrypt**:
   - *Căn nguyên*: Rate limit 5 certs/tuần/domain hoặc Air-Gapped internal Root CA hết hạn sau 2 năm.
   - *Hậu quả*: Trình duyệt báo đỏ `NET::ERR_CERT_DATE_INVALID`, các client mTLS ngắt kết nối thanh toán ngay lập tức.
   - *Guardrail*: Probing TLS định kỳ kiểm tra `NotAfter`; tự động chuyển đổi ACME Staging sang Production khi test; failover sang Internal HashiCorp Vault PKI Issuer khi Let's Encrypt bị rate limit; cảnh báo Telegram trước 30/14/7 ngày.

---

### NHÓM 2: QUẢN TRỊ DATABASE CHUYÊN SÂU (DATABASE DEEP OPS)

1. **PostgreSQL `idle_in_transaction` & Bệnh Phình To Bảng (Table Bloat)**:
   - *Căn nguyên*: Developer mở `BEGIN` transaction, query vài dòng rồi gọi HTTP API bên ngoài mất 15s mới `COMMIT`. Connection bị giữ `idle in transaction`.
   - *Hậu quả*: Cạn kiệt `max_connections`, **chặn đứng tiến trình `VACUUM`** dọn dead tuples, khiến bảng vài trăm MB phình to thành hàng chục GB, làm chậm toàn bộ hệ thống.
   - *Guardrail*: Bắt buộc cấu hình `idle_in_transaction_session_timeout = '10s'`; background monitor quét `pg_stat_activity`, tự động ngắt kết nối treo và gửi cảnh báo về Telegram nêu đích danh câu query và IP thủ phạm.

2. **Replication Lag Spikes & Conflict Cancellation trên Read Replica**:
   - *Căn nguyên*: Standby replica đang chạy query báo cáo dài hạn. Master gửi WAL yêu cầu dọn dữ liệu cũ.
   - *Hậu quả*: Replica buộc phải hủy ngang truy vấn (`canceling statement due to conflict with recovery`) hoặc Master bị nghẽn WAL làm đầy disk.
   - *Guardrail*: Giám sát `replay_lag_bytes`; tự động cấu hình `hot_standby_feedback = on` kèm `max_standby_archive_delay` cân bằng.

3. **Redis Memory Fragmentation Ratio Spike (> 1.5)**:
   - *Căn nguyên*: Redis liên tục lưu/xóa key kích thước không đồng đều. Trình quản lý bộ nhớ `jemalloc` bị phân mảnh nặng (data thực chỉ 2GB nhưng OS phải cấp 6GB RAM).
   - *Hậu quả*: Kernel OOM Killer thẳng tay tiêu diệt tiến trình Redis.
   - *Guardrail*: Tự động theo dõi metric `mem_fragmentation_ratio`; kích hoạt `CONFIG SET activedefrag yes` khi ratio > 1.4 và RAM > 1GB, tự động tắt khi fragmentation giảm dưới 1.1 để tiết kiệm CPU.

4. **DB Schema Migration Lock Contention**:
   - *Căn nguyên*: DDL migration (`ALTER TABLE`, `CREATE INDEX` thiếu `CONCURRENTLY`) đòi hỏi `ACCESS EXCLUSIVE lock`.
   - *Hậu quả*: Chặn đứng mọi query, connection pool cạn kiệt trong 10 giây, toàn bộ API pod sập vì 504 Gateway Timeout.
   - *Guardrail*: Bắt buộc `SET lock_timeout = '2s'; SET statement_timeout = '30s';` trong mọi migration; chặn migration nếu chưa lấy được `pg_advisory_lock`.

---

### NHÓM 3: DEVSECOPS & CHUỖI CUNG ỨNG AIR-GAPPED (AIR-GAPPED SUPPLY CHAIN)

1. **Cấm Tuyệt Đối Docker-in-Docker (DinD) & Mount Docker Socket**:
   - *Căn nguyên*: CI runner mount `/var/run/docker.sock` để build image. Chiếm được runner pod là chiếm được root của cả máy chủ vật lý.
   - *Hậu quả*: Container breakout, đánh cắp cluster-admin token.
   - *Guardrail*: Bắt buộc sử dụng **Kaniko** hoặc **BuildKit (Rootless)** chạy trong sandbox container hoàn toàn không có quyền root (`runAsNonRoot: true`), tự build và push image lên registry mà không cần docker socket.

2. **Secret Leak Trong Memory Dump & Stack Trace**:
   - *Căn nguyên*: Ứng dụng panic in stack trace kèm biến môi trường, chuỗi kết nối DB chứa mật khẩu nằm trần trụi trong log file.
   - *Hậu quả*: Rò rỉ credential vào hệ thống log trung tâm.
   - *Guardrail*: Engine thu thập log trên `k8s-agent` bắt buộc phải có **Tầng Lọc Regex Khử Secret (In-Stream Redaction)**: tự động băm/thay thế các pattern `password=***`, `Bearer ***`, `sk_live_***` thành `[REDACTED_SECRET]` TRƯỚC KHI nén ZSTD và truyền đi.

3. **Admission Control Cưỡng Chế (Kyverno / OPA Gatekeeper)**:
   - *Căn nguyên*: Developer vô tình viết sai YAML vi phạm chính sách bảo mật.
   - *Hậu quả*: Pod chạy root, không giới hạn tài nguyên, kéo sập node.
   - *Guardrail*: Validating Webhook tự động từ chối: (1) Pod dùng `image: ...:latest`, (2) Pod thiếu `readOnlyRootFilesystem: true`, (3) Pod xin cấp `hostNetwork: true` hoặc `hostPID: true`.

4. **Supply Chain Attack (Base Image Bị Đầu Độc)**:
   - *Căn nguyên*: Sử dụng base image không xác thực hoặc package bị cài mã độc.
   - *Guardrail*: **The Fail-Close Security Invariant**: Nếu scanner gặp lỗi hoặc phát hiện Critical CVE > 0, lập tức REJECT deployment; xác thực chữ ký số Cosign trước khi nạp image vào cụm.

---

### NHÓM 4: BARE-METAL & HẠ TẦNG VẬT LÝ AIR-GAPPED (HARDWARE RESILIENCE)

1. **Lệch Pha Đồng Hồ (Time Drift / NTP Desynchronization)**:
   - *Căn nguyên*: Mạng cách ly Internet không có pool NTP công cộng. Đồng hồ giữa các server vật lý lệch nhau > 500ms.
   - *Hậu quả*: etcd rớt leader liên tục; TLS handshake báo lỗi `certificate not yet valid`; mã OTP và token JWT bị từ chối 100%.
   - *Guardrail*: Ansible cấu hình Node Master 1 làm Local NTP Server (Chrony Master), các node còn lại đồng bộ về Master 1; agent giám sát offset thời gian (`node_timex_offset_seconds`), cảnh báo ngay khi lệch > 100ms.

2. **Suy Hao Ổ Cứng Lặng Lẽ (Hardware RAID / NVMe SMART Degradation)**:
   - *Căn nguyên*: Ổ đĩa máy chủ bị bad sector âm thầm, controller liên tục retry I/O làm write latency nhảy từ 0.5ms lên 800ms mà OS không báo lỗi disk chết.
   - *Hậu quả*: Cụm database bị nghẽn toàn bộ luồng ghi.
   - *Guardrail*: k8s-agent định kỳ đọc dữ liệu SMART qua `smartctl` / RAID controller CLI (`megacli`/`storcli`), cảnh báo tỷ lệ bad sectors, nhiệt độ ổ cứng và dự báo hỏng hóc trước khi ổ chết hẳn.

---

### NHÓM 5: SRE ON-CALL & TỰ ĐỘNG HÓA SỰ CỐ (PROGRESSIVE RESILIENCE)

1. **Automated Canary Analysis (ACA) & Progressive Delivery**:
   - *Nghiệp vụ*: Cấm deploy Big Bang 100%. Rollout 10% traffic (Canary) $\to$ Giám sát tỷ lệ lỗi HTTP 5xx và P99 latency trong 5 phút $\to$ Nếu error rate tăng > 0.5%, tự động rollback về 0% và cảnh báo Telegram; Nếu sạch sẽ, tự động tăng 25% $\to$ 50% $\to$ 100%.
2. **Adaptive Backpressure & Load Shedding**:
   - *Nghiệp vụ*: Khi hệ thống quá tải (CPU > 90%, DB pool cạn), tự động shed load (chặn traffic non-critical, trả về 429 Too Many Requests hoặc 503 có `Retry-After: 5`) để giữ các giao dịch cốt lõi sống sót.

---

## 3. MƯỜI QUY TẮC BẤT BIẾN VẬN HÀNH SRE (THE 10 SRE INVARIANTS)

1. **The Fail-Close Security Invariant**: Trivy/Checkov lỗi hoặc missing $\implies$ **REJECT** (`ScannerUnavailable`). Cấm tuyệt đối fake 100% pass.
2. **The Zero-Toy Storage Invariant**: Cấm bịa node ảo, cấm gán cứng 35%. Expand PVC bắt buộc kiểm tra cờ `allowVolumeExpansion` trên StorageClass trước khi patch.
3. **Safe Node Storage Fencing**: Node crash $\implies$ Áp dụng `node.kubernetes.io/out-of-service` taint, giải phóng `VolumeAttachment` an toàn trước khi dời pod DB (tránh Multi-Attach deadlock và Split-Brain).
4. **Zero-Downtime DB Migration**: Bắt buộc bảng `schema_migrations`, lấy `pg_advisory_lock`, và đặt `lock_timeout = '2s'` cho mọi DDL.
5. **Cryptographic Dual-Sync AEAD**: Chuỗi nén ZSTD + mã hóa **AES-256-GCM** có nonce/tag toàn vẹn; S3 stream có `ContentLength`.
6. **GitOps HMAC & Drift Reconciler**: Webhook bắt buộc verify HMAC-SHA256 (`crypto/subtle.ConstantTimeCompare`); background worker chủ động phát hiện drift Git vs Live K8s.
7. **Cluster Cert & Kubelet Watchdog**: Giám sát hạn dùng cert K8s (< 30 ngày tự động renew qua Ansible); giám sát conntrack entries (> 85% cảnh báo).
8. **In-Stream Log Redaction Invariant**: k8s-agent bắt buộc lọc và che giấu toàn bộ JWT, Bearer tokens, connection strings trước khi nén ZSTD.
9. **No-DinD Secure Build Invariant**: Tuyệt đối cấm mount `/var/run/docker.sock` trong container build; sử dụng Kaniko / BuildKit rootless mode.
10. **Emergency Break-Glass Lockdown**: Hỗ trợ tài khoản Break-Glass khẩn cấp qua local MFA/YubiKey khi SSO sập, phát cảnh báo P1 tức thời đến ban chỉ huy.

---

## 4. LỘ TRÌNH 14 TASK THỰC THI CHI TIẾT (TASK 018 — 031)

### PHẦN I: KHẮC PHỤC DỨT ĐIỂM 64 LỖI KỸ THUẬT (BUG REMEDIATION & WIRING)
| Task ID | Tiêu đề Task Chi Tiết | Độ ưu tiên | File Task Đặc Tả |
| :--- | :--- | :--- | :--- |
| **018** | Crash Elimination & Multi-Tenancy Data Isolation | **P0** | `.agents/tasks/inprocess/018-crash-elimination-and-tenancy.md` |
| **019** | Core Platform Wiring & Entrypoint Parity | **P0/P1** | `.agents/tasks/inprocess/019-platform-wiring-and-entrypoint-parity.md` |
| **020** | Backup Drivers, Cryptographic AEAD & Migration Hardening | **P1** | `.agents/tasks/inprocess/020-backup-drivers-crypto-and-migrations.md` |
| **021** | DevSecOps Fail-Close Security Gates & Supply Chain | **P1** | `.agents/tasks/inprocess/021-devsecops-failclose-and-supply-chain.md` |
| **022** | Real Storage Management, PVC Expansion & CSI Fencing | **P1** | `.agents/tasks/inprocess/022-real-storage-expansion-and-fencing.md` |
| **023** | Telegram SRE Copilot & Interactive Runbooks | **P1/P2** | `.agents/tasks/inprocess/023-telegram-sre-copilot-and-runbooks.md` |
| **024** | Sanitized Staging Database Clones & Masking Engine | **P2** | `.agents/tasks/inprocess/024-sanitized-staging-db-clones.md` |
| **025** | Real Disaster Recovery (etcd/Velero) & Restore Drills | **P2** | `.agents/tasks/inprocess/025-disaster-recovery-and-restore-drills.md` |

### PHẦN II: XÂY DỰNG CÁC PHÂN HỆ VẬN HÀNH THỰC CHIẾN (PRODUCTION SRE SUBSYSTEMS)
| Task ID | Tiêu đề Task Chi Tiết | Độ ưu tiên | File Task Đặc Tả |
| :--- | :--- | :--- | :--- |
| **026** | Kubernetes Network & CNI IPAM Failure Mitigator | **P1** | `.agents/tasks/inprocess/026-network-cni-ipam-and-coredns-mitigation.md` |
| **027** | Database Deep Operations & Anti-Bloat SRE Engine | **P1** | `.agents/tasks/inprocess/027-database-deep-sre-engine.md` |
| **028** | In-Stream Secret Redaction & Log Masking Engine on `k8s-agent` | **P1** | `.agents/tasks/inprocess/028-instream-secret-redaction-engine.md` |
| **029** | Bare-Metal Air-Gapped Hardware & Time Drift Watchdog | **P1** | `.agents/tasks/inprocess/029-baremetal-airgapped-hardware-watchdog.md` |
| **030** | Rootless Container Build Runner & Policy Admission Webhook | **P1** | `.agents/tasks/inprocess/030-rootless-build-and-admission-control.md` |
| **031** | Automated Canary Progressive Delivery & Adaptive Load Shedding | **P1** | `.agents/tasks/inprocess/031-automated-canary-and-loadshedding.md` |

