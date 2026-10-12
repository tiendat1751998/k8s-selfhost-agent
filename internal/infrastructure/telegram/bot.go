package telegram

import (
	"context"
	"fmt"
	"strings"
	"sync"

	domainBackup "github.com/datdt/k8sselfhost/internal/domain/backup"
	"github.com/datdt/k8sselfhost/internal/domain/sre"
	"github.com/datdt/k8sselfhost/internal/pkg/errors"
	backupUsecase "github.com/datdt/k8sselfhost/internal/usecase/backup"
	deploymentUsecase "github.com/datdt/k8sselfhost/internal/usecase/deployment"
	"go.uber.org/zap"
	"k8s.io/client-go/kubernetes"
)

type SREBot struct {
	config            TelegramConfig
	client            BotClient
	debouncer         *AlertDebouncer
	deploymentUsecase *deploymentUsecase.Usecase
	backupUsecase     *backupUsecase.Usecase
	k8sClient         kubernetes.Interface
	runbookExecutor   sre.RunbookExecutor
	logFetcher        PodLogFetcher
	logger            *zap.Logger
	adminMap          map[int64]bool
	mu                sync.RWMutex
}

func NewSREBot(
	config TelegramConfig,
	client BotClient,
	deployUsecase *deploymentUsecase.Usecase,
	backupUsecase *backupUsecase.Usecase,
	logger *zap.Logger,
) *SREBot {
	if client == nil && config.BotToken != "" {
		client = NewHTTPBotClient(config.BotToken)
	}

	adminMap := make(map[int64]bool)
	for _, id := range config.AdminChatIDs {
		adminMap[id] = true
	}

	bot := &SREBot{
		config:            config,
		client:            client,
		deploymentUsecase: deployUsecase,
		backupUsecase:     backupUsecase,
		logger:            logger,
		adminMap:          adminMap,
		runbookExecutor:   NewInteractiveRunbookExecutor(nil, logger),
	}

	if config.EnableDeduplication {
		bot.debouncer = NewAlertDebouncer(config.DebounceWindow, func(alert *AlertPayload) {
			_ = bot.broadcastAlert(context.Background(), alert)
		})
	}

	return bot
}

// SetK8sClient injects a Kubernetes interface and wires default logFetcher and runbookExecutor.
func (b *SREBot) SetK8sClient(client kubernetes.Interface) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.k8sClient = client
	if b.runbookExecutor == nil {
		b.runbookExecutor = NewInteractiveRunbookExecutor(client, b.logger)
	}
	if b.logFetcher == nil {
		b.logFetcher = NewK8sPodLogFetcher(client)
	}
}

// SetRunbookExecutor injects an SRE runbook executor.
func (b *SREBot) SetRunbookExecutor(executor sre.RunbookExecutor) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.runbookExecutor = executor
}

// SetLogFetcher injects a pod log fetcher.
func (b *SREBot) SetLogFetcher(fetcher PodLogFetcher) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.logFetcher = fetcher
}

func (b *SREBot) IsAuthorized(chatID int64) bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if len(b.adminMap) == 0 {
		b.logger.Warn("admin map is empty, access denied", zap.Int64("chat_id", chatID))
		return false
	}
	return b.adminMap[chatID]
}

func (b *SREBot) NotifyAlert(alert *AlertPayload) {
	if b.debouncer != nil {
		b.debouncer.Push(alert)
		return
	}
	_ = b.broadcastAlert(context.Background(), alert)
}

func (b *SREBot) FormatAlertMessage(a *AlertPayload) string {
	icon := "⚠️"
	if a.Severity == "CRITICAL" {
		icon = "🚨"
	} else if a.Severity == "INFO" {
		icon = "ℹ️"
	}

	targetType := a.TargetType
	if targetType == "" {
		targetType = "kubernetes"
	}

	cluster := a.Cluster
	if cluster == "" {
		cluster = "production-cluster"
	}

	countStr := ""
	if a.Count > 1 {
		countStr = fmt.Sprintf(" *(Lặp lại: %d lần)*", a.Count)
	}

	msg := fmt.Sprintf("%s *[SRE SỰ CỐ]*: `%s`%s\n", icon, a.Severity, countStr)
	msg += fmt.Sprintf("━━━━━━━━━━━━━━━━━━━━\n")
	msg += fmt.Sprintf("• *Hạ tầng*: `%s` (%s)\n", cluster, targetType)
	msg += fmt.Sprintf("• *Namespace*: `%s`\n", a.Namespace)
	msg += fmt.Sprintf("• *Service*: `%s`\n", a.Service)
	if a.Pod != "" {
		msg += fmt.Sprintf("• *Pod(s)*: `%s`\n", a.Pod)
	}
	msg += fmt.Sprintf("• *Chi tiết lỗi*: %s\n", a.Message)

	if a.RCAAnalysis != "" {
		msg += fmt.Sprintf("\n🧠 *AI-RCA Phân tích nguyên nhân*:\n%s\n", a.RCAAnalysis)
	}
	msg += fmt.Sprintf("\n⏱ _Thời gian: %s_", a.Timestamp.Format("2006-01-02 15:04:05 MST"))

	return msg
}

