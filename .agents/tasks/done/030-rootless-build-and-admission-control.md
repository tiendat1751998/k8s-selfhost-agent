# Task 030 — Rootless Container Build Runner & Policy Admission Webhook (P1)
# Checkpoint: 2026-10-12T10:35:00+07:00

> **Severity**: P1 (Container Isolation, Zero-DinD Security Invariant & Hardened Admission Webhook)  
> **Target Agent**: `backend-coder` / `devops`  
> **Workspace**: Git Worktree Isolated (`Workspace: "branch"`)  
> **Design Rules**: Zero-Docker-Socket Mounting, Rootless Image Compilation, Strict Pod Security Admission  

---

## 1. BỐI CẢNH KỸ THUẬT & PHÂN TÍCH LỖI THỰC TẾ (ROOT CAUSE ANALYSIS)

### Lỗi 1: Cung Cấp Quyền Root Máy Chủ Bằng Việc Mount `/var/run/docker.sock`
* **Hiện tượng thực tế**: Để phục vụ việc build Docker image trong CI/CD pipeline, các hướng dẫn thông thường yêu cầu mount socket máy chủ:
  ```yaml
  volumeMounts:
    - mountPath: /var/run/docker.sock
      name: docker-sock # LỖI BẢO MẬT CHẾT NGƯỜI
  ```
* **Hậu quả thảm khốc**:
  - Container có quyền ghi vào docker socket đồng nghĩa với việc có toàn quyền điều khiển daemon Docker của host.
  - Kẻ tấn công hoặc một npm script độc hại có thể khởi chạy một container mới với `privileged: true`, mount toàn bộ ổ cứng `/` của máy chủ vật lý, đánh cắp `admin.conf` và chiếm quyền kiểm soát toàn bộ Kubernetes cluster trong vòng 5 giây!

### Lỗi 2: Thiếu Admission Webhook Chặn Đứng Các Manifest Sai Phạm
* **Hiện tượng thực tế**: Developer hoặc DevOps đẩy manifest lên cluster với:
  1. `image: my-service:latest` (Dùng tag latest gây bất định và không thể rollback chính xác).
  2. `readOnlyRootFilesystem: false` (Cho phép mã độc tải backdoor ghi vào root container).
  3. `runAsUser: 0` (Chạy quyền root bên trong container).
* **Căn nguyên hiện tại**: Cụm K8s thiếu một Validating Admission Webhook cục bộ để tự động kiểm duyệt và từ chối ngay từ cổng vào của K8s API Server.

---

## 2. ĐẶC TẢ STRUCTS & INTERFACES (CONTRACT DEFINITION)

Trong `internal/domain/security/admission.go`:
```go
package security

import (
    "context"
    admissionv1 "k8s.io/api/admission/v1"
)

type AdmissionRuleViolation struct {
    Field   string `json:"field"`
    Rule    string `json:"rule"`
    Message string `json:"message"`
}

type AdmissionValidationResult struct {
    Allowed    bool                     `json:"allowed"`
    StatusCode int32                    `json:"status_code"`
    Reason     string                   `json:"reason"`
    Violations []AdmissionRuleViolation `json:"violations,omitempty"`
}

type PolicyEnforcer interface {
    ValidatePodSpec(ctx context.Context, review *admissionv1.AdmissionReview) (*AdmissionValidationResult, error)
}

type RootlessBuildSpec struct {
    BuildID       string            `json:"build_id"`
    ContextURL    string            `json:"context_url"` // Git repo hoặc S3 tarball
    Dockerfile    string            `json:"dockerfile"`
    Destination   string            `json:"destination"` // Harbor registry URL
    BuildArgs     map[string]string `json:"build_args"`
    RegistryAuth  string            `json:"registry_auth_secret"`
    MemoryLimit   string            `json:"memory_limit"`
    CPULimit      string            `json:"cpu_limit"`
}

type RootlessBuildRunner interface {
    LaunchKanikoBuild(ctx context.Context, spec RootlessBuildSpec) (string, error) // Returns PodName
    StreamBuildLogs(ctx context.Context, buildID string) (<-chan string, error)
}
```

