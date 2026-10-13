package cloudresilience_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	domain "github.com/datdt/k8sselfhost/internal/domain/cloudresilience"
	usecase "github.com/datdt/k8sselfhost/internal/usecase/cloudresilience"
	"go.uber.org/zap"
)

// FakeStorageClient implements usecase.StorageClient for unit testing without network I/O.
type FakeStorageClient struct {
	mu           sync.RWMutex
	volumes      map[string]*domain.StorageBurstStatus
	modifyErr    error
	describeErr  error
	modifyCalled bool
}

func (f *FakeStorageClient) GetVolumeBurstBalance(ctx context.Context, volumeID string) (*domain.StorageBurstStatus, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	if f.describeErr != nil {
		return nil, f.describeErr
	}
	vol, exists := f.volumes[volumeID]
	if !exists {
		return nil, errors.New("volume not found")
	}
	// Return a copy
	cp := *vol
	return &cp, nil
}

func (f *FakeStorageClient) ModifyVolumeToGP3(ctx context.Context, volumeID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.modifyCalled = true
	if f.modifyErr != nil {
		return f.modifyErr
	}
	vol, exists := f.volumes[volumeID]
	if !exists {
		return errors.New("volume not found")
	}
	vol.VolumeType = "gp3"
	vol.CurrentIOPS = 3000
	vol.BurstBalancePct = 100.0
	vol.NeedsUpgrade = false
	return nil
}

// FakeSpotMetadataClient implements usecase.SpotMetadataClient.
type FakeSpotMetadataClient struct {
	mu     sync.Mutex
	events []*domain.SpotTerminationEvent
	index  int
}

func (f *FakeSpotMetadataClient) PollTerminationNotice(ctx context.Context) (*domain.SpotTerminationEvent, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.index >= len(f.events) {
		return nil, nil // No notice
	}
	ev := f.events[f.index]
	f.index++
	return ev, nil
}

// FakeNodeDrainClient implements usecase.NodeDrainClient.
type FakeNodeDrainClient struct {
	mu              sync.Mutex
	cordonedNodes   map[string]bool
	drainedNodes    map[string]time.Duration
	respectPDB      bool
	skipDaemonSets  bool
	cordonErr       error
	drainErr        error
}

func newFakeNodeDrainClient() *FakeNodeDrainClient {
	return &FakeNodeDrainClient{
		cordonedNodes: make(map[string]bool),
		drainedNodes:  make(map[string]time.Duration),
	}
}

func (f *FakeNodeDrainClient) CordonNode(ctx context.Context, nodeName string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.cordonErr != nil {
		return f.cordonErr
	}
	f.cordonedNodes[nodeName] = true
	return nil
}

func (f *FakeNodeDrainClient) DrainPods(ctx context.Context, nodeName string, gracePeriod time.Duration, respectPDB bool, skipDaemonSets bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.drainErr != nil {
		return f.drainErr
	}
	f.drainedNodes[nodeName] = gracePeriod
	f.respectPDB = respectPDB
	f.skipDaemonSets = skipDaemonSets
	return nil
}

// FakeNATGatewayClient implements usecase.NATGatewayClient.
type FakeNATGatewayClient struct {
	mu        sync.RWMutex
	gateways  map[string]*domain.NATGatewayMetrics
	metricErr error
}

func (f *FakeNATGatewayClient) GetSNATMetrics(ctx context.Context, gatewayID string) (*domain.NATGatewayMetrics, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	if f.metricErr != nil {
		return nil, f.metricErr
	}
	m, ok := f.gateways[gatewayID]
	if !ok {
		return nil, errors.New("nat gateway not found")
	}
	cp := *m
	return &cp, nil
}

