# Task 019 — Core Platform Wiring & Entrypoint Parity (P0/P1)
# Checkpoint: 2026-10-12T10:30:00+07:00

> **Severity**: P0/P1 (Broken Integrations, Orphaned Handlers & Missing Routes)  
> **Target Agent**: `backend-coder`  
> **Workspace**: Git Worktree Isolated (`Workspace: "branch"`)  
> **Design Rules**: Clean Architecture Dependency Injection, Explicit Lifecycle Management, Zero Dead Code  

---

## 1. PHÂN TÍCH LỖI & BẰNG CHỨNG MÃ NGUỒN (ROOT CAUSE ANALYSIS)

### Lỗi 1: 9 Domain Nền Tảng Bị "Bỏ Rơi" (Unwired Handlers)
* **Vị trí**: `cmd/server/bootstrap_services.go:288-324` và `cmd/server/router.go`
* **Đoạn code bị lỗi**:
  ```go
  // CODE HIỆN TẠI (NHIỀU HANDLERS BỊ NIL HOẶC QUÊN MOUNT VÀO ROUTER):
  // platformHandlers.Helm hoàn toàn không được khởi tạo!
  // platformHandlers.DR, Storage, Ecosystem... được new nhưng không có route trong router.go!
  ```
* **Căn nguyên**: Frontend gọi các endpoint như `/api/v1/helm/releases`, `/api/v1/storage/volumes`, `/api/v1/dr/etcd-snapshots` nhưng backend trả về HTTP 404 hoặc bị nil pointer panic do handler chưa được mount hoặc chưa được inject usecase dependencies.

### Lỗi 2: GitOps Controller Quên Đăng Ký Git Providers
* **Vị trí**: `cmd/server/bootstrap_services.go:266`
* **Đoạn code bị lỗi**:
  ```go
  // CODE HIỆN TẠI:
  gitopsController := gitops.NewController(gitopsRepo, auditService)
  // Quên hoàn toàn việc gọi gitopsController.RegisterProvider(...)!
  ```
* **Căn nguyên**: Trong `internal/infrastructure/gitprovider/` đã viết sẵn các provider cho `github`, `gitlab`, và `gitea`. Tuy nhiên tại hàm bootstrap, không có dòng code nào nạp token từ biến môi trường và đăng ký vào controller. Khi xảy ra sự cố OOMKilled cần mở Remediation PR, hệ thống báo lỗi:
  `unsupported git provider: github`.

### Lỗi 3: Telegram SRE Bot Worker Chưa Được Khởi Chạy
* **Vị trí**: `cmd/server/bootstrap_services.go`
* **Căn nguyên**: Code Telegram Bot listener và alert debouncer trong `internal/infrastructure/telegram/bot.go` đã hoàn thiện nhưng không có goroutine nào gọi `telegramBot.Start(ctx)` khi server khởi động.

---

## 2. ĐẶC TẢ STRUCTS & INTERFACES (CONTRACT DEFINITION)

### Platform Handlers Aggregate Struct:
Trong `cmd/server/bootstrap_services.go`:
```go
type PlatformHandlers struct {
    Auth         *adapthttp.AuthHandler
    Cluster      *adapthttp.ClusterHandler
    Deployment   *adapthttp.DeploymentHandler
    Helm         *adapthttp.HelmHandler          // Bổ sung đầy đủ
    Storage      *adapthttp.StorageHandler       // Bổ sung đầy đủ
    DR           *adapthttp.DRHandler            // Bổ sung đầy đủ
    GitOps       *adapthttp.GitOpsHandler        // Bổ sung đầy đủ
    IaC          *adapthttp.IaCHandler           // Bổ sung đầy đủ
    Telegram     *adapthttp.TelegramHandler      // Bổ sung đầy đủ
    HealthCenter *adapthttp.HealthCenterHandler
    Logs         *adapthttp.LogHandler
}
```

### HMAC Webhook Verifier Interface:
Trong `internal/adapter/http/middleware/webhook_hmac.go`:
```go
package middleware

import "net/http"

type WebhookHMACVerifier interface {
    VerifyGitHubSignature(payload []byte, signatureHeader string, secretKey []byte) bool
    VerifyGitLabToken(tokenHeader string, expectedToken string) bool
}
```

