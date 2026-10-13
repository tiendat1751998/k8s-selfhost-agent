// Package http provides HTTP adapters and REST endpoints for k8sselfhost.
package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	domain "github.com/datdt/k8sselfhost/internal/domain/runtimeguard"
	usecase "github.com/datdt/k8sselfhost/internal/usecase/runtimeguard"
)

// MutateImageRequest models incoming payload for image mirror rewrite.
type MutateImageRequest struct {
	Image string `json:"image"`
}

// ConfigureMirrorRequest models payload for primary registry mirror configuration.
type ConfigureMirrorRequest struct {
	PrimaryRegistry        string `json:"primary_registry"`
	MirrorURL              string `json:"mirror_url"`
	FallbackTimeoutSeconds int    `json:"fallback_timeout_seconds"`
	FallbackTimeoutMs      int    `json:"fallback_timeout_ms"`
	RateLimitFloor         int    `json:"rate_limit_floor"`
}

// RuntimeGuardHandler exposes REST API endpoints for containerd storage health,
// dangling image pruning, image mirror rewrite, and P2P fallback configurations.
type RuntimeGuardHandler struct {
	manager *usecase.ResilienceManager
	logger  *zap.Logger
}

// NewRuntimeGuardHandler constructs a new RuntimeGuardHandler instance.
func NewRuntimeGuardHandler(manager *usecase.ResilienceManager, loggers ...*zap.Logger) *RuntimeGuardHandler {
	var l *zap.Logger
	if len(loggers) > 0 && loggers[0] != nil {
		l = loggers[0]
	} else {
		l = zap.NewNop()
	}
	return &RuntimeGuardHandler{
		manager: manager,
		logger:  l,
	}
}

// RegisterRoutes mounts resilience endpoints on the provided chi router.
func (h *RuntimeGuardHandler) RegisterRoutes(r chi.Router) {
	// Scoped routes
	r.Get("/nodes/storage-health", h.AuditStorageHealth)
	r.Post("/nodes/{node}/prune", h.TriggerPrune)
	r.Post("/images/mutate-mirror", h.MutateImageMirror)
	r.Post("/registries/configure-mirror", h.ConfigureMirror)

	// Full-path routes for top-level router mounting
	r.Get("/api/v1/runtime/nodes/storage-health", h.AuditStorageHealth)
	r.Post("/api/v1/runtime/nodes/{node}/prune", h.TriggerPrune)
	r.Post("/api/v1/runtime/images/mutate-mirror", h.MutateImageMirror)
	r.Post("/api/v1/runtime/registries/configure-mirror", h.ConfigureMirror)
}

// AuditStorageHealth handles GET /api/v1/runtime/nodes/storage-health.
func (h *RuntimeGuardHandler) AuditStorageHealth(w http.ResponseWriter, r *http.Request) {
	healths, err := h.manager.AuditNodeInodes(r.Context())
	if err != nil {
		if errors.Is(err, domain.ErrContainerdInodeSaturation) {
			h.writeProblem(w, http.StatusInsufficientStorage, "Insufficient Storage",
				err.Error(), domain.CodeContainerdInodeSaturation, r.URL.Path)
			return
		}
		h.logger.Error("Failed to audit node storage health", zap.Error(err))
		h.writeProblem(w, http.StatusInternalServerError, "Internal Server Error",
			err.Error(), "INTERNAL_ERROR", r.URL.Path)
			return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"nodes": healths,
		"count": len(healths),
	})
}

// TriggerPrune handles POST /api/v1/runtime/nodes/{node}/prune.
func (h *RuntimeGuardHandler) TriggerPrune(w http.ResponseWriter, r *http.Request) {
	node := chi.URLParam(r, "node")
	if node == "" {
		node = chi.URLParam(r, "node_name")
	}
	node = strings.TrimSpace(node)
	if node == "" {
		h.writeProblem(w, http.StatusBadRequest, "Bad Request",
			"node path parameter is required", "VALIDATION_ERROR", r.URL.Path)
		return
	}

	if err := h.manager.TriggerDanglingPrune(r.Context(), node); err != nil {
		h.logger.Error("Failed to trigger dangling image prune", zap.String("node", node), zap.Error(err))
		h.writeProblem(w, http.StatusInternalServerError, "Internal Server Error",
			err.Error(), "INTERNAL_ERROR", r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "pruned",
		"node":   node,
	})
}

// MutateImageMirror handles POST /api/v1/runtime/images/mutate-mirror.
func (h *RuntimeGuardHandler) MutateImageMirror(w http.ResponseWriter, r *http.Request) {
	var req MutateImageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeProblem(w, http.StatusBadRequest, "Bad Request",
			"Invalid JSON request payload", "VALIDATION_ERROR", r.URL.Path)
		return
	}

	valid, valErr := h.manager.ValidateImageManifest(req.Image)
	if !valid || valErr != nil {
		h.writeProblem(w, http.StatusUnprocessableEntity, "Unprocessable Entity",
			"Image manifest verification failed or digest signature malformed",
			domain.CodeManifestValidationFailed, r.URL.Path)
		return
	}

	mutated, err := h.manager.MutateImageForMirror(req.Image)
	if err != nil {
		if errors.Is(err, domain.ErrRegistryRateLimitExceeded) {
			h.writeProblem(w, http.StatusTooManyRequests, "Too Many Requests",
				err.Error(), domain.CodeRegistryRateLimitExceeded, r.URL.Path)
			return
		}
		h.logger.Error("Failed to mutate image pull spec", zap.String("image", req.Image), zap.Error(err))
		h.writeProblem(w, http.StatusInternalServerError, "Internal Server Error",
			err.Error(), "INTERNAL_ERROR", r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"original_image": req.Image,
		"mutated_image":  mutated,
	})
}

// ConfigureMirror handles POST /api/v1/runtime/registries/configure-mirror.
func (h *RuntimeGuardHandler) ConfigureMirror(w http.ResponseWriter, r *http.Request) {
	var req ConfigureMirrorRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeProblem(w, http.StatusBadRequest, "Bad Request",
			"Invalid JSON request payload", "VALIDATION_ERROR", r.URL.Path)
		return
	}

	var timeout time.Duration
	if req.FallbackTimeoutMs > 0 {
		timeout = time.Duration(req.FallbackTimeoutMs) * time.Millisecond
	} else if req.FallbackTimeoutSeconds > 0 {
		timeout = time.Duration(req.FallbackTimeoutSeconds) * time.Second
	} else {
		timeout = domain.DefaultP2PFallbackTimeout
	}

	cfg := domain.RegistryMirrorConfig{
		PrimaryRegistry: req.PrimaryRegistry,
		MirrorURL:       req.MirrorURL,
		FallbackTimeout: timeout,
		RateLimitFloor:  req.RateLimitFloor,
	}

	if err := h.manager.ConfigureRegistryMirror(r.Context(), cfg); err != nil {
		h.writeProblem(w, http.StatusBadRequest, "Bad Request",
			err.Error(), "VALIDATION_ERROR", r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":           "configured",
		"primary_registry": req.PrimaryRegistry,
		"mirror_url":       req.MirrorURL,
	})
}

func (h *RuntimeGuardHandler) writeProblem(w http.ResponseWriter, status int, title, detail, code, instance string) {
	problem := ProblemDetails{
		Type:     "about:blank",
		Title:    title,
		Status:   status,
		Detail:   detail,
		Instance: instance,
		Code:     code,
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(problem)
}
