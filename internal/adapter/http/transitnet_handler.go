// Package http provides HTTP adapters and REST endpoints for the k8sselfhost API.
package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	domain "github.com/datdt/k8sselfhost/internal/domain/transitnet"
	usecase "github.com/datdt/k8sselfhost/internal/usecase/transitnet"
)

// TransitNetHandler exposes REST API endpoints for Hybrid Cloud Transit Gateway,
// Asymmetric BGP Routing detection, VPC peering MTU validation, NLB cross-zone balancing,
// and CoreDNS negative caching sanitization.
type TransitNetHandler struct {
	manager *usecase.RoutingManager
	logger  *zap.Logger
}

// NewTransitNetHandler constructs a TransitNetHandler instance.
func NewTransitNetHandler(manager *usecase.RoutingManager, loggers ...*zap.Logger) *TransitNetHandler {
	var l *zap.Logger
	if len(loggers) > 0 && loggers[0] != nil {
		l = loggers[0]
	} else {
		l = zap.NewNop()
	}
	return &TransitNetHandler{
		manager: manager,
		logger:  l,
	}
}

// RegisterRoutes mounts TransitNet endpoints on the provided chi router.
func (h *TransitNetHandler) RegisterRoutes(r chi.Router) {
	r.Post("/audit-symmetry", h.AuditRoutingSymmetry)
	r.Post("/validate-path-mtu", h.ValidateVPCPathMTU)
	r.Get("/nlb/cross-zone-balance", h.AuditNLBCrossZoneBalance)
	r.Post("/coredns/sanitize-negative-cache", h.SanitizeCoreDNSNegativeCache)
}

// MountAPIRoutes registers TransitNet endpoints under /api/v1/cloud/network.
func (h *TransitNetHandler) MountAPIRoutes(r chi.Router) {
	r.Route("/api/v1/cloud/network", h.RegisterRoutes)
}

// AuditSymmetryRequest represents payload for POST /api/v1/cloud/network/audit-symmetry.
type AuditSymmetryRequest struct {
	TargetEndpoint string `json:"target_endpoint"`
}

// AuditRoutingSymmetry handles POST /api/v1/cloud/network/audit-symmetry.
func (h *TransitNetHandler) AuditRoutingSymmetry(w http.ResponseWriter, r *http.Request) {
	var req AuditSymmetryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeProblem(w, http.StatusBadRequest, "Bad Request", "Malformed JSON request body", "MALFORMED_JSON", r.URL.Path)
		return
	}

	if strings.TrimSpace(req.TargetEndpoint) == "" {
		h.writeProblem(w, http.StatusBadRequest, "Bad Request", "target_endpoint is required", "VALIDATION_ERROR", r.URL.Path)
		return
	}

	integrity, err := h.manager.AuditRoutingSymmetry(r.Context(), req.TargetEndpoint)
	if err != nil {
		if errors.Is(err, domain.ErrBGPAsymmetricPath) {
			h.logger.Warn("Asymmetric BGP path detected",
				zap.String("target_endpoint", req.TargetEndpoint),
				zap.Error(err),
			)
			h.writeProblem(w, http.StatusBadGateway, "Bad Gateway", err.Error(), domain.CodeBGPAsymmetricPath, r.URL.Path)
			return
		}
		h.logger.Error("Failed to audit routing symmetry",
			zap.String("target_endpoint", req.TargetEndpoint),
			zap.Error(err),
		)
		h.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", err.Error(), "INTERNAL_ERROR", r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "symmetric",
		"integrity": integrity,
	})
}

// ValidateVPCPathMTURequest represents payload for POST /api/v1/cloud/network/validate-path-mtu.
type ValidateVPCPathMTURequest struct {
	TargetHost string `json:"target_host"`
	TestBytes  int    `json:"test_bytes"`
}

