package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/domain/k8sresilience"
)

// K8sResilienceHandler exposes REST endpoints for Kubernetes control plane resilience,
// including admission webhook deadlock breaking, terminating namespace finalizer reaping,
// and Kubelet PLEG watchdogging.
type K8sResilienceHandler struct {
	watchdog k8sresilience.ControlPlaneWatchdog
	logger   *zap.Logger
}

// NewK8sResilienceHandler constructs a new K8sResilienceHandler instance.
func NewK8sResilienceHandler(watchdog k8sresilience.ControlPlaneWatchdog, logger *zap.Logger) *K8sResilienceHandler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &K8sResilienceHandler{
		watchdog: watchdog,
		logger:   logger,
	}
}

// RegisterRoutes mounts the resilience routes onto the given chi.Router.
func (h *K8sResilienceHandler) RegisterRoutes(r chi.Router) {
	r.Get("/webhooks/audit", h.AuditWebhooks)
	r.Post("/webhooks/{name}/bypass", h.BypassWebhook)
	r.Post("/namespaces/{ns}/reap-finalizers", h.ReapFinalizers)
	r.Get("/nodes/{node}/pleg-health", h.GetPLEGHealth)
	r.Post("/nodes/{node}/restart-containerd", h.RestartContainerd)
}

// WireToRouter mounts the resilience endpoints under /api/v1/k8s.
func (h *K8sResilienceHandler) WireToRouter(r chi.Router) {
	r.Route("/api/v1/k8s", func(sub chi.Router) {
		h.RegisterRoutes(sub)
	})
}

