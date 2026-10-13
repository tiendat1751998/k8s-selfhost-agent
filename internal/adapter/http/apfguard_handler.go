package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/domain/apfguard"
)

// APFGuardHandler exposes REST API endpoints for Kubernetes API Priority and Fairness (APF)
// queue optimization, watch cache 410 Gone thundering herd mitigation, and HPA fallback scaling.
type APFGuardHandler struct {
	engine apfguard.K8sControlPlaneResilienceEngine
	logger *zap.Logger
}

// NewAPFGuardHandler constructs a new APFGuardHandler instance.
func NewAPFGuardHandler(engine apfguard.K8sControlPlaneResilienceEngine, logger *zap.Logger) *APFGuardHandler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &APFGuardHandler{
		engine: engine,
		logger: logger,
	}
}

// RegisterRoutes mounts the APF guard endpoints onto the given chi.Router.
func (h *APFGuardHandler) RegisterRoutes(r chi.Router) {
	r.Get("/apf/queue-status", h.AuditQueueStatus)
	r.Post("/watch-cache/mitigate-thundering-herd", h.MitigateThunderingHerd)
	r.Post("/hpa/check-health-fallback", h.CheckHPAFallback)
}

// WireToRouter mounts the APF guard endpoints under /api/v1/k8s.
func (h *APFGuardHandler) WireToRouter(r chi.Router) {
	r.Route("/api/v1/k8s", func(sub chi.Router) {
		h.RegisterRoutes(sub)
	})
}

// AuditQueueStatus handles GET /api/v1/k8s/apf/queue-status.
// Audits APF priority level queue depths, drops, and starvation states.
func (h *APFGuardHandler) AuditQueueStatus(w http.ResponseWriter, r *http.Request) {
	if h.engine == nil {
		h.writeProblem(w, http.StatusServiceUnavailable, "Service Unavailable",
			"Control plane resilience engine is not configured", "ERR_ENGINE_UNAVAILABLE", r.URL.Path)
		return
	}

	statuses, err := h.engine.AuditAPFQueueStatus(r.Context())
	if err != nil {
		h.logger.Error("Failed to audit APF queue statuses", zap.Error(err))
		if errors.Is(err, apfguard.ErrAPFQueueSaturated) {
			h.writeProblem(w, http.StatusServiceUnavailable, "APF Queue Saturated",
				err.Error(), "ERR_APF_QUEUE_SATURATED", r.URL.Path)
			return
		}
		if errors.Is(err, apfguard.ErrLeaderElectionStarved) {
			h.writeProblem(w, http.StatusServiceUnavailable, "Leader Election Starved",
				err.Error(), "ERR_LEADER_ELECTION_STARVED", r.URL.Path)
			return
		}
		h.writeProblem(w, http.StatusInternalServerError, "Internal Server Error",
			err.Error(), "ERR_AUDIT_FAILED", r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(statuses)
}

// MitigateThunderingHerdRequest represents the incoming JSON payload for watch cache mitigation.
type MitigateThunderingHerdRequest struct {
	MaxJitterWindow string `json:"max_jitter_window"`
	BatchSize       int    `json:"batch_size"`
	CompactionAge   string `json:"compaction_age"`
}

// MitigateThunderingHerd handles POST /api/v1/k8s/watch-cache/mitigate-thundering-herd.
// Applies randomized jitter and chunked paginated relists to recover from 410 Gone compaction events.
func (h *APFGuardHandler) MitigateThunderingHerd(w http.ResponseWriter, r *http.Request) {
	if h.engine == nil {
		h.writeProblem(w, http.StatusServiceUnavailable, "Service Unavailable",
			"Control plane resilience engine is not configured", "ERR_ENGINE_UNAVAILABLE", r.URL.Path)
		return
	}

	policy := apfguard.DefaultWatchCacheCompactionPolicy()

	if r.Body != nil && r.ContentLength > 0 {
		var req MitigateThunderingHerdRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err == nil {
			if req.BatchSize > 0 {
				policy.BatchSize = req.BatchSize
			}
			if req.MaxJitterWindow != "" {
				if d, parseErr := time.ParseDuration(req.MaxJitterWindow); parseErr == nil && d > 0 {
					policy.MaxJitterWindow = d
				}
			}
			if req.CompactionAge != "" {
				if d, parseErr := time.ParseDuration(req.CompactionAge); parseErr == nil && d > 0 {
					policy.CompactionAge = d
				}
			}
		}
	}

	if err := h.engine.MitigateWatchCacheCompactionThunderingHerd(r.Context(), policy); err != nil {
		h.logger.Error("Failed to mitigate watch cache compaction", zap.Error(err))
		if errors.Is(err, apfguard.ErrWatchCacheCompacted) {
			h.writeProblem(w, http.StatusGone, "Watch Cache Compacted",
				err.Error(), "ERR_WATCH_CACHE_COMPACTED", r.URL.Path)
			return
		}
		h.writeProblem(w, http.StatusInternalServerError, "Internal Server Error",
			err.Error(), "ERR_MITIGATION_FAILED", r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":            "mitigated",
		"batch_size":        policy.BatchSize,
		"max_jitter_window": policy.MaxJitterWindow.String(),
		"message":           "Exponential jitter applied and chunked paginated relisting completed",
	})
}

// CheckHPAFallbackRequest represents the incoming JSON payload for HPA fallback evaluation.
type CheckHPAFallbackRequest struct {
	Namespace  string `json:"namespace"`
	Deployment string `json:"deployment"`
}

// CheckHPAFallback handles POST /api/v1/k8s/hpa/check-health-fallback.
// Probes metrics-server and applies fallback replica scaling if unavailable >2m.
func (h *APFGuardHandler) CheckHPAFallback(w http.ResponseWriter, r *http.Request) {
	if h.engine == nil {
		h.writeProblem(w, http.StatusServiceUnavailable, "Service Unavailable",
			"Control plane resilience engine is not configured", "ERR_ENGINE_UNAVAILABLE", r.URL.Path)
		return
	}

	var req CheckHPAFallbackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeProblem(w, http.StatusBadRequest, "Bad Request",
			"Malformed request body", "ERR_INVALID_BODY", r.URL.Path)
		return
	}

	req.Namespace = strings.TrimSpace(req.Namespace)
	req.Deployment = strings.TrimSpace(req.Deployment)
	if req.Namespace == "" || req.Deployment == "" {
		h.writeProblem(w, http.StatusBadRequest, "Bad Request",
			"namespace and deployment parameters are required", "ERR_INVALID_PARAM", r.URL.Path)
		return
	}

	report, err := h.engine.CheckHPAHealthAndApplyFallback(r.Context(), req.Namespace, req.Deployment)
	if err != nil {
		h.logger.Error("Failed to check HPA health and fallback",
			zap.String("namespace", req.Namespace),
			zap.String("deployment", req.Deployment),
			zap.Error(err),
		)
		if errors.Is(err, apfguard.ErrMetricsServerUnavailable) {
			h.writeProblem(w, http.StatusServiceUnavailable, "Metrics Server Unavailable",
				err.Error(), "ERR_METRICS_SERVER_UNAVAILABLE", r.URL.Path)
			return
		}
		h.writeProblem(w, http.StatusInternalServerError, "Internal Server Error",
			err.Error(), "ERR_HPA_FALLBACK_FAILED", r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(report)
}

// writeProblem writes an RFC 7807 problem details response with application/problem+json.
func (h *APFGuardHandler) writeProblem(w http.ResponseWriter, status int, title, detail, code, instance string) {
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
