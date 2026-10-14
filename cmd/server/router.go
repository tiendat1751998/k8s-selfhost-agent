package main

import (
	"github.com/go-chi/chi/v5"

	adapthttp "github.com/datdt/k8sselfhost/internal/adapter/http"
	"github.com/datdt/k8sselfhost/internal/pkg/health"
)

// newRouter creates a new chi router with health checks, WebSocket hub, and mounted platform routes.
func newRouter(healthHandler *health.Handler, wsHub *adapthttp.WSHub, platform *adapthttp.PlatformHandlers) *chi.Mux {
	return adapthttp.NewRouterWithPlatform(healthHandler, wsHub, platform)
}