func (b *SREBot) BuildInlineMarkup(a *AlertPayload) interface{} {
	targetType := a.TargetType
	if targetType == "" {
		targetType = "kubernetes"
	}
	cluster := a.Cluster
	if cluster == "" {
		cluster = "production-cluster"
	}

	keyboard := [][]map[string]string{
		{
			{
				"text":          "🔄 Khởi động lại (Restart)",
				"callback_data": fmt.Sprintf("restart:%s:%s:%s:%s", targetType, cluster, a.Namespace, a.Service),
			},
			{
				"text":          "⏮️ Rollback bản trước",
				"callback_data": fmt.Sprintf("rollback:%s:%s:%s:%s", targetType, cluster, a.Namespace, a.Service),
			},
		},
	}

	podTarget := a.Pod
	if podTarget == "" {
		podTarget = a.Service
	}
	if podTarget != "" {
		keyboard = append(keyboard, []map[string]string{
			{
				"text":          "📄 Xem 50 Dòng Log",
				"callback_data": fmt.Sprintf("logs:%s:%s", a.Namespace, podTarget),
			},
		})
	}

	if a.BackupJobID != "" {
		keyboard = append(keyboard, []map[string]string{
			{
				"text":          "📦 Khôi phục DB (Restore)",
				"callback_data": fmt.Sprintf("restore_db:%s:%s:%s", a.Namespace, a.Service, a.BackupJobID),
			},
		})
	}

	return map[string]interface{}{
		"inline_keyboard": keyboard,
	}
}

func (b *SREBot) broadcastAlert(ctx context.Context, a *AlertPayload) error {
	if b.client == nil {
		return nil
	}
	text := b.FormatAlertMessage(a)
	markup := b.BuildInlineMarkup(a)

	for _, chatID := range b.config.AdminChatIDs {
		if err := b.client.SendMessage(ctx, chatID, text, markup); err != nil {
			b.logger.Error("failed to send telegram alert message", zap.Int64("chat_id", chatID), zap.Error(err))
		}
	}
	return nil
}

