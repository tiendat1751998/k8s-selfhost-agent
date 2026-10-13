package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/domain/cluster"
)

// FencingHandler exposes REST endpoints for STONITH fencing, anti-split-brain readiness,
// and IMDSv2 Zero-Trust metadata protection.
type FencingHandler struct {
	manager cluster.MultiDCFencingManager
	logger  *zap.Logger
}

// NewFencingHandler constructs a new FencingHandler.
func NewFencingHandler(manager cluster.MultiDCFencingManager, logger *zap.Logger) *FencingHandler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &FencingHandler{
		manager: manager,
		logger:  logger,
	}
}

// RegisterRoutes registers fencing endpoints on a cluster sub-router.
func (h *FencingHandler) RegisterRoutes(r chi.Router) {
	r.Post("/fencing/stonith", h.ExecuteSTONITH)
	r.Get("/fencing/readiness", h.GetFailoverReadiness)
	r.Post("/security/imdsv2", h.EnforceIMDSv2)
}

// WireToRouter mounts fencing routes under both /clusters/{clusterId} and /api/v1/clusters/{clusterId}.
func (h *FencingHandler) WireToRouter(r chi.Router) {
	r.Route("/clusters/{clusterId}", func(sub chi.Router) {
		h.RegisterRoutes(sub)
	})
	r.Route("/api/v1/clusters/{clusterId}", func(sub chi.Router) {
		h.RegisterRoutes(sub)
	})
}

// ExecuteSTONITHRequest defines the JSON payload for triggering STONITH fencing.
type ExecuteSTONITHRequest struct {
	TargetNodeID string `json:"target_node_id"`
	Datacenter   string `json:"datacenter"`
	Method       string `json:"method,omitempty"`
}

// ExecuteSTONITH handles POST /api/v1/clusters/:clusterId/fencing/stonith.
func (h *FencingHandler) ExecuteSTONITH(w http.ResponseWriter, r *http.Request) {
	if h.manager == nil {
		writeError(w, http.StatusServiceUnavailable, "fencing manager is not configured", nil)
		return
	}

	clusterID := extractClusterID(r)
	if clusterID == "" {
		writeError(w, http.StatusBadRequest, "clusterId parameter is required", nil)
		return
	}

	var req ExecuteSTONITHRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}

	req.TargetNodeID = strings.TrimSpace(req.TargetNodeID)
	if req.TargetNodeID == "" {
		writeError(w, http.StatusBadRequest, "target_node_id is required", nil)
		return
	}

	req.Datacenter = strings.TrimSpace(req.Datacenter)
	if req.Datacenter == "" {
		req.Datacenter = "default"
	}

	report, err := h.manager.ExecuteSTONITH(r.Context(), req.TargetNodeID, req.Datacenter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to execute STONITH fencing", err)
		return
	}

	writeJSON(w, http.StatusOK, report)
}

// GetFailoverReadiness handles GET /api/v1/clusters/:clusterId/fencing/readiness.
func (h *FencingHandler) GetFailoverReadiness(w http.ResponseWriter, r *http.Request) {
	if h.manager == nil {
		writeError(w, http.StatusServiceUnavailable, "fencing manager is not configured", nil)
		return
	}

	clusterID := extractClusterID(r)
	if clusterID == "" {
		writeError(w, http.StatusBadRequest, "clusterId parameter is required", nil)
		return
	}

	status, err := h.manager.ValidateFailoverReadiness(r.Context(), clusterID)
	if err != nil {
		if errors.Is(err, cluster.ErrFailoverPrimaryNotFenced) || errors.Is(err, cluster.ErrGracePeriodActive) {
			writeJSON(w, http.StatusConflict, map[string]interface{}{
				"error":     err.Error(),
				"readiness": status,
			})
			return
		}

		writeError(w, http.StatusInternalServerError, "failed to check failover readiness", err)
		return
	}

	writeJSON(w, http.StatusOK, status)
}

// EnforceIMDSv2 handles POST /api/v1/clusters/:clusterId/security/imdsv2.
func (h *FencingHandler) EnforceIMDSv2(w http.ResponseWriter, r *http.Request) {
	if h.manager == nil {
		writeError(w, http.StatusServiceUnavailable, "fencing manager is not configured", nil)
		return
	}

	clusterID := extractClusterID(r)
	if clusterID == "" {
		writeError(w, http.StatusBadRequest, "clusterId parameter is required", nil)
		return
	}

	if err := h.manager.EnforceIMDSv2Security(r.Context(), clusterID); err != nil {
		if errors.Is(err, cluster.ErrIMDSAccessBlocked) {
			writeError(w, http.StatusForbidden, err.Error(), err)
			return
		}

		writeError(w, http.StatusInternalServerError, "failed to enforce IMDSv2 security", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"cluster_id":       clusterID,
		"status":           "enforced",
		"network_policy":   "block-imds-metadata-access",
		"metadata_version": "IMDSv2",
		"hop_limit":        1,
		"message":          "Zero-trust metadata egress NetworkPolicy applied and IMDSv2 token enforcement active",
	})
}
