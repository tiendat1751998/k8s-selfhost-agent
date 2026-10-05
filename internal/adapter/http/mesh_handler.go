package http

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// MeshHandler handles service mesh and traffic splitting operations.
// Per CON-25 / Section 00: Return 501 Not Implemented because service mesh controller is not configured in this edition.
type MeshHandler struct{}

// NewMeshHandler creates a new MeshHandler.
func NewMeshHandler() *MeshHandler {
	return &MeshHandler{}
}

// RegisterRoutes registers service mesh and traffic splitting routes.
func (h *MeshHandler) RegisterRoutes(r chi.Router) {
	r.Post("/split", h.NotConfigured)
	r.Post("/traffic/split", h.NotConfigured)
	r.Post("/canary", h.NotConfigured)
	r.Post("/bluegreen", h.NotConfigured)
	r.Post("/routes", h.NotConfigured)
}

// NotConfigured returns 501 Not Implemented for unconfigured service mesh controller.
func (h *MeshHandler) NotConfigured(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"error": "Service mesh controller not configured in this edition",
		"code":  http.StatusNotImplemented,
	})
}
