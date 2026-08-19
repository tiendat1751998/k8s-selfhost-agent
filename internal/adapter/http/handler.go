// Package http provides HTTP handlers for the dashboard API.
package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	domainGitops "github.com/datdt/k8sselfhost/internal/domain/gitops"
	"github.com/datdt/k8sselfhost/internal/domain/incident"
	"github.com/datdt/k8sselfhost/internal/domain/report"
	infraNats "github.com/datdt/k8sselfhost/internal/infrastructure/nats"
	"github.com/datdt/k8sselfhost/internal/usecase/gitops"
	"github.com/datdt/k8sselfhost/internal/pkg/errors"
	"github.com/datdt/k8sselfhost/internal/pkg/logger"
)

// Handler provides HTTP handlers for the dashboard API.
type Handler struct {
	incidentRepo     incident.Repository
	reportRepo       report.Repository
	prRepo           domainGitops.Repository
	publisher        *infraNats.Publisher
	gitopsController *gitops.Controller
}

// NewHandler creates a new dashboard HTTP handler.
func NewHandler(incidentRepo incident.Repository, reportRepo report.Repository, prRepo domainGitops.Repository, publisher *infraNats.Publisher, gitopsController *gitops.Controller) *Handler {
	return &Handler{
		incidentRepo:     incidentRepo,
		reportRepo:       reportRepo,
		prRepo:           prRepo,
		publisher:        publisher,
		gitopsController: gitopsController,
	}
}

// RegisterRoutes registers dashboard API routes on the given chi router.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/incidents", func(r chi.Router) {
		r.Get("/", h.ListIncidents)
		r.Get("/{id}", h.GetIncident)
		r.Get("/{id}/report", h.GetIncidentReport)
		r.Get("/{id}/pr", h.GetIncidentPR)
		r.Post("/{id}/analyze", h.AnalyzeIncident)
	})

	r.Route("/reports", func(r chi.Router) {
		r.Get("/", h.ListReports)
		r.Get("/{id}", h.GetReport)
	})

	r.Route("/prs", func(r chi.Router) {
		r.Get("/", h.ListPRs)
		r.Post("/", h.CreatePR)
		r.Get("/{id}", h.GetPR)
		r.Post("/{id}/merge", h.MergePR)
		r.Post("/{id}/close", h.ClosePR)
	})
}

// ListIncidents handles GET /api/v1/incidents
func (h *Handler) ListIncidents(w http.ResponseWriter, r *http.Request) {
	filter := incident.Filter{
		Limit:  parseIntParam(r, "limit", 50),
		Offset: parseIntParam(r, "offset", 0),
	}

	if ns := r.URL.Query().Get("namespace"); ns != "" {
		filter.Namespace = ns
	}
	if cluster := r.URL.Query().Get("cluster"); cluster != "" {
		filter.ClusterName = cluster
	}
	if status := r.URL.Query().Get("status"); status != "" {
		s := incident.Status(status)
		filter.Status = &s
	}
	if typ := r.URL.Query().Get("type"); typ != "" {
		t := incident.Type(typ)
		filter.Type = &t
	}
	if sev := r.URL.Query().Get("severity"); sev != "" {
		s := incident.Severity(sev)
		filter.Severity = &s
	}

	incidents, total, err := h.incidentRepo.List(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list incidents", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data":   incidents,
		"total":  total,
		"limit":  filter.Limit,
		"offset": filter.Offset,
	})
}

// GetIncident handles GET /api/v1/incidents/{id}
func (h *Handler) GetIncident(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	inc, err := h.incidentRepo.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, errors.ErrNotFound) {
			writeError(w, http.StatusNotFound, "incident not found", err)
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get incident", err)
		return
	}

	writeJSON(w, http.StatusOK, inc)
}

