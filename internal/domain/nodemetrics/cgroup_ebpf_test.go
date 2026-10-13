package nodemetrics_test

import (
	"context"
	"sync"
	"testing"
	"time"
	"unsafe"

	"github.com/datdt/k8sselfhost/internal/domain/nodemetrics"
)

func TestCGroupV2MemoryEvent_ValidationAndPadding(t *testing.T) {
	evt := nodemetrics.CGroupV2MemoryEvent{
		ContainerID:       "container-123",
		PodName:           "payment-pod",
		Namespace:         "production",
		LowEvents:         10,
		HighEvents:        50,
		MaxEvents:         2,
		OOMKillEvents:     1,
		ThrottledDuration: 350 * time.Millisecond,
		UpdatedAt:         time.Now().UTC(),
	}

	if evt.ContainerID != "container-123" || evt.HighEvents != 50 {
		t.Fatalf("unexpected cgroup event data: %+v", evt)
	}

	// Verify cache-line padded counter structure isolates CPU cache lines
	counter := nodemetrics.PaddedKernelEventCounter{}
	if unsafe.Sizeof(counter) < 64 {
		t.Fatalf("PaddedKernelEventCounter must be at least 64 bytes to eliminate false sharing, got %d", unsafe.Sizeof(counter))
	}

	ebpfMetric := nodemetrics.PaddedEBPFMetric{}
	if unsafe.Sizeof(ebpfMetric) < 64 {
		t.Fatalf("PaddedEBPFMetric must be at least 64 bytes to prevent cache line contention, got %d", unsafe.Sizeof(ebpfMetric))
	}
}

func TestEBPFMapUtilization_StatusCalculation(t *testing.T) {
	tests := []struct {
		name       string
		current    int64
		max        int64
		wantStatus string
		wantRecom  int64
	}{
		{
			name:       "Normal usage",
			current:    200000,
			max:        512000,
			wantStatus: nodemetrics.MapStatusNormal,
			wantRecom:  512000,
		},
		{
			name:       "Warning usage between 70% and 80%",
			current:    380000,
			max:        512000, // ~74.2%
			wantStatus: nodemetrics.MapStatusWarning,
			wantRecom:  512000,
		},
		{
			name:       "Critical usage over 80%",
			current:    450000,
			max:        512000, // ~87.8%
			wantStatus: nodemetrics.MapStatusCritical,
			wantRecom:  1024000, // Doubled capacity
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			util := nodemetrics.NewEBPFMapUtilization("cilium_ct_any4_global", tc.current, tc.max)
			if util.Status != tc.wantStatus {
				t.Fatalf("expected status %s, got %s", tc.wantStatus, util.Status)
			}
			if util.RecommendedSize != tc.wantRecom {
				t.Fatalf("expected recommended size %d, got %d", tc.wantRecom, util.RecommendedSize)
			}
			if util.PressureRatio < 0.0 || util.PressureRatio > 1.0 {
				t.Fatalf("invalid pressure ratio %f", util.PressureRatio)
			}
		})
	}
}

func TestKernelEventRingBuffer_BoundedConcurrencyAndBackpressure(t *testing.T) {
	bufferSize := 16
	rb := nodemetrics.NewKernelEventRingBuffer(bufferSize)

	var wg sync.WaitGroup
	workers := 8
	itemsPerWorker := 10

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < itemsPerWorker; j++ {
				evt := nodemetrics.CGroupV2MemoryEvent{
					ContainerID: "cont-1",
					PodName:     "worker-pod",
					HighEvents:  int64(j),
					UpdatedAt:   time.Now(),
				}
				rb.Push(evt)
			}
		}(i)
	}

	wg.Wait()

	totalAccepted := rb.TotalAccepted()
	totalDropped := rb.TotalDropped()

	if totalAccepted+totalDropped != int64(workers*itemsPerWorker) {
		t.Fatalf("sum of accepted (%d) and dropped (%d) must match total pushed (%d)",
			totalAccepted, totalDropped, workers*itemsPerWorker)
	}

	// Drain ring buffer
	drained := rb.Drain()
	if len(drained) > bufferSize {
		t.Fatalf("drained items (%d) exceeded ring buffer capacity (%d)", len(drained), bufferSize)
	}
}

type mockKernelManager struct {
	events []nodemetrics.CGroupV2MemoryEvent
	maps   []nodemetrics.EBPFMapUtilization
	err    error
}

func (m *mockKernelManager) AuditCGroupV2Events(ctx context.Context, nodeName string) ([]nodemetrics.CGroupV2MemoryEvent, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.events, nil
}

func (m *mockKernelManager) InspectEBPFMaps(ctx context.Context, clusterID string) ([]nodemetrics.EBPFMapUtilization, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.maps, nil
}

func (m *mockKernelManager) AutoResizeEBPFConntrackMap(ctx context.Context, clusterID string, newCapacity int64) error {
	return m.err
}

func (m *mockKernelManager) EnforceNodeSlabTuning(ctx context.Context, nodeName string) error {
	return m.err
}

func TestKernelInterceptorManager_InterfaceContract(t *testing.T) {
	var mgr nodemetrics.KernelInterceptorManager = &mockKernelManager{}
	ctx := context.Background()

	evts, err := mgr.AuditCGroupV2Events(ctx, "worker-node-1")
	if err != nil || evts != nil {
		t.Fatalf("unexpected response from mock KernelInterceptorManager")
	}
}
