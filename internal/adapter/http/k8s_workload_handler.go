package http

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/datdt/k8sselfhost/internal/adapter/http/middleware"
	"github.com/datdt/k8sselfhost/internal/pkg/logger"
)

type scaleWorkloadRequest struct {
	Namespace string `json:"namespace,omitempty"`
	Replicas  *int32 `json:"replicas"`
}

type suspendCronJobRequest struct {
	Suspend *bool `json:"suspend"`
}

// ScaleDeployment handles POST /api/v1/k8s/{cluster}/resources/{kind}/{name}/scale (or /deployments/{name}/scale)
func (h *K8sResourceHandler) ScaleDeployment(w http.ResponseWriter, r *http.Request) {
	if h.repo == nil {
		writeK8sUnavailable(w)
		return
	}

	cluster := chi.URLParam(r, "cluster")
	kind := chi.URLParam(r, "kind")
	name := chi.URLParam(r, "name")

	var req scaleWorkloadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Replicas == nil || *req.Replicas < 0 {
		writeError(w, http.StatusBadRequest, "invalid request body: replicas must be a non-negative integer", err)
		return
	}

	namespace := req.Namespace
	if namespace == "" {
		namespace = getNamespaceQuery(r)
	}

	k := strings.ToLower(strings.TrimSpace(kind))
	isSts := k == "statefulsets" || k == "statefulset" || k == "sts"

	var err error
	targetType := "k8s_deployment"
	successMsg := "deployment scaled successfully"
	notFoundMsg := "deployment not found"
	failMsg := "failed to scale deployment"

	if isSts {
		targetType = "k8s_statefulset"
		successMsg = "statefulset scaled successfully"
		notFoundMsg = "statefulset not found"
		failMsg = "failed to scale statefulset"
		err = h.repo.ScaleStatefulSet(r.Context(), cluster, namespace, name, *req.Replicas)
	} else {
		err = h.repo.ScaleDeployment(r.Context(), cluster, namespace, name, *req.Replicas)
	}

	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		userID = "system"
	}
	result := "success"
	details := map[string]interface{}{"cluster": cluster, "namespace": namespace, "name": name, "replicas": *req.Replicas}
	if err != nil {
		result = "failure"
		details["error"] = err.Error()
	}
	if h.auditRepo != nil {
		if auditErr := h.auditRepo.RecordAction(r.Context(), userID, "scale", targetType, name, name, result, details, r.RemoteAddr, r.Header.Get("User-Agent")); auditErr != nil {
			logger.Get().Error("failed to record audit action for workload scale", zap.Error(auditErr))
		}
	}

	if err != nil {
		if isK8sUnavailable(err) {
			writeK8sUnavailable(w)
			return
		}
		if k8serrors.IsNotFound(err) {
			writeError(w, http.StatusNotFound, notFoundMsg, err)
			return
		}
		writeError(w, http.StatusInternalServerError, failMsg, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message":   successMsg,
		"name":      name,
		"namespace": namespace,
		"replicas":  *req.Replicas,
		"cluster":   cluster,
	})
}