func TestResilienceManager_EBSBurstWatchAndUpgrade(t *testing.T) {
	ctx := context.Background()
	volID := "vol-0123456789abcdef"

	storageClient := &FakeStorageClient{
		volumes: map[string]*domain.StorageBurstStatus{
			volID: {
				VolumeID:        volID,
				VolumeType:      "gp2",
				BurstBalancePct: 15.0,
				CurrentIOPS:     1000,
				NeedsUpgrade:    true,
			},
		},
	}

	mgr := usecase.NewResilienceManager(
		usecase.WithStorageClient(storageClient),
		usecase.WithLogger(zap.NewNop()),
	)

	// Step 1: Check burst balance -> Should flag NeedsUpgrade: true
	status, err := mgr.CheckEBSBurstBalance(ctx, volID)
	if err != nil {
		t.Fatalf("unexpected error checking burst balance: %v", err)
	}
	if !status.NeedsUpgrade {
		t.Errorf("expected NeedsUpgrade=true for burst balance 15%%")
	}

	// Step 2: Trigger online upgrade to gp3 (baseline 3000 IOPS)
	if err := mgr.UpgradeVolumeToGP3(ctx, volID); err != nil {
		t.Fatalf("unexpected error upgrading volume: %v", err)
	}

	// Step 3: Check burst balance again -> Volume is now gp3 with 3000 IOPS and NeedsUpgrade=false
	upgradedStatus, err := mgr.CheckEBSBurstBalance(ctx, volID)
	if err != nil {
		t.Fatalf("unexpected error checking upgraded volume: %v", err)
	}
	if upgradedStatus.VolumeType != "gp3" {
		t.Errorf("expected volumeType=gp3, got %s", upgradedStatus.VolumeType)
	}
	if upgradedStatus.CurrentIOPS < 3000 {
		t.Errorf("expected baseline 3000 IOPS, got %d", upgradedStatus.CurrentIOPS)
	}
	if upgradedStatus.NeedsUpgrade {
		t.Errorf("expected NeedsUpgrade=false on gp3 volume")
	}
}

func TestResilienceManager_EBSBurstExhaustion(t *testing.T) {
	ctx := context.Background()
	volID := "vol-exhausted"

	storageClient := &FakeStorageClient{
		volumes: map[string]*domain.StorageBurstStatus{
			volID: {
				VolumeID:        volID,
				VolumeType:      "gp2",
				BurstBalancePct: 0.0,
				CurrentIOPS:     300,
				NeedsUpgrade:    true,
			},
		},
	}

	mgr := usecase.NewResilienceManager(
		usecase.WithStorageClient(storageClient),
	)

	_, err := mgr.CheckEBSBurstBalance(ctx, volID)
	if !errors.Is(err, domain.ErrCloudEBSBurstExhausted) {
		t.Fatalf("expected ErrCloudEBSBurstExhausted, got %v", err)
	}
}

func TestResilienceManager_SpotNoticeAndGracefulDrain(t *testing.T) {
	ctx := context.Background()
	nodeName := "node-spot-worker-1"
	instanceID := "i-0987654321fedcba"

	drainClient := newFakeNodeDrainClient()
	spotClient := &FakeSpotMetadataClient{
		events: []*domain.SpotTerminationEvent{
			{
				InstanceID: instanceID,
				ActionTime: time.Now().UTC().Add(120 * time.Second),
			},
		},
	}

	mgr := usecase.NewResilienceManager(
		usecase.WithSpotClient(spotClient),
		usecase.WithDrainClient(drainClient),
	)

	// Step 1: Listen for spot notices
	var receivedEvent domain.SpotTerminationEvent
	handler := func(ev domain.SpotTerminationEvent) {
		receivedEvent = ev
	}

	if err := mgr.ListenSpotTerminationNotices(ctx, handler); err != nil {
		t.Fatalf("unexpected error listening spot notices: %v", err)
	}

	if receivedEvent.InstanceID != instanceID {
		t.Fatalf("expected instance %s, got %s", instanceID, receivedEvent.InstanceID)
	}

	// Step 2: Drain node gracefully with 90s grace period (respecting PDB and skipping DaemonSets)
	err := mgr.DrainNodeGracefully(ctx, nodeName, 90*time.Second)
	if err != nil {
		t.Fatalf("unexpected error draining node: %v", err)
	}

	if !drainClient.cordonedNodes[nodeName] {
		t.Errorf("expected node %s to be cordoned", nodeName)
	}
	if drainClient.drainedNodes[nodeName] != 90*time.Second {
		t.Errorf("expected 90s grace period, got %v", drainClient.drainedNodes[nodeName])
	}
	if !drainClient.respectPDB {
		t.Errorf("expected drain to respect PDB")
	}
	if !drainClient.skipDaemonSets {
		t.Errorf("expected drain to skip DaemonSets")
	}
}

