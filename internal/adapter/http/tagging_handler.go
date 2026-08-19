package http

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/datdt/k8sselfhost/internal/domain/tagging"
)

// TaggingHandler provides HTTP handlers for the tagging system API.
type TaggingHandler struct {
	repo tagging.Repository
}

// NewTaggingHandler creates a new tagging HTTP handler.
func NewTaggingHandler(repo tagging.Repository) *TaggingHandler {
	return &TaggingHandler{repo: repo}
}

// RegisterRoutes registers tagging routes.
func (h *TaggingHandler) RegisterRoutes(r chi.Router) {
	r.Get("/", h.ListTags)
	r.Post("/", h.CreateTag)
	r.Delete("/{id}", h.DeleteTag)

	r.Get("/resource/{resourceID}", h.GetResourceTags)
	r.Post("/resource/{resourceID}", h.TagResource)
	r.Delete("/resource/{resourceID}/tag/{tagID}", h.UntagResource)
}

// ListTags handles GET /api/v1/tags
func (h *TaggingHandler) ListTags(w http.ResponseWriter, r *http.Request) {
	items, err := h.repo.ListTags(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list tags", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": items})
}

type createTagRequest struct {
	tagging.Tag
}

func (r *createTagRequest) Validate() error {
	ve := NewValidationError("validation failed")
	if strings.TrimSpace(r.Key) == "" {
		ve.Add("key", "key is required")
	}
	if strings.TrimSpace(r.Value) == "" {
		ve.Add("value", "value is required")
	}
	if ve.HasErrors() {
		return ve
	}
	return nil
}

// CreateTag handles POST /api/v1/tags
func (h *TaggingHandler) CreateTag(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeJSON[createTagRequest](w, r)
	if !ok {
		return
	}
	if err := h.repo.CreateTag(r.Context(), &req.Tag); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create tag", err)
		return
	}
	writeJSON(w, http.StatusCreated, req.Tag)
}

// DeleteTag handles DELETE /api/v1/tags/{id}
func (h *TaggingHandler) DeleteTag(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "tag id is required", err)
		return
	}
	if err := h.repo.DeleteTag(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete tag", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// GetResourceTags handles GET /api/v1/tags/resource/{resourceID}
func (h *TaggingHandler) GetResourceTags(w http.ResponseWriter, r *http.Request) {
	resID := chi.URLParam(r, "resourceID")
	items, err := h.repo.GetResourceTags(r.Context(), resID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get resource tags", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"data": items})
}

type tagResourceRequest struct {
	TagID string `json:"tag_id"`
}

func (r *tagResourceRequest) Validate() error {
	ve := NewValidationError("validation failed")
	if strings.TrimSpace(r.TagID) == "" {
		ve.Add("tag_id", "tag_id is required")
	}
	if ve.HasErrors() {
		return ve
	}
	return nil
}

// TagResource handles POST /api/v1/tags/resource/{resourceID}
func (h *TaggingHandler) TagResource(w http.ResponseWriter, r *http.Request) {
	resID := chi.URLParam(r, "resourceID")

	req, ok := decodeJSON[tagResourceRequest](w, r)
	if !ok {
		return
	}

	if err := h.repo.TagResource(r.Context(), resID, req.TagID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to tag resource", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "tagged"})
}

// UntagResource handles DELETE /api/v1/tags/resource/{resourceID}/tag/{tagID}
func (h *TaggingHandler) UntagResource(w http.ResponseWriter, r *http.Request) {
	resID := chi.URLParam(r, "resourceID")
	tagID := chi.URLParam(r, "tagID")

	if err := h.repo.UntagResource(r.Context(), resID, tagID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to untag resource", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "untagged"})
}
