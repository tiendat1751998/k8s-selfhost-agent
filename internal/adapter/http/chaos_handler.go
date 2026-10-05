package http

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// ChaosHandler handles chaos engineering operations.
// Per CON-26..30 / Section 00: Return 501 Not Implemented because chaos engineering controller is not configured in this edition.
type ChaosHandler struct{}

// NewChaosHandler creates a new ChaosHandler.
func NewChaosHandler() *ChaosHandler {
	return &ChaosHandler{}
}

// RegisterRoutes registers chaos engineering routes.
func (h *ChaosHandler) RegisterRoutes(r chi.Router) {
	r.Post("/experiments", h.NotConfigured)
	r.Get("/experiments", h.NotConfigured)
	r.Post("/simulate", h.NotConfigured)
	r.Post("/inject", h.NotConfigured)
}

// NotConfigured returns 501 Not Implemented for unconfigured chaos engineering controller.
func (h *ChaosHandler) NotConfigured(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusNotImplemented, map[string]interface{}{
		"error": "Chaos engineering controller not configured in this edition",
		"code":  http.StatusNotImplemented,
	})
}
