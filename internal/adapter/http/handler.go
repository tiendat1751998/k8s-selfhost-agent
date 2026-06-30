// Package http provides HTTP handlers for the dashboard API.
package http

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/datdt/k8sselfhost/internal/domain/incident"
	"github.com/datdt/k8sselfhost/internal/domain/report"
	domainGitops "github.com/datdt/k8sselfhost/internal/domain/gitops"
	infraNats "github.com/datdt/k8sselfhost/internal/infrastructure/nats"
	"github.com/datdt/k8sselfhost/pkg/errors"
)

// Handler provides HTTP handlers for the dashboard API.
type Handler struct {
	incidentRepo incident.Repository
	reportRepo   report.Repository
	prRepo       domainGitops.Repository
	publisher    *infraNats.Publisher
}

// NewHandler creates a new dashboard HTTP handler.
func NewHandler(incidentRepo incident.Repository, reportRepo report.Repository, prRepo domainGitops.Repository, publisher *infraNats.Publisher) *Handler {
	return &Handler{
		incidentRepo: incidentRepo,
		reportRepo:   reportRepo,
		prRepo:       prRepo,
		publisher:    publisher,
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
		"data":  incidents,
		"total": total,
		"limit": filter.Limit,
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
		_ = h.incidentRepo.Update(r.Context(), inc)
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

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string, err error) {
	resp := map[string]string{"error": message}
	if err != nil {
		resp["detail"] = err.Error()
	}
	writeJSON(w, status, resp)
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

// CreatePR handles POST /api/v1/prs
func (h *Handler) CreatePR(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IncidentID   string                    `json:"incident_id"`
		Provider     domainGitops.Provider     `json:"provider"`
		RepoURL      string                    `json:"repo_url"`
		Branch       string                    `json:"branch"`
		BaseBranch   string                    `json:"base_branch"`
		Title        string                    `json:"title"`
		Description  string                    `json:"description"`
		FilesChanged []struct {
			Path    string                  `json:"path"`
			Content string                  `json:"content"`
			Action  domainGitops.FileAction `json:"action"`
		} `json:"files_changed"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}

	// In CreatePR, check if an incident exists to satisfy foreign key constraints, or link/create a default system incident.
	var incID = req.IncidentID
	if incID != "" {
		_, err := h.incidentRepo.GetByID(r.Context(), incID)
		if err != nil {
			incID = ""
		}
	}

	if incID == "" {
		// Find or create default system incident
		filter := incident.Filter{
			ClusterName: "system",
			Namespace:   "system",
			Limit:       1,
		}
		incidents, _, err := h.incidentRepo.List(r.Context(), filter)
		if err == nil && len(incidents) > 0 {
			incID = incidents[0].ID
		} else {
			defaultInc, err := incident.New("system", "system", "system-pod", incident.TypeCrashLoopBackOff, incident.SeverityMedium, "Default system incident for orphaned PRs")
			if err != nil {
				writeError(w, http.StatusInternalServerError, "failed to create default incident struct", err)
				return
			}
			if err := h.incidentRepo.Create(r.Context(), defaultInc); err != nil {
				writeError(w, http.StatusInternalServerError, "failed to persist default incident", err)
				return
			}
			incID = defaultInc.ID
		}
	}

	provider := req.Provider
	if provider == "" {
		provider = domainGitops.ProviderGitHub
	}
	repoURL := req.RepoURL
	if repoURL == "" {
		repoURL = "https://github.com/k8s-selfhost-agent/k8s-selfhost-agent"
	}
	branch := req.Branch
	if branch == "" {
		branch = "gitops-patch"
	}
	baseBranch := req.BaseBranch
	if baseBranch == "" {
		baseBranch = "main"
	}
	title := req.Title
	if title == "" {
		title = "Remediation PR"
	}

	pr, err := domainGitops.New(incID, provider, repoURL, branch, baseBranch, title, req.Description)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid PR fields", err)
		return
	}

	for _, f := range req.FilesChanged {
		action := f.Action
		if action == "" {
			action = domainGitops.FileActionModify
		}
		pr.AddFileChange(f.Path, f.Content, action)
	}

	// Since we are registering it, make it default to Open so it can be merged/closed in UI
	_ = pr.MarkOpen(repoURL+"/pull/1", 1)

	if err := h.prRepo.Create(r.Context(), pr); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create PR", err)
		return
	}

	writeJSON(w, http.StatusCreated, pr)
}

// MergePR handles POST /api/v1/prs/{id}/merge
func (h *Handler) MergePR(w http.ResponseWriter, r *http.Request) {
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

	if err := pr.MarkMerged(); err != nil {
		writeError(w, http.StatusConflict, "failed to mark PR as merged", err)
		return
	}

	if err := h.prRepo.Update(r.Context(), pr); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update PR", err)
		return
	}

	// Update the associated incident to resolved if there is one
	if pr.IncidentID != "" {
		inc, err := h.incidentRepo.GetByID(r.Context(), pr.IncidentID)
		if err == nil && inc != nil {
			if err := inc.MarkResolved(); err == nil {
				_ = h.incidentRepo.Update(r.Context(), inc)
			}
		}
	}

	writeJSON(w, http.StatusOK, pr)
}

// ClosePR handles POST /api/v1/prs/{id}/close
func (h *Handler) ClosePR(w http.ResponseWriter, r *http.Request) {
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

	if err := pr.MarkClosed(); err != nil {
		writeError(w, http.StatusConflict, "failed to mark PR as closed", err)
		return
	}

	if err := h.prRepo.Update(r.Context(), pr); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update PR", err)
		return
	}

	writeJSON(w, http.StatusOK, pr)
}
