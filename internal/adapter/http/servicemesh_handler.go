// Package http provides HTTP adapters and handlers for the k8sselfhost API.
package http

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	domain "github.com/datdt/k8sselfhost/internal/domain/servicemesh"
	usecase "github.com/datdt/k8sselfhost/internal/usecase/servicemesh"
)

// ServiceMeshHandler provides REST API endpoints for service mesh SDS socket watchdog health,
// Envoy egress memory boundary enforcement, and overlay PMTU black hole remediation.
type ServiceMeshHandler struct {
	engine domain.ServiceMeshResilienceEngine
	logger *zap.Logger
}

// NewServiceMeshHandler creates a new ServiceMeshHandler instance.
func NewServiceMeshHandler(engine domain.ServiceMeshResilienceEngine, loggers ...*zap.Logger) *ServiceMeshHandler {
	var l *zap.Logger
	if len(loggers) > 0 && loggers[0] != nil {
		l = loggers[0]
	} else {
		l = zap.NewNop()
	}
	return &ServiceMeshHandler{
		engine: engine,
		logger: l,
	}
}

// RegisterRoutes mounts service mesh resilience endpoints onto the provided chi router.
func (h *ServiceMeshHandler) RegisterRoutes(r chi.Router) {
	r.Get("/sds/socket-status", h.GetSDSSocketStatus)
	r.Post("/sds/recover", h.RecoverSDSSocket)
	r.Post("/egress/enforce-scope", h.EnforceEgressScope)
	r.Post("/tunnel/mtu-audit", h.AuditTunnelMTU)
}

// RegisterAPIRoutes mounts the endpoints with the full /api/v1/mesh prefix.
func (h *ServiceMeshHandler) RegisterAPIRoutes(r chi.Router) {
	r.Route("/api/v1/mesh", func(sub chi.Router) {
		h.RegisterRoutes(sub)
	})
}

