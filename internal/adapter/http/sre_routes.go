// Package http provides HTTP routing and handler setup.
package http

import (
	"github.com/go-chi/chi/v5"

	mw "github.com/datdt/k8sselfhost/internal/adapter/http/middleware"
)

// SREResilienceHandlers holds all 30 SRE resilience handler dependencies.
type SREResilienceHandlers struct {
	StorageResiliency *StorageResiliencyHandler
	StorageHealing    *StorageResiliencyHandler // Alias for StorageResiliency
	CGroup            *CGroupHandler
	Vault             *JWKSHandler
	JWKS              *JWKSHandler // Alias for Vault
	Fencing           *FencingHandler
	SecretDrift       *SecretDriftHandler
	EBPFWatch         *EBPFWatchHandler
	NetGuard          *NetGuardHandler
	IngestGuard       *IngestGuardHandler
	RuntimeGuard      *RuntimeGuardHandler
	GPUWatch          *GPUWatchHandler
	DRHealing         *DRHealingHandler
	FinOpsGuard       *FinOpsGuardHandler
	Netscaler         *NetscalerHandler
	VFSWatchdog       *VFSWatchdogHandler
	K8sResilience     *K8sResilienceHandler
	DBSafeguard       *DBSafeguardHandler
	SecDefense        *SecDefenseHandler
	CloudResilience   *CloudResilienceHandler
	ServiceMesh       *ServiceMeshHandler
	Mesh              *MeshHandler
	NUMAMemory        *NUMAMemoryHandler
	Hardware          *HardwareHandler
	Crypto            *CryptoHandler
	GitOpsSyncWave    *GitOpsSyncWaveHandler
	RDMAWatch         *RDMAWatchHandler
	APFGuard          *APFGuardHandler
	TransitNet        *TransitNetHandler
	SPIFFEGuard       *SPIFFEGuardHandler
	Consensus         *ConsensusHandler
	WorkloadGuard     *WorkloadGuardHandler
	Messaging         *MessagingHandler
}

