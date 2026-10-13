package nodemetrics_test

import (
	"context"
	"errors"
	"testing"
	"time"

	domain "github.com/datdt/k8sselfhost/internal/domain/nodemetrics"
	usecase "github.com/datdt/k8sselfhost/internal/usecase/nodemetrics"
	"go.uber.org/zap"
)

func TestCGroupV2_MonotonicCounterDeltaTracking(t *testing.T) {
	ctx := context.Background()
	svc := usecase.NewCGroupScalerService(
		usecase.WithLogger(zap.NewNop()),
	)

	now := time.Now().UTC()

	// Initial baseline sample: high = 100 at T0
	svc.RegisterCGroupEvent("node-1", domain.CGroupV2MemoryEvent{
		ContainerID: "c-100",
		PodName:     "payment-service",
		Namespace:   "default",
		HighEvents:  100,
		UpdatedAt:   now,
	})

	events, err := svc.AuditCGroupV2Events(ctx, "node-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	// Initial sample baseline must NOT produce alert spam
	if events[0].Recommendation != "" {
		t.Errorf("initial baseline sample should not trigger alert spam, got: %s", events[0].Recommendation)
	}

	// Second sample 20s later: high = 105 (delta = 5 in 20s, NOT a spike)
	svc.RegisterCGroupEvent("node-1", domain.CGroupV2MemoryEvent{
		ContainerID: "c-100",
		PodName:     "payment-service",
		Namespace:   "default",
		HighEvents:  105,
		UpdatedAt:   now.Add(20 * time.Second),
	})

	events, err = svc.AuditCGroupV2Events(ctx, "node-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Last event should have no recommendation (delta=5 is under threshold)
	lastEv := events[len(events)-1]
	if lastEv.Recommendation != "" {
		t.Errorf("delta 5 in 20s should not trigger spike alert, got: %s", lastEv.Recommendation)
	}

	// Third sample 25s later: high = 135 (delta = 30 in 25s, SPIKE!)
	svc.RegisterCGroupEvent("node-1", domain.CGroupV2MemoryEvent{
		ContainerID: "c-100",
		PodName:     "payment-service",
		Namespace:   "default",
		HighEvents:  135,
		UpdatedAt:   now.Add(45 * time.Second),
	})

	events, err = svc.AuditCGroupV2Events(ctx, "node-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	spikeEv := events[len(events)-1]
	if spikeEv.Recommendation == "" {
		t.Errorf("expected delta=30 in 25s to trigger spike recommendation, got empty")
	}
}

func TestEBPF_InspectMaps(t *testing.T) {
	ctx := context.Background()
	svc := usecase.NewCGroupScalerService()

	svc.RegisterEBPFMap("cluster-prod", domain.EBPFMapUtilization{
		MapName:        "cilium_ct4_global",
		CurrentEntries: 220000,
		MaxEntries:     262144,
	})

	maps, err := svc.InspectEBPFMaps(ctx, "cluster-prod")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(maps) != 1 {
		t.Fatalf("expected 1 map, got %d", len(maps))
	}
	if maps[0].Status != domain.StatusCritical {
		t.Errorf("expected CRITICAL, got %s", maps[0].Status)
	}
	if maps[0].RecommendedSize != 524288 {
		t.Errorf("expected 524288, got %d", maps[0].RecommendedSize)
	}
}

func TestEBPF_AutoResizeAndCiliumPatch(t *testing.T) {
	ctx := context.Background()
	svc := usecase.NewCGroupScalerService(
		usecase.WithHostRAM(16 * 1024 * 1024 * 1024),
	)

	err := svc.AutoResizeEBPFConntrackMap(ctx, "cluster-prod", 524288)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	patch, ok := svc.GetConfigMapPatch("cluster-prod")
	if !ok {
		t.Fatalf("expected patch to be recorded")
	}
	if patch["bpf-ct-global-any-max"] != "524288" {
		t.Errorf("expected 524288, got %s", patch["bpf-ct-global-any-max"])
	}
	if patch["bpf-ct-timeout-regular"] != "60s" {
		t.Errorf("expected 60s, got %s", patch["bpf-ct-timeout-regular"])
	}
}

func TestEBPF_GuardrailRejections(t *testing.T) {
	ctx := context.Background()
	svc := usecase.NewCGroupScalerService(
		usecase.WithHostRAM(16 * 1024 * 1024 * 1024), // 1.6GB limit
	)

	// Ceiling check rejection: > 2,097,152
	err := svc.AutoResizeEBPFConntrackMap(ctx, "cluster-prod", 3000000)
	if err == nil {
		t.Errorf("expected ceiling error, got nil")
	}

	// Host RAM guardrail check rejection: on 512MB RAM, 1,000,000 entries (256MB) > 51.2MB limit
	smallSvc := usecase.NewCGroupScalerService(
		usecase.WithHostRAM(512 * 1024 * 1024),
	)
	err = smallSvc.AutoResizeEBPFConntrackMap(ctx, "cluster-prod", 1000000)
	if err == nil {
		t.Errorf("expected host RAM guardrail rejection, got nil")
	}
	if !errors.Is(err, domain.ErrEBPFMemoryLimitExceeded) {
		t.Errorf("expected ErrEBPFMemoryLimitExceeded, got %v", err)
	}
}

func TestSlabTuning_AppliesSysctl(t *testing.T) {
	ctx := context.Background()
	svc := usecase.NewCGroupScalerService()

	err := svc.EnforceNodeSlabTuning(ctx, "worker-node-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	val, found := svc.GetNodeSysctl("worker-node-1", domain.SysctlVFSNodeCachePressure)
	if !found || val != 150 {
		t.Errorf("expected sysctl 150, got found=%v val=%d", found, val)
	}
}
