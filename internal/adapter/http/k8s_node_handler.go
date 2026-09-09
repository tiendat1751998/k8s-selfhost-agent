package http

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/datdt/k8sselfhost/internal/adapter/http/middleware"
	"github.com/datdt/k8sselfhost/internal/pkg/logger"
)

type drainNodeRequest struct {
	GracePeriodSeconds *int64 `json:"gracePeriodSeconds"`
	IgnoreDaemonSets   *bool  `json:"ignoreDaemonSets"`
}

type updateNodeTaintsRequest struct {
	Taints []corev1.Taint `json:"taints"`
}

type updateNodeLabelsRequest struct {
	Labels map[string]string `json:"labels"`
}

// CordonNode handles POST /api/v1/k8s/{cluster}/nodes/{name}/cordon
func (h *K8sResourceHandler) CordonNode(w http.ResponseWriter, r *http.Request) {
	if h.repo == nil {
		writeK8sUnavailable(w)
		return
	}

	cluster := chi.URLParam(r, "cluster")
	name := chi.URLParam(r, "name")

	err := h.repo.CordonNode(r.Context(), cluster, name)

	// Audit logging
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		userID = "system"
	}
	result := "success"
	details := map[string]interface{}{"cluster": cluster, "node": name, "action": "cordon"}
	if err != nil {
		result = "failure"
		details["error"] = err.Error()
	}
	if h.auditRepo != nil {
		if auditErr := h.auditRepo.RecordAction(r.Context(), userID, "cordon", "k8s_node", name, name, result, details, r.RemoteAddr, r.Header.Get("User-Agent")); auditErr != nil {
			logger.Get().Error("failed to record audit action for node cordon", zap.Error(auditErr))
		}
	}

	if err != nil {
		if isK8sUnavailable(err) {
			writeK8sUnavailable(w)
			return
		}
		if k8serrors.IsNotFound(err) {
			writeError(w, http.StatusNotFound, "node not found", err)
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to cordon node", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "cordoned",
		"name":    name,
		"cluster": cluster,
	})
}

// UncordonNode handles POST /api/v1/k8s/{cluster}/nodes/{name}/uncordon
func (h *K8sResourceHandler) UncordonNode(w http.ResponseWriter, r *http.Request) {
	if h.repo == nil {
		writeK8sUnavailable(w)
		return
	}

	cluster := chi.URLParam(r, "cluster")
	name := chi.URLParam(r, "name")

	err := h.repo.UncordonNode(r.Context(), cluster, name)

	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		userID = "system"
	}
	result := "success"
	details := map[string]interface{}{"cluster": cluster, "node": name, "action": "uncordon"}
	if err != nil {
		result = "failure"
		details["error"] = err.Error()
	}
	if h.auditRepo != nil {
		if auditErr := h.auditRepo.RecordAction(r.Context(), userID, "uncordon", "k8s_node", name, name, result, details, r.RemoteAddr, r.Header.Get("User-Agent")); auditErr != nil {
			logger.Get().Error("failed to record audit action for node uncordon", zap.Error(auditErr))
		}
	}

	if err != nil {
		if isK8sUnavailable(err) {
			writeK8sUnavailable(w)
			return
		}
		if k8serrors.IsNotFound(err) {
			writeError(w, http.StatusNotFound, "node not found", err)
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to uncordon node", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "uncordoned",
		"name":    name,
		"cluster": cluster,
	})
}

