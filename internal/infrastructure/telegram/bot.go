package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	domainBackup "github.com/datdt/k8sselfhost/internal/domain/backup"
	"github.com/datdt/k8sselfhost/internal/pkg/errors"
	backupUsecase "github.com/datdt/k8sselfhost/internal/usecase/backup"
	deploymentUsecase "github.com/datdt/k8sselfhost/internal/usecase/deployment"
	"go.uber.org/zap"
)

type BotClient interface {
	SendMessage(ctx context.Context, chatID int64, text string, replyMarkup interface{}) error
	AnswerCallbackQuery(ctx context.Context, callbackQueryID string, text string) error
}

type HTTPBotClient struct {
	botToken   string
	httpClient *http.Client
}

func NewHTTPBotClient(botToken string) *HTTPBotClient {
	return &HTTPBotClient{
		botToken:   botToken,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *HTTPBotClient) SendMessage(ctx context.Context, chatID int64, text string, replyMarkup interface{}) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", c.botToken)
	payload := map[string]interface{}{
		"chat_id":    chatID,
		"text":       text,
		"parse_mode": "Markdown",
	}
	if replyMarkup != nil {
		payload["reply_markup"] = replyMarkup
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return errors.Wrap(err, "marshaling telegram message payload")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return errors.Wrap(err, "creating telegram request")
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return errors.Wrap(err, "sending telegram message")
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return errors.NewInternal(fmt.Sprintf("telegram api returned status %d", resp.StatusCode), errors.ErrInternal)
	}
	return nil
}

func (c *HTTPBotClient) AnswerCallbackQuery(ctx context.Context, callbackQueryID string, text string) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/answerCallbackQuery", c.botToken)
	payload := map[string]interface{}{
		"callback_query_id": callbackQueryID,
		"text":              text,
		"show_alert":        true,
	}

	body, _ := json.Marshal(payload)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return errors.Wrap(err, "answering telegram callback query")
	}
	defer resp.Body.Close()
	return nil
}

type SREBot struct {
	config            TelegramConfig
	client            BotClient
	debouncer         *AlertDebouncer
	deploymentUsecase *deploymentUsecase.Usecase
	backupUsecase     *backupUsecase.Usecase
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
	}

	if config.EnableDeduplication {
		bot.debouncer = NewAlertDebouncer(config.DebounceWindow, func(alert *AlertPayload) {
			_ = bot.broadcastAlert(context.Background(), alert)
		})
	}

	return bot
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
		_ = b.client.AnswerCallbackQuery(ctx, callbackID, "❌ Bạn không có quyền thực thi thao tác này!")
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
				_ = b.client.AnswerCallbackQuery(ctx, callbackID, fmt.Sprintf("❌ Lỗi restart: %v", err))
				return "", err
			}
		}
		result := fmt.Sprintf("✅ Đã kích hoạt restart thành công cho service %s/%s", ns, name)
		_ = b.client.AnswerCallbackQuery(ctx, callbackID, result)
		return result, nil

	case "rollback":
		err := errors.NewValidation("action", "rollback is not yet implemented")
		_ = b.client.AnswerCallbackQuery(ctx, callbackID, fmt.Sprintf("❌ Lỗi: %v", err))
		return "", err

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
				_ = b.client.AnswerCallbackQuery(ctx, callbackID, fmt.Sprintf("❌ Lỗi restore DB: %v", err))
				return "", err
			}
		}
		result := fmt.Sprintf("✅ Đã đưa yêu cầu Restore DB (Job: %s) vào hàng đợi thực thi!", jobID)
		_ = b.client.AnswerCallbackQuery(ctx, callbackID, result)
		return result, nil

	default:
		_ = b.client.AnswerCallbackQuery(ctx, callbackID, "Hành động không xác định")
		return "", errors.NewValidation("action", "unknown action")
	}
}
