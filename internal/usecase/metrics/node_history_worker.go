package metrics

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/datdt/k8sselfhost/internal/domain/incident"
	"github.com/datdt/k8sselfhost/internal/domain/nodemetrics"
	"github.com/datdt/k8sselfhost/internal/domain/provider/docker"
	"github.com/datdt/k8sselfhost/internal/pkg/tenancy"
)

// SnapshotProvider supplies the latest system overview metrics.
type SnapshotProvider interface {
	GetLastSnapshot() *SystemOverview
}

// ComputeHostLister lists active compute hosts.
type ComputeHostLister interface {
	ListAll(ctx context.Context) ([]docker.ComputeHost, error)
}

// NodeHistoryWorker periodically consolidates node metrics into time-series rollups,
// triggers pre-crash snapshots on node degradation, and downsamples historical data.
type NodeHistoryWorker struct {
	snapshotProvider SnapshotProvider
	nodeMetricsRepo  nodemetrics.Repository
	incidentRepo     incident.Repository
	computeHostRepo  ComputeHostLister
	logger           *zap.Logger

	rollupInterval time.Duration
	pruneInterval  time.Duration

	mu             sync.Mutex
	prevNodeStatus map[string]string
	peakCPUTracker map[string]float64
	stopCh         chan struct{}
	running        bool
}

// NodeHistoryWorkerOption configures the worker.
type NodeHistoryWorkerOption func(*NodeHistoryWorker)

// WithRollupInterval sets the interval for recording 1-minute historical metric rollups.
func WithRollupInterval(d time.Duration) NodeHistoryWorkerOption {
	return func(w *NodeHistoryWorker) {
		if d > 0 {
			w.rollupInterval = d
		}
	}
}

// WithPruneInterval sets the interval for downsampling and pruning old rollups.
func WithPruneInterval(d time.Duration) NodeHistoryWorkerOption {
	return func(w *NodeHistoryWorker) {
		if d > 0 {
			w.pruneInterval = d
		}
	}
}

// WithComputeHostLister configures the compute host lister to prevent phantom incidents.
func WithComputeHostLister(lister ComputeHostLister) NodeHistoryWorkerOption {
	return func(w *NodeHistoryWorker) {
		w.computeHostRepo = lister
	}
}

// NewNodeHistoryWorker creates a new NodeHistoryWorker instance.
func NewNodeHistoryWorker(
	sp SnapshotProvider,
	nmRepo nodemetrics.Repository,
	incRepo incident.Repository,
	logger *zap.Logger,
	opts ...NodeHistoryWorkerOption,
) *NodeHistoryWorker {
	if logger == nil {
		logger = zap.NewNop()
	}

	w := &NodeHistoryWorker{
		snapshotProvider: sp,
		nodeMetricsRepo:  nmRepo,
		incidentRepo:     incRepo,
		logger:           logger,
		rollupInterval:   1 * time.Minute,
		pruneInterval:    24 * time.Hour,
		prevNodeStatus:   make(map[string]string),
		peakCPUTracker:   make(map[string]float64),
		stopCh:           make(chan struct{}),
	}

	for _, opt := range opts {
		opt(w)
	}

	return w
}

// Start begins background metric rollup collection and pruning routines.
func (w *NodeHistoryWorker) Start(ctx context.Context) error {
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return nil
	}
	w.running = true
	w.stopCh = make(chan struct{})
	w.mu.Unlock()

	w.logger.Info("Starting Node History & Telemetry Rollup Worker",
		zap.Duration("rollup_interval", w.rollupInterval),
		zap.Duration("prune_interval", w.pruneInterval),
	)

	// Run initial rollup immediately
	w.recordRollup(ctx)

	go w.runRollupLoop(ctx)
	go w.runPruneLoop(ctx)

	return nil
}

// Stop terminates background worker goroutines cleanly.
func (w *NodeHistoryWorker) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.running {
		return
	}
	w.running = false
	close(w.stopCh)
	w.logger.Info("Node History Worker stopped")
}

func (w *NodeHistoryWorker) runRollupLoop(ctx context.Context) {
	ticker := time.NewTicker(w.rollupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stopCh:
			return
		case <-ticker.C:
			w.recordRollup(ctx)
		}
	}
}

func (w *NodeHistoryWorker) runPruneLoop(ctx context.Context) {
	ticker := time.NewTicker(w.pruneInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stopCh:
			return
		case <-ticker.C:
			w.pruneOldData(ctx)
		}
	}
}

// RecordRollupOnce allows manual or test invocation of a single rollup cycle.
func (w *NodeHistoryWorker) RecordRollupOnce(ctx context.Context) error {
	return w.recordRollup(ctx)
}

