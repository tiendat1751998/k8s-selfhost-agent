package http

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/datdt/k8sselfhost/internal/domain/incident"
	"github.com/datdt/k8sselfhost/internal/pkg/errors"
	"github.com/datdt/k8sselfhost/internal/pkg/tenancy"
	usecaseIncident "github.com/datdt/k8sselfhost/internal/usecase/incident"
)

type createIncidentRequest struct {
	Title       string            `json:"title"`
	Severity    string            `json:"severity"`
	Description string            `json:"description"`
	ClusterID   string            `json:"cluster_id"`
	ServiceName string            `json:"service_name"`
	ClusterName string            `json:"cluster_name"`
	Namespace   string            `json:"namespace"`
	PodName     string            `json:"pod_name"`
	Type        string            `json:"type"`
	Message     string            `json:"message"`
	RawData     map[string]string `json:"raw_data"`
}

type mitigateIncidentRequest struct {
	Notes  string `json:"notes"`
	Status string `json:"status"`
}

type resolveIncidentRequest struct {
	Notes string `json:"notes"`
}

func (h *Handler) getIncidentUsecase() *usecaseIncident.Usecase {
	if h.incidentUsecase == nil && h.incidentRepo != nil {
		h.incidentUsecase = usecaseIncident.NewUsecase(h.incidentRepo)
	}
	return h.incidentUsecase
}

// CreateIncident handles POST /api/v1/incidents
func (h *Handler) CreateIncident(w http.ResponseWriter, r *http.Request) {
	var req createIncidentRequest
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err != io.EOF {
			writeError(w, http.StatusBadRequest, "invalid request body", err)
			return
		}
	}

	title := strings.TrimSpace(req.Title)
	if title == "" && strings.TrimSpace(req.Message) != "" {
		title = strings.TrimSpace(req.Message)
	}
	if title == "" {
		writeError(w, http.StatusBadRequest, "title is required", errors.NewValidation("title", "must not be empty"))
		return
	}

	sevStr := strings.ToLower(strings.TrimSpace(req.Severity))
	if sevStr == "" || !incident.Severity(sevStr).IsValid() {
		writeError(w, http.StatusBadRequest, "invalid or missing severity", errors.NewValidation("severity", "must be critical, high, medium, or low"))
		return
	}

	description := strings.TrimSpace(req.Description)
	if description == "" && strings.TrimSpace(req.Message) != "" {
		description = strings.TrimSpace(req.Message)
	}
	if description == "" {
		writeError(w, http.StatusBadRequest, "description is required", errors.NewValidation("description", "must not be empty"))
		return
	}

	clusterID := strings.TrimSpace(req.ClusterID)
	if clusterID == "" {
		clusterID = strings.TrimSpace(req.ClusterName)
	}
	if clusterID == "" {
		writeError(w, http.StatusBadRequest, "cluster_id is required", errors.NewValidation("cluster_id", "must not be empty"))
		return
	}

	serviceName := strings.TrimSpace(req.ServiceName)
	if serviceName == "" {
		serviceName = strings.TrimSpace(req.PodName)
	}
	if serviceName == "" {
		writeError(w, http.StatusBadRequest, "service_name is required", errors.NewValidation("service_name", "must not be empty"))
		return
	}

	tenantID := resolveTenant(r.Context(), r)
	if tenantID == "" {
		tenantID = "default-tenant"
	}
	ctx := tenancy.WithTenantID(r.Context(), tenantID)

	uc := h.getIncidentUsecase()
	if uc == nil {
		writeError(w, http.StatusInternalServerError, "incident usecase not configured", errors.ErrInternal)
		return
	}

	cmd := usecaseIncident.CreateCommand{
		Title:       title,
		Severity:    sevStr,
		Description: description,
		ClusterID:   clusterID,
		ServiceName: serviceName,
		ClusterName: clusterID,
		Namespace:   strings.TrimSpace(req.Namespace),
		PodName:     serviceName,
		Type:        strings.TrimSpace(req.Type),
		TenantID:    tenantID,
		RawData:     req.RawData,
	}

	inc, err := uc.CreateIncident(ctx, cmd)
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to create incident", err)
		return
	}

	writeJSON(w, http.StatusCreated, inc)
}