// DrainNode handles POST /api/v1/k8s/{cluster}/nodes/{name}/drain
func (h *K8sResourceHandler) DrainNode(w http.ResponseWriter, r *http.Request) {
	if h.repo == nil {
		writeK8sUnavailable(w)
		return
	}

	cluster := chi.URLParam(r, "cluster")
	name := chi.URLParam(r, "name")

	var req drainNodeRequest
	if r.Body != nil && r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
			writeError(w, http.StatusBadRequest, "invalid request body", err)
			return
		}
	}

	gracePeriod := int64(30)
	if req.GracePeriodSeconds != nil {
		gracePeriod = *req.GracePeriodSeconds
	}
	ignoreDaemonSets := true
	if req.IgnoreDaemonSets != nil {
		ignoreDaemonSets = *req.IgnoreDaemonSets
	}

	err := h.repo.DrainNode(r.Context(), cluster, name, gracePeriod, ignoreDaemonSets)

	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		userID = "system"
	}
	result := "success"
	details := map[string]interface{}{
		"cluster":            cluster,
		"node":               name,
		"action":             "drain",
		"gracePeriodSeconds": gracePeriod,
		"ignoreDaemonSets":   ignoreDaemonSets,
	}
	if err != nil {
		result = "failure"
		details["error"] = err.Error()
	}
	if h.auditRepo != nil {
		if auditErr := h.auditRepo.RecordAction(r.Context(), userID, "drain", "k8s_node", name, name, result, details, r.RemoteAddr, r.Header.Get("User-Agent")); auditErr != nil {
			logger.Get().Error("failed to record audit action for node drain", zap.Error(auditErr))
		}
	}

	if err != nil {
		if isK8sUnavailable(err) {
			writeK8sUnavailable(w)
			return
		}
		if k8serrors.IsNotFound(err) {
			writeError(w, http.StatusNotFound, "node not found", err)
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to drain node", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":             "drained",
		"name":               name,
		"cluster":            cluster,
		"gracePeriodSeconds": gracePeriod,
		"ignoreDaemonSets":   ignoreDaemonSets,
	})
}

// UpdateNodeTaints handles PUT /api/v1/k8s/{cluster}/nodes/{name}/taints
func (h *K8sResourceHandler) UpdateNodeTaints(w http.ResponseWriter, r *http.Request) {
	if h.repo == nil {
		writeK8sUnavailable(w)
		return
	}

	cluster := chi.URLParam(r, "cluster")
	name := chi.URLParam(r, "name")

	var req updateNodeTaintsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}

	if req.Taints == nil {
		req.Taints = []corev1.Taint{}
	}

	err := h.repo.UpdateNodeTaints(r.Context(), cluster, name, req.Taints)

	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		userID = "system"
	}
	result := "success"
	details := map[string]interface{}{"cluster": cluster, "node": name, "taints": req.Taints}
	if err != nil {
		result = "failure"
		details["error"] = err.Error()
	}
	if h.auditRepo != nil {
		if auditErr := h.auditRepo.RecordAction(r.Context(), userID, "update_taints", "k8s_node", name, name, result, details, r.RemoteAddr, r.Header.Get("User-Agent")); auditErr != nil {
			logger.Get().Error("failed to record audit action for node taints update", zap.Error(auditErr))
		}
	}

	if err != nil {
		if isK8sUnavailable(err) {
			writeK8sUnavailable(w)
			return
		}
		if k8serrors.IsNotFound(err) {
			writeError(w, http.StatusNotFound, "node not found", err)
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to update node taints", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "updated",
		"name":    name,
		"cluster": cluster,
		"taints":  req.Taints,
	})
}

// UpdateNodeLabels handles PUT /api/v1/k8s/{cluster}/nodes/{name}/labels
func (h *K8sResourceHandler) UpdateNodeLabels(w http.ResponseWriter, r *http.Request) {
	if h.repo == nil {
		writeK8sUnavailable(w)
		return
	}

	cluster := chi.URLParam(r, "cluster")
	name := chi.URLParam(r, "name")

	var req updateNodeLabelsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}

	if req.Labels == nil {
		req.Labels = map[string]string{}
	}

	err := h.repo.UpdateNodeLabels(r.Context(), cluster, name, req.Labels)

	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		userID = "system"
	}
	result := "success"
	details := map[string]interface{}{"cluster": cluster, "node": name, "labels": req.Labels}
	if err != nil {
		result = "failure"
		details["error"] = err.Error()
	}
	if h.auditRepo != nil {
		if auditErr := h.auditRepo.RecordAction(r.Context(), userID, "update_labels", "k8s_node", name, name, result, details, r.RemoteAddr, r.Header.Get("User-Agent")); auditErr != nil {
			logger.Get().Error("failed to record audit action for node labels update", zap.Error(auditErr))
		}
	}

	if err != nil {
		if isK8sUnavailable(err) {
			writeK8sUnavailable(w)
			return
		}
		if k8serrors.IsNotFound(err) {
			writeError(w, http.StatusNotFound, "node not found", err)
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to update node labels", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "updated",
		"name":    name,
		"cluster": cluster,
		"labels":  req.Labels,
	})
}