func (w *NodeHistoryWorker) isHostActive(ctx context.Context, nodeName, nodeID string) bool {
	if w.computeHostRepo == nil {
		return true
	}

	hosts, err := w.computeHostRepo.ListAll(ctx)
	if err != nil {
		w.logger.Warn("Failed to list compute hosts for anomaly guard, suppressing incident", zap.Error(err))
		return false
	}

	for _, h := range hosts {
		if strings.EqualFold(h.Name, nodeName) || strings.EqualFold(h.ID, nodeID) || strings.EqualFold(h.Name, nodeID) {
			return true
		}
	}

	return false
}

func (w *NodeHistoryWorker) recordRollup(ctx context.Context) error {
	if w.snapshotProvider == nil || w.nodeMetricsRepo == nil {
		return nil
	}

	// LES-009: Wrap context with platform admin context for background worker DB queries
	ctx = tenancy.WithPlatformAdminContext(ctx)

	snap := w.snapshotProvider.GetLastSnapshot()
	if snap == nil || len(snap.Nodes) == 0 {
		return nil
	}

	now := time.Now().UTC()
	rollups := make([]nodemetrics.NodeMetricRollup, 0, len(snap.Nodes))

	type anomalyTrigger struct {
		metrics NodeMetrics
		status  string
	}
	var anomalies []anomalyTrigger

	w.mu.Lock()
	for _, n := range snap.Nodes {
		nodeID := n.NodeID
		if nodeID == "" {
			nodeID = n.NodeName
		}
		if nodeID == "" {
			continue
		}

		// Track peak CPU within current collection window
		peak := n.CPUPercent
		if prevPeak, ok := w.peakCPUTracker[nodeID]; ok && prevPeak > peak {
			peak = prevPeak
		}
		w.peakCPUTracker[nodeID] = n.CPUPercent // reset to current for next window

		status := "online"
		if strings.EqualFold(n.Status, "down") || strings.EqualFold(n.Status, "offline") {
			status = "offline"
		} else if strings.EqualFold(n.Status, "degraded") || n.CPUPercent > 85.0 || n.MemoryPercent > 90.0 {
			status = "degraded"
		}

		// Sentinel check: detect node transition to offline or degraded
		// Skip anomaly/incident generation for synthetic nodes and hosts NOT in compute_hosts
		isSynthetic := n.Source == "host" || n.Role == "standalone" || strings.HasPrefix(n.NodeID, "host-")
		prevStat, hasPrev := w.prevNodeStatus[nodeID]
		if !isSynthetic && hasPrev && prevStat == "online" && (status == "offline" || status == "degraded") {
			if w.isHostActive(ctx, n.NodeName, n.NodeID) {
				anomalies = append(anomalies, anomalyTrigger{metrics: n, status: status})
			} else {
				w.logger.Debug("Skipping anomaly incident for unmanaged or phantom host", zap.String("node", n.NodeName))
			}
		}
		w.prevNodeStatus[nodeID] = status

		var diskDevs []nodemetrics.DiskIOStats
		if len(n.DiskDevices) > 0 {
			diskDevs = make([]nodemetrics.DiskIOStats, len(n.DiskDevices))
			for i, d := range n.DiskDevices {
				diskDevs[i] = nodemetrics.DiskIOStats{
					DeviceName:        d.DeviceName,
					ReadBytesPerSec:   d.ReadBytesPerSec,
					WriteBytesPerSec:  d.WriteBytesPerSec,
					ReadIOPS:          d.ReadIOPS,
					WriteIOPS:         d.WriteIOPS,
					AvgWaitMs:         d.AvgWaitMs,
					AvgRequestSizeKB:  d.AvgRequestSizeKB,
					CurrentQueueDepth: d.CurrentQueueDepth,
					IoUtilizationPct:  d.IoUtilizationPct,
					IsRootDevice:      d.IsRootDevice,
				}
			}
		}

		rollup := nodemetrics.NodeMetricRollup{
			NodeID:               nodeID,
			NodeName:             n.NodeName,
			CPUPercent:           n.CPUPercent,
			CPUPeak:              peak,
			MemUsedBytes:         n.MemoryUsed,
			MemTotalBytes:        n.MemoryTotal,
			MemPercent:           n.MemoryPercent,
			DiskUsedBytes:        n.DiskUsed,
			DiskTotalBytes:       n.DiskTotal,
			DiskPercent:          n.DiskPercent,
			DiskReadBytesPerSec:  n.DiskReadBytesPerSec,
			DiskWriteBytesPerSec: n.DiskWriteBytesPerSec,
			DiskReadIOPS:         n.DiskReadIOPS,
			DiskWriteIOPS:        n.DiskWriteIOPS,
			DiskAvgAwaitMs:       n.DiskAvgAwaitMs,
			DiskMaxIoUtilPct:     n.DiskMaxIoUtilPct,
			DiskDevices:          diskDevs,
			RxBytesPerSec:        n.NetworkRxBytes,
			TxBytesPerSec:        n.NetworkTxBytes,
			ProcessCount:         n.Processes,
			ContainerCount:       n.ContainerCount,
			Status:               status,
			Resolution:           "1m",
			RecordedAt:           now,
		}
		rollups = append(rollups, rollup)
	}
	w.mu.Unlock()

	for _, a := range anomalies {
		w.triggerNodeAnomalyIncident(ctx, a.metrics, a.status, now)
	}

	if len(rollups) > 0 {
		if err := w.nodeMetricsRepo.InsertBatch(ctx, rollups); err != nil {
			w.logger.Warn("Failed to persist node metric rollups", zap.Error(err))
			return err
		}
	}

	return nil
}

