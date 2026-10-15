package hardware_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	domain "github.com/datdt/k8sselfhost/internal/domain/hardware"
	usecase "github.com/datdt/k8sselfhost/internal/usecase/hardware"
	"go.uber.org/zap"
)

// fakeEDACReader implements domain.EDACReader for testing without mocks.
type fakeEDACReader struct {
	mu     sync.Mutex
	status []domain.ECCDIMMStatus
	err    error
}

func (f *fakeEDACReader) ReadDIMMErrors(ctx context.Context) ([]domain.ECCDIMMStatus, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	res := make([]domain.ECCDIMMStatus, len(f.status))
	copy(res, f.status)
	return res, nil
}

// fakeThermalReader implements domain.CPUThermalReader for testing.
type fakeThermalReader struct {
	mu     sync.Mutex
	status domain.CPUThermalStatus
	err    error
}

func (f *fakeThermalReader) ReadCPUThermal(ctx context.Context) (*domain.CPUThermalStatus, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	copyStatus := f.status
	return &copyStatus, nil
}

// fakePowerSupplyReader implements domain.PowerSupplyReader for testing.
type fakePowerSupplyReader struct {
	mu       sync.Mutex
	supplies []domain.PowerSupplyStatus
	err      error
}

func (f *fakePowerSupplyReader) ReadPowerSupplies(ctx context.Context) ([]domain.PowerSupplyStatus, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	res := make([]domain.PowerSupplyStatus, len(f.supplies))
	copy(res, f.supplies)
	return res, nil
}

// fakeNodeRemediator tracks remediation actions without mocks.
type fakeNodeRemediator struct {
	mu            sync.Mutex
	cordonedNodes []string
	labeledNodes  map[string]map[string]string
	taintedNodes  map[string]string
	drainedNodes  map[string]time.Duration
	evictedPods   map[string][]string
}

func newFakeRemediator() *fakeNodeRemediator {
	return &fakeNodeRemediator{
		labeledNodes: make(map[string]map[string]string),
		taintedNodes: make(map[string]string),
		drainedNodes: make(map[string]time.Duration),
		evictedPods:  make(map[string][]string),
	}
}

func (r *fakeNodeRemediator) CordonNode(ctx context.Context, nodeName string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cordonedNodes = append(r.cordonedNodes, nodeName)
	return nil
}

func (r *fakeNodeRemediator) LabelNode(ctx context.Context, nodeName string, key, value string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.labeledNodes[nodeName] == nil {
		r.labeledNodes[nodeName] = make(map[string]string)
	}
	r.labeledNodes[nodeName][key] = value
	return nil
}

func (r *fakeNodeRemediator) TaintNode(ctx context.Context, nodeName string, key, value, effect string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.taintedNodes[nodeName] = key + "=" + value + ":" + effect
	return nil
}

func (r *fakeNodeRemediator) DrainNode(ctx context.Context, nodeName string, gracePeriod time.Duration) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.drainedNodes[nodeName] = gracePeriod
	return nil
}

func (r *fakeNodeRemediator) EvictLatencyCriticalPods(ctx context.Context, nodeName string) ([]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	evicted := []string{"payment-gateway-pod-1", "order-processor-pod-4"}
	r.evictedPods[nodeName] = evicted
	return evicted, nil
}

// fakeRAPLController tracks power cap settings.
type fakeRAPLController struct {
	mu         sync.Mutex
	powerRatio float64
}

func (c *fakeRAPLController) SetRAPLPowerLimit(ctx context.Context, ratio float64) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.powerRatio = ratio
	return nil
}

func (c *fakeRAPLController) GetRAPLPowerLimit(ctx context.Context) (float64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.powerRatio, nil
}

// fakeAlertSink collects alerts.
type fakeAlertSink struct {
	mu     sync.Mutex
	alerts []domain.HardwareAlert
}

func (s *fakeAlertSink) EmitAlert(ctx context.Context, alert domain.HardwareAlert) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.alerts = append(s.alerts, alert)
	return nil
}

