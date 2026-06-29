package http

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/datdt/k8sselfhost/internal/domain/promotion"
)

// PromotionHandler provides HTTP handlers for the deployment promotion API.
type PromotionHandler struct {
	repo promotion.Repository
}

// NewPromotionHandler creates a new promotion HTTP handler.
func NewPromotionHandler(repo promotion.Repository) *PromotionHandler {
	return &PromotionHandler{repo: repo}
}

// RegisterRoutes registers deployment promotion routes.
func (h *PromotionHandler) RegisterRoutes(r chi.Router) {
	r.Get("/", h.ListPromotions)
	r.Post("/", h.CreatePromotion)
	r.Put("/{id}/approve", h.ApprovePromotion)
	r.Put("/{id}/complete", h.CompletePromotion)
}

// ListPromotions handles GET /api/v1/promotions
func (h *PromotionHandler) ListPromotions(w http.ResponseWriter, r *http.Request) {
	limit := parseIntParam(r, "limit", 50)
	offset := parseIntParam(r, "offset", 0)
	status := r.URL.Query().Get("status")

	items, total, err := h.repo.List(r.Context(), status, limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list promotions", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": items, "total": total})
}

// CreatePromotion handles POST /api/v1/promotions
func (h *PromotionHandler) CreatePromotion(w http.ResponseWriter, r *http.Request) {
	var p promotion.Promotion
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}
	if err := h.repo.Create(r.Context(), &p); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create promotion", err)
		return
	}
	writeJSON(w, http.StatusCreated, p)
}

// ApprovePromotion handles PUT /api/v1/promotions/{id}/approve
func (h *PromotionHandler) ApprovePromotion(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	approver := r.URL.Query().Get("approver")
	if approver == "" {
		approver = "system"
	}

	if err := h.repo.Approve(r.Context(), id, approver); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to approve promotion", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "approved"})
}

// CompletePromotion handles PUT /api/v1/promotions/{id}/complete
func (h *PromotionHandler) CompletePromotion(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.repo.Complete(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to complete promotion", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "completed"})
}
