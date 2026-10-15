// Package cloudresilience provides usecase orchestration and resilience algorithms
// for cloud burst balance conservation, spot interruption 120s drainage, and NAT Gateway SNAT shielding.
package cloudresilience

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"

	domain "github.com/datdt/k8sselfhost/internal/domain/cloudresilience"
)

// StorageClient defines the port for auditing and mutating cloud block storage volumes.
type StorageClient interface {
	GetVolumeBurstBalance(ctx context.Context, volumeID string) (*domain.StorageBurstStatus, error)
	ModifyVolumeToGP3(ctx context.Context, volumeID string) error
}

// SpotMetadataClient defines the port for polling cloud metadata spot interruption notices.
type SpotMetadataClient interface {
	PollTerminationNotice(ctx context.Context) (*domain.SpotTerminationEvent, error)
}

// NodeDrainClient defines the port for Kubernetes worker node cordon and graceful eviction.
type NodeDrainClient interface {
	CordonNode(ctx context.Context, nodeName string) error
	DrainPods(ctx context.Context, nodeName string, gracePeriod time.Duration, respectPDB bool, skipDaemonSets bool) error
}

// NATGatewayClient defines the port for querying NAT Gateway connection and SNAT metrics.
type NATGatewayClient interface {
	GetSNATMetrics(ctx context.Context, gatewayID string) (*domain.NATGatewayMetrics, error)
}

// Option configures the ResilienceManager.
type Option func(*ResilienceManager)

// WithStorageClient injects the StorageClient port.
func WithStorageClient(c StorageClient) Option {
	return func(m *ResilienceManager) {
		m.storageClient = c
	}
}

// WithSpotClient injects the SpotMetadataClient port.
func WithSpotClient(c SpotMetadataClient) Option {
	return func(m *ResilienceManager) {
		m.spotClient = c
	}
}

// WithDrainClient injects the NodeDrainClient port.
func WithDrainClient(c NodeDrainClient) Option {
	return func(m *ResilienceManager) {
		m.drainClient = c
	}
}

// WithNATClient injects the NATGatewayClient port.
func WithNATClient(c NATGatewayClient) Option {
	return func(m *ResilienceManager) {
		m.natClient = c
	}
}

// WithLogger injects the structured logger.
func WithLogger(l *zap.Logger) Option {
	return func(m *ResilienceManager) {
		if l != nil {
			m.logger = l
		}
	}
}

// ResilienceManager orchestrates EBS burst protection, Spot termination drains, and NAT Gateway shields.
type ResilienceManager struct {
	storageClient StorageClient
	spotClient    SpotMetadataClient
	drainClient   NodeDrainClient
	natClient     NATGatewayClient
	logger        *zap.Logger

	// Cache-line padded atomic trackers to eliminate false sharing under high-scale telemetry polling.
	burstTracker domain.PaddedBurstTracker
	spotTimer    domain.PaddedSpotTimer
	snatCounter  domain.PaddedSNATPortCounter

	// In-memory state tracking guarded by RWMutex
	mu               sync.RWMutex
	drainingNodes    map[string]time.Time
	managedVolumes   map[string]*domain.StorageBurstStatus
	monitoredGateways map[string]*domain.NATGatewayMetrics
}

// NewResilienceManager creates a new instance of ResilienceManager with configured options.
func NewResilienceManager(opts ...Option) *ResilienceManager {
	mgr := &ResilienceManager{
		logger:            zap.NewNop(),
		drainingNodes:     make(map[string]time.Time),
		managedVolumes:    make(map[string]*domain.StorageBurstStatus),
		monitoredGateways: make(map[string]*domain.NATGatewayMetrics),
	}

	for _, opt := range opts {
		opt(mgr)
	}

	return mgr
}

