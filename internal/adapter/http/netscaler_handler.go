// Package http provides REST HTTP adapters and handlers for the k8sselfhost API.
package http

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	domain "github.com/datdt/k8sselfhost/internal/domain/netscaler"
)

// NetscalerHandler exposes HTTP REST endpoints for Linux kernel socket telemetry,
// TCP TIME_WAIT port starvation mitigation, conntrack auto-expansion, and health audits.
type NetscalerHandler struct {
	watchdog domain.SocketWatchdog
	logger   *zap.Logger
}

// NewNetscalerHandler constructs a NetscalerHandler instance.
func NewNetscalerHandler(watchdog domain.SocketWatchdog, loggers ...*zap.Logger) *NetscalerHandler {
	var l *zap.Logger
	if len(loggers) > 0 && loggers[0] != nil {
		l = loggers[0]
	} else {
		l = zap.NewNop()
	}
	return &NetscalerHandler{
		watchdog: watchdog,
		logger:   l,
	}
}

// RegisterRoutes mounts netscaler endpoints on the provided chi router.
// It supports both base mounting (under /api/v1) and sub-route mounting (under /api/v1/kernel/network).
func (h *NetscalerHandler) RegisterRoutes(r chi.Router) {
	// Full path mounting when router prefix is /api/v1
	r.Get("/kernel/network/socket-metrics", h.GetSocketMetrics)
	r.Post("/kernel/network/apply-tuning", h.ApplyTuning)
	r.Get("/kernel/network/health", h.CheckHealth)

	// Subroute mounting when router prefix is /api/v1/kernel/network
	r.Get("/socket-metrics", h.GetSocketMetrics)
	r.Post("/apply-tuning", h.ApplyTuning)
	r.Get("/health", h.CheckHealth)
}

// GetSocketMetrics handles GET /api/v1/kernel/network/socket-metrics.
func (h *NetscalerHandler) GetSocketMetrics(w http.ResponseWriter, r *http.Request) {
	metrics, err := h.watchdog.CollectMetrics(r.Context())
	if err != nil {
		h.logger.Error("Failed to collect kernel socket metrics", zap.Error(err))
		h.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", err.Error(), "INTERNAL_ERROR", r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(metrics)
}

// ApplyTuning handles POST /api/v1/kernel/network/apply-tuning.
func (h *NetscalerHandler) ApplyTuning(w http.ResponseWriter, r *http.Request) {
	var cfg domain.KernelNetConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		h.writeProblem(w, http.StatusBadRequest, "Bad Request", "Malformed JSON request payload", "MALFORMED_JSON", r.URL.Path)
		return
	}

	if err := cfg.Validate(); err != nil {
		h.writeProblem(w, http.StatusBadRequest, "Bad Request", err.Error(), domain.CodeInvalidNetConfig, r.URL.Path)
		return
	}

	if err := h.watchdog.ApplyKernelTuning(r.Context(), cfg); err != nil {
		if errors.Is(err, domain.ErrKernelSysctlFailed) {
			h.logger.Error("Failed to apply kernel sysctl tuning", zap.Error(err))
			h.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", err.Error(), domain.CodeKernelSysctlFailed, r.URL.Path)
			return
		}
		if errors.Is(err, domain.ErrInvalidNetConfig) {
			h.writeProblem(w, http.StatusBadRequest, "Bad Request", err.Error(), domain.CodeInvalidNetConfig, r.URL.Path)
			return
		}

		h.logger.Error("Unexpected error applying kernel tuning", zap.Error(err))
		h.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", err.Error(), "INTERNAL_ERROR", r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "applied",
		"message": "Kernel network configuration applied successfully with rollback snapshot",
		"config":  cfg,
	})
}

// CheckHealth handles GET /api/v1/kernel/network/health.
func (h *NetscalerHandler) CheckHealth(w http.ResponseWriter, r *http.Request) {
	if err := h.watchdog.CheckHealth(r.Context()); err != nil {
		if errors.Is(err, domain.ErrNetEphemeralPortsExhausted) {
			h.writeProblem(w, http.StatusServiceUnavailable, "Service Unavailable",
				err.Error(), domain.CodeNetEphemeralPortsExhausted, r.URL.Path)
			return
		}

		if errors.Is(err, domain.ErrNetConntrackSaturation) {
			h.writeProblem(w, http.StatusInternalServerError, "Internal Server Error",
				err.Error(), domain.CodeNetConntrackSaturation, r.URL.Path)
			return
		}

		h.logger.Error("Socket watchdog health check encountered error", zap.Error(err))
		h.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", err.Error(), "INTERNAL_ERROR", r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "healthy",
		"message": "Socket backlog and conntrack table within operational thresholds",
	})
}

func (h *NetscalerHandler) writeProblem(w http.ResponseWriter, status int, title, detail, code, instance string) {
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
