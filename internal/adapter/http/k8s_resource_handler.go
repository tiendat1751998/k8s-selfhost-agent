package http

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/datdt/k8sselfhost/internal/adapter/http/middleware"
	"github.com/datdt/k8sselfhost/internal/domain/audit"
	infraK8s "github.com/datdt/k8sselfhost/internal/infrastructure/kubernetes"
	domainerrors "github.com/datdt/k8sselfhost/internal/pkg/errors"
	"github.com/datdt/k8sselfhost/internal/pkg/logger"
)

// K8sResourceHandler provides HTTP handlers for managing generic Kubernetes resources.
type K8sResourceHandler struct {
	repo      *infraK8s.ResourceRepo
	auditRepo audit.Repository
}

// NewK8sResourceHandler creates a new K8sResourceHandler.
func NewK8sResourceHandler(repo *infraK8s.ResourceRepo, auditRepo audit.Repository) *K8sResourceHandler {
	return &K8sResourceHandler{
		repo:      repo,
		auditRepo: auditRepo,
	}
}

// RegisterRoutes registers the Kubernetes resource routes on the router mounted at /api/v1/k8s/{cluster}.
func (h *K8sResourceHandler) RegisterRoutes(r chi.Router) {
	r.Get("/namespaces", h.ListNamespaces)
	r.Post("/namespaces", h.CreateNamespace)
	r.Delete("/namespaces/{name}", h.DeleteNamespace)

	r.Route("/resources/{kind}", func(r chi.Router) {
		r.Get("/", h.ListResources)
		r.Post("/", h.CreateResource)
		r.Get("/{name}", h.GetResource)
		r.Put("/{name}", h.UpdateResource)
		r.Delete("/{name}", h.DeleteResource)
	})

	r.Get("/events", h.ListEvents)

	r.With(middleware.RBACMiddleware("platform_admin", "tenant_admin")).Route("/nodes/{name}", func(r chi.Router) {
		r.Post("/cordon", h.CordonNode)
		r.Post("/uncordon", h.UncordonNode)
		r.Post("/drain", h.DrainNode)
		r.Put("/taints", h.UpdateNodeTaints)
		r.Put("/labels", h.UpdateNodeLabels)
	})

	r.Post("/apply", h.ApplyYAML)
}

func isK8sUnavailable(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, infraK8s.ErrK8sUnavailable) || errors.Is(err, domainerrors.ErrK8sUnavailable) {
		return true
	}
	var domErr *domainerrors.DomainError
	if errors.As(err, &domErr) && domErr.Code == domainerrors.CodeK8sUnavailable {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "kubernetes not connected") ||
		strings.Contains(msg, "kubernetes cluster not connected") ||
		strings.Contains(msg, "kubernetes client not initialized") ||
		strings.Contains(msg, "cluster is unreachable") ||
		strings.Contains(msg, "not found in fleet repository") ||
		strings.Contains(msg, "no token or kubeconfig data") ||
		strings.Contains(msg, "unconfigured") ||
		strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "no such host") ||
		strings.Contains(msg, "i/o timeout") ||
		strings.Contains(msg, "context deadline exceeded") ||
		strings.Contains(msg, "unable to connect to the server") ||
		strings.Contains(msg, "k8s_unavailable") ||
		strings.Contains(msg, "dial tcp") ||
		strings.Contains(msg, "connectex") ||
		strings.Contains(msg, "certificate signed by unknown authority") ||
		strings.Contains(msg, "x509") ||
		strings.Contains(msg, "tls") ||
		strings.Contains(msg, "connection reset by peer")
}

func writeK8sUnavailable(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusServiceUnavailable)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error":   "Kubernetes cluster not connected or unconfigured",
		"code":    "K8S_UNAVAILABLE",
		"message": "Import a kubeconfig via Fleet to enable this feature",
	})
}

func getNamespaceQuery(r *http.Request) string {
	ns := r.URL.Query().Get("ns")
	if ns == "" {
		ns = r.URL.Query().Get("namespace")
	}
	ns = strings.TrimSpace(ns)
	if ns == "all" || ns == "_all" {
		return ""
	}
	return ns
}

