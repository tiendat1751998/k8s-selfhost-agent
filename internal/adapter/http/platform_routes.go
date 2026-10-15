package http

import (
	"github.com/go-chi/chi/v5"

	mw "github.com/datdt/k8sselfhost/internal/adapter/http/middleware"
)

// RegisterPlatformRoutes registers all platform extension handlers onto the Chi router (under /api/v1).
func RegisterPlatformRoutes(r chi.Router, platform *PlatformHandlers) {
	if platform == nil {
		return
	}

	mutRoles := mw.RequireRolesForMutations("platform_admin", "tenant_admin", "operator")
	adminOnly := mw.RBACMiddleware("platform_admin")

	// 1. Helm Package Manager
	if platform.Helm != nil {
		r.With(mutRoles).Route("/helm", func(hr chi.Router) {
			platform.Helm.RegisterRoutes(hr)
			hr.Get("/releases", platform.Helm.ListReleases)
			hr.With(mutRoles).Post("/releases", platform.Helm.InstallRelease)
			hr.Get("/releases/{name}", platform.Helm.GetRelease)
			hr.With(mutRoles).Delete("/releases/{name}", platform.Helm.UninstallRelease)
			hr.With(mutRoles).Post("/releases/{name}/rollback", platform.Helm.RollbackRelease)
		})
	}

	// 2. Storage & Distributed Volumes (Platform Storage + SRE Storage Resiliency)
	sreStorage := (*StorageResiliencyHandler)(nil)
	if platform.SRE != nil {
		sreStorage = platform.SRE.StorageResiliency
		if sreStorage == nil {
			sreStorage = platform.SRE.StorageHealing
		}
	}
	if platform.Storage != nil || sreStorage != nil {
		r.With(mutRoles).Route("/storage", func(sr chi.Router) {
			if platform.Storage != nil {
				sr.Route("/volumes", platform.Storage.RegisterRoutes)
				sr.Get("/", platform.Storage.ListVolumes)
				sr.With(mutRoles).Post("/{name}/expand", platform.Storage.ExpandVolume)
				sr.With(mutRoles).Post("/{name}/snapshot", platform.Storage.CreateSnapshot)
			}
			if sreStorage != nil {
				sr.Route("/resiliency", sreStorage.RegisterRoutes)
			}
		})
	}

	// 3. Disaster Recovery (Platform DR + SRE DR Healing)
	if platform.DR != nil || (platform.SRE != nil && platform.SRE.DRHealing != nil) {
		r.With(mutRoles).Route("/dr", func(drRouter chi.Router) {
			if platform.DR != nil {
				platform.DR.RegisterRoutes(drRouter)
			}
			if platform.SRE != nil && platform.SRE.DRHealing != nil {
				platform.SRE.DRHealing.RegisterRoutes(drRouter)
			}
		})
	}

	// 4. GitOps & Declarative Workflows (Platform GitOps + SRE GitOpsSyncWave)
	if platform.GitOps != nil || (platform.SRE != nil && platform.SRE.GitOpsSyncWave != nil) {
		r.With(mutRoles).Route("/gitops", func(gr chi.Router) {
			if platform.GitOps != nil {
				platform.GitOps.RegisterRoutes(gr)
			}
			if platform.SRE != nil && platform.SRE.GitOpsSyncWave != nil {
				platform.SRE.GitOpsSyncWave.RegisterRoutes(gr)
			}
		})
	}

	// 5. Infrastructure as Code (IaC)
	if platform.IaC != nil {
		r.With(mutRoles).Route("/iac", platform.IaC.RegisterRoutes)
	}

	// 6. Telegram SRE Copilot
	if platform.Telegram != nil {
		r.With(mutRoles).Route("/telegram", platform.Telegram.RegisterRoutes)
	}

	// 7. Operations & Observability Extensions
	if platform.Dashboard != nil {
		r.With(mutRoles).Group(func(dashRouter chi.Router) { platform.Dashboard.RegisterRoutes(dashRouter) })
	}
	if platform.Notification != nil {
		r.With(mutRoles).Route("/notifications", platform.Notification.RegisterRoutes)
	}
	if platform.Automation != nil {
		r.With(mutRoles).Route("/automation", platform.Automation.RegisterRoutes)
	}
	if platform.Compliance != nil {
		r.With(mutRoles).Route("/compliance", platform.Compliance.RegisterRoutes)
	}
	if platform.Runbook != nil {
		r.With(mutRoles).Route("/runbooks", platform.Runbook.RegisterRoutes)
	}
	if platform.Observability != nil {
		r.With(mutRoles).Route("/observability", platform.Observability.RegisterRoutes)
	}
	if platform.Timeline != nil {
		r.With(mutRoles).Route("/timeline", platform.Timeline.RegisterRoutes)
	}
	if platform.Drift != nil {
		r.With(mutRoles).Route("/drift", platform.Drift.RegisterRoutes)
	}
	if platform.Correlation != nil {
		r.With(mutRoles).Route("/correlation", platform.Correlation.RegisterRoutes)
	}
	if platform.Changes != nil {
		r.With(mutRoles).Route("/changes", platform.Changes.RegisterRoutes)
	}
	if platform.Canary != nil {
		r.With(mutRoles).Route("/promotions/canary", platform.Canary.RegisterRoutes)
	}
	if platform.Promotion != nil {
		r.With(mutRoles).Route("/promotions", platform.Promotion.RegisterRoutes)
	}
	if platform.Tagging != nil {
		r.With(mutRoles).Route("/tags", platform.Tagging.RegisterRoutes)
	}
	if platform.Reporting != nil {
		r.With(mutRoles).Route("/reports-center", platform.Reporting.RegisterRoutes)
	}
	if platform.Fleet != nil {
		r.With(adminOnly).Route("/fleet", platform.Fleet.RegisterRoutes)
		r.With(mutRoles).Get("/clusters", platform.Fleet.ListClusters)
	}
	if platform.Audit != nil {
		r.With(adminOnly).Route("/audit", platform.Audit.RegisterRoutes)
	}
	if platform.Docker != nil {
		r.With(mutRoles).Route("/docker", platform.Docker.RegisterRoutes)
		r.With(mutRoles).Route("/hosts", platform.Docker.RegisterHostRoutes)
	}
	if platform.Overview != nil {
		r.With(mutRoles).Route("/overview", platform.Overview.RegisterRoutes)
		r.With(mutRoles).Route("/metrics", func(mr chi.Router) {
			mr.Get("/tps", platform.Overview.GetTPS)
		})
	}
	if platform.AI != nil {
		r.With(mutRoles).Route("/ai", platform.AI.RegisterRoutes)
	}
	if platform.Search != nil {
		r.Get("/search", platform.Search.Search)
	}
	if platform.Cost != nil {
		r.With(mutRoles).Route("/cost", platform.Cost.RegisterRoutes)
	}
	if platform.Backup != nil {
		r.With(mutRoles).Route("/backup", platform.Backup.RegisterRoutes)
	}
	if platform.Agents != nil {
		r.With(mutRoles).Route("/agents", platform.Agents.RegisterRoutes)
	}
	if platform.Tenancy != nil {
		r.With(adminOnly).Route("/tenancy", platform.Tenancy.RegisterRoutes)
	}
	if platform.Alert != nil {
		r.With(mutRoles).Route("/alerts", platform.Alert.RegisterRoutes)
		r.With(mutRoles).Route("/alertmanager", platform.Alert.RegisterAlertmanagerRoutes)
	}
	if platform.Cloud != nil {
		r.With(adminOnly).Route("/cloud", platform.Cloud.RegisterRoutes)
	}
	if platform.Settings != nil {
		r.With(adminOnly).Route("/settings", platform.Settings.RegisterRoutes)
		r.With(adminOnly).Route("/apikeys", func(ar chi.Router) {
			ar.Get("/", platform.Settings.ListAPIKeys)
			ar.Post("/", platform.Settings.CreateAPIKey)
			ar.Delete("/{id}", platform.Settings.DeleteAPIKey)
		})
	}
	if platform.Catalog != nil {
		r.With(mutRoles).Route("/catalog", platform.Catalog.RegisterRoutes)
	}
	if platform.Scaffolder != nil {
		r.With(mutRoles).Route("/scaffolder", platform.Scaffolder.RegisterRoutes)
	}
	if platform.Plugin != nil {
		r.With(mutRoles).Route("/plugins", platform.Plugin.RegisterRoutes)
	}
	if platform.Ecosystem != nil {
		r.With(mutRoles).Route("/ecosystem", platform.Ecosystem.RegisterRoutes)
	}

	// 8. Log Engine & Centralized Streaming
	if platform.CentralizedLogs != nil {
		r.With(mw.RequestBodyLimit(10 << 20)).With(mw.RequireRolesForMutations("platform_admin", "tenant_admin", "operator", "logs:write")).Route("/logs", func(sub chi.Router) {
			if platform.LogStream != nil {
				platform.CentralizedLogs.RegisterRoutes(skipStreamRouter{Router: sub})
				sub.Get("/stream", platform.LogStream.ServeHTTP)
			} else {
				platform.CentralizedLogs.RegisterRoutes(sub)
			}
		})
	} else if platform.LogStream != nil {
		r.Get("/logs/stream", platform.LogStream.ServeHTTP)
	}

	// 9. SRE Resilience Suites & Mesh
	if platform.SRE == nil {
		if platform.Mesh != nil {
			r.With(mutRoles).Route("/mesh", platform.Mesh.RegisterRoutes)
		}
		if platform.MessagingSRE != nil {
			r.With(mutRoles).Route("/messaging", platform.MessagingSRE.RegisterRoutes)
			r.With(mutRoles).Route("/clusters/{clusterId}/messaging", platform.MessagingSRE.RegisterRoutes)
		}
	} else {
		if platform.Mesh != nil && platform.SRE.Mesh == nil {
			platform.SRE.Mesh = platform.Mesh
		}
		if platform.MessagingSRE != nil && platform.SRE.Messaging == nil {
			platform.SRE.Messaging = platform.MessagingSRE
		}
		RegisterSRERoutes(r, platform.SRE)
	}
}