// MitigateIncident handles POST /api/v1/incidents/{id}/mitigate
func (h *Handler) MitigateIncident(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if strings.TrimSpace(id) == "" {
		writeError(w, http.StatusBadRequest, "incident id is required", errors.NewValidation("id", "must not be empty"))
		return
	}

	var req mitigateIncidentRequest
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}

	tenantID := resolveTenant(r.Context(), r)
	if tenantID == "" {
		tenantID = "default-tenant"
	}
	ctx := tenancy.WithTenantID(r.Context(), tenantID)

	operator := tenancy.UserIDFromContext(r.Context())
	if operator == "" {
		operator = "operator"
	}

	targetStatus := incident.StatusMitigating
	if strings.ToLower(strings.TrimSpace(req.Status)) == string(incident.StatusMitigated) {
		targetStatus = incident.StatusMitigated
	}

	uc := h.getIncidentUsecase()
	if uc == nil {
		writeError(w, http.StatusInternalServerError, "incident usecase not configured", errors.ErrInternal)
		return
	}

	cmd := usecaseIncident.MitigateCommand{
		Operator: operator,
		Notes:    strings.TrimSpace(req.Notes),
		Status:   targetStatus,
	}

	inc, err := uc.MitigateIncident(ctx, id, cmd)
	if err != nil {
		if errors.Is(err, errors.ErrNotFound) {
			writeError(w, http.StatusNotFound, "incident not found", err)
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to mitigate incident", err)
		return
	}

	writeJSON(w, http.StatusOK, inc)
}

// ResolveIncident handles POST /api/v1/incidents/{id}/resolve
func (h *Handler) ResolveIncident(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if strings.TrimSpace(id) == "" {
		writeError(w, http.StatusBadRequest, "incident id is required", errors.NewValidation("id", "must not be empty"))
		return
	}

	var req resolveIncidentRequest
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}

	tenantID := resolveTenant(r.Context(), r)
	if tenantID == "" {
		tenantID = "default-tenant"
	}
	ctx := tenancy.WithTenantID(r.Context(), tenantID)

	operator := tenancy.UserIDFromContext(r.Context())
	if operator == "" {
		operator = "operator"
	}

	uc := h.getIncidentUsecase()
	if uc == nil {
		writeError(w, http.StatusInternalServerError, "incident usecase not configured", errors.ErrInternal)
		return
	}

	cmd := usecaseIncident.ResolveCommand{
		Operator: operator,
		Notes:    strings.TrimSpace(req.Notes),
	}

	inc, err := uc.ResolveIncident(ctx, id, cmd)
	if err != nil {
		if errors.Is(err, errors.ErrNotFound) {
			writeError(w, http.StatusNotFound, "incident not found", err)
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to resolve incident", err)
		return
	}

	writeJSON(w, http.StatusOK, inc)
}

// ListIncidents handles GET /api/v1/incidents
func (h *Handler) ListIncidents(w http.ResponseWriter, r *http.Request) {
	tenantID := resolveTenant(r.Context(), r)
	if tenantID == "" {
		tenantID = "default-tenant"
	}
	ctx := tenancy.WithTenantID(r.Context(), tenantID)

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

	incidents, total, err := h.incidentRepo.List(ctx, filter)
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
	tenantID := resolveTenant(r.Context(), r)
	if tenantID == "" {
		tenantID = "default-tenant"
	}
	ctx := tenancy.WithTenantID(r.Context(), tenantID)

	id := chi.URLParam(r, "id")
	inc, err := h.incidentRepo.GetByID(ctx, id)
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
