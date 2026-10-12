package http

import (
	"encoding/json"
	"net/http"

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
	UpdateID      int64 `json:"update_id"`
	CallbackQuery *struct {
		ID   string `json:"id"`
		From struct {
			ID int64 `json:"id"`
		} `json:"from"`
		Data string `json:"data"`
	} `json:"callback_query,omitempty"`
}

// HandleWebhook handles POST /api/v1/telegram/webhook
func (h *TelegramHandler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	if h.bot == nil {
		writeError(w, http.StatusServiceUnavailable, "Telegram bot is not configured", nil)
		return
	}

	var update telegramWebhookUpdate
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		writeError(w, http.StatusBadRequest, "invalid webhook update payload", err)
		return
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
