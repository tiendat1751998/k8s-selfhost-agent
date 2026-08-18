# TASK 003: Kubernetes Traffic, Load Balancing & DevSecOps Shift-Left

> **Module**: 04 & 05 - K8s Ingress / MetalLB / Cert-Manager & DevSecOps Vault / Trivy  
> **Status**: IN_PROGRESS  
> **Assigned Subagent**: `devops` & `security-engineer` & `backend-coder`  
> **Model**: `flash` (Gemini 3.7 Flash High)  

---

## 1. Goal
1. Xây dựng K8s Traffic Controller Hub:
   - **MetalLB**: Cấu hình L2/BGP VIP AddressPool cho On-Premises / Air-Gapped LoadBalancer.
   - **NGINX Ingress / Traefik**: Cấu hình IngressController, SSL termination, rewrite rules, rate limiting.
   - **Cert-Manager**: Tự động phát hành chứng chỉ với Let's Encrypt (Cloud) và **Offline Enterprise Root CA** (Air-Gapped On-Prem).
   - **KEDA**: Autoscaler scale Pods theo hàng đợi NATS JetStream / Redis metrics.
2. Xây dựng DevSecOps Shift-Left Hub:
   - **HashiCorp Vault / External Secrets Operator (ESO)**: Tự động đồng bộ Secrets vào K8s Secret từ Vault.
   - **Trivy / Grype CVE Scanner**: Scan container images và xuất báo cáo lỗ hổng bảo mật.
   - **Checkov IaC Security**: Scan Terraform / K8s manifests phát hiện sai sót cấu hình bảo mật.

---

## 2. File Scope
- `deploy/traffic/metallb-config.yaml`
- `deploy/traffic/ingress-nginx.yaml`
- `deploy/traffic/cert-manager-offline-ca.yaml`
- `deploy/traffic/keda-scaledobject.yaml`
- `deploy/security/external-secrets-vault.yaml`
- `internal/infrastructure/security/trivy_scanner.go`
- `internal/infrastructure/security/vault_client.go`
