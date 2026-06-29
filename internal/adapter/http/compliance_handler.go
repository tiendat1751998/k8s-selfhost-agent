package http

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/datdt/k8sselfhost/internal/domain/compliance"
)

// ComplianceHandler provides HTTP handlers for the compliance API.
type ComplianceHandler struct {
	repo compliance.Repository
}

// NewComplianceHandler creates a new compliance HTTP handler.
func NewComplianceHandler(repo compliance.Repository) *ComplianceHandler {
	return &ComplianceHandler{repo: repo}
}

// RegisterRoutes registers compliance routes.
func (h *ComplianceHandler) RegisterRoutes(r chi.Router) {
	r.Get("/frameworks", h.ListFrameworks)
	r.Get("/violations", h.ListViolations)
}

// ListFrameworks handles GET /api/v1/compliance/frameworks
func (h *ComplianceHandler) ListFrameworks(w http.ResponseWriter, r *http.Request) {
	frameworks, err := h.repo.ListFrameworks(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list frameworks", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": frameworks})
}

// ListViolations handles GET /api/v1/compliance/violations
func (h *ComplianceHandler) ListViolations(w http.ResponseWriter, r *http.Request) {
	limit := parseIntParam(r, "limit", 50)
	offset := parseIntParam(r, "offset", 0)
	var sev *compliance.ViolationSeverity
	if s := r.URL.Query().Get("severity"); s != "" {
		sv := compliance.ViolationSeverity(s)
		sev = &sv
	}
	items, total, err := h.repo.ListViolations(r.Context(), sev, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list violations", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": items, "total": total})
}
