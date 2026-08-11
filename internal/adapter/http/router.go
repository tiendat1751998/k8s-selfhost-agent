// Package http provides HTTP routing and handler setup.
package http

import (
	"encoding/json"
	"net/http"
	"net/http/pprof"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"

	mw "github.com/datdt/k8sselfhost/internal/adapter/http/middleware"
	"github.com/datdt/k8sselfhost/internal/pkg/health"
	"github.com/datdt/k8sselfhost/internal/pkg/logger"
)

// PlatformHandlers holds all handler dependencies for platform feature routes.
type PlatformHandlers struct {
	Dashboard     *Handler
	Notification  *NotificationHandler
	Automation    *AutomationHandler
	Compliance    *ComplianceHandler
	Runbook       *RunbookHandler
	Observability *ObservabilityHandler
	Timeline      *TimelineHandler
	Capacity      *CapacityHandler
	Drift         *DriftHandler
	Correlation   *CorrelationHandler
	Changes       *ChangeHandler
	Promotion     *PromotionHandler
	Explorer      *ExplorerHandler
	Tagging       *TaggingHandler
	Reporting     *ReportingHandler
	HealthCenter  *HealthCenterHandler
	Fleet         *FleetHandler
	Audit         *AuditHandler
	Docker        *DockerHandler
	AI            *AIHandler
	Auth          *AuthHandler
	Search        *SearchHandler
	Cost          *CostHandler
	Backup        *BackupHandler
	Agents        *AgentHandler
	Deployments   *DeploymentHandler
	Tenancy       *TenancyHandler
}

// NewRouter creates a new chi router with standard middleware and health endpoints.
func NewRouter(healthHandler *health.Handler) *chi.Mux {
	return NewRouterWithWS(healthHandler, nil, nil)
}

