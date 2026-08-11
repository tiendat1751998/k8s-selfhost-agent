package http

import (
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/datdt/k8sselfhost/internal/domain/reporting"
)

// ReportingHandler provides HTTP handlers for the reporting API.
type ReportingHandler struct {
	repo reporting.Repository
}

// NewReportingHandler creates a new reporting HTTP handler.
func NewReportingHandler(repo reporting.Repository) *ReportingHandler {
	return &ReportingHandler{repo: repo}
}

// RegisterRoutes registers reporting routes.
func (h *ReportingHandler) RegisterRoutes(r chi.Router) {
	r.Get("/", h.ListReports)
	r.Post("/", h.GenerateReport)
	r.Delete("/{id}", h.DeleteReport)
}

// ListReports handles GET /api/v1/reports
func (h *ReportingHandler) ListReports(w http.ResponseWriter, r *http.Request) {
	limit := parseIntParam(r, "limit", 50)
	offset := parseIntParam(r, "offset", 0)

	items, total, err := h.repo.ListReports(r.Context(), limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list reports", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": items, "total": total})
}

type generateReportRequest struct {
	reporting.Report
}

func (r *generateReportRequest) Validate() error {
	ve := NewValidationError("validation failed")
	if strings.TrimSpace(r.Title) == "" {
		ve.Add("title", "title is required")
	}
	if strings.TrimSpace(r.Type) == "" {
		ve.Add("type", "type is required")
	}
	if ve.HasErrors() {
		return ve
	}
	return nil
}

// GenerateReport handles POST /api/v1/reports
func (h *ReportingHandler) GenerateReport(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeJSON[generateReportRequest](w, r)
	if !ok {
		return
	}
	rep := req.Report
	rep.Status = "pending"
	rep.ExpiresAt = time.Now().AddDate(0, 1, 0) // Expires in 1 month

	if err := h.repo.CreateReport(r.Context(), &rep); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create report request", err)
		return
	}
	writeJSON(w, http.StatusCreated, rep)
}

// DeleteReport handles DELETE /api/v1/reports/{id}
func (h *ReportingHandler) DeleteReport(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.repo.DeleteReport(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete report", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
