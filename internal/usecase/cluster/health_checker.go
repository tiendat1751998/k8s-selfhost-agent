package cluster

import (
	"context"
	"time"

	"github.com/datdt/k8sselfhost/internal/domain/fleet"
	"github.com/datdt/k8sselfhost/internal/pkg/crypto"
	"go.uber.org/zap"
)

type HealthChecker struct {
	fleetRepo fleet.Repository
	discovery DiscoveryPort
	logger    *zap.Logger
}

func NewHealthChecker(fleetRepo fleet.Repository, discovery DiscoveryPort, logger *zap.Logger) *HealthChecker {
	return &HealthChecker{
		fleetRepo: fleetRepo,
		discovery: discovery,
		logger:    logger,
	}
}

func (h *HealthChecker) Start(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			h.logger.Info("health checker stopping")
			return
		case <-ticker.C:
			h.checkAll(ctx)
		}
	}
}

func (h *HealthChecker) checkAll(ctx context.Context) {
	clusters, err := h.fleetRepo.ListClusters(ctx)
	if err != nil {
		h.logger.Error("failed to list clusters for health check", zap.Error(err))
		return
	}

	for _, c := range clusters {
		if c.ImportMethod != "kubeconfig" {
			continue
		}

		go h.checkCluster(ctx, c)
	}
}

func (h *HealthChecker) checkCluster(ctx context.Context, c fleet.Cluster) {
	status := "healthy"
	now := time.Now().UTC()
	
	rawKubeconfig := c.EncryptedToken
	if decrypted, decErr := crypto.Decrypt(c.EncryptedToken); decErr == nil && decrypted != "" {
		rawKubeconfig = decrypted
	}

	if err := h.discovery.ValidateConnectivity(ctx, []byte(rawKubeconfig)); err != nil {
		h.logger.Warn("cluster health check failed", zap.String("id", c.ID), zap.Error(err))
		status = "unhealthy"
	}

	if err := h.fleetRepo.UpdateHealth(ctx, c.ID, status, now); err != nil {
		h.logger.Error("failed to update cluster health", zap.String("id", c.ID), zap.Error(err))
	}
}
