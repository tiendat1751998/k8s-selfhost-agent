package http

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/adapter/http/middleware"
	"github.com/datdt/k8sselfhost/internal/domain/audit"
	"github.com/datdt/k8sselfhost/internal/infrastructure/helm"
	"github.com/datdt/k8sselfhost/internal/pkg/logger"
)

// HelmHandler provides HTTP endpoints for managing Helm repositories, searching charts, and release lifecycle.
type HelmHandler struct {
	releaseManager *helm.ReleaseManager
	auditRepo      audit.Repository
}

// NewHelmHandler creates a new HelmHandler.
func NewHelmHandler(releaseManager *helm.ReleaseManager, auditRepo audit.Repository) *HelmHandler {
	return &HelmHandler{
		releaseManager: releaseManager,
		auditRepo:      auditRepo,
	}
}

// RegisterRoutes registers all Helm endpoints on the given router.
func (h *HelmHandler) RegisterRoutes(r chi.Router) {
	// Repositories
	r.Get("/repos", h.ListRepos)
	r.With(middleware.RequireRolesForMutations("platform_admin", "tenant_admin")).Post("/repos", h.AddRepo)
	r.With(middleware.RequireRolesForMutations("platform_admin", "tenant_admin")).Put("/repos", h.UpdateRepos)

	// Charts
	r.Get("/charts", h.SearchCharts)
	r.Get("/charts/{repo}/{chart}/values", h.GetChartValues)

	// Cluster releases
	r.Route("/{cluster}/releases", func(cr chi.Router) {
		cr.Get("/", h.ListReleases)
		cr.With(middleware.RequireRolesForMutations("platform_admin", "tenant_admin")).Post("/", h.InstallRelease)
		cr.Get("/{name}", h.GetRelease)
		cr.With(middleware.RequireRolesForMutations("platform_admin", "tenant_admin")).Put("/{name}", h.UpgradeRelease)
		cr.With(middleware.RequireRolesForMutations("platform_admin", "tenant_admin")).Delete("/{name}", h.UninstallRelease)
		cr.With(middleware.RequireRolesForMutations("platform_admin", "tenant_admin")).Post("/{name}/rollback", h.RollbackRelease)
		cr.Get("/{name}/history", h.GetReleaseHistory)
	})
}

type addRepoRequest struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// ListRepos handles GET /helm/repos - lists configured Helm repositories.
func (h *HelmHandler) ListRepos(w http.ResponseWriter, r *http.Request) {
	if h.releaseManager == nil {
		writeError(w, http.StatusServiceUnavailable, "Helm release manager not initialized", nil)
		return
	}
	repos, err := h.releaseManager.ListRepos()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list repositories", err)
		return
	}
	writeJSON(w, http.StatusOK, repos)
}

// AddRepo handles POST /helm/repos - adds a new Helm repository.
func (h *HelmHandler) AddRepo(w http.ResponseWriter, r *http.Request) {
	if h.releaseManager == nil {
		writeError(w, http.StatusServiceUnavailable, "Helm release manager not initialized", nil)
		return
	}
	req, ok := decodeJSON[addRepoRequest](w, r)
	if !ok {
		return
	}
	if strings.TrimSpace(req.Name) == "" || strings.TrimSpace(req.URL) == "" {
		writeError(w, http.StatusBadRequest, "name and url are required", nil)
		return
	}

	err := h.releaseManager.AddRepo(req.Name, req.URL)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to add repository", err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{
		"status":  "success",
		"message": "repository added successfully",
		"name":    req.Name,
		"url":     req.URL,
	})
}

// UpdateRepos handles PUT /helm/repos - updates all configured Helm repositories.
func (h *HelmHandler) UpdateRepos(w http.ResponseWriter, r *http.Request) {
	if h.releaseManager == nil {
		writeError(w, http.StatusServiceUnavailable, "Helm release manager not initialized", nil)
		return
	}
	err := h.releaseManager.UpdateRepos()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update repositories", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "success",
		"message": "repositories updated successfully",
	})
}

// SearchCharts handles GET /helm/charts - searches charts across configured repositories.
func (h *HelmHandler) SearchCharts(w http.ResponseWriter, r *http.Request) {
	if h.releaseManager == nil {
		writeError(w, http.StatusServiceUnavailable, "Helm release manager not initialized", nil)
		return
	}
	keyword := r.URL.Query().Get("keyword")
	if keyword == "" {
		keyword = r.URL.Query().Get("q")
	}
	if keyword == "" {
		keyword = r.URL.Query().Get("query")
	}

	results, err := h.releaseManager.SearchCharts(keyword)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to search charts", err)
		return
	}
	writeJSON(w, http.StatusOK, results)
}

// GetChartValues handles GET /helm/charts/{repo}/{chart}/values - retrieves default values.yaml for a chart.
func (h *HelmHandler) GetChartValues(w http.ResponseWriter, r *http.Request) {
	if h.releaseManager == nil {
		writeError(w, http.StatusServiceUnavailable, "Helm release manager not initialized", nil)
		return
	}
	repoName := chi.URLParam(r, "repo")
	chartName := chi.URLParam(r, "chart")
	version := r.URL.Query().Get("version")

	if chartName == "" {
		writeError(w, http.StatusBadRequest, "chart name is required", nil)
		return
	}

	values, err := h.releaseManager.GetChartValues(repoName, chartName, version)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get chart values", err)
		return
	}
	writeJSON(w, http.StatusOK, values)
}