func TestHardwareManager_ECCMemoryErrorWatchdog_Surge(t *testing.T) {
	edac := &fakeEDACReader{
		status: []domain.ECCDIMMStatus{
			{
				SlotID:           "mc0/dimm0",
				CorrectableCount: 15,
				ErrorRatePerMin:  0.2, // ~12/hour
				IsDegraded:       false,
			},
			{
				SlotID:           "mc0/dimm1",
				CorrectableCount: 520,
				ErrorRatePerMin:  2.5, // 150/hour > 100/hour threshold
				IsDegraded:       false,
			},
		},
	}
	remediator := newFakeRemediator()
	alerts := &fakeAlertSink{}
	rapl := &fakeRAPLController{}
	thermal := &fakeThermalReader{status: domain.CPUThermalStatus{PackageTempC: 55, BaseFrequencyMHz: 3000, CurrentFrequencyMHz: 3000}}
	psu := &fakePowerSupplyReader{supplies: []domain.PowerSupplyStatus{{PSUID: "PSU0", IsPresent: true}, {PSUID: "PSU1", IsPresent: true}}}

	mgr := usecase.NewHardwareManager(usecase.Config{
		NodeName: "baremetal-node-01",
	}, edac, thermal, psu, remediator, rapl, alerts, zap.NewNop())

	ctx := context.Background()
	dimms, err := mgr.AuditECCMemoryErrors(ctx)
	if !errors.Is(err, domain.ErrECCMemoryCorrectableSurge) {
		t.Fatalf("expected ErrECCMemoryCorrectableSurge, got: %v", err)
	}

	if len(dimms) != 2 {
		t.Fatalf("expected 2 dimms, got %d", len(dimms))
	}
	if !dimms[1].IsDegraded {
		t.Errorf("expected dimm mc0/dimm1 to be marked degraded")
	}

	// Verify node cordon, label, and drain with 120s grace period
	remediator.mu.Lock()
	defer remediator.mu.Unlock()
	if len(remediator.cordonedNodes) == 0 || remediator.cordonedNodes[0] != "baremetal-node-01" {
		t.Errorf("node baremetal-node-01 was not cordoned: %+v", remediator.cordonedNodes)
	}
	if remediator.labeledNodes["baremetal-node-01"][domain.DegradedNodeLabelKey] != domain.DegradedNodeLabelValue {
		t.Errorf("degraded label was not applied: %+v", remediator.labeledNodes)
	}
	if remediator.drainedNodes["baremetal-node-01"] != domain.DefaultDrainGracePeriod {
		t.Errorf("drain grace period mismatch: got %v, expected %v",
			remediator.drainedNodes["baremetal-node-01"], domain.DefaultDrainGracePeriod)
	}

	// Verify P0 alert emitted
	alerts.mu.Lock()
	defer alerts.mu.Unlock()
	if len(alerts.alerts) == 0 {
		t.Fatal("expected P0 alert to be emitted")
	}
	if alerts.alerts[0].Severity != domain.SeverityP0 || alerts.alerts[0].Target != "mc0/dimm1" {
		t.Errorf("alert content mismatch: %+v", alerts.alerts[0])
	}
}

func TestHardwareManager_ECCMemoryErrorWatchdog_Healthy(t *testing.T) {
	edac := &fakeEDACReader{
		status: []domain.ECCDIMMStatus{
			{SlotID: "mc0/dimm0", CorrectableCount: 2, ErrorRatePerMin: 0.05},
		},
	}
	remediator := newFakeRemediator()
	alerts := &fakeAlertSink{}
	mgr := usecase.NewHardwareManager(usecase.Config{NodeName: "node-healthy"}, edac, nil, nil, remediator, nil, alerts, zap.NewNop())

	dimms, err := mgr.AuditECCMemoryErrors(context.Background())
	if err != nil {
		t.Fatalf("unexpected error for healthy ECC: %v", err)
	}
	if len(dimms) != 1 || dimms[0].IsDegraded {
		t.Errorf("unexpected dimms: %+v", dimms)
	}
	if len(remediator.cordonedNodes) != 0 {
		t.Errorf("expected no cordon on healthy node")
	}
}

func TestHardwareManager_CPUThermalWatchdog_Throttling(t *testing.T) {
	thermal := &fakeThermalReader{
		status: domain.CPUThermalStatus{
			PackageTempC:        91.0, // > 85°C
			BaseFrequencyMHz:    3600.0,
			CurrentFrequencyMHz: 1400.0, // < 50% of 3600 MHz
		},
	}
	remediator := newFakeRemediator()
	alerts := &fakeAlertSink{}

	mgr := usecase.NewHardwareManager(usecase.Config{NodeName: "node-hot"}, nil, thermal, nil, remediator, nil, alerts, zap.NewNop())

	status, err := mgr.AuditCPUThermalThrottling(context.Background())
	if !errors.Is(err, domain.ErrCPUThermalThrottled) {
		t.Fatalf("expected ErrCPUThermalThrottled, got: %v", err)
	}
	if !status.IsThrottled {
		t.Errorf("expected IsThrottled = true")
	}

	remediator.mu.Lock()
	defer remediator.mu.Unlock()
	expectedTaint := domain.ThermalThrottledTaintKey + "=true:" + domain.ThermalThrottledTaintEffect
	if remediator.taintedNodes["node-hot"] != expectedTaint {
		t.Errorf("taint mismatch: got %s, expected %s", remediator.taintedNodes["node-hot"], expectedTaint)
	}
	if len(remediator.evictedPods["node-hot"]) == 0 {
		t.Errorf("expected latency-critical pods to be evicted")
	}
}