func (w *NodeHistoryWorker) triggerNodeAnomalyIncident(
	ctx context.Context,
	n NodeMetrics,
	status string,
	now time.Time,
) {
	if w.incidentRepo == nil {
		return
	}
	if n.Source == "host" || n.Role == "standalone" || strings.HasPrefix(n.NodeID, "host-") {
		return
	}

	// LES-009: Wrap context with platform admin context for incident creation
	ctx = tenancy.WithPlatformAdminContext(ctx)

	// Anomaly Worker Guard: NEVER generate incidents for phantom or unmanaged hostnames
	if !w.isHostActive(ctx, n.NodeName, n.NodeID) {
		w.logger.Warn("Blocked anomaly incident creation for unmanaged host", zap.String("node", n.NodeName))
		return
	}

	msg := fmt.Sprintf("Node %s entered %s state (CPU: %.1f%%, Memory: %.1f%%)",
		n.NodeName, status, n.CPUPercent, n.MemoryPercent,
	)

	severity := incident.SeverityHigh
	if status == "offline" {
		severity = incident.SeverityCritical
	}

	inc, err := incident.New("default-cluster", "infrastructure", n.NodeName, incident.TypeNodeNotReady, severity, msg)
	if err != nil {
		w.logger.Warn("Failed to create node anomaly incident entity", zap.Error(err))
		return
	}

	inc.AddRawData("node_id", n.NodeID)
	inc.AddRawData("node_name", n.NodeName)
	inc.AddRawData("pre_incident_cpu", fmt.Sprintf("%.2f%%", n.CPUPercent))
	inc.AddRawData("pre_incident_memory", fmt.Sprintf("%.2f GB / %.2f GB (%.1f%%)",
		float64(n.MemoryUsed)/(1024*1024*1024), float64(n.MemoryTotal)/(1024*1024*1024), n.MemoryPercent),
	)
	inc.AddRawData("pre_incident_disk", fmt.Sprintf("%.2f GB / %.2f GB (%.1f%%)",
		float64(n.DiskUsed)/(1024*1024*1024), float64(n.DiskTotal)/(1024*1024*1024), n.DiskPercent),
	)
	inc.AddRawData("container_count", fmt.Sprintf("%d", n.ContainerCount))

	if len(n.TopProcesses) > 0 {
		if procsJSON, err := json.Marshal(n.TopProcesses); err == nil {
			inc.AddRawData("top_processes", string(procsJSON))
		}
	}

	if err := w.incidentRepo.Create(ctx, inc); err != nil {
		w.logger.Warn("Failed to persist node anomaly incident", zap.String("node", n.NodeName), zap.Error(err))
	} else {
		w.logger.Info("Logged pre-crash node anomaly incident evidence", zap.String("node", n.NodeName), zap.String("incident_id", inc.ID))
	}
}

func (w *NodeHistoryWorker) pruneOldData(ctx context.Context) {
	if w.nodeMetricsRepo == nil {
		return
	}

	// LES-009: Wrap context with platform admin context for background worker pruning
	ctx = tenancy.WithPlatformAdminContext(ctx)

	now := time.Now().UTC()
	olderThan7Days := now.Add(-7 * 24 * time.Hour)
	olderThan90Days := now.Add(-90 * 24 * time.Hour)

	if err := w.nodeMetricsRepo.DownsampleAndPrune(ctx, olderThan7Days, olderThan90Days); err != nil {
		w.logger.Warn("Failed to downsample and prune node historical rollups", zap.Error(err))
	} else {
		w.logger.Info("Successfully downsampled 7d rollups to 1h and pruned expired records")
	}
}
