// Package http provides HTTP adapters and REST endpoints for k8sselfhost.
package http

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	domain "github.com/datdt/k8sselfhost/internal/domain/workloadguard"
)

// ResolveDigestRequest represents the payload for image digest resolution.
type ResolveDigestRequest struct {
	Image string `json:"image"`
}

// WorkloadGuardHandler exposes REST API endpoints for container image digest enforcement,
// ephemeral storage boundary auditing, and pod PID exhaustion containment.
type WorkloadGuardHandler struct {
	guard  domain.WorkloadArtifactGuard
	logger *zap.Logger
}

// NewWorkloadGuardHandler instantiates a new WorkloadGuardHandler.
func NewWorkloadGuardHandler(guard domain.WorkloadArtifactGuard, loggers ...*zap.Logger) *WorkloadGuardHandler {
	var l *zap.Logger
	if len(loggers) > 0 && loggers[0] != nil {
		l = loggers[0]
	} else {
		l = zap.NewNop()
	}
	return &WorkloadGuardHandler{
		guard:  guard,
		logger: l,
	}
}

// RegisterRoutes registers workload guard endpoints on a Chi router.
func (h *WorkloadGuardHandler) RegisterRoutes(r chi.Router) {
	// Scoped routes
	r.Post("/images/resolve-digest", h.ResolveImageDigest)
	r.Post("/storage/validate-ephemeral", h.ValidateEphemeralStorage)
	r.Post("/pids/enforce-limits", h.EnforcePIDLimits)

	// Top-level API routes
	r.Post("/api/v1/workload/images/resolve-digest", h.ResolveImageDigest)
	r.Post("/api/v1/workload/storage/validate-ephemeral", h.ValidateEphemeralStorage)
	r.Post("/api/v1/workload/pids/enforce-limits", h.EnforcePIDLimits)
}

// ResolveImageDigest handles POST /api/v1/workload/images/resolve-digest.
func (h *WorkloadGuardHandler) ResolveImageDigest(w http.ResponseWriter, r *http.Request) {
	var req ResolveDigestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeProblem(w, http.StatusBadRequest, "Bad Request",
			"Invalid JSON request payload", "VALIDATION_ERROR", r.URL.Path)
		return
	}

	if strings.TrimSpace(req.Image) == "" {
		h.writeProblem(w, http.StatusBadRequest, "Bad Request",
			"image parameter is required", "VALIDATION_ERROR", r.URL.Path)
		return
	}

	res, err := h.guard.ResolveImageToDigest(r.Context(), req.Image)
	if err != nil {
		if errors.Is(err, domain.ErrMutableImageTagRejected) {
			h.writeProblem(w, http.StatusUnprocessableEntity, "Unprocessable Entity",
				err.Error(), domain.CodeMutableImageTagRejected, r.URL.Path)
			return
		}
		h.logger.Error("Failed to resolve image digest", zap.String("image", req.Image), zap.Error(err))
		h.writeProblem(w, http.StatusInternalServerError, "Internal Server Error",
			err.Error(), "INTERNAL_ERROR", r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(res)
}

// ValidateEphemeralStorage handles POST /api/v1/workload/storage/validate-ephemeral.
func (h *WorkloadGuardHandler) ValidateEphemeralStorage(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil || len(bytesTrim(body)) == 0 {
		h.writeProblem(w, http.StatusBadRequest, "Bad Request",
			"Pod specification payload is required", "VALIDATION_ERROR", r.URL.Path)
		return
	}

	check, err := h.guard.ValidateEphemeralStoragePolicy(r.Context(), body)
	if err != nil {
		h.writeProblem(w, http.StatusBadRequest, "Bad Request",
			err.Error(), "VALIDATION_ERROR", r.URL.Path)
		return
	}

	if !check.IsPassed {
		h.writeProblem(w, http.StatusBadRequest, "Bad Request",
			check.RejectReason, domain.CodeEphemeralStorageLimitMissing, r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(check)
}

// EnforcePIDLimits handles POST /api/v1/workload/pids/enforce-limits.
func (h *WorkloadGuardHandler) EnforcePIDLimits(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil || len(bytesTrim(body)) == 0 {
		h.writeProblem(w, http.StatusBadRequest, "Bad Request",
			"Pod specification payload is required", "VALIDATION_ERROR", r.URL.Path)
		return
	}

	var maxPIDs int64
	if q := r.URL.Query().Get("default_max_pids"); q != "" {
		if val, parseErr := strconv.ParseInt(q, 10, 64); parseErr == nil {
			maxPIDs = val
		}
	} else if q := r.URL.Query().Get("max_pids"); q != "" {
		if val, parseErr := strconv.ParseInt(q, 10, 64); parseErr == nil {
			maxPIDs = val
		}
	}

	// Check if body is an envelope with pod_spec and default_max_pids
	var envelope struct {
		PodSpec        json.RawMessage `json:"pod_spec"`
		DefaultMaxPIDs int64           `json:"default_max_pids"`
	}
	podBytes := body
	if err := json.Unmarshal(body, &envelope); err == nil && len(envelope.PodSpec) > 0 {
		podBytes = envelope.PodSpec
		if envelope.DefaultMaxPIDs > 0 && maxPIDs == 0 {
			maxPIDs = envelope.DefaultMaxPIDs
		}
	}

	if maxPIDs <= 0 {
		maxPIDs = domain.DefaultPodPIDLimit
	}

	mutatedBytes, err := h.guard.EnforcePodPIDLimit(r.Context(), podBytes, maxPIDs)
	if err != nil {
		if errors.Is(err, domain.ErrPIDExhaustionRisk) {
			h.writeProblem(w, http.StatusTooManyRequests, "Too Many Requests",
				err.Error(), domain.CodePIDExhaustionRisk, r.URL.Path)
			return
		}
		h.writeProblem(w, http.StatusBadRequest, "Bad Request",
			err.Error(), "VALIDATION_ERROR", r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":         "enforced",
		"pod_pids_limit": maxPIDs,
		"pod_spec":       json.RawMessage(mutatedBytes),
	})
}

func (h *WorkloadGuardHandler) writeProblem(w http.ResponseWriter, status int, title, detail, code, instance string) {
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

func bytesTrim(b []byte) []byte {
	return []byte(strings.TrimSpace(string(b)))
}