func TestHardwareManager_PowerSupplyWatchdog_RedundancyLoss(t *testing.T) {
	psu := &fakePowerSupplyReader{
		supplies: []domain.PowerSupplyStatus{
			{PSUID: "PSU0", IsPresent: true, InputVoltage: 220, OutputPowerW: 400, RedundancyLost: true},
			{PSUID: "PSU1", IsPresent: false, InputVoltage: 0, OutputPowerW: 0, RedundancyLost: true},
		},
	}
	rapl := &fakeRAPLController{}
	alerts := &fakeAlertSink{}

	mgr := usecase.NewHardwareManager(usecase.Config{NodeName: "node-pdu"}, nil, nil, psu, nil, rapl, alerts, zap.NewNop())

	supplies, err := mgr.AuditPowerSupplyRedundancy(context.Background())
	if !errors.Is(err, domain.ErrPSURedundancyFailed) {
		t.Fatalf("expected ErrPSURedundancyFailed, got: %v", err)
	}
	if len(supplies) != 2 {
		t.Fatalf("expected 2 supplies, got %d", len(supplies))
	}

	// Verify RAPL power limit set to 70%
	rapl.mu.Lock()
	defer rapl.mu.Unlock()
	if rapl.powerRatio != domain.RAPLPowerCapRatio {
		t.Errorf("expected RAPL power limit ratio %f, got %f", domain.RAPLPowerCapRatio, rapl.powerRatio)
	}

	// Verify Datacenter Ops alert emitted
	alerts.mu.Lock()
	defer alerts.mu.Unlock()
	if len(alerts.alerts) == 0 {
		t.Fatal("expected Datacenter Ops alert to be emitted")
	}
}

func TestHardwareManager_ConcurrentAccessUnderLoad(t *testing.T) {
	thermal := &fakeThermalReader{status: domain.CPUThermalStatus{PackageTempC: 60, BaseFrequencyMHz: 3200, CurrentFrequencyMHz: 3200}}
	edac := &fakeEDACReader{status: []domain.ECCDIMMStatus{{SlotID: "dimm0", CorrectableCount: 5, ErrorRatePerMin: 0.1}}}
	psu := &fakePowerSupplyReader{supplies: []domain.PowerSupplyStatus{{PSUID: "psu0", IsPresent: true}}}
	remediator := newFakeRemediator()
	rapl := &fakeRAPLController{}
	alerts := &fakeAlertSink{}

	mgr := usecase.NewHardwareManager(usecase.Config{NodeName: "concurrent-node"}, edac, thermal, psu, remediator, rapl, alerts, zap.NewNop())

	const numGoroutines = 50
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	var opsCompleted uint64

	for i := 0; i < numGoroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			ctx := context.Background()
			switch idx % 3 {
			case 0:
				_, _ = mgr.AuditECCMemoryErrors(ctx)
			case 1:
				_, _ = mgr.AuditCPUThermalThrottling(ctx)
			case 2:
				_, _ = mgr.AuditPowerSupplyRedundancy(ctx)
			}
			atomic.AddUint64(&opsCompleted, 1)
		}(i)
	}

	wg.Wait()
	if opsCompleted != numGoroutines {
		t.Fatalf("expected %d ops, got %d", numGoroutines, opsCompleted)
	}
}

func TestHardwareManager_NilReaders_ReturnError(t *testing.T) {
	mgr := usecase.NewHardwareManager(usecase.Config{NodeName: "test-node"}, nil, nil, nil, nil, nil, nil, zap.NewNop())
	ctx := context.Background()

	_, err := mgr.AuditECCMemoryErrors(ctx)
	if err == nil {
		t.Error("expected error when edacReader is nil, got nil")
	}

	_, err = mgr.AuditCPUThermalThrottling(ctx)
	if err == nil {
		t.Error("expected error when thermalReader is nil, got nil")
	}

	_, err = mgr.AuditPowerSupplyRedundancy(ctx)
	if err == nil {
		t.Error("expected error when psuReader is nil, got nil")
	}
}
