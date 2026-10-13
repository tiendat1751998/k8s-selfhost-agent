// Package messaging implements application usecases and automation engines
// for message queue reliability, rebalance storm mitigation, and kernel tuning.
package messaging

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	domain "github.com/datdt/k8sselfhost/internal/domain/messaging"
)

// KafkaGateway abstracts infrastructure calls to Apache Kafka Admin clients.
type KafkaGateway interface {
	DescribeConsumerGroups(ctx context.Context, clusterID string) ([]domain.KafkaGroupHealth, error)
	ApplyConsumerGroupPolicy(ctx context.Context, clusterID, groupID, assignor, instanceID string) error
}

// RabbitMQGateway abstracts infrastructure calls to RabbitMQ HTTP Management API.
type RabbitMQGateway interface {
	GetClusterHealth(ctx context.Context, clusterID string) (*domain.RabbitMQHealthReport, error)
	TriggerMemoryPurgeAndGC(ctx context.Context, clusterID string) error
	SetConnectionBackpressure(ctx context.Context, clusterID string, blocked bool) error
}

// KernelNodeGateway abstracts host node sysctl tuning execution.
type KernelNodeGateway interface {
	ApplySysctl(ctx context.Context, nodeName, key, value string) error
}

// SREManagerConfig holds operational parameters and thresholds for the SRE manager.
type SREManagerConfig struct {
	MaxConcurrentOps int
	OperationTimeout time.Duration
}

// DefaultSREManagerConfig provides production-safe concurrency bounds and timeouts.
func DefaultSREManagerConfig() SREManagerConfig {
	return SREManagerConfig{
		MaxConcurrentOps: 16,
		OperationTimeout: 10 * time.Second,
	}
}

// Manager implements domain.MessageQueueSREManager with bounded concurrency and telemetry.
type Manager struct {
	kafkaGateway    KafkaGateway
	rabbitmqGateway RabbitMQGateway
	kernelGateway   KernelNodeGateway
	metrics         *domain.PaddedThroughputMetric
	auditQueue      *domain.PaddedSREQueue
	semaphore       chan struct{}
	cfg             SREManagerConfig
	mu              sync.RWMutex
}

// NewManager creates a new Message Queue SRE Manager with injected infrastructure gateways.
func NewManager(
	kafkaGateway KafkaGateway,
	rabbitmqGateway RabbitMQGateway,
	kernelGateway KernelNodeGateway,
	cfg SREManagerConfig,
) *Manager {
	if cfg.MaxConcurrentOps <= 0 {
		cfg.MaxConcurrentOps = 16
	}
	if cfg.OperationTimeout <= 0 {
		cfg.OperationTimeout = 10 * time.Second
	}

	return &Manager{
		kafkaGateway:    kafkaGateway,
		rabbitmqGateway: rabbitmqGateway,
		kernelGateway:   kernelGateway,
		metrics:         domain.NewPaddedThroughputMetric(),
		auditQueue:      domain.NewPaddedSREQueue(4096),
		semaphore:       make(chan struct{}, cfg.MaxConcurrentOps),
		cfg:             cfg,
	}
}

// Metrics returns the cache-line padded operational metrics for observability.
func (m *Manager) Metrics() *domain.PaddedThroughputMetric {
	return m.metrics
}

// InspectKafkaConsumerGroups retrieves consumer group health and diagnoses rebalance status.
func (m *Manager) InspectKafkaConsumerGroups(ctx context.Context, clusterID string) ([]domain.KafkaGroupHealth, error) {
	if strings.TrimSpace(clusterID) == "" {
		return nil, domain.ErrInvalidClusterID
	}

	m.metrics.IncActiveOperations()
	defer m.metrics.DecActiveOperations()
	m.metrics.IncTotalInspections()

	ctx, cancel := context.WithTimeout(ctx, m.cfg.OperationTimeout)
	defer cancel()

	if m.kafkaGateway == nil {
		return nil, errors.New("kafka gateway not configured")
	}

	groups, err := m.kafkaGateway.DescribeConsumerGroups(ctx, clusterID)
	if err != nil {
		return nil, fmt.Errorf("inspecting kafka consumer groups for cluster %s: %w", clusterID, err)
	}

	return groups, nil
}