// ListNamespaces handles GET /api/v1/k8s/{cluster}/namespaces
func (h *K8sResourceHandler) ListNamespaces(w http.ResponseWriter, r *http.Request) {
	if h.repo == nil {
		writeK8sUnavailable(w)
		return
	}

	cluster := chi.URLParam(r, "cluster")
	namespaces, err := h.repo.ListNamespaces(r.Context(), cluster)
	if err != nil {
		if isK8sUnavailable(err) {
			writeK8sUnavailable(w)
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to list namespaces", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data":  namespaces,
		"total": len(namespaces),
	})
}

type createNamespaceRequest struct {
	Name     string `json:"name"`
	Metadata struct {
		Name string `json:"name"`
	} `json:"metadata"`
}

// CreateNamespace handles POST /api/v1/k8s/{cluster}/namespaces
func (h *K8sResourceHandler) CreateNamespace(w http.ResponseWriter, r *http.Request) {
	if h.repo == nil {
		writeK8sUnavailable(w)
		return
	}

	cluster := chi.URLParam(r, "cluster")
	var req createNamespaceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", err)
		return
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = strings.TrimSpace(req.Metadata.Name)
	}
	if name == "" {
		writeError(w, http.StatusBadRequest, "namespace name is required", nil)
		return
	}

	ns, err := h.repo.CreateNamespace(r.Context(), cluster, name)

	// Audit logging
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		userID = "system"
	}
	result := "success"
	details := map[string]interface{}{"cluster": cluster, "namespace": name}
	if err != nil {
		result = "failure"
		details["error"] = err.Error()
	}
	if h.auditRepo != nil {
		if auditErr := h.auditRepo.RecordAction(r.Context(), userID, "create", "k8s_namespace", name, name, result, details, r.RemoteAddr, r.Header.Get("User-Agent")); auditErr != nil {
			logger.Get().Error("failed to record audit action for namespace creation", zap.Error(auditErr))
		}
	}

	if err != nil {
		if isK8sUnavailable(err) {
			writeK8sUnavailable(w)
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to create namespace", err)
		return
	}

	writeJSON(w, http.StatusCreated, ns)
}

// DeleteNamespace handles DELETE /api/v1/k8s/{cluster}/namespaces/{name}
func (h *K8sResourceHandler) DeleteNamespace(w http.ResponseWriter, r *http.Request) {
	if h.repo == nil {
		writeK8sUnavailable(w)
		return
	}

	cluster := chi.URLParam(r, "cluster")
	name := chi.URLParam(r, "name")

	err := h.repo.DeleteNamespace(r.Context(), cluster, name)

	// Audit logging
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		userID = "system"
	}
	result := "success"
	details := map[string]interface{}{"cluster": cluster, "namespace": name}
	if err != nil {
		result = "failure"
		details["error"] = err.Error()
	}
	if h.auditRepo != nil {
		if auditErr := h.auditRepo.RecordAction(r.Context(), userID, "delete", "k8s_namespace", name, name, result, details, r.RemoteAddr, r.Header.Get("User-Agent")); auditErr != nil {
			logger.Get().Error("failed to record audit action for namespace deletion", zap.Error(auditErr))
		}
	}

	if err != nil {
		if isK8sUnavailable(err) {
			writeK8sUnavailable(w)
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to delete namespace", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status": "deleted",
		"name":   name,
	})
}

// ListResources handles GET /api/v1/k8s/{cluster}/resources/{kind}?ns=default
func (h *K8sResourceHandler) ListResources(w http.ResponseWriter, r *http.Request) {
	if h.repo == nil {
		writeK8sUnavailable(w)
		return
	}

	cluster := chi.URLParam(r, "cluster")
	kind := chi.URLParam(r, "kind")
	ns := getNamespaceQuery(r)

	items, err := h.repo.ListResources(r.Context(), cluster, kind, ns)
	if err != nil {
		if isK8sUnavailable(err) {
			writeK8sUnavailable(w)
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to list resources", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data":  items,
		"total": len(items),
	})
}

// GetResource handles GET /api/v1/k8s/{cluster}/resources/{kind}/{name}?ns=default
func (h *K8sResourceHandler) GetResource(w http.ResponseWriter, r *http.Request) {
	if h.repo == nil {
		writeK8sUnavailable(w)
		return
	}

	cluster := chi.URLParam(r, "cluster")
	kind := chi.URLParam(r, "kind")
	name := chi.URLParam(r, "name")
	ns := getNamespaceQuery(r)

	item, err := h.repo.GetResource(r.Context(), cluster, kind, ns, name)
	if err != nil {
		if isK8sUnavailable(err) {
			writeK8sUnavailable(w)
			return
		}
		writeError(w, http.StatusNotFound, "resource not found", err)
		return
	}

	writeJSON(w, http.StatusOK, item)
}