// GetIncidentReport handles GET /api/v1/incidents/{id}/report
func (h *Handler) GetIncidentReport(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	rpt, err := h.reportRepo.GetByIncidentID(r.Context(), id)
	if err != nil {
		if errors.Is(err, errors.ErrNotFound) {
			writeError(w, http.StatusNotFound, "report not found", err)
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get report", err)
		return
	}

	writeJSON(w, http.StatusOK, rpt)
}

// GetIncidentPR handles GET /api/v1/incidents/{id}/pr
func (h *Handler) GetIncidentPR(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	pr, err := h.prRepo.GetByIncidentID(r.Context(), id)
	if err != nil {
		if errors.Is(err, errors.ErrNotFound) {
			writeError(w, http.StatusNotFound, "pull request not found", err)
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get PR", err)
		return
	}

	writeJSON(w, http.StatusOK, pr)
}

// ListReports handles GET /api/v1/reports
func (h *Handler) ListReports(w http.ResponseWriter, r *http.Request) {
	limit := parseIntParam(r, "limit", 50)
	offset := parseIntParam(r, "offset", 0)

	reports, total, err := h.reportRepo.List(r.Context(), limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list reports", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data":   reports,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// GetReport handles GET /api/v1/reports/{id}
func (h *Handler) GetReport(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	rpt, err := h.reportRepo.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, errors.ErrNotFound) {
			writeError(w, http.StatusNotFound, "report not found", err)
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get report", err)
		return
	}

	writeJSON(w, http.StatusOK, rpt)
}

// ListPRs handles GET /api/v1/prs
func (h *Handler) ListPRs(w http.ResponseWriter, r *http.Request) {
	limit := parseIntParam(r, "limit", 50)
	offset := parseIntParam(r, "offset", 0)

	var status *domainGitops.PRStatus
	if s := r.URL.Query().Get("status"); s != "" {
		st := domainGitops.PRStatus(s)
		status = &st
	}

	prs, total, err := h.prRepo.List(r.Context(), status, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list PRs", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data":   prs,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// GetPR handles GET /api/v1/prs/{id}
func (h *Handler) GetPR(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	pr, err := h.prRepo.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, errors.ErrNotFound) {
			writeError(w, http.StatusNotFound, "pull request not found", err)
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get PR", err)
		return
	}

	writeJSON(w, http.StatusOK, pr)
}

// AnalyzeIncident handles POST /api/v1/incidents/{id}/analyze
func (h *Handler) AnalyzeIncident(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	inc, err := h.incidentRepo.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, errors.ErrNotFound) {
			writeError(w, http.StatusNotFound, "incident not found", err)
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get incident", err)
		return
	}

	// Update status to analyzing
	if err := inc.MarkAnalyzing(); err != nil {
		writeError(w, http.StatusConflict, "invalid incident status for analysis", err)
		return
	}
	if err := h.incidentRepo.Update(r.Context(), inc); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update incident status", err)
		return
	}

	// Publish analyze task to NATS
	task := map[string]string{"incident_id": inc.ID}
	if err := h.publisher.Publish(r.Context(), "incidents.analyze", task); err != nil {
		// Rollback status to detected
		inc.Status = incident.StatusDetected
		if rbErr := h.incidentRepo.Update(r.Context(), inc); rbErr != nil {
			logger.Get().Error("failed to rollback incident status", zap.Error(rbErr))
		}
		writeError(w, http.StatusInternalServerError, "failed to publish analyze task", err)
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]string{"status": "analyzing"})
}

// Middleware: AuthMiddleware provides basic token authentication.
func AuthMiddleware(token string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if token == "" {
				next.ServeHTTP(w, r)
				return
			}

			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				writeError(w, http.StatusUnauthorized, "missing authorization header", nil)
				return
			}

			if authHeader != "Bearer "+token {
				writeError(w, http.StatusForbidden, "invalid token", nil)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string, err error) {
	finalStatus := status
	finalMessage := message

	if err != nil {
		// Structured logging of the raw internal error for debugging
		logger.Get().Error("HTTP handler error",
			zap.Int("status", status),
			zap.String("message", message),
			zap.Error(err),
		)

		// Map domain error to appropriate HTTP status code if default/internal error is used
		if status == 0 || status == http.StatusInternalServerError {
			mappedStatus, mappedMsg := mapDomainErrorToHTTP(err)
			finalStatus = mappedStatus
			if finalMessage == "" || finalMessage == "Internal server error" {
				finalMessage = mappedMsg
			}
		}
	}

	if finalStatus == 0 {
		finalStatus = http.StatusInternalServerError
	}
	if finalMessage == "" {
		finalMessage = http.StatusText(finalStatus)
	}

	resp := map[string]string{"error": finalMessage}
	writeJSON(w, finalStatus, resp)
}

// decodeJSON decodes the request body into a value of type T and validates it if T or *T implements Validator.
// Returns the decoded value and true on success, or writes a 400 error and returns false.
func decodeJSON[T any](w http.ResponseWriter, r *http.Request) (*T, bool) {
	var v T
	if err := json.NewDecoder(r.Body).Decode(&v); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", err)
		return nil, false
	}

	var valErr error
	if validator, ok := any(&v).(Validator); ok {
		valErr = validator.Validate()
	} else if validator, ok := any(v).(Validator); ok {
		valErr = validator.Validate()
	}

	if valErr != nil {
		writeValidationError(w, valErr)
		return nil, false
	}

	return &v, true
}