---

## 3. THUẬT TOÁN XỬ LÝ TỪNG BƯỚC (STEP-BY-STEP WORKFLOW)

### Quy trình 1: Khởi Chạy Kaniko Pod Hoàn Toàn Rootless (Zero Docker Socket)
1. Nhận yêu cầu build container từ GitOps webhook hoặc UI.
2. Sinh Pod manifest tạm thời chạy image `gcr.io/kaniko-project/executor:v1.20.0-debug`:
   ```yaml
   apiVersion: v1
   kind: Pod
   metadata:
     name: kaniko-build-{id}
     namespace: build-sandbox
   spec:
     restartPolicy: Never
     securityContext:
       runAsNonRoot: true
       runAsUser: 1000
       fsGroup: 1000
     containers:
       - name: kaniko
         image: gcr.io/kaniko-project/executor:v1.20.0
         args:
           - "--dockerfile=Dockerfile"
           - "--context=git://..."
           - "--destination=harbor.internal/app/service:v1.0.0"
           - "--cache=true"
         volumeMounts:
           - name: harbor-config
             mountPath: /kaniko/.docker
   ```
3. Kaniko thực thi build trong user space hoàn toàn không cần đặc quyền kernel.
4. Stream log build về Central Server qua WebSocket/SSE.
5. Sau khi push thành công lên Harbor, tự động xóa Pod sandbox.

### Quy trình 2: Validating Admission Webhook Cưỡng Chế Chính Sách
1. K8s API Server gửi `AdmissionReview` tới endpoint `POST /api/v1/security/admission/validate`.
2. Parser đọc `PodTemplateSpec`:
   - **Check 1**: Nếu có bất kỳ volume mount nào trỏ vào `/var/run/docker.sock` hoặc `/var/run/containerd/containerd.sock` $\implies$ **REJECT**.
   - **Check 2**: Nếu image tag kết thúc bằng `:latest` hoặc không có tag $\implies$ **REJECT** ("Immutable tags required").
   - **Check 3**: Nếu `securityContext.privileged == true` $\implies$ **REJECT**.
   - **Check 4**: Nếu `securityContext.readOnlyRootFilesystem != true` $\implies$ **REJECT** (hoặc cảnh báo theo policy profile).
3. Trả về `AdmissionResponse`:
   ```json
   {
     "response": {
       "allowed": false,
       "status": {
         "code": 403,
         "message": "Policy violation: Mounting docker.sock is strictly forbidden under Enterprise Zero-Trust policy."
       }
     }
   }
   ```

---

## 4. CHUẨN HÓA MÃ LỖI (RFC 7807 ERROR SPECIFICATION)

* `ERR_DIND_NOT_ALLOWED`: `HTTP 403 Forbidden` — "Mounting docker.sock or privileged DinD is forbidden. Use Kaniko rootless builder."
* `ERR_ADMISSION_MUTATION_FAILED`: `HTTP 422 Unprocessable Entity` — "Admission webhook rejected pod manifest due to 3 policy violations."

---

## 5. MA TRẬN TEST CASE (VERIFICATION SUITE)

1. `TestAdmission_DockerSockMount_Rejected`: Gửi Pod spec có mount `/var/run/docker.sock`. Assert webhook trả về `allowed: false` với mã lỗi chính xác.
2. `TestAdmission_LatestTag_Rejected`: Gửi Pod spec có `image: nginx:latest`. Assert webhook từ chối.
3. `TestAdmission_CompliantPod_Allowed`: Gửi Pod spec đạt chuẩn (non-root, read-only rootfs, SHA256 digest). Assert webhook trả về `allowed: true`.
4. `TestKaniko_PodSpecGeneration`: Assert pod spec của Kaniko có `runAsNonRoot: true` và không có bất kỳ volume hostPath nào.

---

## 6. KẾ HOẠCH ROLLBACK & GUARDRAILS AN TOÀN

* **Guardrail**: Cấu hình `failurePolicy: Ignore` cho các namespace hệ thống (`kube-system`, `kube-public`) để tránh tình trạng admission controller tự khóa chính mình nếu webhook pod gặp sự cố.