func TestResilienceManager_NATGatewaySNATMonitoring(t *testing.T) {
	ctx := context.Background()
	gwHealthy := "nat-healthy"
	gwSaturated := "nat-saturated"

	natClient := &FakeNATGatewayClient{
		gateways: map[string]*domain.NATGatewayMetrics{
			gwHealthy: {
				GatewayID:            gwHealthy,
				AllocatedIPs:         2,
				ActiveConnections:    50000,
				PortAllocationErrors: 0,
				IsSaturated:          false,
			},
			gwSaturated: {
				GatewayID:            gwSaturated,
				AllocatedIPs:         1,
				ActiveConnections:    64512,
				PortAllocationErrors: 120,
				IsSaturated:          true,
			},
		},
	}

	mgr := usecase.NewResilienceManager(
		usecase.WithNATClient(natClient),
	)

	// Test healthy gateway
	metrics, err := mgr.MonitorNATGatewaySNAT(ctx, gwHealthy)
	if err != nil {
		t.Fatalf("unexpected error monitoring healthy NAT gateway: %v", err)
	}
	if metrics.IsSaturated {
		t.Errorf("healthy gateway should not be marked saturated")
	}

	// Test saturated gateway
	satMetrics, err := mgr.MonitorNATGatewaySNAT(ctx, gwSaturated)
	if !errors.Is(err, domain.ErrNATGatewaySaturated) {
		t.Fatalf("expected ErrNATGatewaySaturated, got %v", err)
	}
	if satMetrics == nil || !satMetrics.IsSaturated {
		t.Errorf("expected IsSaturated=true on saturated gateway")
	}
}

func TestResilienceManager_HighConcurrencyMetrics(t *testing.T) {
	ctx := context.Background()
	volID := "vol-concurrent"
	gwID := "nat-concurrent"

	storageClient := &FakeStorageClient{
		volumes: map[string]*domain.StorageBurstStatus{
			volID: {
				VolumeID:        volID,
				VolumeType:      "gp2",
				BurstBalancePct: 50.0,
				CurrentIOPS:     1000,
			},
		},
	}
	natClient := &FakeNATGatewayClient{
		gateways: map[string]*domain.NATGatewayMetrics{
			gwID: {
				GatewayID:         gwID,
				AllocatedIPs:      1,
				ActiveConnections: 1000,
			},
		},
	}

	mgr := usecase.NewResilienceManager(
		usecase.WithStorageClient(storageClient),
		usecase.WithNATClient(natClient),
	)

	var wg sync.WaitGroup
	goroutines := 30
	iterations := 100

	wg.Add(goroutines * 2)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				_, _ = mgr.CheckEBSBurstBalance(ctx, volID)
			}
		}()

		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				_, _ = mgr.MonitorNATGatewaySNAT(ctx, gwID)
			}
		}()
	}

	wg.Wait()

	burstTracker, _, snatCounter := mgr.GetTelemetrySnapshots()
	expectedCalls := uint64(goroutines * iterations)
	if burstTracker.AuditCount != expectedCalls {
		t.Errorf("expected %d burst audits, got %d", expectedCalls, burstTracker.AuditCount)
	}
	if snatCounter.TotalQueries != expectedCalls {
		t.Errorf("expected %d SNAT queries, got %d", expectedCalls, snatCounter.TotalQueries)
	}
}
