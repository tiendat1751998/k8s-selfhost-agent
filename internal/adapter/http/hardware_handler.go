// Package http provides HTTP adapters and handlers for bare-metal hardware watchdog telemetry.
package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	domain "github.com/datdt/k8sselfhost/internal/domain/hardware"
)

// HardwareHandler exposes REST API endpoints for low-level bare-metal server health auditing,
// including ECC memory controller errors, CPU thermal throttling, and dual-PSU power redundancy.
type HardwareHandler struct {
	engine domain.HardwareWatchdogEngine
	logger *zap.Logger
}

// NewHardwareHandler constructs a new HardwareHandler with the watchdog engine.
func NewHardwareHandler(engine domain.HardwareWatchdogEngine, loggers ...*zap.Logger) *HardwareHandler {
	var l *zap.Logger
	if len(loggers) > 0 && loggers[0] != nil {
		l = loggers[0]
	} else {
		l = zap.NewNop()
	}
	return &HardwareHandler{
		engine: engine,
		logger: l,
	}
}

// RegisterRoutes mounts hardware watchdog REST endpoints on the given chi router.
func (h *HardwareHandler) RegisterRoutes(r chi.Router) {
	r.Get("/ecc/dimm-status", h.GetECCDIMMStatus)
	r.Get("/cpu/thermal-status", h.GetCPUThermalStatus)
	r.Get("/psu/redundancy-status", h.GetPSURedundancyStatus)
	r.Post("/nodes/{node}/drain-degraded", h.DrainDegradedNode)
	r.Post("/nodes/{node_name}/drain-degraded", h.DrainDegradedNode)
}

// GetECCDIMMStatus handles GET /api/v1/hardware/ecc/dimm-status.
// Audits memory controller correctable errors. If an error surge is active (>100/hr),
// returns HTTP 500 with RFC 7807 problem details.
func (h *HardwareHandler) GetECCDIMMStatus(w http.ResponseWriter, r *http.Request) {
	if h.engine == nil {
		h.writeProblem(w, http.StatusServiceUnavailable, "Service Unavailable", "Hardware watchdog engine not initialized", domain.CodeInternalError, r.URL.Path)
		return
	}

	dimms, err := h.engine.AuditECCMemoryErrors(r.Context())
	if err != nil {
		if errors.Is(err, domain.ErrECCMemoryCorrectableSurge) {
			h.logger.Warn("ECC memory correctable error surge reported", zap.Error(err))
			h.writeProblem(w, http.StatusInternalServerError, "ECC Memory Correctable Surge",
				fmt.Sprintf("Critical error rate exceeded on DIMM slot: %v", err),
				domain.CodeECCMemoryCorrectableSurge, r.URL.Path)
			return
		}
		h.logger.Error("failed to audit ECC memory status", zap.Error(err))
		h.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", err.Error(), domain.CodeInternalError, r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":        "healthy",
		"dimm_statuses": dimms,
		"count":         len(dimms),
	})
}

// GetCPUThermalStatus handles GET /api/v1/hardware/cpu/thermal-status.
// Inspects CPU package temperature and frequency degradation. If throttled, returns HTTP 503 RFC 7807.
func (h *HardwareHandler) GetCPUThermalStatus(w http.ResponseWriter, r *http.Request) {
	if h.engine == nil {
		h.writeProblem(w, http.StatusServiceUnavailable, "Service Unavailable", "Hardware watchdog engine not initialized", domain.CodeInternalError, r.URL.Path)
		return
	}

	status, err := h.engine.AuditCPUThermalThrottling(r.Context())
	if err != nil {
		if errors.Is(err, domain.ErrCPUThermalThrottled) {
			h.logger.Warn("CPU thermal throttling active", zap.Error(err))
			h.writeProblem(w, http.StatusServiceUnavailable, "CPU Thermal Throttled",
				fmt.Sprintf("CPU package thermal threshold exceeded (>85°C) or clock throttled below 50%%: %v", err),
				domain.CodeCPUThermalThrottled, r.URL.Path)
			return
		}
		h.logger.Error("failed to audit CPU thermal status", zap.Error(err))
		h.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", err.Error(), domain.CodeInternalError, r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":         "nominal",
		"thermal_status": status,
	})
}

// GetPSURedundancyStatus handles GET /api/v1/hardware/psu/redundancy-status.
// Audits dual-PSU power redundancy. If redundancy is lost, returns HTTP 500 RFC 7807.
func (h *HardwareHandler) GetPSURedundancyStatus(w http.ResponseWriter, r *http.Request) {
	if h.engine == nil {
		h.writeProblem(w, http.StatusServiceUnavailable, "Service Unavailable", "Hardware watchdog engine not initialized", domain.CodeInternalError, r.URL.Path)
		return
	}

	supplies, err := h.engine.AuditPowerSupplyRedundancy(r.Context())
	if err != nil {
		if errors.Is(err, domain.ErrPSURedundancyFailed) {
			h.logger.Warn("Dual-PSU power redundancy lost", zap.Error(err))
			h.writeProblem(w, http.StatusInternalServerError, "PSU Redundancy Failed",
				fmt.Sprintf("Power redundancy lost; danger of whole-rack PDU breaker trip: %v", err),
				domain.CodePSURedundancyFailed, r.URL.Path)
			return
		}
		h.logger.Error("failed to audit PSU redundancy status", zap.Error(err))
		h.writeProblem(w, http.StatusInternalServerError, "Internal Server Error", err.Error(), domain.CodeInternalError, r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":          "nominal",
		"power_supplies":  supplies,
		"count":           len(supplies),
		"redundancy_loss": false,
	})
}

type drainDegradedNodeRequest struct {
	Reason string `json:"reason"`
}

// DrainDegradedNode handles POST /api/v1/hardware/nodes/:node/drain-degraded.
// Triggers cordon, degraded labelling, and graceful pod eviction on the specified bare-metal node.
func (h *HardwareHandler) DrainDegradedNode(w http.ResponseWriter, r *http.Request) {
	nodeName := chi.URLParam(r, "node")
	if nodeName == "" {
		nodeName = chi.URLParam(r, "node_name")
	}
	if nodeName == "" {
		nodeName = chi.URLParam(r, "name")
	}
	if strings.TrimSpace(nodeName) == "" {
		h.writeProblem(w, http.StatusBadRequest, "Bad Request", "node parameter is required", domain.CodeValidation, r.URL.Path)
		return
	}

	var req drainDegradedNodeRequest
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}
	if req.Reason == "" {
		req.Reason = "Hardware watchdog degraded node remediation"
	}

	if err := h.engine.DrainAndCordonDegradedNode(r.Context(), nodeName, req.Reason); err != nil {
		h.logger.Error("failed to drain degraded bare-metal node", zap.String("node", nodeName), zap.Error(err))
		h.writeProblem(w, http.StatusInternalServerError, "Node Drain Failed", err.Error(), domain.CodeInternalError, r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "drained_and_cordoned",
		"node":   nodeName,
		"reason": req.Reason,
	})
}

func (h *HardwareHandler) writeProblem(w http.ResponseWriter, status int, title, detail, code, instance string) {
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
