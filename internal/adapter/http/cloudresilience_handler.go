// Package http provides HTTP adapters and request routing.
package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/domain/cloudresilience"
)

// CloudResilienceHandler provides REST API endpoints for cloud burst credit monitoring,
// online EBS volume migration, Spot interruption node draining, and NAT Gateway SNAT shields.
type CloudResilienceHandler struct {
	engine cloudresilience.CloudResilienceEngine
	logger *zap.Logger
}

// NewCloudResilienceHandler creates a new CloudResilienceHandler instance.
func NewCloudResilienceHandler(engine cloudresilience.CloudResilienceEngine, logger *zap.Logger) *CloudResilienceHandler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &CloudResilienceHandler{
		engine: engine,
		logger: logger,
	}
}

// RegisterRoutes registers the cloud resilience routes on the provided chi router.
func (h *CloudResilienceHandler) RegisterRoutes(r chi.Router) {
	r.Get("/storage/burst-status", h.GetBurstStatus)
	r.Post("/storage/upgrade-gp3", h.UpgradeGP3)
	r.Get("/nat/snat-metrics", h.GetSNATMetrics)
	r.Post("/spot/drain-node", h.DrainNode)
}

// WireToRouter mounts the resilience endpoints under /api/v1/cloud.
func (h *CloudResilienceHandler) WireToRouter(r chi.Router) {
	r.Route("/api/v1/cloud", func(sub chi.Router) {
		h.RegisterRoutes(sub)
	})
}

// writeProblem writes an RFC 7807 Problem Details object with application/problem+json.
func (h *CloudResilienceHandler) writeProblem(w http.ResponseWriter, status int, title, detail, code, instance string) {
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

// GetBurstStatus handles GET /api/v1/cloud/storage/burst-status.
// Audits EBS volume burst balance; flags NeedsUpgrade when < 20%; returns 500 RFC 7807 on 0% exhaustion.
func (h *CloudResilienceHandler) GetBurstStatus(w http.ResponseWriter, r *http.Request) {
	volumeID := strings.TrimSpace(r.URL.Query().Get("volume_id"))
	if volumeID == "" {
		writeError(w, http.StatusBadRequest, "volume_id query parameter is required", nil)
		return
	}

	if h.engine == nil {
		h.writeProblem(w, http.StatusServiceUnavailable, "Engine Unavailable",
			"Cloud resilience engine is not configured", "ERR_ENGINE_UNAVAILABLE", r.URL.Path)
		return
	}

	status, err := h.engine.CheckEBSBurstBalance(r.Context(), volumeID)
	if err != nil {
		if errors.Is(err, cloudresilience.ErrCloudEBSBurstExhausted) {
			h.writeProblem(w, http.StatusInternalServerError, "EBS Burst Balance Exhausted",
				err.Error(), "ERR_CLOUD_EBS_BURST_EXHAUSTED", r.URL.Path)
			return
		}
		if errors.Is(err, cloudresilience.ErrInvalidVolumeID) {
			writeError(w, http.StatusBadRequest, err.Error(), nil)
			return
		}
		h.logger.Error("failed to check ebs burst balance", zap.Error(err), zap.String("volume_id", volumeID))
		h.writeProblem(w, http.StatusInternalServerError, "Burst Check Failed",
			err.Error(), "ERR_BURST_CHECK_FAILED", r.URL.Path)
		return
	}

	writeJSON(w, http.StatusOK, status)
}

type upgradeGP3Request struct {
	VolumeID string `json:"volume_id"`
}

// UpgradeGP3 handles POST /api/v1/cloud/storage/upgrade-gp3.
// Triggers online Elastic Volumes migration from gp2 to gp3 (baseline 3000 IOPS).
func (h *CloudResilienceHandler) UpgradeGP3(w http.ResponseWriter, r *http.Request) {
	var req upgradeGP3Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}

	volumeID := strings.TrimSpace(req.VolumeID)
	if volumeID == "" {
		writeError(w, http.StatusBadRequest, "volume_id is required", nil)
		return
	}

	if h.engine == nil {
		h.writeProblem(w, http.StatusServiceUnavailable, "Engine Unavailable",
			"Cloud resilience engine is not configured", "ERR_ENGINE_UNAVAILABLE", r.URL.Path)
		return
	}

	if err := h.engine.UpgradeVolumeToGP3(r.Context(), volumeID); err != nil {
		h.logger.Error("failed to upgrade volume to gp3", zap.Error(err), zap.String("volume_id", volumeID))
		h.writeProblem(w, http.StatusInternalServerError, "Volume Upgrade Failed",
			err.Error(), "ERR_VOLUME_UPGRADE_FAILED", r.URL.Path)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":      "upgraded",
		"volume_id":   volumeID,
		"target_type": "gp3",
		"message":     "Volume successfully migrated to gp3 with baseline 3000 IOPS",
	})
}