func (b *SREBot) HandleCallbackQuery(ctx context.Context, callbackID string, chatID int64, data string) (string, error) {
	if !b.IsAuthorized(chatID) {
		if b.client != nil {
			_ = b.client.AnswerCallbackQuery(ctx, callbackID, "❌ Bạn không có quyền thực thi thao tác này!")
		}
		return "Unauthorized", errors.ErrUnauthorized
	}

	parts := strings.Split(data, ":")
	if len(parts) == 0 {
		return "", errors.NewValidation("data", "invalid callback format")
	}

	action := parts[0]
	switch action {
	case "restart":
		if len(parts) < 5 {
			return "", errors.NewValidation("data", "insufficient params for restart")
		}
		targetType, cluster, ns, name := parts[1], parts[2], parts[3], parts[4]
		if b.deploymentUsecase != nil {
			if err := b.deploymentUsecase.RestartDeployment(ctx, targetType, cluster, ns, name); err != nil {
				if b.client != nil {
					_ = b.client.AnswerCallbackQuery(ctx, callbackID, fmt.Sprintf("❌ Lỗi restart: %v", err))
				}
				return "", err
			}
		}
		result := fmt.Sprintf("✅ Đã kích hoạt restart thành công cho service %s/%s", ns, name)
		if b.client != nil {
			_ = b.client.AnswerCallbackQuery(ctx, callbackID, result)
		}
		return result, nil

	case "rollback":
		var ns, name string
		switch {
		case len(parts) >= 5:
			ns, name = parts[3], parts[4]
		case len(parts) == 4:
			ns, name = parts[2], parts[3]
		case len(parts) == 3:
			ns, name = parts[1], parts[2]
		default:
			return "", errors.NewValidation("data", "insufficient params for rollback")
		}

		prevRev, err := b.rollbackDeployment(ctx, ns, name)
		if err != nil {
			if b.client != nil {
				_ = b.client.AnswerCallbackQuery(ctx, callbackID, fmt.Sprintf("❌ Lỗi rollback: %v", err))
			}
			return "", err
		}

		result := fmt.Sprintf("✅ Rollback deployment %s/%s thành công về Revision %s. Dịch vụ đã hoạt động ổn định.", ns, name, prevRev)
		if b.client != nil {
			_ = b.client.SendMessage(ctx, chatID, result, nil)
			_ = b.client.AnswerCallbackQuery(ctx, callbackID, fmt.Sprintf("✅ Đã rollback %s về rev %s", name, prevRev))
		}
		return result, nil

	case "logs":
		var ns, pod string
		switch {
		case len(parts) >= 5:
			ns, pod = parts[3], parts[4]
		case len(parts) == 3:
			ns, pod = parts[1], parts[2]
		default:
			return "", errors.NewValidation("data", "insufficient params for logs")
		}

		rawLogs, err := b.fetchLogs(ctx, ns, pod, 50)
		if err != nil {
			if b.client != nil {
				_ = b.client.AnswerCallbackQuery(ctx, callbackID, fmt.Sprintf("❌ Lỗi lấy log: %v", err))
			}
			return "", err
		}

		formattedLogs := FormatPodLogs(rawLogs)
		if b.client != nil {
			_ = b.client.SendMessage(ctx, chatID, formattedLogs, nil)
			_ = b.client.AnswerCallbackQuery(ctx, callbackID, "✅ Đã tải 50 dòng log mới nhất")
		}
		return formattedLogs, nil

	case "restore_db":
		if len(parts) < 4 {
			return "", errors.NewValidation("data", "insufficient params for restore_db")
		}
		ns, name, jobID := parts[1], parts[2], parts[3]
		if b.backupUsecase != nil {
			restoreJob := &domainBackup.RestoreJob{
				TenantID:     ns,
				BackupJobID:  jobID,
				TargetDBName: name,
				Status:       "pending",
			}
			if err := b.backupUsecase.TriggerRestore(ctx, restoreJob); err != nil {
				if b.client != nil {
					_ = b.client.AnswerCallbackQuery(ctx, callbackID, fmt.Sprintf("❌ Lỗi restore DB: %v", err))
				}
				return "", err
			}
		}
		result := fmt.Sprintf("✅ Đã đưa yêu cầu Restore DB (Job: %s) vào hàng đợi thực thi!", jobID)
		if b.client != nil {
			_ = b.client.AnswerCallbackQuery(ctx, callbackID, result)
		}
		return result, nil

	case "runbook":
		if len(parts) < 2 {
			return "", errors.NewValidation("data", "insufficient params for runbook")
		}
		rbID := sre.RunbookID(parts[1])
		req := sre.RunbookExecutionRequest{
			RunbookID:   rbID,
			TriggeredBy: fmt.Sprintf("Telegram:%d", chatID),
			Parameters:  make(map[string]string),
		}
		if len(parts) >= 4 {
			req.Parameters["namespace"] = parts[2]
			req.Parameters["deployment"] = parts[3]
		}
		b.mu.RLock()
		executor := b.runbookExecutor
		b.mu.RUnlock()
		if executor == nil {
			return "", errors.NewInternal("runbook executor not configured", errors.ErrInternal)
		}
		res, err := executor.Execute(ctx, req)
		if err != nil {
			if b.client != nil {
				_ = b.client.AnswerCallbackQuery(ctx, callbackID, fmt.Sprintf("❌ Lỗi runbook: %v", err))
			}
			return "", err
		}
		if b.client != nil {
			_ = b.client.SendMessage(ctx, chatID, res.Output, nil)
			_ = b.client.AnswerCallbackQuery(ctx, callbackID, "✅ Runbook hoàn tất")
		}
		return res.Output, nil

	default:
		if b.client != nil {
			_ = b.client.AnswerCallbackQuery(ctx, callbackID, "Hành động không xác định")
		}
		return "", errors.NewValidation("action", "unknown action")
	}
}

func (b *SREBot) rollbackDeployment(ctx context.Context, namespace, name string) (string, error) {
	b.mu.RLock()
	client := b.k8sClient
	executor := b.runbookExecutor
	b.mu.RUnlock()

	if client != nil {
		return RollbackDeployment(ctx, client, namespace, name)
	}
	if executor != nil {
		res, err := executor.Execute(ctx, sre.RunbookExecutionRequest{
			RunbookID: sre.RunbookRollbackDeployment,
			Parameters: map[string]string{
				"namespace":  namespace,
				"deployment": name,
			},
		})
		if err != nil {
			return "", err
		}
		return res.Output, nil
	}
	return "", errors.NewInternal("k8s client or runbook executor not configured for rollback", errors.ErrInternal)
}

func (b *SREBot) fetchLogs(ctx context.Context, namespace, pod string, tailLines int64) (string, error) {
	b.mu.RLock()
	fetcher := b.logFetcher
	client := b.k8sClient
	b.mu.RUnlock()

	if fetcher != nil {
		return fetcher.GetPodLogs(ctx, namespace, pod, tailLines)
	}
	if client != nil {
		f := NewK8sPodLogFetcher(client)
		return f.GetPodLogs(ctx, namespace, pod, tailLines)
	}
	return "", errors.NewInternal("log fetcher or k8s client not configured", errors.ErrInternal)
}

// Start starts the Telegram bot worker and listens until the context is canceled.
func (b *SREBot) Start(ctx context.Context) error {
	b.logger.Info("Telegram SRE bot worker started")
	<-ctx.Done()
	b.logger.Info("Telegram SRE bot worker stopped")
	return ctx.Err()
}