func writeValidationError(w http.ResponseWriter, err error) {
	if ve, ok := err.(*ValidationError); ok {
		writeJSON(w, http.StatusBadRequest, map[string]interface{}{
			"error":   "validation failed",
			"details": ve.Fields,
		})
		return
	}
	writeJSON(w, http.StatusBadRequest, map[string]interface{}{
		"error":  "validation failed",
		"detail": err.Error(),
	})
}

func parseIntParam(r *http.Request, key string, defaultVal int) int {
	val := r.URL.Query().Get(key)
	if val == "" {
		return defaultVal
	}
	parsed, err := strconv.Atoi(val)
	if err != nil {
		return defaultVal
	}
	return parsed
}

// parseUUIDParam extracts and validates an ID URL parameter.
func parseUUIDParam(r *http.Request, param string) (string, error) {
	id := chi.URLParam(r, param)
	if id == "" {
		return "", fmt.Errorf("missing parameter: %s", param)
	}
	if len(id) > 128 {
		return "", fmt.Errorf("parameter too long: %s", param)
	}
	return id, nil
}

type createPRRequest struct {
	gitops.CreatePRRequest
}

func (r *createPRRequest) Validate() error {
	ve := NewValidationError("validation failed")
	if strings.TrimSpace(r.IncidentID) == "" && strings.TrimSpace(r.Title) == "" {
		ve.Add("incident_id", "incident_id or title is required")
	}
	if ve.HasErrors() {
		return ve
	}
	return nil
}

// CreatePR handles POST /api/v1/prs
func (h *Handler) CreatePR(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeJSON[createPRRequest](w, r)
	if !ok {
		return
	}

	pr, err := h.gitopsController.CreatePR(r.Context(), req.CreatePRRequest)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create PR", err)
		return
	}

	writeJSON(w, http.StatusCreated, pr)
}

// MergePR handles POST /api/v1/prs/{id}/merge
func (h *Handler) MergePR(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "PR id is required", err)
		return
	}
	pr, err := h.gitopsController.MergePR(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to merge PR", err)
		return
	}

	writeJSON(w, http.StatusOK, pr)
}

// ClosePR handles POST /api/v1/prs/{id}/close
func (h *Handler) ClosePR(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "PR id is required", err)
		return
	}
	pr, err := h.gitopsController.ClosePR(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to close PR", err)
		return
	}

	writeJSON(w, http.StatusOK, pr)
}
