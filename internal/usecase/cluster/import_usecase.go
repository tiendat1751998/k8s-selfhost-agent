package cluster

import (
	"context"
	"fmt"
	"time"
	"crypto/sha256"
	"encoding/hex"

	"github.com/datdt/k8sselfhost/internal/domain/fleet"
	"github.com/datdt/k8sselfhost/internal/pkg/crypto"
	"go.uber.org/zap"
)

type DiscoveryPort interface {
	DiscoverResources(ctx context.Context, kubeconfig []byte) (map[string]interface{}, error)
	ValidateConnectivity(ctx context.Context, kubeconfig []byte) error
}

// TokenEncryptor abstracts credential encryption and decryption for cluster tokens.
type TokenEncryptor interface {
	Encrypt(plaintext string) (string, error)
	Decrypt(ciphertext string) (string, error)
}

type ImportUsecase struct {
	fleetRepo fleet.Repository
	discovery DiscoveryPort
	encryptor TokenEncryptor
	logger    *zap.Logger
}

type ImportOption func(*ImportUsecase)

// WithEncryptor configures a custom TokenEncryptor.
func WithEncryptor(encryptor TokenEncryptor) ImportOption {
	return func(u *ImportUsecase) {
		if encryptor != nil {
			u.encryptor = encryptor
		}
	}
}

func NewImportUsecase(fleetRepo fleet.Repository, discovery DiscoveryPort, logger *zap.Logger, opts ...ImportOption) *ImportUsecase {
	u := &ImportUsecase{
		fleetRepo: fleetRepo,
		discovery: discovery,
		encryptor: crypto.DefaultEncryptor{},
		logger:    logger,
	}
	for _, opt := range opts {
		opt(u)
	}
	return u
}

func (u *ImportUsecase) ImportCluster(ctx context.Context, id, name, group, region, provider string, kubeconfig []byte) (*fleet.Cluster, error) {
	if err := u.discovery.ValidateConnectivity(ctx, kubeconfig); err != nil {
		return nil, fmt.Errorf("cluster connectivity validation failed: %w", err)
	}

	resources, err := u.discovery.DiscoverResources(ctx, kubeconfig)
	if err != nil {
		u.logger.Warn("auto-discovery failed during import", zap.Error(err))
		resources = map[string]interface{}{}
	}

	hash := sha256.Sum256(kubeconfig)
	hashStr := hex.EncodeToString(hash[:])
	now := time.Now().UTC()

	encryptedToken, err := u.encryptor.Encrypt(string(kubeconfig))
	if err != nil {
		return nil, fmt.Errorf("encrypting cluster kubeconfig: %w", err)
	}

	cluster := &fleet.Cluster{
		ID:                  id,
		Name:                name,
		Group:               group,
		Region:              region,
		Provider:            provider,
		Status:              "active",
		Version:             "unknown",
		Nodes:               0,
		ImportMethod:        "kubeconfig",
		KubeconfigHash:      hashStr,
		EncryptedToken:      encryptedToken,
		HealthStatus:        "healthy",
		LastHealthCheck:     &now,
		DiscoveredResources: resources,
	}

	if v, ok := resources["version"].(string); ok {
		cluster.Version = v
	}
	if n, ok := resources["nodes_count"].(int); ok {
		cluster.Nodes = n
	}

	if err := u.fleetRepo.RegisterCluster(ctx, cluster); err != nil {
		return nil, fmt.Errorf("failed to register cluster: %w", err)
	}

	return cluster, nil
}
