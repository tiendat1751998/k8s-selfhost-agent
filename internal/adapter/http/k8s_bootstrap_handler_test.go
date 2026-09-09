package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/datdt/k8sselfhost/internal/adapter/http/middleware"
	clusterDomain "github.com/datdt/k8sselfhost/internal/domain/cluster"
	infraK8s "github.com/datdt/k8sselfhost/internal/infrastructure/kubernetes"
	"github.com/datdt/k8sselfhost/internal/pkg/health"
	usecaseCluster "github.com/datdt/k8sselfhost/internal/usecase/cluster"
)

func TestK8sBootstrapHandler_Offline(t *testing.T) {
	handler := NewK8sBootstrapHandler(nil)
	r := chi.NewRouter()
	r.Route("/k8s/{cluster}", func(sub chi.Router) {
		sub.Get("/essentials", handler.GetEssentialsStatus)
		sub.Post("/bootstrap", handler.ExecuteBootstrap)
		sub.Get("/metrics/pods", handler.GetPodMetrics)
	})

	endpoints := []struct {
		method string
		path   string
	}{
		{"GET", "/k8s/local/essentials"},
		{"POST", "/k8s/local/bootstrap"},
		{"GET", "/k8s/local/metrics/pods"},
	}

	for _, ep := range endpoints {
		t.Run(ep.method+" "+ep.path, func(t *testing.T) {
			req := httptest.NewRequest(ep.method, ep.path, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, http.StatusServiceUnavailable, w.Code)
		})
	}
}

func TestK8sBootstrapHandler_Live(t *testing.T) {
	fakeClient := fake.NewSimpleClientset()
	repo := infraK8s.NewResourceRepoWithInterface(fakeClient, nil)
	uc := usecaseCluster.NewBootstrapUsecase(repo, nil, zap.NewNop())
	handler := NewK8sBootstrapHandler(uc)

	r := chi.NewRouter()
	r.Route("/k8s/{cluster}", func(sub chi.Router) {
		sub.Get("/essentials", handler.GetEssentialsStatus)
		sub.Post("/bootstrap", handler.ExecuteBootstrap)
		sub.Get("/metrics/pods", handler.GetPodMetrics)
	})

	t.Run("GetEssentialsStatus", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/k8s/local/essentials", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var status clusterDomain.ClusterEssentialsStatus
		err := json.Unmarshal(w.Body.Bytes(), &status)
		require.NoError(t, err)
		assert.Equal(t, "local", status.ClusterID)
		assert.False(t, status.Ready)
		assert.Len(t, status.Components, 3)
	})

	t.Run("ExecuteBootstrap", func(t *testing.T) {
		body, _ := json.Marshal(clusterDomain.BootstrapRequest{
			InstallMetricsServer: true,
			InstallStorageClass:  true,
		})
		req := httptest.NewRequest("POST", "/k8s/local/bootstrap", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var res clusterDomain.BootstrapResult
		err := json.Unmarshal(w.Body.Bytes(), &res)
		require.NoError(t, err)
		assert.True(t, res.Success)
		assert.Contains(t, res.Installed, "metrics-server")
		assert.Contains(t, res.Installed, "local-path-storageclass")
	})

	t.Run("GetPodMetrics", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/k8s/local/metrics/pods", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
		assert.Contains(t, w.Body.String(), "PodMetricsList")
	})
}

func TestRouter_K8sBootstrap_IntegrationAndRBAC(t *testing.T) {
	healthHandler := health.NewHandler(5 * time.Second)

	adminToken, err := middleware.GenerateJWT("admin-user", "platform_admin", "default-tenant")
	require.NoError(t, err)

	operatorToken, err := middleware.GenerateJWT("operator-user", "operator", "default-tenant")
	require.NoError(t, err)

	viewerToken, err := middleware.GenerateJWT("viewer-user", "viewer", "default-tenant")
	require.NoError(t, err)

	fakeClient := fake.NewSimpleClientset()
	repo := infraK8s.NewResourceRepoWithInterface(fakeClient, nil)
	uc := usecaseCluster.NewBootstrapUsecase(repo, nil, zap.NewNop())
	bootstrapHandler := NewK8sBootstrapHandler(uc)

	platform := &PlatformHandlers{
		K8sBootstrap: bootstrapHandler,
	}

	router := NewRouterWithWS(healthHandler, nil, platform)

	t.Run("Viewer can read essentials status", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/k8s/local/essentials", nil)
		req.Header.Set("Authorization", "Bearer "+viewerToken)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("Viewer can read pod metrics", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/k8s/local/metrics/pods", nil)
		req.Header.Set("Authorization", "Bearer "+viewerToken)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("Viewer is forbidden from executing bootstrap", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/k8s/local/bootstrap", bytes.NewBufferString(`{}`))
		req.Header.Set("Authorization", "Bearer "+viewerToken)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("Operator is forbidden from executing bootstrap", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/k8s/local/bootstrap", bytes.NewBufferString(`{}`))
		req.Header.Set("Authorization", "Bearer "+operatorToken)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusForbidden, rec.Code)
	})

	t.Run("Platform admin can execute bootstrap", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/k8s/local/bootstrap", bytes.NewBufferString(`{"install_metrics_server": true}`))
		req.Header.Set("Authorization", "Bearer "+adminToken)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
	})

	t.Run("Gracefully handles nil K8sBootstrap", func(t *testing.T) {
		nilPlatform := &PlatformHandlers{
			K8s: NewK8sResourceHandler(nil, nil),
		}
		nilRouter := NewRouterWithWS(healthHandler, nil, nilPlatform)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/k8s/local/essentials", nil)
		req.Header.Set("Authorization", "Bearer "+adminToken)
		rec := httptest.NewRecorder()
		nilRouter.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNotFound, rec.Code)
	})
}
