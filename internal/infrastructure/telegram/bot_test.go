package telegram_test

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/datdt/k8sselfhost/internal/infrastructure/telegram"
	"go.uber.org/zap"
)

type MockBotClient struct {
	mu            sync.Mutex
	messages      []string
	callbackReply []string
}

func (m *MockBotClient) SendMessage(ctx context.Context, chatID int64, text string, replyMarkup interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, text)
	return nil
}

func (m *MockBotClient) AnswerCallbackQuery(ctx context.Context, callbackQueryID string, text string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callbackReply = append(m.callbackReply, text)
	return nil
}

func TestSREBot_FormatAndDeduplication(t *testing.T) {
	mockClient := &MockBotClient{}
	logger, _ := zap.NewDevelopment()

	cfg := telegram.TelegramConfig{
		BotToken:            "test-token",
		AdminChatIDs:        []int64{123456789},
		EnableDeduplication: true,
		DebounceWindow:      50 * time.Millisecond,
	}

	bot := telegram.NewSREBot(cfg, mockClient, nil, nil, logger)

	alert1 := &telegram.AlertPayload{
		IncidentID:  "inc-001",
		Severity:    "CRITICAL",
		Cluster:     "prod-us-east",
		Namespace:   "backend",
		Service:     "auth-service",
		Pod:         "auth-service-7f89b-x1",
		Message:     "OOMKilled: Memory limit 512Mi exceeded",
		RCAAnalysis: "Memory leak detected in session cache. Recommended fix: increase memory to 1Gi.",
		BackupJobID: "job-backup-999",
	}

	alert2 := &telegram.AlertPayload{
		IncidentID: "inc-001",
		Severity:   "CRITICAL",
		Cluster:    "prod-us-east",
		Namespace:  "backend",
		Service:    "auth-service",
		Pod:        "auth-service-7f89b-x2",
		Message:    "OOMKilled: Memory limit 512Mi exceeded",
	}

	// Push 2 duplicate storm alerts in rapid succession
	bot.NotifyAlert(alert1)
	bot.NotifyAlert(alert2)

	// Wait for debounce flush window
	time.Sleep(100 * time.Millisecond)

	mockClient.mu.Lock()
	count := len(mockClient.messages)
	var lastMsg string
	if count > 0 {
		lastMsg = mockClient.messages[0]
	}
	mockClient.mu.Unlock()

	if count != 1 {
		t.Fatalf("expected exactly 1 debounced aggregated message, got %d", count)
	}

	if !strings.Contains(lastMsg, "Lặp lại: 2 lần") {
		t.Errorf("expected aggregated count in message, got: %s", lastMsg)
	}

	if !strings.Contains(lastMsg, "AI-RCA Phân tích nguyên nhân") {
		t.Errorf("expected RCA section in message, got: %s", lastMsg)
	}
}

func TestSREBot_CallbackQuery_Unauthorized(t *testing.T) {
	mockClient := &MockBotClient{}
	logger, _ := zap.NewDevelopment()

	cfg := telegram.TelegramConfig{
		BotToken:     "test-token",
		AdminChatIDs: []int64{999}, // only 999 is admin
	}

	bot := telegram.NewSREBot(cfg, mockClient, nil, nil, logger)

	_, err := bot.HandleCallbackQuery(context.Background(), "cb-1", 111, "restart:kubernetes:prod:default:nginx")
	if err == nil {
		t.Fatal("expected unauthorized error for non-admin chatID 111")
	}
}

func TestSREBot_CallbackQuery_Authorized(t *testing.T) {
	mockClient := &MockBotClient{}
	logger, _ := zap.NewDevelopment()

	cfg := telegram.TelegramConfig{
		BotToken:     "test-token",
		AdminChatIDs: []int64{999},
	}

	bot := telegram.NewSREBot(cfg, mockClient, nil, nil, logger)

	result, err := bot.HandleCallbackQuery(context.Background(), "cb-1", 999, "restart:kubernetes:prod:default:nginx")
	if err != nil {
		t.Fatalf("unexpected error for authorized admin: %v", err)
	}
	if !strings.Contains(result, "restart thành công") {
		t.Errorf("expected restart success message, got: %s", result)
	}
}
