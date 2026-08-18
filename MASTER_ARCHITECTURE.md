# MASTER ARCHITECTURE SPECIFICATION
## Enterprise Hybrid Dev / DevOps / DevSecOps All-in-One Platform

> **Status**: Approved Master Specification  
> **Target Audience**: Dev, DevOps, DevSecOps, SRE Engineers  
> **Environment**: Hybrid Multi-Cloud (AWS, GKE, Azure) & On-Premises Air-Gapped (Zero Internet)  
> **Runtime Engines**: Linux OS, Docker, Podman, K3s, K8s, K9s  

---

## 1. System Vision & Architecture Overview

Nền tảng được thiết kế như một **"Trung Tâm Chỉ Huy Hợp Nhất (Unified Command Center)"** dành cho kỹ sư toàn diện Dev - DevOps - DevSecOps. Hệ thống hoạt động độc lập (Self-Hosted), hỗ trợ 100% môi trường nội bộ không có Internet (Air-Gapped), đóng gói thành single-binary Go hoặc Docker/Helm chart nội bộ.

```
┌──────────────────────────────────────────────────────────────────────────────────────────────────┐
│                         HYBRID DEV / DEVOPS / DEVSECOPS CONTROL PLANE                            │
└─────────────────────────────────┬────────────────────────────────────────────────────────────────┘
                                  │
    ┌─────────────────────────────┼─────────────────────────────┬────────────────────────────┐
    │                             │                             │                            │
┌───▼──────────────────────┐ ┌────▼──────────────────────┐ ┌────▼─────────────────────┐ ┌───▼─────────────────────┐
│ 1. DEV TOOLING ENGINE    │ │ 2. DEVSECOPS & SECRETS    │ │ 3. DEVOPS & IAC ENGINE   │ │ 4. SRE & OPERATIONS    │
│ • Web K9s / Live Exec    │ │ • Vault / ESO Secret Sync │ │ • Terraform (Day-0 Cloud)│ │ • Traffic & MetalLB L4/7│
│ • Local Sandboxes        │ │ • Trivy & Grype CVE Scan  │ │ • Ansible (Day-1/2 OS)   │ │ • ELK / OpenSearch Logs │
│ • DB Sanitized Clones    │ │ • Checkov IaC Policy      │ │ • GitOps Sync (Argo-like)│ │ • Dual-Target DB Backup │
│ • Pre-commit Guardrails  │ │ • Cosign & SBOM Generator │ │ • K3s / K8s / Podman     │ │ • Telegram SRE Copilot  │
└──────────────────────────┘ └───────────────────────────┘ └──────────────────────────┘ └─────────────────────────┘
```

---

## 2. Các Phân Hệ Cốt Lõi (Core Subsystems)

### A. Database Backup & Disaster Recovery Engine (Dual-Target)
* **Động cơ hỗ trợ**: PostgreSQL, MySQL, MongoDB, Redis, SQLite.
* **Chiến lược Dual-Sync (1 Local + 1 Cloud)**:
  * Snapshot $\rightarrow$ Stream nén `zstd` $\rightarrow$ Mã hóa AES-256-GCM $\rightarrow$ Đẩy đồng thời vào:
    1. **Local Storage**: Local NVMe/SSD, NFS, hoặc On-Premise MinIO (khôi phục siêu tốc trong LAN).
    2. **Cloud Storage**: AWS S3, Google Cloud Storage, Azure Blob, hoặc Cloudflare R2 (Disaster Recovery).
* **Phục hồi thảm họa (Disaster Recovery)**:
  * Hỗ trợ Point-in-Time Recovery (PITR) qua WAL streaming (PostgreSQL).
  * 1-Click Restore với tính năng Dry-run kiểm tra tính toàn vẹn dữ liệu trước khi ghi đè production.

### B. Kubernetes Traffic, Ingress & Load Balancing Engine
* **Layer 4 Load Balancing**:
  * **On-Premises / Air-Gapped**: Tích hợp và tự động quản lý **MetalLB** (Layer 2 & BGP VIPs) và **HAProxy / Keepalived**.
  * **Cloud**: Tự động liên kết AWS NLB/ALB, GKE Network Load Balancers, Azure Load Balancers.
* **Layer 7 Ingress & Gateway API**:
  * NGINX Ingress Controller & Traefik: Host/Path routing, Rate-limiting, WAF filtering.
* **SSL/TLS Automation (Cert-Manager)**:
  * Public: Tự động cấp Let's Encrypt.
  * Air-Gapped: Tự động cấp qua Internal Enterprise Root CA / HashiCorp Vault mà không cần Internet.
* **Autoscaling**:
  * HPA (CPU / RAM metrics) và **KEDA** (Event-driven autoscaling theo RabbitMQ/Kafka/NATS queue depth).

