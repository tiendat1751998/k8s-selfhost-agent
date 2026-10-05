package http

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// NetworkHandler handles network policy operations.
// Per CON-24 / Section 00: Return 501 Not Implemented because network policy controller is not configured in this edition.
type NetworkHandler struct{}

// NewNetworkHandler creates a new NetworkHandler.
func NewNetworkHandler() *NetworkHandler {
	return &NetworkHandler{}
}

// RegisterRoutes registers network policy routes.
func (h *NetworkHandler) RegisterRoutes(r chi.Router) {
	r.Post("/policies", h.NotConfigured)
	r.Put("/policies/{id}", h.NotConfigured)
	r.Delete("/policies/{id}", h.NotConfigured)
	r.Post("/mutate", h.NotConfigured)
}

// NotConfigured returns 501 Not Implemented for unconfigured network policy controller.
func (h *NetworkHandler) NotConfigured(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusNotImplemented, map[string]interface{}{
		"error": "Network policy controller not configured in this edition",
		"code":  http.StatusNotImplemented,
	})
}
