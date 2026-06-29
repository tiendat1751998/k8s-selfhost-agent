package http

import (
	"encoding/json"
	"net/http"
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

// GenerateReport handles POST /api/v1/reports
func (h *ReportingHandler) GenerateReport(w http.ResponseWriter, r *http.Request) {
	var rep reporting.Report
	if err := json.NewDecoder(r.Body).Decode(&rep); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}
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
