// Package http provides HTTP adapters and handlers for the k8sselfhost API.
package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	domainmesh "github.com/datdt/k8sselfhost/internal/domain/mesh"
	usecasemesh "github.com/datdt/k8sselfhost/internal/usecase/mesh"
)

// MeshHandler provides HTTP endpoints for service mesh sidecar lifecycle,
// Ingress ReDoS regex safety auditing, and graceful TLS drain orchestration.
type MeshHandler struct {
	manager domainmesh.MeshLifecycleManager
	logger  *zap.Logger
}

// NewMeshHandler creates a new MeshHandler instance.
func NewMeshHandler(manager domainmesh.MeshLifecycleManager, loggers ...*zap.Logger) *MeshHandler {
	var l *zap.Logger
	if len(loggers) > 0 && loggers[0] != nil {
		l = loggers[0]
	} else {
		l = zap.NewNop()
	}
	return &MeshHandler{
		manager: manager,
		logger:  l,
	}
}

// RegisterRoutes mounts mesh management endpoints onto the provided chi router.
func (h *MeshHandler) RegisterRoutes(r chi.Router) {
	r.Post("/sidecar/inject-guard", h.InjectSidecarGuard)
	r.Post("/waf/verify-regex", h.VerifyWAFRegex)
	r.Post("/tls/drain", h.ConfigureTLSDrain)
}

// InjectSidecarGuard handles POST /api/v1/mesh/sidecar/inject-guard.
// It accepts a Pod manifest or AdmissionReview and generates startup race prevention patches.
func (h *MeshHandler) InjectSidecarGuard(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.writeProblem(w, http.StatusBadRequest, "Bad Request", "Failed to read request body", "READ_ERROR", r.URL.Path)
		return
	}
	defer r.Body.Close()

	if len(strings.TrimSpace(string(body))) == 0 {
		h.writeProblem(w, http.StatusBadRequest, "Bad Request", "Request body cannot be empty", "VALIDATION_ERROR", r.URL.Path)
		return
	}

	ctx := r.Context()
	if v := r.URL.Query().Get("version"); strings.TrimSpace(v) != "" {
		ctx = usecasemesh.WithTargetK8sVersion(ctx, v)
	} else if hv := r.Header.Get("X-K8s-Version"); strings.TrimSpace(hv) != "" {
		ctx = usecasemesh.WithTargetK8sVersion(ctx, hv)
	}

	patchBytes, err := h.manager.InjectSidecarStartupGuard(ctx, body)
	if err != nil {
		h.logger.Warn("Failed to inject sidecar startup guard", zap.Error(err))
		h.writeProblem(w, http.StatusUnprocessableEntity, "Injection Failed", err.Error(), "ERR_SIDECAR_INJECTION_FAILED", r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(patchBytes)
}

type verifyRegexRequest struct {
	Pattern string `json:"pattern"`
}

// VerifyWAFRegex handles POST /api/v1/mesh/waf/verify-regex.
// It enforces RE2 DFA linear time constraints and rejects ReDoS-prone expressions under RFC 7807.
func (h *MeshHandler) VerifyWAFRegex(w http.ResponseWriter, r *http.Request) {
	var req verifyRegexRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeProblem(w, http.StatusBadRequest, "Bad Request", "Invalid JSON request body", "VALIDATION_ERROR", r.URL.Path)
		return
	}
	defer r.Body.Close()

	trimmed := strings.TrimSpace(req.Pattern)
	if trimmed == "" {
		h.writeProblem(w, http.StatusBadRequest, "Bad Request", "Regular expression pattern cannot be empty", "VALIDATION_ERROR", r.URL.Path)
		return
	}

	report, err := h.manager.VerifyWAFRegexSafety(trimmed)
	if errors.Is(err, domainmesh.ErrWAFNonLinearRegex) || (report != nil && report.Verdict == "REJECTED") {
		h.logger.Warn("WAF regex rejected due to ReDoS risk", zap.String("pattern", trimmed), zap.Any("report", report))
		h.writeProblem(w, http.StatusUnprocessableEntity, "WAF Rule Rejected",
			"WAF rule rejected: Regular expression contains non-linear backtracking constructs vulnerable to ReDoS.",
			domainmesh.CodeWAFNonLinearRegex, r.URL.Path)
		return
	}
	if err != nil {
		h.writeProblem(w, http.StatusBadRequest, "Verification Error", err.Error(), "ERR_WAF_VERIFICATION_FAILED", r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(report)
}

type tlsDrainRequest struct {
	Namespace           string `json:"namespace"`
	Deployment          string `json:"deployment"`
	DrainDuration       string `json:"drain_duration,omitempty"`
	DrainDurationSec    int    `json:"drain_duration_seconds,omitempty"`
}

// ConfigureTLSDrain handles POST /api/v1/mesh/tls/drain.
// It coordinates Envoy graceful connection drainage during pod termination.
func (h *MeshHandler) ConfigureTLSDrain(w http.ResponseWriter, r *http.Request) {
	var req tlsDrainRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeProblem(w, http.StatusBadRequest, "Bad Request", "Invalid JSON request body", "VALIDATION_ERROR", r.URL.Path)
		return
	}
	defer r.Body.Close()

	if strings.TrimSpace(req.Namespace) == "" || strings.TrimSpace(req.Deployment) == "" {
		h.writeProblem(w, http.StatusBadRequest, "Bad Request", "Fields 'namespace' and 'deployment' are required", "VALIDATION_ERROR", r.URL.Path)
		return
	}

	duration := 30 * time.Second
	if req.DrainDurationSec > 0 {
		duration = time.Duration(req.DrainDurationSec) * time.Second
	} else if strings.TrimSpace(req.DrainDuration) != "" {
		parsed, err := time.ParseDuration(req.DrainDuration)
		if err != nil {
			h.writeProblem(w, http.StatusBadRequest, "Bad Request", fmt.Sprintf("Invalid drain_duration: %v", err), "VALIDATION_ERROR", r.URL.Path)
			return
		}
		duration = parsed
	}

	if err := h.manager.ConfigureGracefulTLSDrain(r.Context(), req.Namespace, req.Deployment, duration); err != nil {
		if errors.Is(err, domainmesh.ErrMeshProxyStartupTimeout) {
			h.writeProblem(w, http.StatusGatewayTimeout, "Gateway Timeout",
				"Proxy sidecar failed to reach ready state within 60s.",
				domainmesh.CodeMeshProxyStartupTimeout, r.URL.Path)
			return
		}
		h.logger.Error("Failed to configure graceful TLS drain", zap.Error(err))
		h.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", err.Error(), "ERR_TLS_DRAIN_FAILED", r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":         "configured",
		"namespace":      req.Namespace,
		"deployment":     req.Deployment,
		"drain_duration": duration.String(),
	})
}

func (h *MeshHandler) writeProblem(w http.ResponseWriter, status int, title, detail, code, instance string) {
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