---

## 3. THUẬT TOÁN XỬ LÝ TỪNG BƯỚC (STEP-BY-STEP WORKFLOW)

### Bước 1: Khởi Tạo & Đăng Ký Git Providers
1. Đọc biến môi trường trong `bootstrap_services.go`:
   ```go
   if cfg.GitOps.GitHubToken != "" {
       ghProvider := gitprovider.NewGitHubProvider(cfg.GitOps.GitHubToken)
       gitopsController.RegisterProvider("github", ghProvider)
   }
   if cfg.GitOps.GitLabToken != "" {
       glProvider := gitprovider.NewGitLabProvider(cfg.GitOps.GitLabToken, cfg.GitOps.GitLabURL)
       gitopsController.RegisterProvider("gitlab", glProvider)
   }
   if cfg.GitOps.GiteaToken != "" {
       giteaProvider := gitprovider.NewGiteaProvider(cfg.GitOps.GiteaToken, cfg.GitOps.GiteaURL)
       gitopsController.RegisterProvider("gitea", giteaProvider)
   }
   ```

### Bước 2: Khởi Tạo Helm v3 SDK Handler & Mount Routes
1. Khởi tạo `helmManager := helm.NewReleaseManager(k8sClient, cfg.Helm)`
2. Khởi tạo `platformHandlers.Helm = adapthttp.NewHelmHandler(helmManager)`
3. Trong `cmd/server/router.go`, đăng ký các routes:
   ```go
   helmGroup := apiV1.Group("/helm", middleware.RequireAuth)
   {
       helmGroup.GET("/releases", platformHandlers.Helm.ListReleases)
       helmGroup.POST("/releases", platformHandlers.Helm.InstallRelease)
       helmGroup.GET("/releases/:name", platformHandlers.Helm.GetRelease)
       helmGroup.DELETE("/releases/:name", platformHandlers.Helm.UninstallRelease)
       helmGroup.POST("/releases/:name/rollback", platformHandlers.Helm.RollbackRelease)
   }
   ```

### Bước 3: Khởi Động Background Worker Telegram Bot
1. Nếu `cfg.Telegram.BotToken != ""`:
   ```go
   go func() {
       log.Info().Msg("Starting Telegram SRE Copilot background bot...")
       if err := telegramBot.Start(serverCtx); err != nil && !errors.Is(err, context.Canceled) {
           log.Error().Err(err).Msg("Telegram bot listener exited with error")
       }
   }()
   ```

---

## 4. CHUẨN HÓA MÃ LỖI (RFC 7807 ERROR SPECIFICATION)

* `ERR_GITOPS_PROVIDER_NOT_CONFIGURED`: `HTTP 422 Unprocessable Entity` — "Git provider '%s' is not configured. Provide API token in platform settings."
* `ERR_WEBHOOK_HMAC_INVALID`: `HTTP 401 Unauthorized` — "Invalid X-Hub-Signature-256 HMAC signature."

---

## 5. MA TRẬN TEST CASE (VERIFICATION SUITE)

1. `TestBootstrap_AllHandlersInitialized`: Chạy bootstrap trong test mode, duyệt qua tất cả trường trong `PlatformHandlers`. Assert không có trường nào bị `nil`.
2. `TestRouter_AllRoutesResolvable`: Kiểm tra router mux table. Assert các route `/api/v1/helm/*`, `/api/v1/storage/*`, `/api/v1/dr/*` được đăng ký thành công và không trả về 404 cho mock request.
3. `TestGitOps_ProviderResolution`: Gọi controller tạo PR trên repo `github.com/org/repo`. Assert controller gọi đúng `GitHubProvider` mà không báo lỗi unsupported.
4. `TestWebhook_HMAC_Security`:
   - Gửi request với signature đúng $\implies$ 200 OK.
   - Gửi request với signature sai hoặc thiếu $\implies$ 401 Unauthorized.

---

## 6. KẾ HOẠCH ROLLBACK & GUARDRAILS AN TOÀN

* **Guardrail**: Kiểm tra thời gian khởi động server: `go run ./cmd/server` phải boot thành công trong vòng < 2 giây và log rõ các domain đã mount.
* **Rollback Plan**: Revert commit nếu việc mount route làm xung đột với các middleware sẵn có.
