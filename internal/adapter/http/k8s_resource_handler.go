package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

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

	r.Route("/resources/{kind}", func(resSub chi.Router) {
		resSub.Get("/", h.ListResources)
		resSub.Post("/", h.CreateResource)
		resSub.Get("/{name}", h.GetResource)
		resSub.Put("/{name}", h.UpdateResource)
		resSub.Delete("/{name}", h.DeleteResource)

		// Workload actions on individual resource
		resSub.With(middleware.RBACMiddleware("platform_admin", "tenant_admin")).Group(func(actSub chi.Router) {
			actSub.Post("/{name}/scale", h.ScaleDeployment)
			actSub.Post("/{name}/restart", h.RestartDeployment)
			actSub.Post("/{name}/trigger", h.TriggerCronJob)
			actSub.Put("/{name}/suspend", h.SuspendCronJob)
		})
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
	writeK8sUnavailableWithErr(w, nil)
}

func writeK8sUnavailableWithErr(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusServiceUnavailable)
	errMsg := "Kubernetes cluster not connected or unconfigured"
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error":   errMsg,
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
			writeK8sUnavailableWithErr(w, err)
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
