package slo

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/swarm"
	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/domain/observability"
)

// DockerClient defines the Docker API operations required for live health monitoring.
type DockerClient interface {
	ServiceList(ctx context.Context, options swarm.ServiceListOptions) ([]swarm.Service, error)
	TaskList(ctx context.Context, options swarm.TaskListOptions) ([]swarm.Task, error)
	ContainerList(ctx context.Context, options container.ListOptions) ([]container.Summary, error)
}

// SLOCollector periodically queries Docker Engine / Swarm to record real-time service health samples.
type SLOCollector struct {
	dockerClient DockerClient
	repo         observability.Repository
	logger       *zap.Logger
	interval     time.Duration
	tenantID     string
	stopCh       chan struct{}
	mu           sync.Mutex
}

// CollectorOption configures SLOCollector.
type CollectorOption func(*SLOCollector)

// WithCollectorInterval sets the sampling interval (default: 30s).
func WithCollectorInterval(d time.Duration) CollectorOption {
	return func(c *SLOCollector) {
		if d > 0 {
			c.interval = d
		}
	}
}

// WithCollectorTenantID sets the tenant ID for recorded samples.
func WithCollectorTenantID(tenantID string) CollectorOption {
	return func(c *SLOCollector) {
		if tenantID != "" {
			c.tenantID = tenantID
		}
	}
}

// NewSLOCollector creates a new real-time health sample collector.
func NewSLOCollector(dockerClient DockerClient, repo observability.Repository, logger *zap.Logger, opts ...CollectorOption) *SLOCollector {
	if logger == nil {
		logger = zap.NewNop()
	}
	c := &SLOCollector{
		dockerClient: dockerClient,
		repo:         repo,
		logger:       logger,
		interval:     30 * time.Second,
		tenantID:     "00000000-0000-0000-0000-000000000000",
		stopCh:       make(chan struct{}),
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Start begins periodic polling of Docker services and containers.
func (c *SLOCollector) Start(ctx context.Context) {
	c.logger.Info("Starting real SLO health sample collector", zap.Duration("interval", c.interval))

	// Initial collection immediately
	c.CollectOnce(ctx)

	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			c.logger.Info("Stopping SLO health sample collector (context cancelled)")
			return
		case <-c.stopCh:
			c.logger.Info("Stopping SLO health sample collector (stop requested)")
			return
		case <-ticker.C:
			c.CollectOnce(ctx)
		}
	}
}

// Stop terminates the collector background loop.
func (c *SLOCollector) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()
	select {
	case <-c.stopCh:
	default:
		close(c.stopCh)
	}
}

// CollectOnce polls Docker Swarm services and standalone containers to record point-in-time health samples.
func (c *SLOCollector) CollectOnce(ctx context.Context) {
	if c.dockerClient == nil || c.repo == nil {
		return
	}

	collectCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	start := time.Now()

	// 1. Collect Docker Swarm services
	swarmServicesCollected := make(map[string]bool)
	services, err := c.dockerClient.ServiceList(collectCtx, swarm.ServiceListOptions{})
	if err == nil && len(services) > 0 {
		tasks, taskErr := c.dockerClient.TaskList(collectCtx, swarm.TaskListOptions{})
		if taskErr != nil {
			c.logger.Debug("Failed to query swarm tasks for health sampling", zap.Error(taskErr))
		}

		for _, s := range services {
			desired := 1
			if s.Spec.Mode.Replicated != nil && s.Spec.Mode.Replicated.Replicas != nil {
				desired = int(*s.Spec.Mode.Replicated.Replicas)
			} else if s.Spec.Mode.Global != nil {
				desired = 1
			}

			running := 0
			if taskErr == nil {
				for _, t := range tasks {
					if t.ServiceID == s.ID && t.Status.State == swarm.TaskStateRunning {
						running++
					}
				}
			}

			isHealthy := (running >= desired && desired > 0) || (desired == 0 && running == 0)
			latencyMs := int(time.Since(start).Milliseconds())

			serviceName := s.Spec.Name
			swarmServicesCollected[serviceName] = true

			if recErr := c.repo.RecordHealthSample(collectCtx, c.tenantID, serviceName, desired, running, isHealthy, &latencyMs); recErr != nil {
				c.logger.Warn("Failed to record swarm service health sample", zap.String("service", serviceName), zap.Error(recErr))
			}
		}
	}

	// 2. Collect Standalone Containers
	containers, cErr := c.dockerClient.ContainerList(collectCtx, container.ListOptions{All: true})
	if cErr != nil {
		c.logger.Debug("Failed to list containers for health sampling", zap.Error(cErr))
		return
	}

	for _, cnt := range containers {
		// If part of swarm service already collected, skip to avoid duplicate sampling
		if swarmSvc, ok := cnt.Labels["com.docker.swarm.service.name"]; ok && swarmSvc != "" {
			if swarmServicesCollected[swarmSvc] {
				continue
			}
		}

		name := cnt.ID
		if len(cnt.Names) > 0 {
			name = strings.TrimPrefix(cnt.Names[0], "/")
		}

		desired := 1
		running := 0
		if cnt.State == "running" {
			running = 1
		}

		statusLower := strings.ToLower(cnt.Status)
		isHealthy := (running == 1 && !strings.Contains(statusLower, "unhealthy") && !strings.Contains(statusLower, "dead"))
		latencyMs := int(time.Since(start).Milliseconds())

		if recErr := c.repo.RecordHealthSample(collectCtx, c.tenantID, name, desired, running, isHealthy, &latencyMs); recErr != nil {
			c.logger.Debug("Failed to record container health sample", zap.String("container", name), zap.Error(recErr))
		}
	}
}
