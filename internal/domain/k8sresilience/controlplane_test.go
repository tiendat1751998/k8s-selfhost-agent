package k8sresilience_test

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/datdt/k8sselfhost/internal/domain/k8sresilience"
)

func TestPaddedMetrics_Initialization(t *testing.T) {
	var wm k8sresilience.PaddedWebhookMetrics
	var pm k8sresilience.PaddedPLEGMetrics

	if wm.ConsecutiveFailures != 0 {
		t.Fatalf("expected 0, got %d", wm.ConsecutiveFailures)
	}
	if pm.TimeoutCount != 0 {
		t.Fatalf("expected 0, got %d", pm.TimeoutCount)
	}
}

func TestWebhookHealthStatus_DeadlockDetection(t *testing.T) {
	status := k8sresilience.NewWebhookHealthStatus(
		"calico-kube-controllers",
		"validating",
		"Fail",
		"https://calico-webhook.kube-system.svc:443/validate",
		5,
		250*time.Millisecond,
		false,
	)

	if !status.IsDeadlocked {
		t.Errorf("expected IsDeadlocked to be true when failures >= 5, failurePolicy=Fail, and pods unready")
	}

	// Healthy webhook
	healthy := k8sresilience.NewWebhookHealthStatus(
		"cert-manager-webhook",
		"validating",
		"Fail",
		"https://cert-manager-webhook.cert-manager.svc:443/validate",
		0,
		15*time.Millisecond,
		true,
	)
	if healthy.IsDeadlocked {
		t.Errorf("expected IsDeadlocked to be false for healthy webhook with ready pods")
	}

	// Webhook with failurePolicy=Ignore should not be deadlocked even with 10 failures
	ignored := k8sresilience.NewWebhookHealthStatus(
		"optional-webhook",
		"mutating",
		"Ignore",
		"https://optional-webhook.default.svc:443/mutate",
		10,
		500*time.Millisecond,
		false,
	)
	if ignored.IsDeadlocked {
		t.Errorf("expected IsDeadlocked to be false when failurePolicy is Ignore")
	}
}

func TestPLEGStatus_TimeoutEvaluation(t *testing.T) {
	statusHealthy := k8sresilience.NewPLEGStatus("worker-node-01", 45*time.Second, true)
	if !statusHealthy.IsHealthy {
		t.Errorf("expected node with 45s relist duration to be healthy")
	}

	statusTimeout := k8sresilience.NewPLEGStatus("worker-node-02", 185*time.Second, false)
	if statusTimeout.IsHealthy {
		t.Errorf("expected node with 185s relist duration and containerd unresponsive to be unhealthy")
	}
	if statusTimeout.RelistDuration <= 180*time.Second {
		t.Errorf("expected relist duration to exceed 180s threshold")
	}
}

func TestSentinelErrors(t *testing.T) {
	if k8sresilience.ErrK8sWebhookDeadlock == nil {
		t.Fatal("ErrK8sWebhookDeadlock must be defined")
	}
	if k8sresilience.ErrK8sPLEGTimeout == nil {
		t.Fatal("ErrK8sPLEGTimeout must be defined")
	}
	if k8sresilience.ErrFinalizerReapFailed == nil {
		t.Fatal("ErrFinalizerReapFailed must be defined")
	}
}

func TestPaddedMetrics_ConcurrentUpdates(t *testing.T) {
	var wm k8sresilience.PaddedWebhookMetrics
	var pm k8sresilience.PaddedPLEGMetrics

	const workers = 50
	const iterations = 1000

	var wg sync.WaitGroup
	wg.Add(workers * 2)

	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				atomic.AddUint64(&wm.ConsecutiveFailures, 1)
				atomic.AddUint64(&wm.TotalAudits, 1)
				atomic.StoreUint64(&wm.LastLatencyMicros, uint64(j*100))
			}
		}()
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				atomic.AddUint64(&pm.HealthyChecks, 1)
				atomic.AddUint64(&pm.TimeoutCount, 1)
				atomic.StoreUint64(&pm.RelistDurationMicros, uint64(j*50))
			}
		}()
	}

	wg.Wait()

	if expected := uint64(workers * iterations); wm.TotalAudits != expected {
		t.Errorf("expected wm.TotalAudits to be %d, got %d", expected, wm.TotalAudits)
	}
	if expected := uint64(workers * iterations); pm.HealthyChecks != expected {
		t.Errorf("expected pm.HealthyChecks to be %d, got %d", expected, pm.HealthyChecks)
	}
}

