package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	dockerclient "github.com/docker/docker/client"
	"go.uber.org/zap"
	"k8s.io/client-go/kubernetes"

	domainDocker "github.com/datdt/k8sselfhost/internal/domain/provider/docker"
	"github.com/datdt/k8sselfhost/internal/infrastructure/config"
	infraK8s "github.com/datdt/k8sselfhost/internal/infrastructure/kubernetes"
	infraNats "github.com/datdt/k8sselfhost/internal/infrastructure/nats"
	"github.com/datdt/k8sselfhost/internal/infrastructure/postgres"
	infraDocker "github.com/datdt/k8sselfhost/internal/infrastructure/provider/docker"
	infraRedis "github.com/datdt/k8sselfhost/internal/infrastructure/redis"
	"github.com/datdt/k8sselfhost/internal/pkg/health"
	"github.com/datdt/k8sselfhost/internal/pkg/telemetry"
)

type infrastructure struct {
	tp           *telemetry.Provider
	pgClient     *postgres.Client
	redisClient  *infraRedis.Client
	cacheManager *infraRedis.CacheManager
	natsClient   *infraNats.Client
	k8sClient    *kubernetes.Clientset
	k8sAvailable bool
	dockerClient *dockerclient.Client
	dockerRepo   domainDocker.Repository
	health       *health.Handler
}

func initInfrastructure(ctx context.Context, cfg *config.Config, log *zap.Logger) (*infrastructure, error) {
	// Initialize telemetry
	tp, err := telemetry.Init(ctx, telemetry.Config{
		ServiceName:    cfg.Telemetry.ServiceName,
		ServiceVersion: cfg.Telemetry.ServiceVersion,
		OTLPEndpoint:   cfg.Telemetry.OTLPEndpoint,
		Environment:    cfg.Telemetry.Environment,
	})
	if err != nil {
		return nil, fmt.Errorf("initializing telemetry: %w", err)
	}

	// Connect to PostgreSQL
	pgClient, err := postgres.NewClient(ctx, cfg.Postgres)
	if err != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = tp.Shutdown(shutdownCtx)
		cancel()
		return nil, fmt.Errorf("connecting to postgres: %w", err)
	}

	// Connect to Redis
	redisClient, err := infraRedis.NewClient(ctx, cfg.Redis)
	if err != nil {
		pgClient.Close()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = tp.Shutdown(shutdownCtx)
		cancel()
		return nil, fmt.Errorf("connecting to redis: %w", err)
	}

	cacheManager := infraRedis.NewCacheManager(redisClient)

	// Connect to NATS
	natsClient, err := infraNats.NewClient(ctx, cfg.NATS)
	if err != nil {
		_ = redisClient.Close()
		pgClient.Close()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = tp.Shutdown(shutdownCtx)
		cancel()
		return nil, fmt.Errorf("connecting to nats: %w", err)
	}

	// Setup health checks
	healthHandler := health.NewHandler(5 * time.Second)
	healthHandler.Register("postgres", pgClient.HealthCheck)
	healthHandler.Register("redis", redisClient.HealthCheck)
	healthHandler.Register("nats", natsClient.HealthCheck)

	// Initialize K8s Client
	k8sClient, k8sAvailable := initK8sClient(log)

	// Initialize Docker Client
	dockerClient, dockerRepo := initDockerClient(cfg, log)

	return &infrastructure{
		tp:           tp,
		pgClient:     pgClient,
		redisClient:  redisClient,
		cacheManager: cacheManager,
		natsClient:   natsClient,
		k8sClient:    k8sClient,
		k8sAvailable: k8sAvailable,
		dockerClient: dockerClient,
		dockerRepo:   dockerRepo,
		health:       healthHandler,
	}, nil
}

func initK8sClient(log *zap.Logger) (*kubernetes.Clientset, bool) {
	kubeconfigPath := os.Getenv("KUBECONFIG")
	if kubeconfigPath == "" {
		if home := os.Getenv("USERPROFILE"); home != "" {
			kubeconfigPath = filepath.Join(home, ".kube", "config")
		} else if home = os.Getenv("HOME"); home != "" {
			kubeconfigPath = filepath.Join(home, ".kube", "config")
		}
	}
	k8sClient, err := infraK8s.NewClient(kubeconfigPath)
	if err != nil {
		log.Warn("failed to initialize Kubernetes client, falling back to cached state", zap.Error(err))
		return nil, false
	}
	return k8sClient, true
}

func initDockerClient(cfg *config.Config, log *zap.Logger) (*dockerclient.Client, domainDocker.Repository) {
	dockerClient, err := infraDocker.NewDockerClient(cfg.Docker.Host, cfg.Docker.Version)
	if err != nil {
		log.Warn("failed to initialize real docker client, docker features will be unavailable", zap.Error(err))
		return nil, nil
	}
	return dockerClient, infraDocker.NewDockerRepoWithClient(dockerClient)
}

func (i *infrastructure) close(log *zap.Logger) {
	if i.natsClient != nil {
		func() {
			defer func() {
				if p := recover(); p != nil {
					log.Error("panic closing nats", zap.Any("panic", p))
				}
			}()
			if closeErr := i.natsClient.Close(); closeErr != nil {
				log.Error("failed to close nats", zap.Error(closeErr))
			}
		}()
	}

	if i.redisClient != nil {
		func() {
			defer func() {
				if p := recover(); p != nil {
					log.Error("panic closing redis", zap.Any("panic", p))
				}
			}()
			if closeErr := i.redisClient.Close(); closeErr != nil {
				log.Error("failed to close redis", zap.Error(closeErr))
			}
		}()
	}

	if i.pgClient != nil {
		func() {
			defer func() {
				if p := recover(); p != nil {
					log.Error("panic closing postgres", zap.Any("panic", p))
				}
			}()
			i.pgClient.Close()
		}()
	}

	if i.tp != nil {
		func() {
			defer func() {
				if p := recover(); p != nil {
					log.Error("panic closing telemetry", zap.Any("panic", p))
				}
			}()
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if shutdownErr := i.tp.Shutdown(shutdownCtx); shutdownErr != nil {
				log.Error("failed to shutdown telemetry", zap.Error(shutdownErr))
			}
		}()
	}
}