// EnforceKafkaCooperativePolicy mitigates rebalance storms by enforcing CooperativeStickyAssignor
// and static group membership (group.instance.id) to stop cascading rebalances.
func (m *Manager) EnforceKafkaCooperativePolicy(ctx context.Context, clusterID, groupID string) error {
	if strings.TrimSpace(clusterID) == "" {
		return domain.ErrInvalidClusterID
	}
	if strings.TrimSpace(groupID) == "" {
		return domain.ErrInvalidGroupID
	}

	select {
	case m.semaphore <- struct{}{}:
		defer func() { <-m.semaphore }()
	case <-ctx.Done():
		return ctx.Err()
	}

	m.metrics.IncActiveOperations()
	defer m.metrics.DecActiveOperations()

	ctx, cancel := context.WithTimeout(ctx, m.cfg.OperationTimeout)
	defer cancel()

	if m.kafkaGateway == nil {
		return errors.New("kafka gateway not configured")
	}

	staticInstanceID := fmt.Sprintf("%s-static-%d", groupID, time.Now().UnixNano())
	err := m.kafkaGateway.ApplyConsumerGroupPolicy(
		ctx,
		clusterID,
		groupID,
		domain.CooperativeStickyAssignor,
		staticInstanceID,
	)
	if err != nil {
		return fmt.Errorf("enforcing cooperative sticky policy on group %s: %w", groupID, err)
	}

	m.metrics.IncRebalanceStormsMitigated()
	_ = m.auditQueue.TryPush(domain.SREAuditRecord{
		Timestamp: time.Now().Unix(),
		Action:    "cooperative_sticky_enforced",
		Target:    groupID,
	})

	return nil
}

// AuditRabbitMQMemoryAndTopology checks memory high-watermarks and triggers backpressure if >85%.
func (m *Manager) AuditRabbitMQMemoryAndTopology(ctx context.Context, clusterID string) (*domain.RabbitMQHealthReport, error) {
	if strings.TrimSpace(clusterID) == "" {
		return nil, domain.ErrInvalidClusterID
	}

	m.metrics.IncActiveOperations()
	defer m.metrics.DecActiveOperations()
	m.metrics.IncTotalInspections()

	ctx, cancel := context.WithTimeout(ctx, m.cfg.OperationTimeout)
	defer cancel()

	if m.rabbitmqGateway == nil {
		return nil, errors.New("rabbitmq gateway not configured")
	}

	report, err := m.rabbitmqGateway.GetClusterHealth(ctx, clusterID)
	if err != nil {
		return nil, fmt.Errorf("auditing rabbitmq cluster %s: %w", clusterID, err)
	}

	// Automated rescue policy: if memory exceeds 85%, throttle publishers and trigger GC.
	if report.MemoryPercentage >= 85.0 {
		m.metrics.IncWatermarkRescuesTriggered()
		_ = m.rabbitmqGateway.SetConnectionBackpressure(ctx, clusterID, true)
		_ = m.rabbitmqGateway.TriggerMemoryPurgeAndGC(ctx, clusterID)
		_ = m.auditQueue.TryPush(domain.SREAuditRecord{
			Timestamp: time.Now().Unix(),
			Action:    "high_watermark_backpressure_gc_enacted",
			Target:    clusterID,
		})
	}

	return report, nil
}

// ApplyKafkaKernelTuning configures Linux sysctl dirty page ratios to prevent replication I/O stalls.
func (m *Manager) ApplyKafkaKernelTuning(ctx context.Context, nodeName string) error {
	if strings.TrimSpace(nodeName) == "" {
		return domain.ErrInvalidNodeName
	}

	select {
	case m.semaphore <- struct{}{}:
		defer func() { <-m.semaphore }()
	case <-ctx.Done():
		return ctx.Err()
	}

	m.metrics.IncActiveOperations()
	defer m.metrics.DecActiveOperations()

	ctx, cancel := context.WithTimeout(ctx, m.cfg.OperationTimeout)
	defer cancel()

	if m.kernelGateway == nil {
		return errors.New("kernel node gateway not configured")
	}

	profile := domain.DefaultKafkaKernelProfile()
	params := map[string]string{
		"vm.dirty_background_ratio": strconv.Itoa(profile.DirtyBackgroundRatio),
		"vm.dirty_ratio":            strconv.Itoa(profile.DirtyRatio),
		"vm.dirty_expire_centisecs": strconv.Itoa(profile.DirtyExpireCentisecs),
	}

	for key, val := range params {
		if err := m.kernelGateway.ApplySysctl(ctx, nodeName, key, val); err != nil {
			return fmt.Errorf("%w: failed to apply %s=%s on node %s: %v", domain.ErrKernelTuningExecutionFailed, key, val, nodeName, err)
		}
	}

	_ = m.auditQueue.TryPush(domain.SREAuditRecord{
		Timestamp: time.Now().Unix(),
		Action:    "kafka_kernel_dirty_pages_tuned",
		Target:    nodeName,
	})

	return nil
}