// CreateResource handles POST /api/v1/k8s/{cluster}/resources/{kind}?ns=default
func (h *K8sResourceHandler) CreateResource(w http.ResponseWriter, r *http.Request) {
	if h.repo == nil {
		writeK8sUnavailable(w)
		return
	}

	cluster := chi.URLParam(r, "cluster")
	kind := chi.URLParam(r, "kind")
	ns := getNamespaceQuery(r)

	var manifest map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&manifest); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json manifest body", err)
		return
	}

	created, err := h.repo.CreateResource(r.Context(), cluster, kind, ns, manifest)

	// Extract resource name for audit
	targetName := ""
	if meta, ok := manifest["metadata"].(map[string]interface{}); ok {
		if n, ok := meta["name"].(string); ok {
			targetName = n
		}
	}

	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		userID = "system"
	}
	result := "success"
	details := map[string]interface{}{"cluster": cluster, "kind": kind, "namespace": ns, "name": targetName}
	if err != nil {
		result = "failure"
		details["error"] = err.Error()
	}
	if h.auditRepo != nil {
		if auditErr := h.auditRepo.RecordAction(r.Context(), userID, "create", kind, targetName, targetName, result, details, r.RemoteAddr, r.Header.Get("User-Agent")); auditErr != nil {
			logger.Get().Error("failed to record audit action for resource creation", zap.Error(auditErr))
		}
	}

	if err != nil {
		if isK8sUnavailable(err) {
			writeK8sUnavailable(w)
			return
		}
		writeError(w, http.StatusBadRequest, "failed to create resource", err)
		return
	}

	writeJSON(w, http.StatusCreated, created)
}

// UpdateResource handles PUT /api/v1/k8s/{cluster}/resources/{kind}/{name}?ns=default
func (h *K8sResourceHandler) UpdateResource(w http.ResponseWriter, r *http.Request) {
	if h.repo == nil {
		writeK8sUnavailable(w)
		return
	}

	cluster := chi.URLParam(r, "cluster")
	kind := chi.URLParam(r, "kind")
	name := chi.URLParam(r, "name")
	ns := getNamespaceQuery(r)

	var manifest map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&manifest); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json manifest body", err)
		return
	}

	updated, err := h.repo.UpdateResource(r.Context(), cluster, kind, ns, name, manifest)

	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		userID = "system"
	}
	result := "success"
	details := map[string]interface{}{"cluster": cluster, "kind": kind, "namespace": ns, "name": name}
	if err != nil {
		result = "failure"
		details["error"] = err.Error()
	}
	if h.auditRepo != nil {
		if auditErr := h.auditRepo.RecordAction(r.Context(), userID, "update", kind, name, name, result, details, r.RemoteAddr, r.Header.Get("User-Agent")); auditErr != nil {
			logger.Get().Error("failed to record audit action for resource update", zap.Error(auditErr))
		}
	}

	if err != nil {
		if isK8sUnavailable(err) {
			writeK8sUnavailable(w)
			return
		}
		writeError(w, http.StatusBadRequest, "failed to update resource", err)
		return
	}

	writeJSON(w, http.StatusOK, updated)
}

// DeleteResource handles DELETE /api/v1/k8s/{cluster}/resources/{kind}/{name}?ns=default
func (h *K8sResourceHandler) DeleteResource(w http.ResponseWriter, r *http.Request) {
	if h.repo == nil {
		writeK8sUnavailable(w)
		return
	}

	cluster := chi.URLParam(r, "cluster")
	kind := chi.URLParam(r, "kind")
	name := chi.URLParam(r, "name")
	ns := getNamespaceQuery(r)

	err := h.repo.DeleteResource(r.Context(), cluster, kind, ns, name)

	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		userID = "system"
	}
	result := "success"
	details := map[string]interface{}{"cluster": cluster, "kind": kind, "namespace": ns, "name": name}
	if err != nil {
		result = "failure"
		details["error"] = err.Error()
	}
	if h.auditRepo != nil {
		if auditErr := h.auditRepo.RecordAction(r.Context(), userID, "delete", kind, name, name, result, details, r.RemoteAddr, r.Header.Get("User-Agent")); auditErr != nil {
			logger.Get().Error("failed to record audit action for resource deletion", zap.Error(auditErr))
		}
	}

	if err != nil {
		if isK8sUnavailable(err) {
			writeK8sUnavailable(w)
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to delete resource", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status": "deleted",
		"name":   name,
		"kind":   kind,
	})
}

