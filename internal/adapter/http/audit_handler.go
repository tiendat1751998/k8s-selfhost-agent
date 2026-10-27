package http

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/datdt/k8sselfhost/internal/domain/audit"
	"github.com/datdt/k8sselfhost/internal/infrastructure/postgres"
	usecaseCompliance "github.com/datdt/k8sselfhost/internal/usecase/compliance"
)

var render = struct {
	JSON func(w http.ResponseWriter, r *http.Request, v any)
}{
	JSON: func(w http.ResponseWriter, r *http.Request, v any) {
		writeJSON(w, http.StatusOK, v)
	},
}

// AuditOption configures AuditHandler.
type AuditOption func(*AuditHandler)

// WithAuditScanner sets custom compliance scanner on AuditHandler.
func WithAuditScanner(s *usecaseCompliance.FleetComplianceScanner) AuditOption {
	return func(h *AuditHandler) {
		h.scanner = s
	}
}

// AuditHandler provides HTTP handlers for the platform audit API.
type AuditHandler struct {
	repo    audit.Repository
	scanner *usecaseCompliance.FleetComplianceScanner
}

// NewAuditHandler creates a new audit HTTP handler.
func NewAuditHandler(repo audit.Repository, opts ...AuditOption) *AuditHandler {
	h := &AuditHandler{repo: repo}
	for _, opt := range opts {
		opt(h)
	}

	if h.scanner == nil && repo != nil {
		if dbp, ok := repo.(interface{ DB() postgres.DBTX }); ok && dbp.DB() != nil {
			store := postgres.NewComplianceScannerStore(dbp.DB())
			h.scanner = usecaseCompliance.NewScanner(usecaseCompliance.WithDBStore(store))
		}
	}

	return h
}

// RegisterRoutes registers audit routes.
func (h *AuditHandler) RegisterRoutes(r chi.Router) {
	r.Get("/findings", h.ListFindings)
	r.Post("/findings/{id}/resolve", h.ResolveFinding)
	r.Post("/run", h.TriggerAuditRun)
	r.Get("/runs/latest", h.GetLatestRun)
	r.Get("/logs", h.ListLogs)
}

// ListFindings handles GET /api/v1/audit/findings
func (h *AuditHandler) ListFindings(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	if status == "" {
		status = "open"
	}

	items, err := h.repo.ListFindings(r.Context(), status)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list findings", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": items})
}

// ResolveFinding handles POST /api/v1/audit/findings/{id}/resolve
func (h *AuditHandler) ResolveFinding(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "finding id is required", err)
		return
	}
	if err := h.repo.ResolveFinding(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to resolve finding", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "resolved"})
}

// TriggerAuditRun handles POST /api/v1/audit/run
func (h *AuditHandler) TriggerAuditRun(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now().UTC()
	var findingsCount int

	if h.scanner != nil {
		res, err := h.scanner.Scan(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to execute compliance scan during audit run", err)
			return
		}
		if res != nil {
			findingsCount = len(res.Violations)
		}
	}

	endTime := time.Now().UTC()
	run := &audit.AuditRun{
		Status:        "completed",
		StartTime:     startTime,
		EndTime:       &endTime,
		FindingsCount: findingsCount,
	}

	if err := h.repo.RecordRun(r.Context(), run); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to record audit run", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":         "completed",
		"run_id":         run.ID,
		"findings_count": findingsCount,
		"message":        "Platform compliance audit completed successfully.",
	})
}

// GetLatestRun handles GET /api/v1/audit/runs/latest
func (h *AuditHandler) GetLatestRun(w http.ResponseWriter, r *http.Request) {
	run, err := h.repo.GetLastRun(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get latest run", err)
		return
	}
	writeJSON(w, http.StatusOK, run)
}

// ListLogs handles GET /api/v1/audit/logs
func (h *AuditHandler) ListLogs(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	offset := 0
	if o := r.URL.Query().Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	filter := audit.AuditLogFilter{
		Search:     r.URL.Query().Get("search"),
		ActionType: r.URL.Query().Get("action_type"),
		Severity:   r.URL.Query().Get("severity"),
		Actor:      r.URL.Query().Get("actor"),
		Status:     r.URL.Query().Get("status"),
		Limit:      limit,
		Offset:     offset,
	}

	logs, total, err := h.repo.ListLogs(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list audit logs", err)
		return
	}
	if logs == nil {
		logs = []audit.AuditLog{}
	}

	render.JSON(w, r, map[string]interface{}{
		"data":  logs,
		"total": total,
	})
}