package main

import (
	"context"
	"fmt"
	"net/http"

	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/infrastructure/config"
)

func newHTTPServer(_ context.Context, cfg *config.Config, infra *infrastructure, services *appServices, _ *zap.Logger) *http.Server {
	router := newRouter(infra.health, services.wsHub, services.platformHandlers)

	return &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}
}

