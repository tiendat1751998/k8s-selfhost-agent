package http

import (
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/datdt/k8sselfhost/internal/domain/observability"
)

// ObservabilityHandler provides HTTP handlers for the observability API.
type ObservabilityHandler struct {
	repo observability.Repository
}

// NewObservabilityHandler creates a new observability HTTP handler.
func NewObservabilityHandler(repo observability.Repository) *ObservabilityHandler {
	return &ObservabilityHandler{repo: repo}
}

// RegisterRoutes registers observability routes.
func (h *ObservabilityHandler) RegisterRoutes(r chi.Router) {
	r.Get("/slo", h.ListSLOSnapshots)
	r.Get("/slo/definitions", h.ListSLODefinitions)
	r.Get("/slo/definitions/{id}", h.GetSLODefinition)
	r.Post("/slo/definitions", h.CreateSLODefinition)
	r.Put("/slo/definitions/{id}", h.UpdateSLODefinition)
	r.Delete("/slo/definitions/{id}", h.DeleteSLODefinition)
	r.Post("/slo/definitions/{id}/trigger-alert", h.TriggerBurnAlert)
}

// ListSLODefinitions handles GET /api/v1/observability/slo/definitions
func (h *ObservabilityHandler) ListSLODefinitions(w http.ResponseWriter, r *http.Request) {
	defs, err := h.repo.ListSLODefinitions(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list SLO definitions", err)
		return
	}
	if defs == nil {
		defs = make([]observability.SLODefinition, 0)
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": defs})
}

// GetSLODefinition handles GET /api/v1/observability/slo/definitions/{id}
func (h *ObservabilityHandler) GetSLODefinition(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if strings.TrimSpace(id) == "" {
		writeError(w, http.StatusBadRequest, "slo definition id is required", nil)
		return
	}

	def, err := h.repo.GetSLODefinition(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get SLO definition", err)
		return
	}
	if def == nil {
		writeError(w, http.StatusNotFound, "SLO definition not found", nil)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"data": def})
}

type createSLODefinitionRequest struct {
	Service        string  `json:"service"`
	Target         float64 `json:"target"`
	IndicatorType  string  `json:"indicator_type"`
	Window         string  `json:"window"`
	Query          string  `json:"query"`
	AlertThreshold float64 `json:"alert_threshold"`
}

func (r *createSLODefinitionRequest) Validate() error {
	if strings.TrimSpace(r.Service) == "" {
		return fmt.Errorf("service is required")
	}
	if r.Target <= 0 {
		return fmt.Errorf("target must be greater than 0")
	}
	return nil
}

// CreateSLODefinition handles POST /api/v1/observability/slo/definitions
func (h *ObservabilityHandler) CreateSLODefinition(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeJSON[createSLODefinitionRequest](w, r)
	if !ok {
		return
	}
	if err := req.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error(), err)
		return
	}

	target := req.Target
	if target <= 1.0 && target > 0 {
		target = target * 100
	}
	indicatorType := req.IndicatorType
	if strings.TrimSpace(indicatorType) == "" {
		indicatorType = "availability"
	}
	window := req.Window
	if strings.TrimSpace(window) == "" {
		window = "30d"
	}
	threshold := req.AlertThreshold
	if threshold <= 0 {
		threshold = 1.5
	}

	def := &observability.SLODefinition{
		ID:             uuid.NewString(),
		Service:        strings.TrimSpace(req.Service),
		Target:         target,
		IndicatorType:  indicatorType,
		Window:         window,
		Query:          strings.TrimSpace(req.Query),
		AlertThreshold: threshold,
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	}

	if err := h.repo.CreateSLODefinition(r.Context(), def); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create SLO definition", err)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{"data": def})
}

type updateSLODefinitionRequest struct {
	Service        string  `json:"service"`
	Target         float64 `json:"target"`
	IndicatorType  string  `json:"indicator_type"`
	Window         string  `json:"window"`
	Query          string  `json:"query"`
	AlertThreshold float64 `json:"alert_threshold"`
}