// GetSNATMetrics handles GET /api/v1/cloud/nat/snat-metrics.
// Monitors active connections and port errors; returns 503 RFC 7807 when saturated (> 64,512/IP).
func (h *CloudResilienceHandler) GetSNATMetrics(w http.ResponseWriter, r *http.Request) {
	gatewayID := strings.TrimSpace(r.URL.Query().Get("gateway_id"))
	if gatewayID == "" {
		writeError(w, http.StatusBadRequest, "gateway_id query parameter is required", nil)
		return
	}

	if h.engine == nil {
		h.writeProblem(w, http.StatusServiceUnavailable, "Engine Unavailable",
			"Cloud resilience engine is not configured", "ERR_ENGINE_UNAVAILABLE", r.URL.Path)
		return
	}

	metrics, err := h.engine.MonitorNATGatewaySNAT(r.Context(), gatewayID)
	if err != nil {
		if errors.Is(err, cloudresilience.ErrNATGatewaySaturated) {
			h.writeProblem(w, http.StatusServiceUnavailable, "NAT Gateway SNAT Saturated",
				err.Error(), "ERR_NAT_GATEWAY_SATURATED", r.URL.Path)
			return
		}
		if errors.Is(err, cloudresilience.ErrInvalidGatewayID) {
			writeError(w, http.StatusBadRequest, err.Error(), nil)
			return
		}
		h.logger.Error("failed to monitor nat gateway snat metrics", zap.Error(err), zap.String("gateway_id", gatewayID))
		h.writeProblem(w, http.StatusInternalServerError, "SNAT Monitoring Failed",
			err.Error(), "ERR_SNAT_MONITORING_FAILED", r.URL.Path)
		return
	}

	writeJSON(w, http.StatusOK, metrics)
}

type spotDrainNodeRequest struct {
	NodeName       string `json:"node_name"`
	GracePeriodSec int    `json:"grace_period_sec"`
}

// DrainNode handles POST /api/v1/cloud/spot/drain-node.
// Triggers node cordon and graceful pod evictions respecting PDB within the spot grace window.
func (h *CloudResilienceHandler) DrainNode(w http.ResponseWriter, r *http.Request) {
	var req spotDrainNodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}

	nodeName := strings.TrimSpace(req.NodeName)
	if nodeName == "" {
		writeError(w, http.StatusBadRequest, "node_name is required", nil)
		return
	}

	gracePeriod := time.Duration(req.GracePeriodSec) * time.Second
	if gracePeriod <= 0 {
		gracePeriod = 90 * time.Second
	}

	if h.engine == nil {
		h.writeProblem(w, http.StatusServiceUnavailable, "Engine Unavailable",
			"Cloud resilience engine is not configured", "ERR_ENGINE_UNAVAILABLE", r.URL.Path)
		return
	}

	if err := h.engine.DrainNodeGracefully(r.Context(), nodeName, gracePeriod); err != nil {
		if errors.Is(err, cloudresilience.ErrCloudSpotTerminationDraining) {
			h.writeProblem(w, http.StatusServiceUnavailable, "Spot Instance Termination Draining",
				err.Error(), "ERR_CLOUD_SPOT_TERMINATION_DRAINING", r.URL.Path)
			return
		}
		h.logger.Error("failed to gracefully drain spot node", zap.Error(err), zap.String("node_name", nodeName))
		h.writeProblem(w, http.StatusInternalServerError, "Node Drain Failed",
			err.Error(), "ERR_NODE_DRAIN_FAILED", r.URL.Path)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":           "drained",
		"node_name":        nodeName,
		"grace_period_sec": int(gracePeriod.Seconds()),
		"message":          "Node cordoned and workloads evicted within graceful window",
	})
}
