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
	Overview      *OverviewHandler
	AI            *AIHandler
	Auth          *AuthHandler
	Search        *SearchHandler
	Cost          *CostHandler
	Backup        *BackupHandler
	Agents        *AgentHandler
	Deployments   *DeploymentHandler
	Tenancy       *TenancyHandler
	Alert         *AlertHandler
	K8s           *K8sResourceHandler
	K8sExec       *K8sExecHandler
	K8sLogs       *K8sLogsHandler
	Cloud         *CloudHandler
	Settings      *SettingsHandler
	Catalog       *CatalogHandler
	Scaffolder    *ScaffoldHandler
	Plugin        *PluginHandler
	Ecosystem     *EcosystemHandler
	LogStream     *LogStreamHandler
	Helm          *HelmHandler
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
	r.Use(mw.RequestBodyLimit(1 << 20))
	r.Use(mw.StructuredLogger)
	r.Use(mw.CORS)
	r.Use(mw.Metrics)
	r.Use(mw.RequestCounter)
	r.Use(mw.Tracing)
	r.Use(mw.SecurityHeaders)

	// Health endpoints
	r.Get("/healthz", healthHandler.ServeHTTP)
	r.Get("/readyz", healthHandler.ServeHTTP)
	r.Get("/livez", health.LivenessHandler())

	// Metrics endpoint (protected by METRICS_TOKEN when configured)
	r.With(mw.MetricsAuthMiddleware).Handle("/metrics", promhttp.Handler())

	// Debug/profiling endpoints (gated by env var for safety and protected with platform_admin role)
	if os.Getenv("K8S_ENABLE_PPROF") == "true" {
		r.Group(func(r chi.Router) {
			r.Use(mw.JWTAuthMiddleware)
			r.Use(mw.RBACMiddleware("platform_admin"))
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
		})
	}

	// WebSocket endpoint
	if wsHub != nil {
		r.With(mw.JWTAuthMiddleware).Get("/ws", wsHub.ServeWS)
	}

	// Auth API routes
	r.Route("/api/v1/auth", func(r chi.Router) {
		if platform != nil && platform.Auth != nil {
			r.Post("/login", platform.Auth.Login)
			r.Post("/verify-mfa", platform.Auth.VerifyMFA)
			r.Post("/refresh", platform.Auth.RefreshToken)
			r.Post("/recovery/verify", platform.Auth.VerifyRecoveryCode)
			r.Post("/logout", platform.Auth.Logout)

			// Authenticated TOTP routes
			r.Group(func(r chi.Router) {
				r.Use(mw.JWTAuthMiddleware)
				r.Post("/totp/setup", platform.Auth.SetupTOTP)
				r.Post("/totp/verify-setup", platform.Auth.VerifyTOTPSetup)
				r.Post("/totp/disable", platform.Auth.DisableTOTP)
				r.Get("/totp/status", platform.Auth.TOTPStatus)
			})
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

		// Kubernetes offline graceful handler helper
		k8sUnavailableHandler := func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error":   "Kubernetes cluster not connected or unconfigured",
				"code":    "K8S_UNAVAILABLE",
				"message": "Import a kubeconfig via Fleet to enable this feature",
			})
		}
		mountK8sUnavailable := func(r chi.Router, pattern string) {
			r.Route(pattern, func(sub chi.Router) {
				sub.HandleFunc("/", k8sUnavailableHandler)
				sub.HandleFunc("/*", k8sUnavailableHandler)
			})
		}

		// K8s-dependent routes: ALWAYS mounted. When handler is nil, return 503 Service Unavailable.
		if platform != nil && platform.Explorer != nil {
			r.With(mw.RequireRolesForMutations("platform_admin", "tenant_admin", "operator")).Route("/explorer", platform.Explorer.RegisterRoutes)
		} else {
			mountK8sUnavailable(r, "/explorer")
		}

		if platform != nil && platform.Deployments != nil {
			r.With(mw.RequireRolesForMutations("platform_admin", "tenant_admin", "operator")).Route("/deployments", platform.Deployments.RegisterRoutes)
		} else {
			mountK8sUnavailable(r, "/deployments")
		}

		if platform != nil && platform.Capacity != nil {
			r.Route("/capacity", platform.Capacity.RegisterRoutes)
		} else {
			mountK8sUnavailable(r, "/capacity")
		}

		if platform != nil && platform.HealthCenter != nil {
			r.Route("/health", platform.HealthCenter.RegisterRoutes)
		} else {
			mountK8sUnavailable(r, "/health")
		}

		if platform != nil && (platform.K8s != nil || platform.K8sExec != nil || platform.K8sLogs != nil) {
			r.Route("/k8s/{cluster}", func(sub chi.Router) {
				if platform.K8s != nil {
					sub.With(mw.RBACMiddleware("platform_admin")).Post("/apply", platform.K8s.ApplyYAML)
					sub.With(mw.RequireRolesForMutations("platform_admin", "tenant_admin", "operator")).Group(func(k8sSub chi.Router) {
						k8sSub.Get("/namespaces", platform.K8s.ListNamespaces)
						k8sSub.Post("/namespaces", platform.K8s.CreateNamespace)
						k8sSub.Delete("/namespaces/{name}", platform.K8s.DeleteNamespace)

						k8sSub.Route("/resources/{kind}", func(resSub chi.Router) {
							resSub.Get("/", platform.K8s.ListResources)
							resSub.Post("/", platform.K8s.CreateResource)
							resSub.Get("/{name}", platform.K8s.GetResource)
							resSub.Put("/{name}", platform.K8s.UpdateResource)
							resSub.Delete("/{name}", platform.K8s.DeleteResource)
						})

						k8sSub.Get("/events", platform.K8s.ListEvents)
						k8sSub.With(mw.RBACMiddleware("platform_admin", "tenant_admin")).Route("/nodes/{name}", func(nodeSub chi.Router) {
							nodeSub.Post("/cordon", platform.K8s.CordonNode)
							nodeSub.Post("/uncordon", platform.K8s.UncordonNode)
							nodeSub.Post("/drain", platform.K8s.DrainNode)
							nodeSub.Put("/taints", platform.K8s.UpdateNodeTaints)
							nodeSub.Put("/labels", platform.K8s.UpdateNodeLabels)
						})
					})
				}
				if platform.K8sExec != nil {
					sub.HandleFunc("/exec", platform.K8sExec.HandleExec)
					sub.HandleFunc("/exec/{pod}", platform.K8sExec.HandleExec)
					sub.HandleFunc("/pods/{pod}/exec", platform.K8sExec.HandleExec)
				}
				if platform.K8sLogs != nil {
					sub.Get("/logs", platform.K8sLogs.HandlePodLogs)
					sub.Get("/logs/{pod}", platform.K8sLogs.HandlePodLogs)
					sub.Get("/pods/{pod}/logs", platform.K8sLogs.HandlePodLogs)
				}
			})
		} else {
			mountK8sUnavailable(r, "/k8s")
		}

		// Other platform feature routes (nil-safe for standalone mode)
		if platform != nil {
			if platform.Dashboard != nil {
				platform.Dashboard.RegisterRoutes(r)
			}
			if platform.Notification != nil {
				r.With(mw.RequireRolesForMutations("platform_admin", "tenant_admin", "operator")).Route("/notifications", platform.Notification.RegisterRoutes)
			}
			if platform.Automation != nil {
				r.With(mw.RequireRolesForMutations("platform_admin", "tenant_admin", "operator")).Route("/automation", platform.Automation.RegisterRoutes)
			}
			if platform.Compliance != nil {
				r.With(mw.RequireRolesForMutations("platform_admin", "tenant_admin", "operator")).Route("/compliance", platform.Compliance.RegisterRoutes)
			}
			if platform.Runbook != nil {
				r.With(mw.RequireRolesForMutations("platform_admin", "tenant_admin", "operator")).Route("/runbooks", platform.Runbook.RegisterRoutes)
			}
			if platform.Observability != nil {
				r.With(mw.RequireRolesForMutations("platform_admin", "tenant_admin", "operator")).Route("/observability", platform.Observability.RegisterRoutes)
			}
			if platform.Timeline != nil {
				r.With(mw.RequireRolesForMutations("platform_admin", "tenant_admin", "operator")).Route("/timeline", platform.Timeline.RegisterRoutes)
			}
			if platform.Drift != nil {
				r.With(mw.RequireRolesForMutations("platform_admin", "tenant_admin", "operator")).Route("/drift", platform.Drift.RegisterRoutes)
			}
			if platform.Correlation != nil {
				r.With(mw.RequireRolesForMutations("platform_admin", "tenant_admin", "operator")).Route("/correlation", platform.Correlation.RegisterRoutes)
			}
			if platform.Changes != nil {
				r.With(mw.RequireRolesForMutations("platform_admin", "tenant_admin", "operator")).Route("/changes", platform.Changes.RegisterRoutes)
			}
			if platform.Promotion != nil {
				r.With(mw.RequireRolesForMutations("platform_admin", "tenant_admin", "operator")).Route("/promotions", platform.Promotion.RegisterRoutes)
			}
			if platform.Tagging != nil {
				r.With(mw.RequireRolesForMutations("platform_admin", "tenant_admin", "operator")).Route("/tags", platform.Tagging.RegisterRoutes)
			}
			if platform.Reporting != nil {
				r.With(mw.RequireRolesForMutations("platform_admin", "tenant_admin", "operator")).Route("/reports-center", platform.Reporting.RegisterRoutes)
			}
			if platform.Fleet != nil {
				r.With(mw.RBACMiddleware("platform_admin")).Route("/fleet", platform.Fleet.RegisterRoutes)
			}
			if platform.Audit != nil {
				r.Route("/audit", platform.Audit.RegisterRoutes)
			}
			if platform.Docker != nil {
				r.With(mw.RequireRolesForMutations("platform_admin", "tenant_admin", "operator")).Route("/docker", platform.Docker.RegisterRoutes)
			}
			if platform.Overview != nil {
				r.With(mw.RequireRolesForMutations("platform_admin", "tenant_admin", "operator")).Route("/overview", platform.Overview.RegisterRoutes)
			}
			if platform.AI != nil {
				r.With(mw.RequireRolesForMutations("platform_admin", "tenant_admin", "operator")).Route("/ai", platform.AI.RegisterRoutes)
			}
			if platform.Search != nil {
				r.Get("/search", platform.Search.Search)
			}
			if platform.Cost != nil {
				r.With(mw.RequireRolesForMutations("platform_admin", "tenant_admin", "operator")).Route("/cost", platform.Cost.RegisterRoutes)
			}
			if platform.Backup != nil {
				r.With(mw.RequireRolesForMutations("platform_admin", "tenant_admin", "operator")).Route("/backup", platform.Backup.RegisterRoutes)
			}
			if platform.Agents != nil {
				r.With(mw.RequireRolesForMutations("platform_admin", "tenant_admin", "operator")).Route("/agents", platform.Agents.RegisterRoutes)
			}
			if platform.Tenancy != nil {
				r.With(mw.RBACMiddleware("platform_admin")).Route("/tenancy", platform.Tenancy.RegisterRoutes)
			}
			if platform.Alert != nil {
				r.With(mw.RequireRolesForMutations("platform_admin", "tenant_admin", "operator")).Route("/alerts", platform.Alert.RegisterRoutes)
			}
			if platform.Cloud != nil {
				r.With(mw.RBACMiddleware("platform_admin")).Route("/cloud", platform.Cloud.RegisterRoutes)
			}
			if platform.Settings != nil {
				r.With(mw.RBACMiddleware("platform_admin")).Route("/settings", platform.Settings.RegisterRoutes)
			}
			if platform.Catalog != nil {
				r.With(mw.RequireRolesForMutations("platform_admin", "tenant_admin", "operator")).Route("/catalog", platform.Catalog.RegisterRoutes)
			}
			if platform.Scaffolder != nil {
				r.With(mw.RequireRolesForMutations("platform_admin", "tenant_admin", "operator")).Route("/scaffolder", platform.Scaffolder.RegisterRoutes)
			}
			if platform.Plugin != nil {
				r.With(mw.RequireRolesForMutations("platform_admin", "tenant_admin", "operator")).Route("/plugins", platform.Plugin.RegisterRoutes)
			}
			if platform.Ecosystem != nil {
				r.With(mw.RequireRolesForMutations("platform_admin", "tenant_admin", "operator")).Route("/ecosystem", platform.Ecosystem.RegisterRoutes)
			}
			if platform.LogStream != nil {
				r.Get("/logs/stream", platform.LogStream.ServeHTTP)
			}
			if platform.Helm != nil {
				r.With(mw.RequireRolesForMutations("platform_admin", "tenant_admin")).Route("/helm", platform.Helm.RegisterRoutes)
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
