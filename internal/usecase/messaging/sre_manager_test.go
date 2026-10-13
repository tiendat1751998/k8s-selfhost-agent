package messaging

import (
	"context"
	"errors"
	"sync"
	"testing"

	domain "github.com/datdt/k8sselfhost/internal/domain/messaging"
)

// In-memory test implementations for zero-mock, concurrency-safe testing.
type fakeKafkaGateway struct {
	mu            sync.Mutex
	groups        map[string][]domain.KafkaGroupHealth
	appliedPolicy map[string]string
	returnErr     error
}

func (f *fakeKafkaGateway) DescribeConsumerGroups(ctx context.Context, clusterID string) ([]domain.KafkaGroupHealth, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.returnErr != nil {
		return nil, f.returnErr
	}
	return f.groups[clusterID], nil
}

func (f *fakeKafkaGateway) ApplyConsumerGroupPolicy(ctx context.Context, clusterID, groupID, assignor, instanceID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.returnErr != nil {
		return f.returnErr
	}
	if f.appliedPolicy == nil {
		f.appliedPolicy = make(map[string]string)
	}
	f.appliedPolicy[groupID] = assignor
	return nil
}

type fakeRabbitMQGateway struct {
	mu           sync.Mutex
	report       *domain.RabbitMQHealthReport
	backpressure bool
	gcTriggered  bool
	returnErr    error
}

func (f *fakeRabbitMQGateway) GetClusterHealth(ctx context.Context, clusterID string) (*domain.RabbitMQHealthReport, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.returnErr != nil {
		return nil, f.returnErr
	}
	return f.report, nil
}

func (f *fakeRabbitMQGateway) TriggerMemoryPurgeAndGC(ctx context.Context, clusterID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.gcTriggered = true
	return nil
}

func (f *fakeRabbitMQGateway) SetConnectionBackpressure(ctx context.Context, clusterID string, blocked bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.backpressure = blocked
	return nil
}

type fakeKernelGateway struct {
	mu        sync.Mutex
	applied   map[string]string
	returnErr error
}

func (f *fakeKernelGateway) ApplySysctl(ctx context.Context, nodeName, key, value string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.returnErr != nil {
		return f.returnErr
	}
	if f.applied == nil {
		f.applied = make(map[string]string)
	}
	f.applied[key] = value
	return nil
}

func TestSREManager_KafkaInspectAndEnforce(t *testing.T) {
	ctx := context.Background()
	gh, _ := domain.NewKafkaGroupHealth("order-grp", "orders", 12, 12, 100, domain.RangeAssignor, false, 35)

	kafkaGw := &fakeKafkaGateway{
		groups: map[string][]domain.KafkaGroupHealth{
			"cluster-alpha": {*gh},
		},
	}
	rabbitGw := &fakeRabbitMQGateway{}
	kernelGw := &fakeKernelGateway{}

	mgr := NewManager(kafkaGw, rabbitGw, kernelGw, DefaultSREManagerConfig())

	// Inspect consumer groups
	groups, err := mgr.InspectKafkaConsumerGroups(ctx, "cluster-alpha")
	if err != nil {
		t.Fatalf("unexpected error inspecting groups: %v", err)
	}
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	if groups[0].Status != domain.KafkaGroupStatusRebalanceStorm {
		t.Errorf("expected rebalance storm status, got %s", groups[0].Status)
	}

	// Enforce cooperative sticky policy
	err = mgr.EnforceKafkaCooperativePolicy(ctx, "cluster-alpha", "order-grp")
	if err != nil {
		t.Fatalf("unexpected error enforcing policy: %v", err)
	}

	if kafkaGw.appliedPolicy["order-grp"] != domain.CooperativeStickyAssignor {
		t.Errorf("expected %s, got %s", domain.CooperativeStickyAssignor, kafkaGw.appliedPolicy["order-grp"])
	}
	if mgr.Metrics().RebalanceStormsMitigated() != 1 {
		t.Errorf("expected 1 mitigated storm in metrics, got %d", mgr.Metrics().RebalanceStormsMitigated())
	}
}

func TestSREManager_RabbitMQWatermarkRescue(t *testing.T) {
	ctx := context.Background()
	report, _ := domain.NewRabbitMQHealthReport("rabbit-prod", 90*1024, 100*1024, 5, 10, 2)

	rabbitGw := &fakeRabbitMQGateway{report: report}
	mgr := NewManager(&fakeKafkaGateway{}, rabbitGw, &fakeKernelGateway{}, DefaultSREManagerConfig())

	audited, err := mgr.AuditRabbitMQMemoryAndTopology(ctx, "rabbit-prod")
	if err != nil {
		t.Fatalf("unexpected error auditing rabbitmq: %v", err)
	}
	if audited.Status != domain.RabbitMQStatusHighWatermark {
		t.Errorf("expected high watermark, got %s", audited.Status)
	}

	if !rabbitGw.backpressure {
		t.Errorf("expected publisher backpressure to be applied")
	}
	if !rabbitGw.gcTriggered {
		t.Errorf("expected memory purge and GC to be triggered")
	}
	if mgr.Metrics().WatermarkRescuesTriggered() != 1 {
		t.Errorf("expected 1 watermark rescue in metrics, got %d", mgr.Metrics().WatermarkRescuesTriggered())
	}
}

func TestSREManager_KernelDirtyPageTuning(t *testing.T) {
	ctx := context.Background()
	kernelGw := &fakeKernelGateway{}
	mgr := NewManager(&fakeKafkaGateway{}, &fakeRabbitMQGateway{}, kernelGw, DefaultSREManagerConfig())

	err := mgr.ApplyKafkaKernelTuning(ctx, "node-worker-01")
	if err != nil {
		t.Fatalf("unexpected error applying kernel tuning: %v", err)
	}

	if kernelGw.applied["vm.dirty_background_ratio"] != "3" {
		t.Errorf("expected vm.dirty_background_ratio=3, got %s", kernelGw.applied["vm.dirty_background_ratio"])
	}
	if kernelGw.applied["vm.dirty_ratio"] != "10" {
		t.Errorf("expected vm.dirty_ratio=10, got %s", kernelGw.applied["vm.dirty_ratio"])
	}
	if kernelGw.applied["vm.dirty_expire_centisecs"] != "1000" {
		t.Errorf("expected vm.dirty_expire_centisecs=1000, got %s", kernelGw.applied["vm.dirty_expire_centisecs"])
	}
}

func TestSREManager_ValidationAndErrors(t *testing.T) {
	ctx := context.Background()
	mgr := NewManager(&fakeKafkaGateway{}, &fakeRabbitMQGateway{}, &fakeKernelGateway{}, DefaultSREManagerConfig())

	if _, err := mgr.InspectKafkaConsumerGroups(ctx, ""); !errors.Is(err, domain.ErrInvalidClusterID) {
		t.Errorf("expected ErrInvalidClusterID, got %v", err)
	}
	if err := mgr.EnforceKafkaCooperativePolicy(ctx, "c1", ""); !errors.Is(err, domain.ErrInvalidGroupID) {
		t.Errorf("expected ErrInvalidGroupID, got %v", err)
	}
	if err := mgr.ApplyKafkaKernelTuning(ctx, ""); !errors.Is(err, domain.ErrInvalidNodeName) {
		t.Errorf("expected ErrInvalidNodeName, got %v", err)
	}
}
