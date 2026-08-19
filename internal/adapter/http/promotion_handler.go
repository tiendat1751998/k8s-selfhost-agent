package http

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/datdt/k8sselfhost/internal/adapter/http/middleware"
	"github.com/datdt/k8sselfhost/internal/domain/promotion"
	promotionUsecase "github.com/datdt/k8sselfhost/internal/usecase/promotion"
)

// PromotionHandler provides HTTP handlers for the deployment promotion API.
type PromotionHandler struct {
	usecase *promotionUsecase.Usecase
}

// NewPromotionHandler creates a new promotion HTTP handler with injected usecase.
func NewPromotionHandler(usecase *promotionUsecase.Usecase) *PromotionHandler {
	return &PromotionHandler{usecase: usecase}
}

// RegisterRoutes registers deployment promotion routes.
func (h *PromotionHandler) RegisterRoutes(r chi.Router) {
	r.Get("/", h.ListPromotions)
	r.Post("/", h.CreatePromotion)
	r.Put("/{id}/approve", h.ApprovePromotion)
	r.Put("/{id}/reject", h.RejectPromotion)
	r.Put("/{id}/complete", h.CompletePromotion)
}

// ListPromotions handles GET /api/v1/promotions
func (h *PromotionHandler) ListPromotions(w http.ResponseWriter, r *http.Request) {
	limit := parseIntParam(r, "limit", 50)
	offset := parseIntParam(r, "offset", 0)
	status := r.URL.Query().Get("status")

	items, total, err := h.usecase.List(r.Context(), status, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list promotions", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": items, "total": total})
}

type createPromotionRequest struct {
	promotion.Promotion
}

func (r *createPromotionRequest) Validate() error {
	ve := NewValidationError("validation failed")
	if strings.TrimSpace(r.Service) == "" {
		ve.Add("service", "service is required")
	}
	if strings.TrimSpace(r.Version) == "" {
		ve.Add("version", "version is required")
	}
	if strings.TrimSpace(string(r.FromEnv)) == "" {
		ve.Add("from_env", "from_env is required")
	}
	if strings.TrimSpace(string(r.ToEnv)) == "" {
		ve.Add("to_env", "to_env is required")
	}
	if ve.HasErrors() {
		return ve
	}
	return nil
}

// CreatePromotion handles POST /api/v1/promotions
func (h *PromotionHandler) CreatePromotion(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeJSON[createPromotionRequest](w, r)
	if !ok {
		return
	}
	created, err := h.usecase.Create(r.Context(), &req.Promotion)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create promotion", err)
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

// ApprovePromotion handles PUT /api/v1/promotions/{id}/approve
func (h *PromotionHandler) ApprovePromotion(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "promotion id is required", err)
		return
	}
	approver, _ := r.Context().Value(middleware.UserIDKey).(string)

	if err := h.usecase.Approve(r.Context(), id, approver); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to approve promotion", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "approved"})
}

// CompletePromotion handles PUT /api/v1/promotions/{id}/complete
func (h *PromotionHandler) CompletePromotion(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "promotion id is required", err)
		return
	}
	if err := h.usecase.Complete(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to complete promotion", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "completed"})
}

// RejectPromotion handles PUT /api/v1/promotions/{id}/reject
func (h *PromotionHandler) RejectPromotion(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "promotion id is required", err)
		return
	}
	rejecter, _ := r.Context().Value(middleware.UserIDKey).(string)

	if err := h.usecase.Reject(r.Context(), id, rejecter); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to reject promotion", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "rejected"})
}