// RegisterSRERoutes mounts all 30 SRE resilience route groups onto the Chi router (scoped under /api/v1).
func RegisterSRERoutes(r chi.Router, handlers *SREResilienceHandlers) {
	if handlers == nil {
		return
	}

	mutRoles := mw.RequireRolesForMutations("platform_admin", "tenant_admin", "operator")

	// 1. Storage Resiliency & Healing
	storage := handlers.StorageResiliency
	if storage == nil {
		storage = handlers.StorageHealing
	}
	if storage != nil {
		r.With(mutRoles).Route("/clusters/{clusterId}/storage", storage.RegisterRoutes)
		r.With(mutRoles).Route("/storage/resiliency", storage.RegisterRoutes)
	}

	// 2. Kernel Resiliency: cgroup v2, NUMA memory, and socket netscaler
	if handlers.CGroup != nil || handlers.NUMAMemory != nil || handlers.Netscaler != nil {
		r.With(mutRoles).Route("/kernel", func(kr chi.Router) {
			if handlers.CGroup != nil {
				handlers.CGroup.RegisterRoutes(kr)
			}
			if handlers.NUMAMemory != nil {
				handlers.NUMAMemory.RegisterRoutes(kr)
			}
			if handlers.Netscaler != nil {
				kr.Route("/network", func(nr chi.Router) {
					nr.Get("/socket-metrics", handlers.Netscaler.GetSocketMetrics)
					nr.Post("/apply-tuning", handlers.Netscaler.ApplyTuning)
					nr.Get("/health", handlers.Netscaler.CheckHealth)
				})
			}
		})
	}
	if handlers.Netscaler != nil {
		r.With(mutRoles).Route("/netscaler", handlers.Netscaler.RegisterRoutes)
	}

	// 3. Vault Auto-Unseal & JWKS Shield
	vault := handlers.Vault
	if vault == nil {
		vault = handlers.JWKS
	}
	if vault != nil {
		r.With(mutRoles).Route("/auth", func(ar chi.Router) {
			ar.Get("/jwks/status", vault.GetJWKSStatus)
			ar.Post("/jwks/refresh", vault.RefreshJWKS)
			ar.Get("/vault/unseal-status", vault.GetVaultUnsealStatus)
		})
		r.With(mutRoles).Route("/vault", func(vr chi.Router) {
			vr.Get("/unseal-status", vault.GetVaultUnsealStatus)
		})
	}

	// 4. Multi-Datacenter Fencing / STONITH
	if handlers.Fencing != nil {
		r.With(mutRoles).Route("/clusters/{clusterId}", handlers.Fencing.RegisterRoutes)
		r.With(mutRoles).Route("/fencing", func(fr chi.Router) {
			fr.Post("/stonith", handlers.Fencing.ExecuteSTONITH)
			fr.Get("/readiness", handlers.Fencing.GetFailoverReadiness)
		})
	}

	// 5. Secret Store Drift
	if handlers.SecretDrift != nil {
		r.With(mutRoles).Route("/secrets/drift", handlers.SecretDrift.RegisterRoutes)
	}

	// 6. eBPF Runtime Security
	if handlers.EBPFWatch != nil {
		r.With(mutRoles).Route("/ebpf", handlers.EBPFWatch.RegisterRoutes)
	}

	// 7. Hybrid WAN MTU Black Hole (NetGuard)
	if handlers.NetGuard != nil {
		r.With(mutRoles).Route("/network", handlers.NetGuard.RegisterRoutes)
	}

	// 8. TSDB / ClickHouse Ingest Guard
	if handlers.IngestGuard != nil {
		r.With(mutRoles).Route("/analytics", handlers.IngestGuard.RegisterRoutes)
	}

	// 9. Containerd Inode & Runtime Guard
	if handlers.RuntimeGuard != nil {
		r.With(mutRoles).Route("/runtime", handlers.RuntimeGuard.RegisterRoutes)
	}

	// 10. GPU AI Workload & KV Cache
	if handlers.GPUWatch != nil {
		r.With(mutRoles).Route("/gpu", handlers.GPUWatch.RegisterRoutes)
	}

	// 11. Velero Snapshot Lock & Edge DR Healing
	if handlers.DRHealing != nil {
		r.With(mutRoles).Route("/dr", handlers.DRHealing.RegisterRoutes)
		r.With(mutRoles).Route("/clusters/{clusterId}/dr/healing", handlers.DRHealing.RegisterRoutes)
	}

	// 12. FinOps Cross-AZ & Orphan Reaper
	if handlers.FinOpsGuard != nil {
		r.With(mutRoles).Route("/finops", handlers.FinOpsGuard.RegisterRoutes)
	}

	// 13. VFS Inode & D-State Breaker
	if handlers.VFSWatchdog != nil {
		r.With(mutRoles).Route("/vfs", handlers.VFSWatchdog.RegisterRoutes)
	}

	// 14. K8s Webhook Deadlock, PLEG & APF Queue Guard
	if handlers.K8sResilience != nil || handlers.APFGuard != nil {
		r.With(mutRoles).Route("/k8s", func(kr chi.Router) {
			if handlers.K8sResilience != nil {
				handlers.K8sResilience.RegisterRoutes(kr)
			}
			if handlers.APFGuard != nil {
				handlers.APFGuard.RegisterRoutes(kr)
			}
		})
		r.With(mutRoles).Route("/clusters/{clusterId}/k8s/resilience", func(kr chi.Router) {
			if handlers.K8sResilience != nil {
				handlers.K8sResilience.RegisterRoutes(kr)
			}
			if handlers.APFGuard != nil {
				handlers.APFGuard.RegisterRoutes(kr)
			}
		})
	}

	// 15. Database XID & Kafka Poison Pill
	if handlers.DBSafeguard != nil {
		r.With(mutRoles).Route("/db", handlers.DBSafeguard.RegisterRoutes)
	}

	// 16. DevSecOps Supply Chain & IMDSv2
	if handlers.SecDefense != nil {
		r.With(mutRoles).Route("/security", handlers.SecDefense.RegisterRoutes)
	}

	// 17. Cloud Burst Credit, Spot Interruption & Transit Network
	if handlers.CloudResilience != nil || handlers.TransitNet != nil {
		r.With(mutRoles).Route("/cloud", func(cr chi.Router) {
			if handlers.CloudResilience != nil {
				handlers.CloudResilience.RegisterRoutes(cr)
			}
			if handlers.TransitNet != nil {
				cr.Route("/network", handlers.TransitNet.RegisterRoutes)
			}
		})
	}
	if handlers.TransitNet != nil {
		r.With(mutRoles).Route("/transitnet", handlers.TransitNet.RegisterRoutes)
	}

	// 18. Service Mesh SDS & Egress Shield: registered unified under /mesh in router.go to prevent duplicate mount panics.

	// 19. Bare-Metal Hardware & EDAC
	if handlers.Hardware != nil {
		r.With(mutRoles).Route("/hardware", handlers.Hardware.RegisterRoutes)
	}

	// 20. Enterprise PKI & HSM Pool (Crypto)
	if handlers.Crypto != nil {
		r.With(mutRoles).Route("/crypto", handlers.Crypto.RegisterRoutes)
	}

	// 21. GitOps Sync-Wave DAG
	if handlers.GitOpsSyncWave != nil {
		r.With(mutRoles).Route("/gitops", handlers.GitOpsSyncWave.RegisterRoutes)
	}

	// 22. RoCEv2 PFC Deadlock & GPU RDMA
	if handlers.RDMAWatch != nil {
		r.With(mutRoles).Route("/ai/rdma", handlers.RDMAWatch.RegisterRoutes)
	}

	// 23. SPIFFE/SPIRE Identity
	if handlers.SPIFFEGuard != nil {
		r.With(mutRoles).Route("/identity", handlers.SPIFFEGuard.RegisterRoutes)
	}

	// 24. Distributed Consensus & etcd Defrag
	if handlers.Consensus != nil {
		r.With(mutRoles).Route("/consensus", handlers.Consensus.RegisterRoutes)
	}

	// 25. OCI Immutable Digest & Ephemeral Storage
	if handlers.WorkloadGuard != nil {
		r.With(mutRoles).Route("/workload", handlers.WorkloadGuard.RegisterRoutes)
	}

	// 26. Messaging SRE
	if handlers.Messaging != nil {
		r.With(mutRoles).Route("/messaging", handlers.Messaging.RegisterRoutes)
		r.With(mutRoles).Route("/clusters/{clusterId}/messaging", handlers.Messaging.RegisterRoutes)
	}
}
