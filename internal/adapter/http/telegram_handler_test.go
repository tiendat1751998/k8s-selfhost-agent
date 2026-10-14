package http

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/infrastructure/telegram"
)

func TestTelegramHandler_HandleWebhook_SecretToken(t *testing.T) {
	bot := telegram.NewSREBot(telegram.TelegramConfig{
		BotToken:     "123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11",
		AdminChatIDs: []int64{111222333},
	}, nil, nil, nil, zap.NewNop())

	h := NewTelegramHandler(bot)

	t.Run("valid secret header accepted", func(t *testing.T) {
		t.Setenv("TELEGRAM_WEBHOOK_SECRET", "super-secret-token-xyz")
		body := `{"update_id":1,"message":{"chat":{"id":111222333},"from":{"id":111222333},"text":"/status"}}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/telegram/webhook", bytes.NewBufferString(body))
		req.Header.Set("X-Telegram-Bot-Api-Secret-Token", "super-secret-token-xyz")
		rec := httptest.NewRecorder()

		h.HandleWebhook(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("invalid secret header rejected with 401", func(t *testing.T) {
		t.Setenv("TELEGRAM_WEBHOOK_SECRET", "super-secret-token-xyz")
		body := `{"update_id":1,"message":{"chat":{"id":111222333},"from":{"id":111222333},"text":"/status"}}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/telegram/webhook", bytes.NewBufferString(body))
		req.Header.Set("X-Telegram-Bot-Api-Secret-Token", "wrong-secret-token")
		rec := httptest.NewRecorder()

		h.HandleWebhook(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("missing secret header when secret configured rejected with 401", func(t *testing.T) {
		t.Setenv("TELEGRAM_WEBHOOK_SECRET", "super-secret-token-xyz")
		body := `{"update_id":1,"message":{"chat":{"id":111222333},"from":{"id":111222333},"text":"/status"}}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/telegram/webhook", bytes.NewBufferString(body))
		rec := httptest.NewRecorder()

		h.HandleWebhook(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("no secret configured allows authorized chat and from ID", func(t *testing.T) {
		t.Setenv("TELEGRAM_WEBHOOK_SECRET", "")
		body := `{"update_id":1,"message":{"chat":{"id":111222333},"from":{"id":111222333},"text":"/status"}}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/telegram/webhook", bytes.NewBufferString(body))
		rec := httptest.NewRecorder()

		h.HandleWebhook(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("no secret configured rejects unauthorized sender with 401", func(t *testing.T) {
		t.Setenv("TELEGRAM_WEBHOOK_SECRET", "")
		body := `{"update_id":1,"message":{"chat":{"id":999999999},"from":{"id":999999999},"text":"/status"}}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/telegram/webhook", bytes.NewBufferString(body))
		rec := httptest.NewRecorder()

		h.HandleWebhook(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d: %s", rec.Code, rec.Body.String())
		}
	})
}