// UpdateSLODefinition handles PUT /api/v1/observability/slo/definitions/{id}
func (h *ObservabilityHandler) UpdateSLODefinition(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if strings.TrimSpace(id) == "" {
		writeError(w, http.StatusBadRequest, "slo definition id is required", nil)
		return
	}

	req, ok := decodeJSON[updateSLODefinitionRequest](w, r)
	if !ok {
		return
	}

	existing, err := h.repo.GetSLODefinition(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch SLO definition", err)
		return
	}
	if existing == nil {
		writeError(w, http.StatusNotFound, "SLO definition not found", nil)
		return
	}

	if strings.TrimSpace(req.Service) != "" {
		existing.Service = strings.TrimSpace(req.Service)
	}
	if req.Target > 0 {
		target := req.Target
		if target <= 1.0 && target > 0 {
			target = target * 100
		}
		existing.Target = target
	}
	if strings.TrimSpace(req.IndicatorType) != "" {
		existing.IndicatorType = strings.TrimSpace(req.IndicatorType)
	}
	if strings.TrimSpace(req.Window) != "" {
		existing.Window = strings.TrimSpace(req.Window)
	}
	if req.Query != "" {
		existing.Query = strings.TrimSpace(req.Query)
	}
	if req.AlertThreshold > 0 {
		existing.AlertThreshold = req.AlertThreshold
	}
	existing.UpdatedAt = time.Now().UTC()

	if err := h.repo.UpdateSLODefinition(r.Context(), existing); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update SLO definition", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"data": existing})
}

// DeleteSLODefinition handles DELETE /api/v1/observability/slo/definitions/{id}
func (h *ObservabilityHandler) DeleteSLODefinition(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if strings.TrimSpace(id) == "" {
		writeError(w, http.StatusBadRequest, "slo definition id is required", nil)
		return
	}

	if err := h.repo.DeleteSLODefinition(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete SLO definition", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"message": "SLO definition deleted successfully"})
}

// TriggerBurnAlert handles POST /api/v1/observability/slo/definitions/{id}/trigger-alert
func (h *ObservabilityHandler) TriggerBurnAlert(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if strings.TrimSpace(id) == "" {
		writeError(w, http.StatusBadRequest, "slo definition id is required", nil)
		return
	}

	snap, err := h.repo.GetSLOSnapshotBySLOID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to retrieve SLO snapshot", err)
		return
	}

	if snap == nil {
		def, defErr := h.repo.GetSLODefinition(r.Context(), id)
		if defErr != nil || def == nil {
			writeError(w, http.StatusNotFound, "SLO definition not found", defErr)
			return
		}
		snap = &observability.SLOSnapshot{
			ID:           uuid.NewString(),
			SLOID:        def.ID,
			Service:      def.Service,
			Target:       def.Target,
			Actual:       math.Max(90.0, def.Target-0.45),
			BurnRate:     3.45,
			ErrorBudget:  8.5,
			BudgetStatus: "critical",
			RecordedAt:   time.Now().UTC(),
		}
		_ = h.repo.CreateSLOSnapshot(r.Context(), snap)
	} else {
		snap.BurnRate = 3.25
		snap.Actual = math.Max(90.0, snap.Target-0.35)
		snap.ErrorBudget = 12.0
		snap.BudgetStatus = "critical"
		snap.RecordedAt = time.Now().UTC()
		_ = h.repo.UpdateSLOSnapshot(r.Context(), snap)
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success":  true,
		"message":  fmt.Sprintf("SLO Burn Alert triggered for service %s: fast burn rate %.2fx exceeds threshold", snap.Service, snap.BurnRate),
		"snapshot": snap,
	})
}

// ListSLOSnapshots handles GET /api/v1/observability/slo
func (h *ObservabilityHandler) ListSLOSnapshots(w http.ResponseWriter, r *http.Request) {
	snapshots, err := h.repo.ListSLOSnapshots(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list SLO snapshots", err)
		return
	}
	if snapshots == nil {
		snapshots = make([]observability.SLOSnapshot, 0)
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": snapshots})
}
