package http

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"

	"github.com/datdt/k8sselfhost/internal/adapter/http/middleware"
	"github.com/datdt/k8sselfhost/internal/infrastructure/telegram"
)

// TelegramHandler provides HTTP endpoints for Telegram SRE Copilot bot operations.
type TelegramHandler struct {
	bot *telegram.SREBot
}

// NewTelegramHandler creates a new TelegramHandler instance.
func NewTelegramHandler(bot *telegram.SREBot) *TelegramHandler {
	return &TelegramHandler{
		bot: bot,
	}
}

// RegisterRoutes registers Telegram endpoints on the given Chi router.
func (h *TelegramHandler) RegisterRoutes(r chi.Router) {
	r.Get("/status", h.GetStatus)

	r.With(middleware.RequireRolesForMutations("platform_admin", "tenant_admin")).Group(func(mut chi.Router) {
		mut.Post("/alert", h.SendAlert)
	})

	r.Post("/webhook", h.HandleWebhook)
}

// GetStatus handles GET /api/v1/telegram/status
func (h *TelegramHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":     "ready",
		"configured": h.bot != nil,
	})
}

// SendAlert handles POST /api/v1/telegram/alert
func (h *TelegramHandler) SendAlert(w http.ResponseWriter, r *http.Request) {
	if h.bot == nil {
		writeError(w, http.StatusServiceUnavailable, "Telegram bot is not configured", nil)
		return
	}

	var alert telegram.AlertPayload
	if err := json.NewDecoder(r.Body).Decode(&alert); err != nil {
		writeError(w, http.StatusBadRequest, "invalid alert payload", err)
		return
	}

	h.bot.NotifyAlert(&alert)
	writeJSON(w, http.StatusOK, map[string]string{
		"status": "alert_dispatched",
	})
}

type telegramWebhookUpdate struct {
	UpdateID int64 `json:"update_id"`
	Message  *struct {
		MessageID int64 `json:"message_id"`
		Chat      struct {
			ID int64 `json:"id"`
		} `json:"chat"`
		From struct {
			ID int64 `json:"id"`
		} `json:"from"`
		Text string `json:"text"`
	} `json:"message,omitempty"`
	CallbackQuery *struct {
		ID   string `json:"id"`
		From struct {
			ID int64 `json:"id"`
		} `json:"from"`
		Message *struct {
			Chat struct {
				ID int64 `json:"id"`
			} `json:"chat"`
		} `json:"message,omitempty"`
		Data string `json:"data"`
	} `json:"callback_query,omitempty"`
}

// HandleWebhook handles POST /api/v1/telegram/webhook
func (h *TelegramHandler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	if h.bot == nil {
		writeError(w, http.StatusServiceUnavailable, "Telegram bot is not configured", nil)
		return
	}

	secret := os.Getenv("TELEGRAM_WEBHOOK_SECRET")
	headerVal := r.Header.Get("X-Telegram-Bot-Api-Secret-Token")

	if secret != "" {
		if subtle.ConstantTimeCompare([]byte(headerVal), []byte(secret)) != 1 {
			writeError(w, http.StatusUnauthorized, "invalid webhook secret token", nil)
			return
		}
	}

	var update telegramWebhookUpdate
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		writeError(w, http.StatusBadRequest, "invalid webhook update payload", err)
		return
	}

	// If secret is not set, strictly validate sender authorization
	if secret == "" {
		var chatID, fromID int64
		if update.CallbackQuery != nil {
			fromID = update.CallbackQuery.From.ID
			if update.CallbackQuery.Message != nil {
				chatID = update.CallbackQuery.Message.Chat.ID
			} else {
				chatID = fromID
			}
		} else if update.Message != nil {
			chatID = update.Message.Chat.ID
			fromID = update.Message.From.ID
		}
		if chatID == 0 || fromID == 0 || !h.bot.IsAuthorized(chatID) || !h.bot.IsAuthorized(fromID) {
			writeError(w, http.StatusUnauthorized, "unauthorized telegram caller", nil)
			return
		}
	}

	if update.CallbackQuery != nil {
		result, err := h.bot.HandleCallbackQuery(
			r.Context(),
			update.CallbackQuery.ID,
			update.CallbackQuery.From.ID,
			update.CallbackQuery.Data,
		)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed handling callback query", err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{
			"result": result,
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status": "acknowledged",
	})
}
