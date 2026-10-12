package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	adapthttp "github.com/datdt/k8sselfhost/internal/adapter/http"
	domainGitops "github.com/datdt/k8sselfhost/internal/domain/gitops"
	gitproviderGitea "github.com/datdt/k8sselfhost/internal/infrastructure/gitprovider/gitea"
	gitproviderGithub "github.com/datdt/k8sselfhost/internal/infrastructure/gitprovider/github"
	gitproviderGitlab "github.com/datdt/k8sselfhost/internal/infrastructure/gitprovider/gitlab"
	"github.com/datdt/k8sselfhost/internal/infrastructure/helm"
	"github.com/datdt/k8sselfhost/internal/infrastructure/iac"
	"github.com/datdt/k8sselfhost/internal/infrastructure/telegram"
	usecaseDR "github.com/datdt/k8sselfhost/internal/usecase/dr"
	usecaseGitops "github.com/datdt/k8sselfhost/internal/usecase/gitops"
	usecaseSRE "github.com/datdt/k8sselfhost/internal/usecase/sre"
	usecaseStorage "github.com/datdt/k8sselfhost/internal/usecase/storage"
	"github.com/datdt/k8sselfhost/internal/pkg/health"
)

func init() {
	if os.Getenv("JWT_SECRET") == "" {
		_ = os.Setenv("JWT_SECRET", "this-is-a-test-secret-at-least-32-chars-long")
	}
}

func setupTestRouter() http.Handler {
	healthHandler := health.NewHandler(5 * time.Second)

	gitopsCtrl := usecaseGitops.NewController(nil, nil, nil)
	helmMgr := helm.NewReleaseManager(nil, nil)
	storageUc := usecaseStorage.NewVolumeUsecase(nil, nil, nil)
	drUc := usecaseDR.NewDRUsecase(nil, nil, nil)
	remediationCtrl := usecaseSRE.NewRemediationController(nil, nil, nil)
	tfRunner := iac.NewTerraformRunner("")
	ansibleRunner := iac.NewAnsibleRunner("")
	bot := telegram.NewSREBot(telegram.TelegramConfig{BotToken: "test-token"}, nil, nil, nil, nil)

	handlers := &adapthttp.PlatformHandlers{
		Helm:     adapthttp.NewHelmHandler(helmMgr, nil),
		Storage:  adapthttp.NewStorageHandler(storageUc),
		DR:       adapthttp.NewDRHandler(drUc, remediationCtrl, nil),
		GitOps:   adapthttp.NewGitOpsHandler(gitopsCtrl),
		IaC:      adapthttp.NewIaCHandler(tfRunner, ansibleRunner),
		Telegram: adapthttp.NewTelegramHandler(bot),
	}

	return newRouter(healthHandler, nil, handlers)
}

func TestRouter_MountedRoutes_UnauthorizedNotNotFound(t *testing.T) {
	router := setupTestRouter()

	endpoints := []struct {
		method string
		path   string
		desc   string
	}{
		{http.MethodGet, "/api/v1/helm/releases", "Helm releases route"},
		{http.MethodGet, "/api/v1/storage/volumes", "Storage volumes route"},
		{http.MethodGet, "/api/v1/dr/backups", "DR backups route"},
		{http.MethodGet, "/api/v1/gitops/prs", "GitOps pull requests route"},
		{http.MethodGet, "/api/v1/iac/status", "IaC status route"},
		{http.MethodGet, "/api/v1/telegram/status", "Telegram status route"},
	}

	for _, ep := range endpoints {
		t.Run(ep.desc, func(t *testing.T) {
			req := httptest.NewRequest(ep.method, ep.path, nil)
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			assert.NotEqualf(t, http.StatusNotFound, rec.Code,
				"expected %s %s to be mounted, but got 404 Not Found", ep.method, ep.path)
			assert.Equalf(t, http.StatusUnauthorized, rec.Code,
				"expected %s %s to reject unauthenticated request with 401 Unauthorized, got %d", ep.method, ep.path, rec.Code)
		})
	}

	t.Run("unmapped route returns 404", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/unmapped-endpoint-xyz", nil)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})
}

func TestGitOpsController_ProvidersRegistered(t *testing.T) {
	controller := usecaseGitops.NewController(nil, nil, nil)

	ghProvider := gitproviderGithub.NewProvider("test-gh-token")
	glProvider := gitproviderGitlab.NewProvider("test-gl-token", "https://gitlab.example.com")
	giteaProvider := gitproviderGitea.NewProvider("test-gitea-token", "https://gitea.example.com")

	controller.RegisterProvider(domainGitops.ProviderGitHub, ghProvider)
	controller.RegisterProvider(domainGitops.ProviderGitLab, glProvider)
	controller.RegisterProvider(domainGitops.ProviderGitea, giteaProvider)

	assert.True(t, controller.HasProvider(domainGitops.ProviderGitHub), "expected GitHub provider to be registered")
	assert.True(t, controller.HasProvider(domainGitops.ProviderGitLab), "expected GitLab provider to be registered")
	assert.True(t, controller.HasProvider(domainGitops.ProviderGitea), "expected Gitea provider to be registered")
}

func TestPlatformHandlers_WiringFieldsParity(t *testing.T) {
	gitopsCtrl := usecaseGitops.NewController(nil, nil, nil)
	helmMgr := helm.NewReleaseManager(nil, nil)
	storageUc := usecaseStorage.NewVolumeUsecase(nil, nil, nil)
	drUc := usecaseDR.NewDRUsecase(nil, nil, nil)
	remediationCtrl := usecaseSRE.NewRemediationController(nil, nil, nil)
	tfRunner := iac.NewTerraformRunner("")
	ansibleRunner := iac.NewAnsibleRunner("")
	bot := telegram.NewSREBot(telegram.TelegramConfig{BotToken: "test-token"}, nil, nil, nil, nil)

	handlers := &adapthttp.PlatformHandlers{
		Helm:     adapthttp.NewHelmHandler(helmMgr, nil),
		Storage:  adapthttp.NewStorageHandler(storageUc),
		DR:       adapthttp.NewDRHandler(drUc, remediationCtrl, nil),
		GitOps:   adapthttp.NewGitOpsHandler(gitopsCtrl),
		IaC:      adapthttp.NewIaCHandler(tfRunner, ansibleRunner),
		Telegram: adapthttp.NewTelegramHandler(bot),
	}

	require.NotNil(t, handlers.Helm, "Helm handler must be initialized")
	require.NotNil(t, handlers.Storage, "Storage handler must be initialized")
	require.NotNil(t, handlers.DR, "DR handler must be initialized")
	require.NotNil(t, handlers.GitOps, "GitOps handler must be initialized")
	require.NotNil(t, handlers.IaC, "IaC handler must be initialized")
	require.NotNil(t, handlers.Telegram, "Telegram handler must be initialized")
}