// AuditWebhooks handles GET /api/v1/k8s/webhooks/audit.
// Audits all admission webhooks for cold-start deadlocks and health state.
func (h *K8sResilienceHandler) AuditWebhooks(w http.ResponseWriter, r *http.Request) {
	if h.watchdog == nil {
		h.writeProblem(w, http.StatusServiceUnavailable, "Service Unavailable",
			"Control plane watchdog is not configured", "ERR_WATCHDOG_UNAVAILABLE", r.URL.Path)
		return
	}

	statuses, err := h.watchdog.AuditWebhooks(r.Context())
	if err != nil {
		h.logger.Error("Failed to audit admission webhooks", zap.Error(err))
		if errors.Is(err, k8sresilience.ErrK8sWebhookDeadlock) {
			h.writeProblem(w, http.StatusInternalServerError, "Admission Webhook Deadlock",
				err.Error(), "ERR_K8S_WEBHOOK_DEADLOCK", r.URL.Path)
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

// BypassWebhook handles POST /api/v1/k8s/webhooks/{name}/bypass.
// Emergency bypass patches failurePolicy to Ignore to break chicken-and-egg scheduling deadlocks.
func (h *K8sResilienceHandler) BypassWebhook(w http.ResponseWriter, r *http.Request) {
	if h.watchdog == nil {
		h.writeProblem(w, http.StatusServiceUnavailable, "Service Unavailable",
			"Control plane watchdog is not configured", "ERR_WATCHDOG_UNAVAILABLE", r.URL.Path)
		return
	}

	name := strings.TrimSpace(chi.URLParam(r, "name"))
	if name == "" {
		h.writeProblem(w, http.StatusBadRequest, "Bad Request",
			"Webhook name parameter is required", "ERR_INVALID_PARAM", r.URL.Path)
		return
	}

	if err := h.watchdog.EmergencyBypassDeadlockedWebhook(r.Context(), name); err != nil {
		h.logger.Error("Failed to emergency bypass webhook", zap.String("webhook", name), zap.Error(err))
		if errors.Is(err, k8sresilience.ErrWebhookNotFound) {
			h.writeProblem(w, http.StatusNotFound, "Webhook Not Found",
				err.Error(), "ERR_WEBHOOK_NOT_FOUND", r.URL.Path)
			return
		}
		if errors.Is(err, k8sresilience.ErrK8sWebhookDeadlock) {
			h.writeProblem(w, http.StatusInternalServerError, "Admission Webhook Deadlock",
				err.Error(), "ERR_K8S_WEBHOOK_DEADLOCK", r.URL.Path)
			return
		}
		if strings.Contains(err.Error(), "healthy") || strings.Contains(err.Error(), "rejected") {
			h.writeProblem(w, http.StatusBadRequest, "Bypass Rejected",
				err.Error(), "ERR_GUARDRAIL_VIOLATION", r.URL.Path)
			return
		}
		h.writeProblem(w, http.StatusInternalServerError, "Internal Server Error",
			err.Error(), "ERR_BYPASS_FAILED", r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "bypassed",
		"webhook": name,
		"message": "Webhook failurePolicy successfully patched to Ignore to break scheduling deadlock",
	})
}

// ReapFinalizers handles POST /api/v1/k8s/namespaces/{ns}/reap-finalizers.
// Removes blocking orphaned CRD finalizers from namespaces stuck in Terminating > 10m.
func (h *K8sResilienceHandler) ReapFinalizers(w http.ResponseWriter, r *http.Request) {
	if h.watchdog == nil {
		h.writeProblem(w, http.StatusServiceUnavailable, "Service Unavailable",
			"Control plane watchdog is not configured", "ERR_WATCHDOG_UNAVAILABLE", r.URL.Path)
		return
	}

	ns := strings.TrimSpace(chi.URLParam(r, "ns"))
	if ns == "" {
		h.writeProblem(w, http.StatusBadRequest, "Bad Request",
			"Namespace parameter is required", "ERR_INVALID_PARAM", r.URL.Path)
		return
	}

	if err := h.watchdog.ReapOrphanedFinalizers(r.Context(), ns); err != nil {
		h.logger.Error("Failed to reap namespace finalizers", zap.String("namespace", ns), zap.Error(err))
		if errors.Is(err, k8sresilience.ErrNamespaceNotStuck) {
			h.writeProblem(w, http.StatusConflict, "Namespace Not Stuck",
				err.Error(), "ERR_NAMESPACE_NOT_STUCK", r.URL.Path)
			return
		}
		if errors.Is(err, k8sresilience.ErrFinalizerReapFailed) {
			h.writeProblem(w, http.StatusInternalServerError, "Finalizer Reap Failed",
				err.Error(), "ERR_FINALIZER_REAP_FAILED", r.URL.Path)
			return
		}
		h.writeProblem(w, http.StatusInternalServerError, "Internal Server Error",
			err.Error(), "ERR_REAP_FAILED", r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "reaped",
		"namespace": ns,
		"message":   "Orphaned CRD finalizers successfully reaped from terminating namespace",
	})
}

// GetPLEGHealth handles GET /api/v1/k8s/nodes/{node}/pleg-health.
// Diagnoses Kubelet PLEG relist cycle and containerd responsiveness.
func (h *K8sResilienceHandler) GetPLEGHealth(w http.ResponseWriter, r *http.Request) {
	if h.watchdog == nil {
		h.writeProblem(w, http.StatusServiceUnavailable, "Service Unavailable",
			"Control plane watchdog is not configured", "ERR_WATCHDOG_UNAVAILABLE", r.URL.Path)
		return
	}

	node := strings.TrimSpace(chi.URLParam(r, "node"))
	if node == "" {
		h.writeProblem(w, http.StatusBadRequest, "Bad Request",
			"Node parameter is required", "ERR_INVALID_PARAM", r.URL.Path)
		return
	}

	status, err := h.watchdog.CheckPLEGHealth(r.Context(), node)
	if err != nil {
		h.logger.Error("Failed to check node PLEG health", zap.String("node", node), zap.Error(err))
		if errors.Is(err, k8sresilience.ErrK8sPLEGTimeout) {
			h.writeProblem(w, http.StatusGatewayTimeout, "Kubelet PLEG Timeout",
				err.Error(), "ERR_K8S_PLEG_TIMEOUT", r.URL.Path)
			return
		}
		if errors.Is(err, k8sresilience.ErrNodeNotFound) {
			h.writeProblem(w, http.StatusNotFound, "Node Not Found",
				err.Error(), "ERR_NODE_NOT_FOUND", r.URL.Path)
			return
		}
		h.writeProblem(w, http.StatusInternalServerError, "Internal Server Error",
			err.Error(), "ERR_PLEG_CHECK_FAILED", r.URL.Path)
		return
	}

	// If query param fail_on_unhealthy is set and node is unhealthy with timeout
	if r.URL.Query().Get("fail_on_timeout") == "true" && (!status.IsHealthy && status.RelistDuration > 180*time.Second) {
		h.writeProblem(w, http.StatusGatewayTimeout, "Kubelet PLEG Timeout",
			"PLEG relist duration exceeded 180s threshold", "ERR_K8S_PLEG_TIMEOUT", r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(status)
}

// RestartContainerd handles POST /api/v1/k8s/nodes/{node}/restart-containerd.
// Recycles stalled containerd runtime daemon before Kubelet eviction storm occurs.
func (h *K8sResilienceHandler) RestartContainerd(w http.ResponseWriter, r *http.Request) {
	if h.watchdog == nil {
		h.writeProblem(w, http.StatusServiceUnavailable, "Service Unavailable",
			"Control plane watchdog is not configured", "ERR_WATCHDOG_UNAVAILABLE", r.URL.Path)
		return
	}

	node := strings.TrimSpace(chi.URLParam(r, "node"))
	if node == "" {
		h.writeProblem(w, http.StatusBadRequest, "Bad Request",
			"Node parameter is required", "ERR_INVALID_PARAM", r.URL.Path)
		return
	}

	if err := h.watchdog.RestartStalledContainerd(r.Context(), node); err != nil {
		h.logger.Error("Failed to restart containerd", zap.String("node", node), zap.Error(err))
		if errors.Is(err, k8sresilience.ErrNodeNotFound) {
			h.writeProblem(w, http.StatusNotFound, "Node Not Found",
				err.Error(), "ERR_NODE_NOT_FOUND", r.URL.Path)
			return
		}
		h.writeProblem(w, http.StatusInternalServerError, "Internal Server Error",
			err.Error(), "ERR_CONTAINERD_RESTART_FAILED", r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "restarted",
		"node":    node,
		"message": "Containerd daemon gracefully recycled; CRI socket responsive",
	})
}

// writeProblem writes an RFC 7807 problem details response with application/problem+json.
func (h *K8sResilienceHandler) writeProblem(w http.ResponseWriter, status int, title, detail, code, instance string) {
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