// RestartDeployment handles POST /api/v1/k8s/{cluster}/resources/{kind}/{name}/restart (or /deployments/{name}/restart)
func (h *K8sResourceHandler) RestartDeployment(w http.ResponseWriter, r *http.Request) {
	if h.repo == nil {
		writeK8sUnavailable(w)
		return
	}

	cluster := chi.URLParam(r, "cluster")
	kind := chi.URLParam(r, "kind")
	name := chi.URLParam(r, "name")
	namespace := getNamespaceQuery(r)

	k := strings.ToLower(strings.TrimSpace(kind))
	isDs := k == "daemonsets" || k == "daemonset" || k == "ds"

	var err error
	targetType := "k8s_deployment"
	successMsg := "deployment restart triggered successfully"
	notFoundMsg := "deployment not found"
	failMsg := "failed to restart deployment"

	if isDs {
		targetType = "k8s_daemonset"
		successMsg = "daemonset restart triggered successfully"
		notFoundMsg = "daemonset not found"
		failMsg = "failed to restart daemonset"
		err = h.repo.RestartDaemonSet(r.Context(), cluster, namespace, name)
	} else {
		err = h.repo.RestartDeployment(r.Context(), cluster, namespace, name)
	}

	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		userID = "system"
	}
	result := "success"
	details := map[string]interface{}{"cluster": cluster, "namespace": namespace, "name": name, "action": "restart"}
	if err != nil {
		result = "failure"
		details["error"] = err.Error()
	}
	if h.auditRepo != nil {
		if auditErr := h.auditRepo.RecordAction(r.Context(), userID, "restart", targetType, name, name, result, details, r.RemoteAddr, r.Header.Get("User-Agent")); auditErr != nil {
			logger.Get().Error("failed to record audit action for workload restart", zap.Error(auditErr))
		}
	}

	if err != nil {
		if isK8sUnavailable(err) {
			writeK8sUnavailable(w)
			return
		}
		if k8serrors.IsNotFound(err) {
			writeError(w, http.StatusNotFound, notFoundMsg, err)
			return
		}
		writeError(w, http.StatusInternalServerError, failMsg, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message":   successMsg,
		"name":      name,
		"namespace": namespace,
		"cluster":   cluster,
	})
}

// ScaleStatefulSet handles POST /api/v1/k8s/{cluster}/resources/statefulsets/{name}/scale
func (h *K8sResourceHandler) ScaleStatefulSet(w http.ResponseWriter, r *http.Request) {
	if h.repo == nil {
		writeK8sUnavailable(w)
		return
	}

	cluster := chi.URLParam(r, "cluster")
	name := chi.URLParam(r, "name")

	var req scaleWorkloadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Replicas == nil || *req.Replicas < 0 {
		writeError(w, http.StatusBadRequest, "invalid request body: replicas must be a non-negative integer", err)
		return
	}

	namespace := req.Namespace
	if namespace == "" {
		namespace = getNamespaceQuery(r)
	}

	err := h.repo.ScaleStatefulSet(r.Context(), cluster, namespace, name, *req.Replicas)

	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		userID = "system"
	}
	result := "success"
	details := map[string]interface{}{"cluster": cluster, "namespace": namespace, "name": name, "replicas": *req.Replicas}
	if err != nil {
		result = "failure"
		details["error"] = err.Error()
	}
	if h.auditRepo != nil {
		if auditErr := h.auditRepo.RecordAction(r.Context(), userID, "scale", "k8s_statefulset", name, name, result, details, r.RemoteAddr, r.Header.Get("User-Agent")); auditErr != nil {
			logger.Get().Error("failed to record audit action for statefulset scale", zap.Error(auditErr))
		}
	}

	if err != nil {
		if isK8sUnavailable(err) {
			writeK8sUnavailable(w)
			return
		}
		if k8serrors.IsNotFound(err) {
			writeError(w, http.StatusNotFound, "statefulset not found", err)
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to scale statefulset", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message":   "statefulset scaled successfully",
		"name":      name,
		"namespace": namespace,
		"replicas":  *req.Replicas,
		"cluster":   cluster,
	})
}

// RestartDaemonSet handles POST /api/v1/k8s/{cluster}/resources/daemonsets/{name}/restart
func (h *K8sResourceHandler) RestartDaemonSet(w http.ResponseWriter, r *http.Request) {
	if h.repo == nil {
		writeK8sUnavailable(w)
		return
	}

	cluster := chi.URLParam(r, "cluster")
	name := chi.URLParam(r, "name")
	namespace := getNamespaceQuery(r)

	err := h.repo.RestartDaemonSet(r.Context(), cluster, namespace, name)

	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		userID = "system"
	}
	result := "success"
	details := map[string]interface{}{"cluster": cluster, "namespace": namespace, "name": name, "action": "restart"}
	if err != nil {
		result = "failure"
		details["error"] = err.Error()
	}
	if h.auditRepo != nil {
		if auditErr := h.auditRepo.RecordAction(r.Context(), userID, "restart", "k8s_daemonset", name, name, result, details, r.RemoteAddr, r.Header.Get("User-Agent")); auditErr != nil {
			logger.Get().Error("failed to record audit action for daemonset restart", zap.Error(auditErr))
		}
	}

	if err != nil {
		if isK8sUnavailable(err) {
			writeK8sUnavailable(w)
			return
		}
		if k8serrors.IsNotFound(err) {
			writeError(w, http.StatusNotFound, "daemonset not found", err)
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to restart daemonset", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message":   "daemonset restart triggered successfully",
		"name":      name,
		"namespace": namespace,
		"cluster":   cluster,
	})
}