// ListReleases handles GET /helm/{cluster}/releases - lists releases in the specified cluster/namespace.
func (h *HelmHandler) ListReleases(w http.ResponseWriter, r *http.Request) {
	if h.releaseManager == nil {
		writeError(w, http.StatusServiceUnavailable, "Helm release manager not initialized", nil)
		return
	}
	clusterID := chi.URLParam(r, "cluster")
	namespace := getNamespaceQuery(r)

	releases, err := h.releaseManager.ListReleases(r.Context(), clusterID, namespace)
	if err != nil {
		if isK8sUnavailable(err) {
			writeK8sUnavailable(w)
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to list releases", err)
		return
	}
	writeJSON(w, http.StatusOK, releases)
}

// GetRelease handles GET /helm/{cluster}/releases/{name} - gets release details.
func (h *HelmHandler) GetRelease(w http.ResponseWriter, r *http.Request) {
	if h.releaseManager == nil {
		writeError(w, http.StatusServiceUnavailable, "Helm release manager not initialized", nil)
		return
	}
	clusterID := chi.URLParam(r, "cluster")
	name := chi.URLParam(r, "name")
	namespace := getNamespaceQuery(r)

	if name == "" {
		writeError(w, http.StatusBadRequest, "release name is required", nil)
		return
	}

	rel, err := h.releaseManager.GetRelease(r.Context(), clusterID, name, namespace)
	if err != nil {
		if isK8sUnavailable(err) {
			writeK8sUnavailable(w)
			return
		}
		if strings.Contains(strings.ToLower(err.Error()), "not found") {
			writeError(w, http.StatusNotFound, "release not found", err)
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get release", err)
		return
	}
	writeJSON(w, http.StatusOK, rel)
}

// InstallRelease handles POST /helm/{cluster}/releases - installs a new Helm release.
func (h *HelmHandler) InstallRelease(w http.ResponseWriter, r *http.Request) {
	if h.releaseManager == nil {
		writeError(w, http.StatusServiceUnavailable, "Helm release manager not initialized", nil)
		return
	}
	clusterID := chi.URLParam(r, "cluster")
	req, ok := decodeJSON[helm.InstallRequest](w, r)
	if !ok {
		return
	}

	if strings.TrimSpace(req.ReleaseName) == "" {
		writeError(w, http.StatusBadRequest, "releaseName is required", nil)
		return
	}
	if strings.TrimSpace(req.Chart) == "" {
		writeError(w, http.StatusBadRequest, "chart is required", nil)
		return
	}
	if req.Namespace == "" {
		req.Namespace = getNamespaceQuery(r)
	}
	if req.Namespace == "" {
		req.Namespace = "default"
	}

	rel, err := h.releaseManager.InstallRelease(r.Context(), clusterID, *req)

	h.recordAudit(r, "install", req.ReleaseName, clusterID, map[string]interface{}{
		"cluster":   clusterID,
		"namespace": req.Namespace,
		"chart":     req.Chart,
		"repo":      req.Repo,
		"version":   req.Version,
	}, err)

	if err != nil {
		if isK8sUnavailable(err) {
			writeK8sUnavailable(w)
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to install release", err)
		return
	}

	writeJSON(w, http.StatusCreated, rel)
}

// UpgradeRelease handles PUT /helm/{cluster}/releases/{name} - upgrades an existing Helm release.
func (h *HelmHandler) UpgradeRelease(w http.ResponseWriter, r *http.Request) {
	if h.releaseManager == nil {
		writeError(w, http.StatusServiceUnavailable, "Helm release manager not initialized", nil)
		return
	}
	clusterID := chi.URLParam(r, "cluster")
	name := chi.URLParam(r, "name")

	req, ok := decodeJSON[helm.UpgradeRequest](w, r)
	if !ok {
		return
	}

	if name != "" {
		req.ReleaseName = name
	}
	if strings.TrimSpace(req.ReleaseName) == "" {
		writeError(w, http.StatusBadRequest, "release name is required", nil)
		return
	}
	if req.Namespace == "" {
		req.Namespace = getNamespaceQuery(r)
	}
	if req.Namespace == "" {
		req.Namespace = "default"
	}

	rel, err := h.releaseManager.UpgradeRelease(r.Context(), clusterID, *req)

	h.recordAudit(r, "upgrade", req.ReleaseName, clusterID, map[string]interface{}{
		"cluster":   clusterID,
		"namespace": req.Namespace,
		"chart":     req.Chart,
		"repo":      req.Repo,
		"version":   req.Version,
	}, err)

	if err != nil {
		if isK8sUnavailable(err) {
			writeK8sUnavailable(w)
			return
		}
		if strings.Contains(strings.ToLower(err.Error()), "not found") {
			writeError(w, http.StatusNotFound, "release not found", err)
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to upgrade release", err)
		return
	}

	writeJSON(w, http.StatusOK, rel)
}

type uninstallRequestBody struct {
	Namespace string `json:"namespace"`
}

// UninstallRelease handles DELETE /helm/{cluster}/releases/{name} - uninstalls a Helm release.
func (h *HelmHandler) UninstallRelease(w http.ResponseWriter, r *http.Request) {
	if h.releaseManager == nil {
		writeError(w, http.StatusServiceUnavailable, "Helm release manager not initialized", nil)
		return
	}
	clusterID := chi.URLParam(r, "cluster")
	name := chi.URLParam(r, "name")
	namespace := getNamespaceQuery(r)
	if namespace == "" {
		var req uninstallRequestBody
		if err := json.NewDecoder(r.Body).Decode(&req); err == nil && req.Namespace != "" {
			namespace = req.Namespace
		}
	}
	if namespace == "" {
		namespace = "default"
	}

	if name == "" {
		writeError(w, http.StatusBadRequest, "release name is required", nil)
		return
	}

	err := h.releaseManager.UninstallRelease(r.Context(), clusterID, name, namespace)

	h.recordAudit(r, "uninstall", name, clusterID, map[string]interface{}{
		"cluster":   clusterID,
		"namespace": namespace,
	}, err)

	if err != nil {
		if isK8sUnavailable(err) {
			writeK8sUnavailable(w)
			return
		}
		if strings.Contains(strings.ToLower(err.Error()), "not found") {
			writeError(w, http.StatusNotFound, "release not found", err)
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to uninstall release", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "success",
		"message": "release uninstalled successfully",
		"name":    name,
	})
}

type rollbackRequestBody struct {
	Revision  int    `json:"revision"`
	Namespace string `json:"namespace"`
}

// RollbackRelease handles POST /helm/{cluster}/releases/{name}/rollback - rolls back a Helm release to a previous revision.
func (h *HelmHandler) RollbackRelease(w http.ResponseWriter, r *http.Request) {
	if h.releaseManager == nil {
		writeError(w, http.StatusServiceUnavailable, "Helm release manager not initialized", nil)
		return
	}
	clusterID := chi.URLParam(r, "cluster")
	name := chi.URLParam(r, "name")

	req, ok := decodeJSON[rollbackRequestBody](w, r)
	if !ok {
		return
	}

	if name == "" {
		writeError(w, http.StatusBadRequest, "release name is required", nil)
		return
	}

	namespace := req.Namespace
	if namespace == "" {
		namespace = getNamespaceQuery(r)
	}
	if namespace == "" {
		namespace = "default"
	}

	err := h.releaseManager.RollbackRelease(r.Context(), clusterID, name, namespace, req.Revision)

	h.recordAudit(r, "rollback", name, clusterID, map[string]interface{}{
		"cluster":   clusterID,
		"namespace": namespace,
		"revision":  req.Revision,
	}, err)

	if err != nil {
		if isK8sUnavailable(err) {
			writeK8sUnavailable(w)
			return
		}
		if strings.Contains(strings.ToLower(err.Error()), "not found") {
			writeError(w, http.StatusNotFound, "release not found", err)
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to rollback release", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "success",
		"message": "release rolled back successfully",
		"name":    name,
	})
}

// GetReleaseHistory handles GET /helm/{cluster}/releases/{name}/history - gets release history.
func (h *HelmHandler) GetReleaseHistory(w http.ResponseWriter, r *http.Request) {
	if h.releaseManager == nil {
		writeError(w, http.StatusServiceUnavailable, "Helm release manager not initialized", nil)
		return
	}
	clusterID := chi.URLParam(r, "cluster")
	name := chi.URLParam(r, "name")
	namespace := getNamespaceQuery(r)

	if name == "" {
		writeError(w, http.StatusBadRequest, "release name is required", nil)
		return
	}

	history, err := h.releaseManager.GetReleaseHistory(r.Context(), clusterID, name, namespace)
	if err != nil {
		if isK8sUnavailable(err) {
			writeK8sUnavailable(w)
			return
		}
		if strings.Contains(strings.ToLower(err.Error()), "not found") {
			writeError(w, http.StatusNotFound, "release not found", err)
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get release history", err)
		return
	}
	writeJSON(w, http.StatusOK, history)
}

func (h *HelmHandler) recordAudit(r *http.Request, action, releaseName, clusterID string, details map[string]interface{}, opErr error) {
	if h.auditRepo == nil {
		return
	}
	userID := middleware.UserIDFromContext(r.Context())
	if userID == "" {
		userID = "system"
	}
	result := "success"
	if opErr != nil {
		result = "failure"
		if details == nil {
			details = make(map[string]interface{})
		}
		details["error"] = opErr.Error()
	}
	if err := h.auditRepo.RecordAction(r.Context(), userID, action, "helm_release", releaseName, releaseName, result, details, r.RemoteAddr, r.Header.Get("User-Agent")); err != nil {
		logger.Get().Error("failed to record helm audit action", zap.String("action", action), zap.String("release", releaseName), zap.Error(err))
	}
}
