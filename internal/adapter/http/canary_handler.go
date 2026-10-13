package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/datdt/k8sselfhost/internal/domain/promotion"
)

// CanaryHandler exposes HTTP endpoints for managing automated progressive canary releases.
type CanaryHandler struct {
	engine promotion.ProgressiveDeliveryEngine
}

// NewCanaryHandler creates a new CanaryHandler instance with injected delivery engine.
func NewCanaryHandler(engine promotion.ProgressiveDeliveryEngine) *CanaryHandler {
	return &CanaryHandler{engine: engine}
}

// RegisterRoutes registers canary progressive delivery routes on a chi.Router.
func (h *CanaryHandler) RegisterRoutes(r chi.Router) {
	r.Post("/start", h.StartCanary)
	r.Post("/{id}/evaluate", h.EvaluateCanary)
	r.Post("/{id}/promote", h.PromoteCanary)
	r.Post("/{id}/abort", h.AbortCanary)
	r.Get("/{id}", h.GetCanary)
	r.Get("/", h.ListCanaries)
}

type startCanaryRequest struct {
	Workload  string                 `json:"workload"`
	Namespace string                 `json:"namespace"`
	NewImage  string                 `json:"new_image"`
	Steps     []promotion.CanaryStep `json:"steps,omitempty"`
}

// StartCanary handles POST /api/v1/promotions/canary/start
func (h *CanaryHandler) StartCanary(w http.ResponseWriter, r *http.Request) {
	var req startCanaryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeProblem(w, http.StatusBadRequest, "BAD_REQUEST", "invalid request body")
		return
	}

	if strings.TrimSpace(req.Workload) == "" {
		writeProblem(w, http.StatusBadRequest, "VALIDATION_ERROR", "workload is required")
		return
	}
	if strings.TrimSpace(req.Namespace) == "" {
		writeProblem(w, http.StatusBadRequest, "VALIDATION_ERROR", "namespace is required")
		return
	}
	if strings.TrimSpace(req.NewImage) == "" {
		writeProblem(w, http.StatusBadRequest, "VALIDATION_ERROR", "new_image is required")
		return
	}

	exec, err := h.engine.StartCanaryRelease(r.Context(), req.Workload, req.Namespace, req.NewImage, req.Steps)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, exec)
}

// EvaluateCanary handles POST /api/v1/promotions/canary/:id/evaluate
func (h *CanaryHandler) EvaluateCanary(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		id = chi.URLParam(r, ":id")
	}

	comparison, err := h.engine.EvaluateCanaryHealth(r.Context(), id)
	if err != nil {
		if errors.Is(err, promotion.ErrCanaryNotFound) {
			writeProblem(w, http.StatusNotFound, "NOT_FOUND", fmt.Sprintf("canary release %s not found", id))
			return
		}
		writeProblem(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, comparison)
}

// PromoteCanary handles POST /api/v1/promotions/canary/:id/promote
func (h *CanaryHandler) PromoteCanary(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		id = chi.URLParam(r, ":id")
	}

	err := h.engine.PromoteNextStep(r.Context(), id)
	if err != nil {
		if errors.Is(err, promotion.ErrCanaryNotFound) {
			writeProblem(w, http.StatusNotFound, "NOT_FOUND", fmt.Sprintf("canary release %s not found", id))
			return
		}
		if errors.Is(err, promotion.ErrCanaryAbortedHighErrors) || errors.Is(err, promotion.ErrCanaryAbortedHighLatency) {
			writeProblem(w, http.StatusUnprocessableEntity, promotion.ErrCodeCanaryAbortedHighErrors, err.Error())
			return
		}
		writeProblem(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"status": "promoted", "release_id": id})
}

// AbortCanary handles POST /api/v1/promotions/canary/:id/abort
func (h *CanaryHandler) AbortCanary(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		id = chi.URLParam(r, ":id")
	}

	var req struct {
		Reason string `json:"reason"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	if req.Reason == "" {
		req.Reason = "manual abort"
	}

	err := h.engine.AbortCanaryRollback(r.Context(), id, req.Reason)
	if err != nil {
		if errors.Is(err, promotion.ErrCanaryNotFound) {
			writeProblem(w, http.StatusNotFound, "NOT_FOUND", fmt.Sprintf("canary release %s not found", id))
			return
		}
		writeProblem(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "aborted", "reason": req.Reason, "release_id": id})
}

// GetCanary handles GET /api/v1/promotions/canary/:id
func (h *CanaryHandler) GetCanary(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		id = chi.URLParam(r, ":id")
	}

	exec, err := h.engine.GetCanaryRelease(r.Context(), id)
	if err != nil {
		if errors.Is(err, promotion.ErrCanaryNotFound) {
			writeProblem(w, http.StatusNotFound, "NOT_FOUND", fmt.Sprintf("canary release %s not found", id))
			return
		}
		writeProblem(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, exec)
}

// ListCanaries handles GET /api/v1/promotions/canary
func (h *CanaryHandler) ListCanaries(w http.ResponseWriter, r *http.Request) {
	items, err := h.engine.ListCanaryReleases(r.Context())
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{"data": items, "total": len(items)})
}

func writeProblem(w http.ResponseWriter, status int, code, detail string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"type":   "https://k8sselfhost.io/errors/" + strings.ToLower(code),
		"title":  http.StatusText(status),
		"status": status,
		"detail": detail,
		"code":   code,
	})
}