// NewRouterWithWS creates a new chi router with WebSocket hub and optional platform handlers.
func NewRouterWithWS(healthHandler *health.Handler, wsHub *WSHub, platform *PlatformHandlers) *chi.Mux {
	r := chi.NewRouter()

	// Standard middleware stack
	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(chimw.Recoverer)
	r.Use(chimw.Timeout(60 * time.Second))
	r.Use(chimw.Heartbeat("/ping"))
	r.Use(mw.StructuredLogger)
	r.Use(mw.CORS)
	r.Use(mw.Metrics)
	r.Use(mw.Tracing)
	r.Use(mw.SecurityHeaders)

	// Health endpoints
	r.Get("/healthz", healthHandler.ServeHTTP)
	r.Get("/readyz", healthHandler.ServeHTTP)
	r.Get("/livez", health.LivenessHandler())

	// Metrics endpoint
	r.Handle("/metrics", promhttp.Handler())

	// Debug/profiling endpoints (gated by env var for safety)
	if os.Getenv("K8S_ENABLE_PPROF") == "true" {
		r.Route("/debug/pprof", func(r chi.Router) {
			r.HandleFunc("/", pprof.Index)
			r.HandleFunc("/cmdline", pprof.Cmdline)
			r.HandleFunc("/profile", pprof.Profile)
			r.HandleFunc("/symbol", pprof.Symbol)
			r.HandleFunc("/trace", pprof.Trace)
			r.Handle("/heap", pprof.Handler("heap"))
			r.Handle("/goroutine", pprof.Handler("goroutine"))
			r.Handle("/allocs", pprof.Handler("allocs"))
			r.Handle("/block", pprof.Handler("block"))
			r.Handle("/mutex", pprof.Handler("mutex"))
			r.Handle("/threadcreate", pprof.Handler("threadcreate"))
		})
	}

	// WebSocket endpoint
	if wsHub != nil {
		r.With(mw.JWTAuthMiddleware).Get("/ws", wsHub.ServeWS)
	}

	// Unauthenticated API routes
	r.Route("/api/v1/auth", func(r chi.Router) {
		if platform != nil && platform.Auth != nil {
			r.Post("/login", platform.Auth.Login)
		}
	})

	// API routes
	r.Route("/api/v1", func(r chi.Router) {
		r.Use(mw.JWTAuthMiddleware)

		r.Get("/", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"service":"k8sselfhost","version":"0.1.0"}`))
		})

		r.Post("/telemetry", func(w http.ResponseWriter, r *http.Request) {
			var payload map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				http.Error(w, "invalid json body", http.StatusBadRequest)
				return
			}
			logger.Get().Info("Client Telemetry Received", zap.Any("payload", payload))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"status":"accepted"}`))
		})

		// Platform feature routes (nil-safe for standalone mode)
		if platform != nil {
			if platform.Dashboard != nil {
				platform.Dashboard.RegisterRoutes(r)
			}
			if platform.Notification != nil {
				r.Route("/notifications", platform.Notification.RegisterRoutes)
			}
			if platform.Automation != nil {
				r.Route("/automation", platform.Automation.RegisterRoutes)
			}
			if platform.Compliance != nil {
				r.Route("/compliance", platform.Compliance.RegisterRoutes)
			}
			if platform.Runbook != nil {
				r.Route("/runbooks", platform.Runbook.RegisterRoutes)
			}
			if platform.Observability != nil {
				r.Route("/observability", platform.Observability.RegisterRoutes)
			}
			if platform.Timeline != nil {
				r.Route("/timeline", platform.Timeline.RegisterRoutes)
			}
			if platform.Capacity != nil {
				r.Route("/capacity", platform.Capacity.RegisterRoutes)
			}
			if platform.Drift != nil {
				r.Route("/drift", platform.Drift.RegisterRoutes)
			}
			if platform.Correlation != nil {
				r.Route("/correlation", platform.Correlation.RegisterRoutes)
			}
			if platform.Changes != nil {
				r.Route("/changes", platform.Changes.RegisterRoutes)
			}
			if platform.Promotion != nil {
				r.Route("/promotions", platform.Promotion.RegisterRoutes)
			}
			if platform.Explorer != nil {
				r.Route("/explorer", platform.Explorer.RegisterRoutes)
			}
			if platform.Tagging != nil {
				r.Route("/tags", platform.Tagging.RegisterRoutes)
			}
			if platform.Reporting != nil {
				r.Route("/reports-center", platform.Reporting.RegisterRoutes)
			}
			if platform.HealthCenter != nil {
				r.Route("/health", platform.HealthCenter.RegisterRoutes)
			}
			if platform.Fleet != nil {
				r.Route("/fleet", platform.Fleet.RegisterRoutes)
			}
			if platform.Audit != nil {
				r.Route("/audit", platform.Audit.RegisterRoutes)
			}
			if platform.Docker != nil {
				r.Route("/docker", platform.Docker.RegisterRoutes)
			}
			if platform.AI != nil {
				r.Route("/ai", platform.AI.RegisterRoutes)
			}
			if platform.Search != nil {
				r.Get("/search", platform.Search.Search)
			}
			if platform.Cost != nil {
				r.Route("/cost", platform.Cost.RegisterRoutes)
			}
			if platform.Backup != nil {
				r.Route("/backup", platform.Backup.RegisterRoutes)
			}
			if platform.Agents != nil {
				r.Route("/agents", platform.Agents.RegisterRoutes)
			}
			if platform.Deployments != nil {
				r.Route("/deployments", platform.Deployments.RegisterRoutes)
			}
			if platform.Tenancy != nil {
				r.Route("/tenancy", platform.Tenancy.RegisterRoutes)
			}
		}
	})

	// Serve frontend static files
	frontendDir := findFrontendDir()
	if frontendDir != "" {
		fileServer := http.FileServer(http.Dir(frontendDir))
		r.Handle("/*", fileServer)
	}

	return r
}

// findFrontendDir locates the frontend directory relative to the binary.
func findFrontendDir() string {
	candidates := []string{
		"frontend",
		"../frontend",
		"../../frontend",
	}
	for _, dir := range candidates {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return dir
		}
	}
	return ""
}
