// Package http provides HTTP adapters and REST endpoints for the k8sselfhost API.
package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	domain "github.com/datdt/k8sselfhost/internal/domain/netguard"
	usecase "github.com/datdt/k8sselfhost/internal/usecase/netguard"
)

// NetGuardHandler exposes REST API endpoints for Hybrid WAN watchdog,
// Path MTU black hole detection, TCP MSS clamping, and rp_filter routing audits.
type NetGuardHandler struct {
	manager *usecase.TunnelManager
	logger  *zap.Logger
}

// NewNetGuardHandler constructs a NetGuardHandler instance.
func NewNetGuardHandler(manager *usecase.TunnelManager, loggers ...*zap.Logger) *NetGuardHandler {
	var l *zap.Logger
	if len(loggers) > 0 && loggers[0] != nil {
		l = loggers[0]
	} else {
		l = zap.NewNop()
	}
	return &NetGuardHandler{
		manager: manager,
		logger:  l,
	}
}

// RegisterRoutes mounts NetGuard endpoints on the provided chi router.
func (h *NetGuardHandler) RegisterRoutes(r chi.Router) {
	r.Get("/tunnels/health", h.GetTunnelHealth)
	r.Post("/tunnels/pmtu-probe", h.ProbePMTU)
	r.Post("/tunnels/{iface}/mss-clamp", h.ApplyMSSClamping)
	r.Get("/routing/rp-filter", h.AuditRPFilter)
}

// GetTunnelHealth handles GET /api/v1/network/tunnels/health.
func (h *NetGuardHandler) GetTunnelHealth(w http.ResponseWriter, r *http.Request) {
	tunnels, err := h.manager.CheckTransportHealth(r.Context())
	if err != nil {
		h.logger.Error("Failed to check tunnel transport health", zap.Error(err))
		h.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", err.Error(), "INTERNAL_ERROR", r.URL.Path)
		return
	}

	overallStatus := "healthy"
	var alerts []*usecase.DERPAlert

	for _, tun := range tunnels {
		if alert, derpErr := h.manager.EvaluateDERPFallback(tun); derpErr != nil {
			overallStatus = "degraded"
			if alert != nil {
				alerts = append(alerts, alert)
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  overallStatus,
		"tunnels": tunnels,
		"count":   len(tunnels),
		"alerts":  alerts,
	})
}

// PMTUProbeRequest represents the payload for POST /api/v1/network/tunnels/pmtu-probe.
type PMTUProbeRequest struct {
	TargetIP  string `json:"target_ip"`
	Interface string `json:"interface,omitempty"`
}

// ProbePMTU handles POST /api/v1/network/tunnels/pmtu-probe.
func (h *NetGuardHandler) ProbePMTU(w http.ResponseWriter, r *http.Request) {
	var req PMTUProbeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeProblem(w, http.StatusBadRequest, "Bad Request", "Malformed JSON request body", "MALFORMED_JSON", r.URL.Path)
		return
	}

	if strings.TrimSpace(req.TargetIP) == "" {
		h.writeProblem(w, http.StatusBadRequest, "Bad Request", "target_ip is required", "VALIDATION_ERROR", r.URL.Path)
		return
	}

	res, err := h.manager.SweepPMTU(r.Context(), req.TargetIP)
	if err != nil {
		if errors.Is(err, domain.ErrTunnelMTUBlackHole) {
			h.logger.Warn("MTU black hole detected on target", zap.String("target_ip", req.TargetIP), zap.Error(err))
			h.writeProblem(w, http.StatusBadGateway, "Bad Gateway", err.Error(), domain.CodeTunnelMTUBlackHole, r.URL.Path)
			return
		}
		h.logger.Error("Failed to probe PMTU", zap.String("target_ip", req.TargetIP), zap.Error(err))
		h.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", err.Error(), "INTERNAL_ERROR", r.URL.Path)
		return
	}

	mssV4, _ := h.manager.CalculateMSS(res.DiscoveredPMTU, false)
	mssV6, _ := h.manager.CalculateMSS(res.DiscoveredPMTU, true)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"target_ip":            res.TargetIP,
		"interface":            req.Interface,
		"detected_pmtu":        res.DiscoveredPMTU,
		"iterations":           res.Iterations,
		"probe_count":          res.ProbeCount,
		"blackhole_detected":   res.BlackHoleDetected,
		"recommended_mss_ipv4": mssV4,
		"recommended_mss_ipv6": mssV6,
		"duration_ms":          res.Duration.Milliseconds(),
	})
}

// MSSClampRequest represents the payload for POST /api/v1/network/tunnels/:iface/mss-clamp.
type MSSClampRequest struct {
	PMTU int  `json:"pmtu"`
	IPv6 bool `json:"ipv6"`
}

// ApplyMSSClamping handles POST /api/v1/network/tunnels/:iface/mss-clamp.
func (h *NetGuardHandler) ApplyMSSClamping(w http.ResponseWriter, r *http.Request) {
	iface := chi.URLParam(r, "iface")
	if iface == "" {
		iface = chi.URLParam(r, "interface")
	}
	if strings.TrimSpace(iface) == "" {
		h.writeProblem(w, http.StatusBadRequest, "Bad Request", "iface path parameter is required", "VALIDATION_ERROR", r.URL.Path)
		return
	}

	var req MSSClampRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeProblem(w, http.StatusBadRequest, "Bad Request", "Malformed JSON request body", "MALFORMED_JSON", r.URL.Path)
		return
	}

	if req.PMTU == 0 {
		req.PMTU = 1420 // Standard WAN tunnel default
	}

	targetMSS, err := h.manager.CalculateMSS(req.PMTU, req.IPv6)
	if err != nil {
		h.writeProblem(w, http.StatusBadRequest, "Bad Request", err.Error(), domain.CodeInvalidTunnelConfig, r.URL.Path)
		return
	}

	if err := h.manager.EnforceMSSClamping(r.Context(), iface, req.PMTU); err != nil {
		h.logger.Error("Failed to enforce MSS clamping", zap.String("iface", iface), zap.Error(err))
		h.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", err.Error(), domain.CodeMSSClampingFailed, r.URL.Path)
		return
	}

	rules := h.manager.GenerateIPTablesMSSRules(iface, targetMSS)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":     "applied",
		"interface":  iface,
		"pmtu":       req.PMTU,
		"target_mss": targetMSS,
		"rules":      rules,
	})
}

// AuditRPFilter handles GET /api/v1/network/routing/rp-filter.
func (h *NetGuardHandler) AuditRPFilter(w http.ResponseWriter, r *http.Request) {
	audit, err := h.manager.AuditRPFilterDetailed(r.Context())
	if err != nil {
		h.logger.Error("Failed to audit rp_filter", zap.Error(err))
		h.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", err.Error(), "INTERNAL_ERROR", r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(audit)
}

func (h *NetGuardHandler) writeProblem(w http.ResponseWriter, status int, title, detail, code, instance string) {
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
