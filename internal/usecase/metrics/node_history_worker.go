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
)

// SnapshotProvider supplies the latest system overview metrics.
type SnapshotProvider interface {
	GetLastSnapshot() *SystemOverview
}

// NodeHistoryWorker periodically consolidates node metrics into time-series rollups,
// triggers pre-crash snapshots on node degradation, and downsamples historical data.
type NodeHistoryWorker struct {
	snapshotProvider SnapshotProvider
	nodeMetricsRepo  nodemetrics.Repository
	incidentRepo     incident.Repository
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

func (w *NodeHistoryWorker) recordRollup(ctx context.Context) error {
	if w.snapshotProvider == nil || w.nodeMetricsRepo == nil {
		return nil
	}

	snap := w.snapshotProvider.GetLastSnapshot()
	if snap == nil || len(snap.Nodes) == 0 {
		return nil
	}

	now := time.Now().UTC()
	rollups := make([]nodemetrics.NodeMetricRollup, 0, len(snap.Nodes))

	w.mu.Lock()
	defer w.mu.Unlock()

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
		prevStat, hasPrev := w.prevNodeStatus[nodeID]
		if hasPrev && prevStat == "online" && (status == "offline" || status == "degraded") {
			w.triggerNodeAnomalyIncident(ctx, n, status, now)
		}
		w.prevNodeStatus[nodeID] = status

		rollup := nodemetrics.NodeMetricRollup{
			NodeID:         nodeID,
			NodeName:       n.NodeName,
			CPUPercent:     n.CPUPercent,
			CPUPeak:        peak,
			MemUsedBytes:   n.MemoryUsed,
			MemTotalBytes:  n.MemoryTotal,
			MemPercent:     n.MemoryPercent,
			DiskUsedBytes:  n.DiskUsed,
			DiskTotalBytes: n.DiskTotal,
			DiskPercent:    n.DiskPercent,
			RxBytesPerSec:  n.NetworkRxBytes,
			TxBytesPerSec:  n.NetworkTxBytes,
			ProcessCount:   n.Processes,
			ContainerCount: n.ContainerCount,
			Status:         status,
			Resolution:     "1m",
			RecordedAt:     now,
		}
		rollups = append(rollups, rollup)
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

	now := time.Now().UTC()
	olderThan7Days := now.Add(-7 * 24 * time.Hour)
	olderThan90Days := now.Add(-90 * 24 * time.Hour)

	if err := w.nodeMetricsRepo.DownsampleAndPrune(ctx, olderThan7Days, olderThan90Days); err != nil {
		w.logger.Warn("Failed to downsample and prune node historical rollups", zap.Error(err))
	} else {
		w.logger.Info("Successfully downsampled 7d rollups to 1h and pruned expired records")
	}
}
