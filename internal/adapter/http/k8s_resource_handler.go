package http

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/adapter/http/middleware"
	"github.com/datdt/k8sselfhost/internal/domain/audit"
	infraK8s "github.com/datdt/k8sselfhost/internal/infrastructure/kubernetes"
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

	r.Post("/apply", h.ApplyYAML)
}

func isK8sUnavailable(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, infraK8s.ErrK8sUnavailable) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "kubernetes not connected") ||
		strings.Contains(msg, "kubernetes client not initialized") ||
		strings.Contains(msg, "cluster is unreachable") ||
		strings.Contains(msg, "not found in fleet repository") ||
		strings.Contains(msg, "no token or kubeconfig data")
}

func writeK8sUnavailable(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusServiceUnavailable)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error":   "kubernetes not connected",
		"message": "Import a kubeconfig via Fleet to enable this feature",
	})
}

func getNamespaceQuery(r *http.Request) string {
	ns := r.URL.Query().Get("ns")
	if ns == "" {
		ns = r.URL.Query().Get("namespace")
	}
	return strings.TrimSpace(ns)
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