// GetSDSSocketStatus handles GET /api/v1/mesh/sds/socket-status.
// Inspects SDS Unix domain socket responsiveness and TLS certificate lifetime.
func (h *ServiceMeshHandler) GetSDSSocketStatus(w http.ResponseWriter, r *http.Request) {
	socketPath := r.URL.Query().Get("socket_path")
	if strings.TrimSpace(socketPath) == "" {
		socketPath = "/var/run/secrets/workload-spiffe-uds/socket"
	}

	status, err := h.engine.AuditSDSSocket(r.Context(), socketPath)
	if errors.Is(err, domain.ErrSDSSocketUnresponsive) {
		h.logger.Warn("SDS socket unresponsive during health audit", zap.String("socket_path", socketPath))
		h.writeProblem(w, http.StatusInternalServerError, "SDS Socket Unresponsive",
			err.Error(), domain.CodeSDSSocketUnresponsive, r.URL.Path)
		return
	}
	if err != nil {
		h.logger.Error("Failed to audit SDS socket", zap.Error(err))
		h.writeProblem(w, http.StatusInternalServerError, "SDS Audit Failed",
			err.Error(), "ERR_SDS_AUDIT_FAILED", r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(status)
}

type recoverSDSRequest struct {
	SocketPath string `json:"socket_path"`
}

// RecoverSDSSocket handles POST /api/v1/mesh/sds/recover.
// Dispatches graceful recovery signals to istio-agent.
func (h *ServiceMeshHandler) RecoverSDSSocket(w http.ResponseWriter, r *http.Request) {
	var req recoverSDSRequest
	body, err := io.ReadAll(r.Body)
	if err != nil {
		h.writeProblem(w, http.StatusBadRequest, "Bad Request", "Failed to read request body", "READ_ERROR", r.URL.Path)
		return
	}
	defer r.Body.Close()

	if len(strings.TrimSpace(string(body))) > 0 {
		if err := json.Unmarshal(body, &req); err != nil {
			h.writeProblem(w, http.StatusBadRequest, "Bad Request", "Invalid JSON payload", "VALIDATION_ERROR", r.URL.Path)
			return
		}
	}

	socketPath := strings.TrimSpace(req.SocketPath)
	if socketPath == "" {
		socketPath = "/var/run/secrets/workload-spiffe-uds/socket"
	}

	if err := h.engine.RecoverSDSSocket(r.Context(), socketPath); err != nil {
		h.logger.Error("Failed to recover SDS socket", zap.String("socket_path", socketPath), zap.Error(err))
		h.writeProblem(w, http.StatusInternalServerError, "Recovery Failed",
			err.Error(), "ERR_SDS_RECOVERY_FAILED", r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":      "recovered",
		"socket_path": socketPath,
	})
}

type enforceEgressRequest struct {
	Namespace           string   `json:"namespace"`
	Dependencies        []string `json:"dependencies"`
	FallbackToNamespace bool     `json:"fallback_to_namespace"`
}

// EnforceEgressScope handles POST /api/v1/mesh/egress/enforce-scope.
// Enforces Sidecar egress discovery scoping to drop Envoy memory footprint to < 100MB.
func (h *ServiceMeshHandler) EnforceEgressScope(w http.ResponseWriter, r *http.Request) {
	var req enforceEgressRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeProblem(w, http.StatusBadRequest, "Bad Request", "Invalid JSON payload", "VALIDATION_ERROR", r.URL.Path)
		return
	}
	defer r.Body.Close()

	namespace := strings.TrimSpace(req.Namespace)
	if namespace == "" {
		h.writeProblem(w, http.StatusBadRequest, "Bad Request", "Field 'namespace' is required", "VALIDATION_ERROR", r.URL.Path)
		return
	}

	ctx := r.Context()
	if req.FallbackToNamespace {
		ctx = usecase.WithTriggerFallback(ctx, true)
	}

	if err := h.engine.EnforceEgressScope(ctx, namespace, req.Dependencies); err != nil {
		if errors.Is(err, domain.ErrEnvoyEgressMemoryExhausted) {
			h.logger.Warn("Envoy egress memory exhausted", zap.String("namespace", namespace))
			h.writeProblem(w, http.StatusConflict, "Envoy Egress Memory Exhausted",
				err.Error(), domain.CodeEnvoyEgressMemoryExhausted, r.URL.Path)
			return
		}
		h.logger.Error("Failed to enforce egress scope", zap.String("namespace", namespace), zap.Error(err))
		h.writeProblem(w, http.StatusInternalServerError, "Enforcement Failed",
			err.Error(), "ERR_EGRESS_ENFORCEMENT_FAILED", r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":             "enforced",
		"namespace":          namespace,
		"dependencies_count": len(req.Dependencies),
	})
}

type auditTunnelRequest struct {
	TargetIP        string `json:"target_ip"`
	TestPayloadSize int    `json:"test_payload_size"`
	TunnelInterface string `json:"tunnel_interface,omitempty"`
}

// AuditTunnelMTU handles POST /api/v1/mesh/tunnel/mtu-audit.
// Audits inter-cluster overlay encapsulation and mitigates PMTU black holes via TCP MSS clamping.
func (h *ServiceMeshHandler) AuditTunnelMTU(w http.ResponseWriter, r *http.Request) {
	var req auditTunnelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeProblem(w, http.StatusBadRequest, "Bad Request", "Invalid JSON payload", "VALIDATION_ERROR", r.URL.Path)
		return
	}
	defer r.Body.Close()

	targetIP := strings.TrimSpace(req.TargetIP)
	if targetIP == "" {
		h.writeProblem(w, http.StatusBadRequest, "Bad Request", "Field 'target_ip' is required", "VALIDATION_ERROR", r.URL.Path)
		return
	}

	result, err := h.engine.AuditTunnelMTUBlackHole(r.Context(), targetIP, req.TestPayloadSize)
	if errors.Is(err, domain.ErrTunnelPMTUBlackHole) {
		h.logger.Warn("Overlay PMTU black hole detected", zap.String("target_ip", targetIP))
		h.writeProblem(w, http.StatusBadGateway, "Tunnel PMTU Black Hole",
			err.Error(), domain.CodeTunnelPMTUBlackHole, r.URL.Path)
		return
	}
	if err != nil {
		h.logger.Error("Failed to audit tunnel MTU", zap.String("target_ip", targetIP), zap.Error(err))
		h.writeProblem(w, http.StatusInternalServerError, "Tunnel Audit Failed",
			err.Error(), "ERR_TUNNEL_AUDIT_FAILED", r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(result)
}

func (h *ServiceMeshHandler) writeProblem(w http.ResponseWriter, status int, title, detail, code, instance string) {
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
