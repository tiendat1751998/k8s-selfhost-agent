# TASK 003: Kubernetes Traffic, Load Balancing & DevSecOps Shift-Left

> **Module**: 04 & 05 - K8s Ingress / MetalLB / Cert-Manager & DevSecOps Vault / Trivy  
> **Status**: COMPLETED / SUCCESS  
> **Completed At**: 2026-08-14T08:00:00Z  

---

## 1. Summary of Accomplishments
- **Kubernetes Traffic & Load Balancing Hub (`deploy/traffic/`)**:
  - `metallb-config.yaml`: MetalLB L2/BGP VIP AddressPools cấp phát IP LoadBalancer cho cụm On-Premises / Air-Gapped.
  - `ingress-nginx.yaml`: NGINX Ingress Controller cấu hình tối ưu SSL/TLS, buffers, proxy headers và rate-limiting.
  - `cert-manager-offline-ca.yaml`: Cert-Manager ClusterIssuer hỗ trợ cả Let's Encrypt (Cloud) và **Offline Enterprise Root CA** (Air-Gapped Intranet).
  - `keda-scaledobject.yaml`: KEDA ScaledObject tự động scale Pods theo hàng đợi NATS JetStream, Redis và CPU.
- **DevSecOps Shift-Left & Secrets Hub (`deploy/security/` & `internal/infrastructure/security/`)**:
  - `external-secrets-vault.yaml`: External Secrets Operator (ESO) ClusterSecretStore tự động đồng bộ Secret từ HashiCorp Vault.
  - `vault_client.go`: HashiCorp Vault client hỗ trợ AppRole/Token auth, dynamic secret reading, lease renewal.
  - `trivy_scanner.go`: Scanner phân tích lỗ hổng CVE container image, tính toán Security Gate Pass/Fail dựa trên mức độ nghiêm trọng (CRITICAL/HIGH).
  - `checkov_scanner.go`: Scanner kiểm tra tuân thủ bảo mật IaC (Terraform/Kubernetes manifests) và tính điểm tuân thủ %.
- **Verification**: 100% Tests PASS & 100% YAML manifests valid.
