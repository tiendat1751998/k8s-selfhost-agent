package http

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// SecurityHandler handles security scan endpoints.
// Per CON-22 / Section 00: Return 501 Not Implemented because security scanner controller is not configured in this edition.
type SecurityHandler struct{}

// NewSecurityHandler creates a new SecurityHandler.
func NewSecurityHandler() *SecurityHandler {
	return &SecurityHandler{}
}

// RegisterRoutes registers security scan routes.
func (h *SecurityHandler) RegisterRoutes(r chi.Router) {
	r.Post("/scan", h.NotConfigured)
	r.Get("/scan", h.NotConfigured)
	r.Post("/scans", h.NotConfigured)
	r.Get("/scans", h.NotConfigured)
}

// NotConfigured returns 501 Not Implemented for unconfigured security scanner controller.
func (h *SecurityHandler) NotConfigured(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"error": "Security scanner controller not configured in this edition",
		"code":  http.StatusNotImplemented,
	})
}