// ApplyYAML handles POST /api/v1/k8s/{cluster}/apply?ns=default
func (h *K8sResourceHandler) ApplyYAML(w http.ResponseWriter, r *http.Request) {
	if h.repo == nil {
		writeK8sUnavailable(w)
		return
	}

	cluster := chi.URLParam(r, "cluster")
	ns := getNamespaceQuery(r)

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read request body", err)
		return
	}

	// Support JSON wrapper with "yaml" field if provided
	var jsonBody struct {
		YAML string `json:"yaml"`
	}
	if err := json.Unmarshal(bodyBytes, &jsonBody); err == nil && strings.TrimSpace(jsonBody.YAML) != "" {
		bodyBytes = []byte(jsonBody.YAML)
	}

	if len(strings.TrimSpace(string(bodyBytes))) == 0 {
		writeError(w, http.StatusBadRequest, "empty yaml content", nil)
		return
	}

	err = h.repo.ApplyYAML(r.Context(), cluster, ns, bodyBytes)

	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		userID = "system"
	}
	result := "success"
	details := map[string]interface{}{"cluster": cluster, "namespace": ns}
	if err != nil {
		result = "failure"
		details["error"] = err.Error()
	}
	if h.auditRepo != nil {
		if auditErr := h.auditRepo.RecordAction(r.Context(), userID, "apply", "k8s_manifest", cluster, "yaml_apply", result, details, r.RemoteAddr, r.Header.Get("User-Agent")); auditErr != nil {
			logger.Get().Error("failed to record audit action for yaml apply", zap.Error(auditErr))
		}
	}

	if err != nil {
		if isK8sUnavailable(err) {
			writeK8sUnavailable(w)
			return
		}
		writeError(w, http.StatusBadRequest, "failed to apply yaml", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "applied",
		"cluster": cluster,
	})
}

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

// ListEvents handles GET /api/v1/k8s/{cluster}/events
func (h *K8sResourceHandler) ListEvents(w http.ResponseWriter, r *http.Request) {
	if h.repo == nil {
		writeK8sUnavailable(w)
		return
	}

	cluster := chi.URLParam(r, "cluster")
	ns := getNamespaceQuery(r)
	filterKind := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("kind")))
	filterName := strings.TrimSpace(r.URL.Query().Get("name"))
	filterType := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("type")))
	limitStr := strings.TrimSpace(r.URL.Query().Get("limit"))

	items, err := h.repo.ListResources(r.Context(), cluster, "events", ns)
	if err != nil {
		if isK8sUnavailable(err) {
			writeK8sUnavailable(w)
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to list events", err)
		return
	}

	var filtered []map[string]interface{}
	for _, item := range items {
		// Filter by involvedObject.kind if specified
		if filterKind != "" {
			var objKind string
			if inv, ok := item["involvedObject"].(map[string]interface{}); ok {
				if k, ok := inv["kind"].(string); ok {
					objKind = strings.ToLower(k)
				}
			}
			if objKind == "" {
				if k, ok := item["kind"].(string); ok {
					objKind = strings.ToLower(k)
				}
			}
			if objKind != filterKind {
				continue
			}
		}

		// Filter by involvedObject.name or metadata.name if specified
		if filterName != "" {
			var objName string
			if inv, ok := item["involvedObject"].(map[string]interface{}); ok {
				if n, ok := inv["name"].(string); ok {
					objName = n
				}
			}
			metaName := ""
			if meta, ok := item["metadata"].(map[string]interface{}); ok {
				if n, ok := meta["name"].(string); ok {
					metaName = n
				}
			}
			if objName != filterName && metaName != filterName {
				continue
			}
		}

		// Filter by event type (e.g. Normal, Warning)
		if filterType != "" {
			itemType, _ := item["type"].(string)
			if !strings.EqualFold(itemType, filterType) {
				continue
			}
		}

		filtered = append(filtered, item)
	}

	if filtered == nil {
		filtered = []map[string]interface{}{}
	}

	if limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 && limit < len(filtered) {
			filtered = filtered[:limit]
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"data":  filtered,
		"total": len(filtered),
	})
}