// ValidateVPCPathMTU handles POST /api/v1/cloud/network/validate-path-mtu.
func (h *TransitNetHandler) ValidateVPCPathMTU(w http.ResponseWriter, r *http.Request) {
	var req ValidateVPCPathMTURequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeProblem(w, http.StatusBadRequest, "Bad Request", "Malformed JSON request body", "MALFORMED_JSON", r.URL.Path)
		return
	}

	if strings.TrimSpace(req.TargetHost) == "" {
		h.writeProblem(w, http.StatusBadRequest, "Bad Request", "target_host is required", "VALIDATION_ERROR", r.URL.Path)
		return
	}

	if req.TestBytes <= 0 {
		req.TestBytes = 1500
	}

	validation, err := h.manager.ValidateVPCPathMTU(r.Context(), req.TargetHost, req.TestBytes)
	if err != nil {
		if errors.Is(err, domain.ErrVPCPathMTUBlackHole) {
			h.logger.Warn("VPC peering MTU black hole detected",
				zap.String("target_host", req.TargetHost),
				zap.Error(err),
			)
			h.writeProblem(w, http.StatusBadGateway, "Bad Gateway", err.Error(), domain.CodeVPCPathMTUBlackHole, r.URL.Path)
			return
		}
		h.logger.Error("Failed to validate VPC path MTU",
			zap.String("target_host", req.TargetHost),
			zap.Error(err),
		)
		h.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", err.Error(), "INTERNAL_ERROR", r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":     "valid",
		"validation": validation,
	})
}

// AuditNLBCrossZoneBalance handles GET /api/v1/cloud/network/nlb/cross-zone-balance.
func (h *TransitNetHandler) AuditNLBCrossZoneBalance(w http.ResponseWriter, r *http.Request) {
	lbARN := r.URL.Query().Get("lb_arn")
	if lbARN == "" {
		lbARN = r.URL.Query().Get("load_balancer_arn")
	}

	if strings.TrimSpace(lbARN) == "" {
		h.writeProblem(w, http.StatusBadRequest, "Bad Request", "lb_arn query parameter is required", "VALIDATION_ERROR", r.URL.Path)
		return
	}

	dist, err := h.manager.AuditNLBCrossZoneBalance(r.Context(), lbARN)
	if err != nil {
		if errors.Is(err, domain.ErrNLBAZImbalance) {
			h.logger.Warn("NLB AZ imbalance detected",
				zap.String("lb_arn", lbARN),
				zap.Error(err),
			)
			h.writeProblem(w, http.StatusConflict, "Conflict", err.Error(), domain.CodeNLBAZImbalance, r.URL.Path)
			return
		}
		h.logger.Error("Failed to audit NLB cross-zone distribution",
			zap.String("lb_arn", lbARN),
			zap.Error(err),
		)
		h.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", err.Error(), "INTERNAL_ERROR", r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":       "balanced",
		"distribution": dist,
	})
}

// SanitizeNegativeCacheRequest represents payload for POST /api/v1/cloud/network/coredns/sanitize-negative-cache.
type SanitizeNegativeCacheRequest struct {
	DomainZone             string `json:"domain_zone"`
	MaxNegativeTTLSeconds int    `json:"max_negative_ttl_seconds"`
}

// SanitizeCoreDNSNegativeCache handles POST /api/v1/cloud/network/coredns/sanitize-negative-cache.
func (h *TransitNetHandler) SanitizeCoreDNSNegativeCache(w http.ResponseWriter, r *http.Request) {
	var req SanitizeNegativeCacheRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeProblem(w, http.StatusBadRequest, "Bad Request", "Malformed JSON request body", "MALFORMED_JSON", r.URL.Path)
		return
	}

	if strings.TrimSpace(req.DomainZone) == "" {
		h.writeProblem(w, http.StatusBadRequest, "Bad Request", "domain_zone is required", "VALIDATION_ERROR", r.URL.Path)
		return
	}

	if req.MaxNegativeTTLSeconds <= 0 {
		req.MaxNegativeTTLSeconds = 5
	}

	if err := h.manager.SanitizeCoreDNSNegativeCache(r.Context(), req.DomainZone, req.MaxNegativeTTLSeconds); err != nil {
		h.logger.Error("Failed to sanitize CoreDNS negative caching",
			zap.String("domain_zone", req.DomainZone),
			zap.Error(err),
		)
		h.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", err.Error(), "INTERNAL_ERROR", r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":                   "sanitized",
		"domain_zone":             req.DomainZone,
		"max_negative_ttl_seconds": req.MaxNegativeTTLSeconds,
		"message":                  "CoreDNS negative caching TTL successfully bounded to 5s",
	})
}

func (h *TransitNetHandler) writeProblem(w http.ResponseWriter, status int, title, detail, code, instance string) {
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