// TriggerCronJob handles POST /api/v1/k8s/{cluster}/resources/cronjobs/{name}/trigger
func (h *K8sResourceHandler) TriggerCronJob(w http.ResponseWriter, r *http.Request) {
	if h.repo == nil {
		writeK8sUnavailable(w)
		return
	}

	cluster := chi.URLParam(r, "cluster")
	name := chi.URLParam(r, "name")
	namespace := getNamespaceQuery(r)

	job, err := h.repo.TriggerCronJob(r.Context(), cluster, namespace, name)

	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		userID = "system"
	}
	result := "success"
	details := map[string]interface{}{"cluster": cluster, "namespace": namespace, "cronjob": name}
	if job != nil {
		details["job"] = job.Name
	}
	if err != nil {
		result = "failure"
		details["error"] = err.Error()
	}
	if h.auditRepo != nil {
		if auditErr := h.auditRepo.RecordAction(r.Context(), userID, "trigger", "k8s_cronjob", name, name, result, details, r.RemoteAddr, r.Header.Get("User-Agent")); auditErr != nil {
			logger.Get().Error("failed to record audit action for cronjob trigger", zap.Error(auditErr))
		}
	}

	if err != nil {
		if isK8sUnavailable(err) {
			writeK8sUnavailable(w)
			return
		}
		if k8serrors.IsNotFound(err) {
			writeError(w, http.StatusNotFound, "cronjob not found", err)
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to trigger cronjob", err)
		return
	}

	jobName := ""
	if job != nil {
		jobName = job.Name
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message":   "cronjob triggered successfully",
		"cronjob":   name,
		"job":       jobName,
		"namespace": namespace,
		"cluster":   cluster,
	})
}

// SuspendCronJob handles PUT /api/v1/k8s/{cluster}/resources/cronjobs/{name}/suspend
func (h *K8sResourceHandler) SuspendCronJob(w http.ResponseWriter, r *http.Request) {
	if h.repo == nil {
		writeK8sUnavailable(w)
		return
	}

	cluster := chi.URLParam(r, "cluster")
	name := chi.URLParam(r, "name")
	namespace := getNamespaceQuery(r)

	var req suspendCronJobRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Suspend == nil {
		writeError(w, http.StatusBadRequest, "invalid request body: suspend boolean is required", err)
		return
	}

	err := h.repo.SuspendCronJob(r.Context(), cluster, namespace, name, *req.Suspend)

	action := "suspend"
	if !*req.Suspend {
		action = "resume"
	}

	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		userID = "system"
	}
	result := "success"
	details := map[string]interface{}{"cluster": cluster, "namespace": namespace, "cronjob": name, "suspend": *req.Suspend}
	if err != nil {
		result = "failure"
		details["error"] = err.Error()
	}
	if h.auditRepo != nil {
		if auditErr := h.auditRepo.RecordAction(r.Context(), userID, action, "k8s_cronjob", name, name, result, details, r.RemoteAddr, r.Header.Get("User-Agent")); auditErr != nil {
			logger.Get().Error("failed to record audit action for cronjob suspend", zap.Error(auditErr))
		}
	}

	if err != nil {
		if isK8sUnavailable(err) {
			writeK8sUnavailable(w)
			return
		}
		if k8serrors.IsNotFound(err) {
			writeError(w, http.StatusNotFound, "cronjob not found", err)
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to update cronjob suspend state", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message":   "cronjob suspend status updated successfully",
		"cronjob":   name,
		"suspend":   *req.Suspend,
		"namespace": namespace,
		"cluster":   cluster,
	})
}