### C. Enterprise Logging & Observability Pipeline (ELK / OpenSearch / Loki)
* **Log Collection**: DaemonSet **Vector / Fluent Bit** tối ưu tài nguyên (chạy mượt trên cả edge K3s).
* **Storage & Indexing**:
  * Chế độ Full-Text Search: **Elasticsearch / OpenSearch**.
  * Chế độ High-Density Siêu Nén: **Grafana Loki / ClickHouse**.
* **Log Explorer Web UI**:
  * Live Tail logs thời gian thực theo luồng WebSocket (tương tự K9s).
  * Tự động bóc tách JSON, Nginx log, Golang stacktrace, Java exception trace.
  * Tìm kiếm theo Regex, TraceID, Pod, Namespace, HTTP Status code.

### D. Secrets Management & DevSecOps Engine (Shift-Left)
* **Quản trị Secret tập trung**:
  * Tích hợp HashiCorp Vault, AWS Secrets Manager, Infisical.
  * Đồng bộ tự động vào K8s qua **External Secrets Operator (ESO)** hoặc **SealedSecrets / SOPS**.
  * Tự động xoay vòng (rotation) mật khẩu database và API credentials.
* **Bảo mật chuỗi cung ứng (Supply Chain Security)**:
  * **SAST & Secret Leaks**: Tích hợp Semgrep và Gitleaks.
  * **Container Image CVE**: Quét tự động bằng **Trivy / Grype**.
  * **IaC Security**: Quét Terraform và K8s manifests bằng **Checkov**.
  * **Ký số & SBOM**: Tạo SBOM (Syft) và ký số container image bằng **Cosign (Sigstore)**.

### E. Infrastructure as Code (IaC) & Configuration Management
* **Terraform / OpenTofu (Day-0)**:
  * Quản lý vòng đời hạ tầng Cloud (VPC, Subnet, VMs, EKS, GKE, S3).
  * Quản lý Remote State an toàn (PostgreSQL / S3 backend với distributed lock).
* **Ansible Automation (Day-1 & Day-2)**:
  * Hardening Linux OS (SSH, Firewall iptables/ufw, sysctl kernel).
  * Bootstrap các node K3s/K8s/Docker/Podman trong môi trường On-Premises Air-Gapped.

### F. Telegram SRE Copilot & Interactive Incident Triage
* **Lọc nhiễu sự cố (Alert Deduplication)**: Gom các cảnh báo dồn dập thành 1 Incident có chấm điểm mức độ nghiêm trọng.
* **Phân tích RCA tự động**: Đưa ra nguyên nhân gốc rễ và đề xuất cách xử lý.
* **Nút bấm tương tác trực tiếp trên Telegram**:
  * `[🔄 Restart Pod/Service]`
  * `[⏪ Rollback GitOps Deployment]`
  * `[📦 Trigger Restore Database]`
  * `[📄 Xem 50 Dòng Log Lỗi Gần Nhất]`

### G. Air-Gapped Core (Offline-First Engine)
* Đóng gói toàn bộ nền tảng vào 1 Binary Go duy nhất hoặc On-Premise Helm Chart.
* Tự tích hợp SQLite/PostgreSQL nội bộ, chạy 100% không cần kết nối ra Internet.

---

## 3. Lộ Trình Phân Rã Công Việc Từng Bước (Bite-Sized Modular Roadmap)

Để đảm bảo các AI Specialist Agents thực thi với **độ chính xác 100%, không bị quá tải context hay sinh ảo giác**, công việc được chia thành các Module độc lập:

```
┌──────────────────────────────────────────────────────────────────────────────────────────────────┐
│ MODULE 1: Real Database Backup & Dual-Target Restore Worker (Postgres, MySQL, Mongo, S3/MinIO)   │
├──────────────────────────────────────────────────────────────────────────────────────────────────┤
│ MODULE 2: Telegram SRE Copilot & Interactive Action Bot (Alerts, RCA, One-Touch Recovery)        │
├──────────────────────────────────────────────────────────────────────────────────────────────────┤
│ MODULE 3: Enterprise Log Aggregation & Live Tail Explorer (Vector/Fluentbit -> ELK/Loki/Web)     │
├──────────────────────────────────────────────────────────────────────────────────────────────────┤
│ MODULE 4: K8s Traffic & Load Balancing Controller (MetalLB L4/L7, Ingress, Cert-Manager, KEDA)   │
├──────────────────────────────────────────────────────────────────────────────────────────────────┤
│ MODULE 5: Secrets Vault & DevSecOps Scanner (HashiCorp Vault/ESO, Trivy, Checkov, Cosign)        │
├──────────────────────────────────────────────────────────────────────────────────────────────────┤
│ MODULE 6: IaC & Configuration Management Hub (Terraform/OpenTofu Day-0 + Ansible Air-Gapped)     │
├──────────────────────────────────────────────────────────────────────────────────────────────────┤
│ MODULE 7: Unified Enterprise Dashboard (Vue 3 / TypeScript Glassmorphism Web Portal)             │
└──────────────────────────────────────────────────────────────────────────────────────────────────┘
```
