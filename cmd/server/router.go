package main

import (
	"github.com/go-chi/chi/v5"

	adapthttp "github.com/datdt/k8sselfhost/internal/adapter/http"
	mw "github.com/datdt/k8sselfhost/internal/adapter/http/middleware"
	"github.com/datdt/k8sselfhost/internal/pkg/health"
)

// newRouter creates a new chi router with health checks, WebSocket hub, and mounted platform routes.
func newRouter(healthHandler *health.Handler, wsHub *adapthttp.WSHub, platform *adapthttp.PlatformHandlers) *chi.Mux {
	r := adapthttp.NewRouterWithWS(healthHandler, wsHub, platform)
	mountPlatformRoutes(r, platform)
	return r
}

// mountPlatformRoutes mounts Helm, Storage, DR, GitOps, IaC, and Telegram routes on the Chi router.
func mountPlatformRoutes(r chi.Router, platform *adapthttp.PlatformHandlers) {
	if platform == nil {
		return
	}

	// 1. Helm routes (/api/v1/helm/*)
	r.Route("/api/v1/helm", func(helmRouter chi.Router) {
		helmRouter.Use(mw.JWTAuthMiddleware)
		if platform.Helm != nil {
			platform.Helm.RegisterRoutes(helmRouter)
			helmRouter.Get("/releases", platform.Helm.ListReleases)
			helmRouter.With(mw.RequireRolesForMutations("platform_admin", "tenant_admin")).Post("/releases", platform.Helm.InstallRelease)
			helmRouter.Get("/releases/{name}", platform.Helm.GetRelease)
			helmRouter.With(mw.RequireRolesForMutations("platform_admin", "tenant_admin")).Delete("/releases/{name}", platform.Helm.UninstallRelease)
			helmRouter.With(mw.RequireRolesForMutations("platform_admin", "tenant_admin")).Post("/releases/{name}/rollback", platform.Helm.RollbackRelease)
		}
	})

	// 2. Storage routes (/api/v1/storage/*)
	r.Route("/api/v1/storage", func(storageRouter chi.Router) {
		storageRouter.Use(mw.JWTAuthMiddleware)
		if platform.Storage != nil {
			storageRouter.Route("/volumes", platform.Storage.RegisterRoutes)
			storageRouter.Get("/", platform.Storage.ListVolumes)
			storageRouter.With(mw.RequireRolesForMutations("platform_admin", "tenant_admin")).Post("/{name}/expand", platform.Storage.ExpandVolume)
			storageRouter.With(mw.RequireRolesForMutations("platform_admin", "tenant_admin")).Post("/{name}/snapshot", platform.Storage.CreateSnapshot)
		}
	})

	// 3. Disaster Recovery routes (/api/v1/dr/*)
	r.Route("/api/v1/dr", func(drRouter chi.Router) {
		drRouter.Use(mw.JWTAuthMiddleware)
		if platform.DR != nil {
			platform.DR.RegisterRoutes(drRouter)
		}
	})

	// 4. GitOps routes (/api/v1/gitops/*)
	r.Route("/api/v1/gitops", func(gitopsRouter chi.Router) {
		gitopsRouter.Use(mw.JWTAuthMiddleware)
		if platform.GitOps != nil {
			platform.GitOps.RegisterRoutes(gitopsRouter)
		}
	})

	// 5. IaC routes (/api/v1/iac/*)
	r.Route("/api/v1/iac", func(iacRouter chi.Router) {
		iacRouter.Use(mw.JWTAuthMiddleware)
		if platform.IaC != nil {
			platform.IaC.RegisterRoutes(iacRouter)
		}
	})

	// 6. Telegram SRE Copilot routes (/api/v1/telegram/*)
	r.Route("/api/v1/telegram", func(telegramRouter chi.Router) {
		telegramRouter.Use(mw.JWTAuthMiddleware)
		if platform.Telegram != nil {
			platform.Telegram.RegisterRoutes(telegramRouter)
		}
	})
}