// RegisterVolume manually registers a volume status in the in-memory cache (useful for tests or standalone mode).
func (m *ResilienceManager) RegisterVolume(status *domain.StorageBurstStatus) error {
	if status == nil || strings.TrimSpace(status.VolumeID) == "" {
		return domain.ErrInvalidVolumeID
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.managedVolumes[status.VolumeID] = status
	return nil
}

// CheckEBSBurstBalance audits gp2 burst balance; flags NeedsUpgrade when balance < 20%;
// and detects full burst balance exhaustion (0%) to prevent critical database lockup.
func (m *ResilienceManager) CheckEBSBurstBalance(ctx context.Context, volumeID string) (*domain.StorageBurstStatus, error) {
	volID := strings.TrimSpace(volumeID)
	if volID == "" {
		return nil, domain.ErrInvalidVolumeID
	}

	atomic.AddUint64(&m.burstTracker.AuditCount, 1)

	var (
		status *domain.StorageBurstStatus
		err    error
	)

	if m.storageClient != nil {
		status, err = m.storageClient.GetVolumeBurstBalance(ctx, volID)
		if err != nil {
			return nil, fmt.Errorf("querying ebs burst balance for volume %s: %w", volID, err)
		}
	} else {
		// In-memory lookup
		m.mu.RLock()
		cached, ok := m.managedVolumes[volID]
		m.mu.RUnlock()
		if !ok || cached == nil {
			return nil, fmt.Errorf("volume %s not registered in resilience manager", volID)
		}
		status = cached
	}

	if status == nil {
		return nil, domain.ErrInvalidVolumeID
	}

	// Record atomic telemetry
	permille := uint64(status.BurstBalancePct * 10)
	atomic.StoreUint64(&m.burstTracker.BalancePermille, permille)
	if status.CurrentIOPS > 0 {
		atomic.StoreUint64(&m.burstTracker.CurrentIOPS, uint64(status.CurrentIOPS))
	}

	// Cache state
	m.mu.Lock()
	m.managedVolumes[volID] = status
	m.mu.Unlock()

	// Evaluate burst exhaustion (0%)
	if strings.EqualFold(status.VolumeType, "gp2") && status.BurstBalancePct <= 0.0 {
		atomic.AddUint64(&m.burstTracker.DepletionAlerts, 1)
		m.logger.Error("EBS gp2 burst balance exhausted; critical I/O collapse risk",
			zap.String("volume_id", volID),
			zap.Float64("burst_balance_pct", status.BurstBalancePct),
		)
		status.NeedsUpgrade = true
		return status, domain.ErrCloudEBSBurstExhausted
	}

	// Evaluate threshold (< 20%)
	if strings.EqualFold(status.VolumeType, "gp2") && status.BurstBalancePct < 20.0 {
		status.NeedsUpgrade = true
		m.logger.Warn("EBS gp2 burst balance below 20% safety threshold; recommend gp3 online migration",
			zap.String("volume_id", volID),
			zap.Float64("burst_balance_pct", status.BurstBalancePct),
			zap.Int64("current_iops", status.CurrentIOPS),
		)
	}

	return status, nil
}

// UpgradeVolumeToGP3 triggers online Elastic Volumes migration from gp2 to gp3 (baseline 3000 IOPS).
func (m *ResilienceManager) UpgradeVolumeToGP3(ctx context.Context, volumeID string) error {
	volID := strings.TrimSpace(volumeID)
	if volID == "" {
		return domain.ErrInvalidVolumeID
	}

	if m.storageClient != nil {
		if err := m.storageClient.ModifyVolumeToGP3(ctx, volID); err != nil {
			return fmt.Errorf("modifying volume %s to gp3: %w", volID, err)
		}
	}

	m.mu.Lock()
	vol, exists := m.managedVolumes[volID]
	if !exists {
		vol = &domain.StorageBurstStatus{
			VolumeID: volID,
		}
		m.managedVolumes[volID] = vol
	}
	vol.VolumeType = "gp3"
	vol.BurstBalancePct = 100.0
	vol.CurrentIOPS = 3000
	vol.NeedsUpgrade = false
	m.mu.Unlock()

	m.logger.Info("Successfully triggered online volume migration to gp3",
		zap.String("volume_id", volID),
		zap.Int("baseline_iops", 3000),
		zap.Int("baseline_throughput_mb_s", 125),
	)

	return nil
}

// ListenSpotTerminationNotices queries cloud metadata for spot interruption notices and invokes the handler.
func (m *ResilienceManager) ListenSpotTerminationNotices(ctx context.Context, handler func(event domain.SpotTerminationEvent)) error {
	if m.spotClient == nil {
		return nil
	}

	ev, err := m.spotClient.PollTerminationNotice(ctx)
	if err != nil {
		return fmt.Errorf("polling spot termination metadata: %w", err)
	}

	if ev != nil {
		atomic.AddUint64(&m.spotTimer.TotalNotices, 1)
		atomic.StoreUint64(&m.spotTimer.NoticeReceivedUnixSec, uint64(time.Now().Unix()))

		evCopy := *ev
		evCopy.DrainStarted = true

		if handler != nil {
			handler(evCopy)
		}
	}

	return nil
}

// DrainNodeGracefully cordons the node and evicts pods with a 90s grace period respecting PDB and skipping DaemonSets.
func (m *ResilienceManager) DrainNodeGracefully(ctx context.Context, nodeName string, gracePeriod time.Duration) error {
	name := strings.TrimSpace(nodeName)
	if name == "" {
		return fmt.Errorf("node name must not be empty")
	}

	if gracePeriod <= 0 {
		gracePeriod = 90 * time.Second
	}

	atomic.StoreUint64(&m.spotTimer.DrainDeadlineUnixSec, uint64(time.Now().Add(gracePeriod).Unix()))

	m.mu.Lock()
	m.drainingNodes[name] = time.Now().UTC()
	m.mu.Unlock()

	m.logger.Info("Initiating graceful spot node drain",
		zap.String("node_name", name),
		zap.Duration("grace_period", gracePeriod),
	)

	if m.drainClient != nil {
		// 1. Cordon node (mark unschedulable)
		if err := m.drainClient.CordonNode(ctx, name); err != nil {
			return fmt.Errorf("cordoning node %s: %w", name, err)
		}

		// 2. Evict pods respecting PDB and skipping DaemonSets
		if err := m.drainClient.DrainPods(ctx, name, gracePeriod, true, true); err != nil {
			return fmt.Errorf("evicting pods on node %s: %w", name, err)
		}
	}

	atomic.AddUint64(&m.spotTimer.SuccessfulDrains, 1)

	m.logger.Info("Successfully completed graceful spot node drain",
		zap.String("node_name", name),
	)

	return nil
}

// MonitorNATGatewaySNAT tracks active connections and port allocation errors against 64,512 port limit per IP.
func (m *ResilienceManager) MonitorNATGatewaySNAT(ctx context.Context, gatewayID string) (*domain.NATGatewayMetrics, error) {
	gwID := strings.TrimSpace(gatewayID)
	if gwID == "" {
		return nil, domain.ErrInvalidGatewayID
	}

	atomic.AddUint64(&m.snatCounter.TotalQueries, 1)

	var (
		metrics *domain.NATGatewayMetrics
		err     error
	)

	if m.natClient != nil {
		metrics, err = m.natClient.GetSNATMetrics(ctx, gwID)
		if err != nil {
			return nil, fmt.Errorf("querying nat gateway snat metrics for %s: %w", gwID, err)
		}
	} else {
		m.mu.RLock()
		cached, ok := m.monitoredGateways[gwID]
		m.mu.RUnlock()
		if ok {
			metrics = cached
		} else {
			metrics, err = domain.NewNATGatewayMetrics(gwID, 1, 0, 0)
			if err != nil {
				return nil, err
			}
		}
	}

	if metrics == nil {
		return nil, domain.ErrInvalidGatewayID
	}

	atomic.StoreUint64(&m.snatCounter.ActiveConnections, uint64(metrics.ActiveConnections))
	atomic.StoreUint64(&m.snatCounter.PortAllocationErrors, uint64(metrics.PortAllocationErrors))

	m.mu.Lock()
	m.monitoredGateways[gwID] = metrics
	m.mu.Unlock()

	capacity := int64(metrics.AllocatedIPs * domain.PortsPerNATIP)
	if metrics.ActiveConnections >= capacity || metrics.PortAllocationErrors > 0 {
		atomic.AddUint64(&m.snatCounter.SaturationAlerts, 1)
		metrics.IsSaturated = true
		m.logger.Warn("NAT Gateway SNAT port allocation saturated; connection drop hazard",
			zap.String("gateway_id", gwID),
			zap.Int64("active_connections", metrics.ActiveConnections),
			zap.Int64("total_capacity", capacity),
			zap.Int64("port_allocation_errors", metrics.PortAllocationErrors),
		)
		return metrics, domain.ErrNATGatewaySaturated
	}

	return metrics, nil
}

// GetTelemetrySnapshots retrieves a concurrent snapshot of the high-frequency telemetry counters.
func (m *ResilienceManager) GetTelemetrySnapshots() (domain.PaddedBurstTracker, domain.PaddedSpotTimer, domain.PaddedSNATPortCounter) {
	return domain.PaddedBurstTracker{
			BalancePermille: atomic.LoadUint64(&m.burstTracker.BalancePermille),
			CurrentIOPS:     atomic.LoadUint64(&m.burstTracker.CurrentIOPS),
			AuditCount:      atomic.LoadUint64(&m.burstTracker.AuditCount),
			DepletionAlerts: atomic.LoadUint64(&m.burstTracker.DepletionAlerts),
		},
		domain.PaddedSpotTimer{
			NoticeReceivedUnixSec: atomic.LoadUint64(&m.spotTimer.NoticeReceivedUnixSec),
			DrainDeadlineUnixSec:  atomic.LoadUint64(&m.spotTimer.DrainDeadlineUnixSec),
			TotalNotices:          atomic.LoadUint64(&m.spotTimer.TotalNotices),
			SuccessfulDrains:      atomic.LoadUint64(&m.spotTimer.SuccessfulDrains),
		},
		domain.PaddedSNATPortCounter{
			ActiveConnections:    atomic.LoadUint64(&m.snatCounter.ActiveConnections),
			PortAllocationErrors: atomic.LoadUint64(&m.snatCounter.PortAllocationErrors),
			SaturationAlerts:     atomic.LoadUint64(&m.snatCounter.SaturationAlerts),
			TotalQueries:         atomic.LoadUint64(&m.snatCounter.TotalQueries),
		}
}
