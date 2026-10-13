package cloudresilience_test

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"

	"github.com/datdt/k8sselfhost/internal/domain/cloudresilience"
)

func TestStorageBurstStatus_Evaluation(t *testing.T) {
	tests := []struct {
		name          string
		volumeID      string
		volumeType    string
		burstPct      float64
		currentIOPS   int64
		expectUpgrade bool
		expectErr     error
	}{
		{
			name:          "GP2 healthy burst balance 85%",
			volumeID:      "vol-0123456789abcdef0",
			volumeType:    "gp2",
			burstPct:      85.0,
			currentIOPS:   500,
			expectUpgrade: false,
			expectErr:     nil,
		},
		{
			name:          "GP2 burst balance critical 18% requires upgrade",
			volumeID:      "vol-0123456789abcdef1",
			volumeType:    "gp2",
			burstPct:      18.5,
			currentIOPS:   1200,
			expectUpgrade: true,
			expectErr:     nil,
		},
		{
			name:          "GP2 burst balance exactly 20% borderline healthy",
			volumeID:      "vol-0123456789abcdef2",
			volumeType:    "gp2",
			burstPct:      20.0,
			currentIOPS:   300,
			expectUpgrade: false,
			expectErr:     nil,
		},
		{
			name:          "GP2 burst balance 0% exhausted triggers error",
			volumeID:      "vol-0123456789abcdef3",
			volumeType:    "gp2",
			burstPct:      0.0,
			currentIOPS:   300,
			expectUpgrade: true,
			expectErr:     cloudresilience.ErrCloudEBSBurstExhausted,
		},
		{
			name:          "GP3 volume baseline 3000 IOPS does not need upgrade even with low percentage",
			volumeID:      "vol-0123456789abcdef4",
			volumeType:    "gp3",
			burstPct:      15.0,
			currentIOPS:   3000,
			expectUpgrade: false,
			expectErr:     nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			status, err := cloudresilience.NewStorageBurstStatus(
				tc.volumeID,
				tc.volumeType,
				tc.burstPct,
				tc.currentIOPS,
			)

			if tc.expectErr != nil {
				if err != tc.expectErr {
					t.Fatalf("expected error %v, got %v", tc.expectErr, err)
				}
				if status == nil {
					t.Fatal("expected status object even on burst exhaustion")
				}
				if !status.NeedsUpgrade {
					t.Errorf("expected NeedsUpgrade=true on exhausted volume")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if status.NeedsUpgrade != tc.expectUpgrade {
				t.Errorf("expected NeedsUpgrade=%v, got %v", tc.expectUpgrade, status.NeedsUpgrade)
			}
		})
	}
}

func TestStorageBurstStatus_Validation(t *testing.T) {
	_, err := cloudresilience.NewStorageBurstStatus("", "gp2", 50.0, 300)
	if err == nil {
		t.Error("expected error for empty volume ID")
	}

	_, err = cloudresilience.NewStorageBurstStatus("vol-1", "gp2", -5.0, 300)
	if err == nil {
		t.Error("expected error for negative burst balance")
	}

	_, err = cloudresilience.NewStorageBurstStatus("vol-1", "gp2", 105.0, 300)
	if err == nil {
		t.Error("expected error for burst balance > 100%")
	}
}

func TestSpotTerminationEvent(t *testing.T) {
	now := time.Now().UTC()
	event, err := cloudresilience.NewSpotTerminationEvent("i-0123456789abcdef0", now)
	if err != nil {
		t.Fatalf("unexpected error creating spot event: %v", err)
	}

	if event.InstanceID != "i-0123456789abcdef0" {
		t.Errorf("unexpected instance ID: %s", event.InstanceID)
	}
	if !event.ActionTime.Equal(now) {
		t.Errorf("unexpected action time: %v", event.ActionTime)
	}
	if event.DrainStarted || event.DrainSuccess {
		t.Errorf("new spot termination event should initialize drain flags to false")
	}

	_, err = cloudresilience.NewSpotTerminationEvent("", now)
	if err == nil {
		t.Error("expected error for empty instance ID")
	}
}

func TestNATGatewayMetrics_Evaluation(t *testing.T) {
	tests := []struct {
		name            string
		gatewayID       string
		allocatedIPs    int
		activeConns     int64
		portAllocErrors int64
		expectSaturated bool
		expectErr       error
	}{
		{
			name:            "Healthy NAT Gateway well within port capacity",
			gatewayID:       "nat-0123456789abcdef0",
			allocatedIPs:    1,
			activeConns:     12500,
			portAllocErrors: 0,
			expectSaturated: false,
			expectErr:       nil,
		},
		{
			name:            "Single IP gateway near 64512 limit but not saturated",
			gatewayID:       "nat-0123456789abcdef1",
			allocatedIPs:    1,
			activeConns:     60000,
			portAllocErrors: 0,
			expectSaturated: false,
			expectErr:       nil,
		},
		{
			name:            "Single IP gateway exceeds 64512 limit",
			gatewayID:       "nat-0123456789abcdef2",
			allocatedIPs:    1,
			activeConns:     64512,
			portAllocErrors: 0,
			expectSaturated: true,
			expectErr:       cloudresilience.ErrNATGatewaySaturated,
		},
		{
			name:            "Gateway with active port allocation errors even below capacity",
			gatewayID:       "nat-0123456789abcdef3",
			allocatedIPs:    2,
			activeConns:     30000,
			portAllocErrors: 42,
			expectSaturated: true,
			expectErr:       cloudresilience.ErrNATGatewaySaturated,
		},
		{
			name:            "Multi-IP gateway with 2 IPs has 129024 capacity",
			gatewayID:       "nat-0123456789abcdef4",
			allocatedIPs:    2,
			activeConns:     80000,
			portAllocErrors: 0,
			expectSaturated: false,
			expectErr:       nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			metrics, err := cloudresilience.NewNATGatewayMetrics(
				tc.gatewayID,
				tc.allocatedIPs,
				tc.activeConns,
				tc.portAllocErrors,
			)

			if tc.expectErr != nil {
				if err != tc.expectErr {
					t.Fatalf("expected error %v, got %v", tc.expectErr, err)
				}
				if metrics == nil {
					t.Fatal("expected metrics object even when saturated")
				}
				if !metrics.IsSaturated {
					t.Errorf("expected IsSaturated=true")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if metrics.IsSaturated != tc.expectSaturated {
				t.Errorf("expected IsSaturated=%v, got %v", tc.expectSaturated, metrics.IsSaturated)
			}
		})
	}
}

func TestCacheLinePaddingAndConcurrency(t *testing.T) {
	// Verify 64-byte alignment and sizing on modern CPU cache-lines to eliminate false sharing
	if sz := unsafe.Sizeof(cloudresilience.PaddedBurstTracker{}); sz != 64 {
		t.Errorf("PaddedBurstTracker size = %d bytes; must be exactly 64 bytes", sz)
	}
	if sz := unsafe.Sizeof(cloudresilience.PaddedSpotTimer{}); sz != 64 {
		t.Errorf("PaddedSpotTimer size = %d bytes; must be exactly 64 bytes", sz)
	}
	if sz := unsafe.Sizeof(cloudresilience.PaddedSNATPortCounter{}); sz != 64 {
		t.Errorf("PaddedSNATPortCounter size = %d bytes; must be exactly 64 bytes", sz)
	}

	// Verify high-concurrency atomic safety across parallel polling goroutines
	var (
		tracker cloudresilience.PaddedBurstTracker
		timer   cloudresilience.PaddedSpotTimer
		counter cloudresilience.PaddedSNATPortCounter
		wg      sync.WaitGroup
	)

	goroutines := 50
	iterations := 1000

	wg.Add(goroutines * 3)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				atomic.AddUint64(&tracker.AuditCount, 1)
				atomic.StoreUint64(&tracker.CurrentIOPS, uint64(j*3))
			}
		}()

		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				atomic.AddUint64(&timer.TotalNotices, 1)
				atomic.StoreUint64(&timer.NoticeReceivedUnixSec, uint64(time.Now().Unix()))
			}
		}()

		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				atomic.AddUint64(&counter.TotalQueries, 1)
				atomic.AddUint64(&counter.ActiveConnections, 1)
			}
		}()
	}

	wg.Wait()

	expectedAudits := uint64(goroutines * iterations)
	if actual := atomic.LoadUint64(&tracker.AuditCount); actual != expectedAudits {
		t.Errorf("expected %d audits, got %d", expectedAudits, actual)
	}
	if actual := atomic.LoadUint64(&timer.TotalNotices); actual != expectedAudits {
		t.Errorf("expected %d notices, got %d", expectedAudits, actual)
	}
	if actual := atomic.LoadUint64(&counter.TotalQueries); actual != expectedAudits {
		t.Errorf("expected %d queries, got %d", expectedAudits, actual)
	}
}
