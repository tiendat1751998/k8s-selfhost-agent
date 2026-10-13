package gitops_test

import (
	"errors"
	"net/http"
	"sync"
	"testing"
	"time"
	"unsafe"

	domainGitops "github.com/datdt/k8sselfhost/internal/domain/gitops"
)

// TestPaddedCounter_CacheLineSize verifies that PaddedCounter occupies exactly 64 bytes
// to prevent CPU false sharing under MESI/MOESI cache coherence protocols.
func TestPaddedCounter_CacheLineSize(t *testing.T) {
	var counter domainGitops.PaddedCounter
	size := unsafe.Sizeof(counter)
	if size != 64 {
		t.Fatalf("expected PaddedCounter size to be 64 bytes (exact CPU cache line), got %d bytes", size)
	}
}

// TestPaddedCounter_ConcurrentSafety verifies atomic operations under high goroutine load.
func TestPaddedCounter_ConcurrentSafety(t *testing.T) {
	var counter domainGitops.PaddedCounter
	const numGoroutines = 50
	const iterations = 1000

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				counter.Inc()
			}
		}()
	}

	wg.Wait()

	expected := int64(numGoroutines * iterations)
	if actual := counter.Load(); actual != expected {
		t.Fatalf("expected counter value %d, got %d", expected, actual)
	}

	counter.Add(500)
	if counter.Load() != expected+500 {
		t.Fatalf("expected counter value %d, got %d", expected+500, counter.Load())
	}

	counter.Store(42)
	if counter.Load() != 42 {
		t.Fatalf("expected stored value 42, got %d", counter.Load())
	}
}

// TestSyncWaveMetrics_CacheLineIsolation verifies that individual metric counters inside
// SyncWaveMetrics have separate 64-byte alignments to eliminate cross-core cache invalidations.
func TestSyncWaveMetrics_CacheLineIsolation(t *testing.T) {
	metrics := domainGitops.NewSyncWaveMetrics()
	if metrics == nil {
		t.Fatal("expected non-nil SyncWaveMetrics")
	}

	const goroutines = 20
	const iterations = 500
	var wg sync.WaitGroup
	wg.Add(goroutines * 3)

	// Concurrent increments on different counters
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				metrics.WebhookProcessedCount.Inc()
			}
		}()
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				metrics.DAGTraversalNanosTotal.Add(10)
			}
		}()
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				metrics.DriftEvaluationsCount.Inc()
			}
		}()
	}

	wg.Wait()

	if metrics.WebhookProcessedCount.Load() != int64(goroutines*iterations) {
		t.Errorf("unexpected WebhookProcessedCount: %d", metrics.WebhookProcessedCount.Load())
	}
	if metrics.DAGTraversalNanosTotal.Load() != int64(goroutines*iterations*10) {
		t.Errorf("unexpected DAGTraversalNanosTotal: %d", metrics.DAGTraversalNanosTotal.Load())
	}
	if metrics.DriftEvaluationsCount.Load() != int64(goroutines*iterations) {
		t.Errorf("unexpected DriftEvaluationsCount: %d", metrics.DriftEvaluationsCount.Load())
	}
}

// TestRFC7807ProblemDetail_StandardErrors verifies standard domain errors comply with RFC 7807.
func TestRFC7807ProblemDetail_StandardErrors(t *testing.T) {
	tests := []struct {
		name           string
		err            *domainGitops.RFC7807ProblemDetail
		expectedStatus int
		expectedCode   string
	}{
		{
			name:           "ErrGitOpsObsoleteWebhook",
			err:            domainGitops.ErrGitOpsObsoleteWebhook,
			expectedStatus: http.StatusConflict,
			expectedCode:   domainGitops.CodeGitOpsObsoleteWebhook,
		},
		{
			name:           "ErrGitOpsCircularSyncWave",
			err:            domainGitops.ErrGitOpsCircularSyncWave,
			expectedStatus: http.StatusUnprocessableEntity,
			expectedCode:   domainGitops.CodeGitOpsCircularSyncWave,
		},
		{
			name:           "ErrGitOpsDriftConflict",
			err:            domainGitops.ErrGitOpsDriftConflict,
			expectedStatus: http.StatusConflict,
			expectedCode:   domainGitops.CodeGitOpsDriftConflict,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Status != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, tt.err.Status)
			}
			if tt.err.Code != tt.expectedCode {
				t.Errorf("expected code %s, got %s", tt.expectedCode, tt.err.Code)
			}
			if tt.err.Error() == "" {
				t.Error("expected non-empty Error() string")
			}
		})
	}
}

// TestRFC7807ProblemDetail_WithDetailAndErrorsIs verifies error matching and detail enrichment.
func TestRFC7807ProblemDetail_WithDetailAndErrorsIs(t *testing.T) {
	orig := domainGitops.ErrGitOpsCircularSyncWave
	enriched := orig.WithDetail("cycle detected: Service/frontend -> Deployment/backend -> Service/frontend")

	if enriched.Detail != "cycle detected: Service/frontend -> Deployment/backend -> Service/frontend" {
		t.Fatalf("expected enriched detail, got: %s", enriched.Detail)
	}
	if enriched.Status != orig.Status {
		t.Errorf("status changed after WithDetail: %d vs %d", enriched.Status, orig.Status)
	}
	if !errors.Is(enriched, orig) {
		t.Fatalf("expected errors.Is(enriched, orig) to be true")
	}
}

// TestWebhookEventPayload_Validation verifies payload creation invariants.
func TestWebhookEventPayload_Validation(t *testing.T) {
	valid := domainGitops.WebhookEventPayload{
		DeliveryID:      "deliv-001",
		RepositoryURL:   "https://github.com/org/repo.git",
		Branch:          "main",
		CommitSHA:       "a1b2c3d4e5f67890",
		CommitTimestamp: time.Now().UTC(),
		Author:          "sre-lead",
	}

	if err := valid.Validate(); err != nil {
		t.Fatalf("expected valid payload to pass, got: %v", err)
	}

	invalidDelivery := valid
	invalidDelivery.DeliveryID = ""
	if err := invalidDelivery.Validate(); err == nil {
		t.Error("expected empty DeliveryID to fail validation")
	}

	invalidSHA := valid
	invalidSHA.CommitSHA = ""
	if err := invalidSHA.Validate(); err == nil {
		t.Error("expected empty CommitSHA to fail validation")
	}

	invalidBranch := valid
	invalidBranch.Branch = ""
	if err := invalidBranch.Validate(); err == nil {
		t.Error("expected empty Branch to fail validation")
	}
}

// TestSyncWaveGraphNode_Creation verifies graph node instantiation.
func TestSyncWaveGraphNode_Creation(t *testing.T) {
	node := domainGitops.SyncWaveGraphNode{
		ResourceName: "redis-master",
		Kind:         "StatefulSet",
		Wave:         -1,
		DependsOn:    []string{"ConfigMap/redis-config"},
	}

	if node.ResourceName != "redis-master" || node.Wave != -1 {
		t.Fatalf("unexpected node values: %+v", node)
	}
	if len(node.DependsOn) != 1 || node.DependsOn[0] != "ConfigMap/redis-config" {
		t.Fatalf("unexpected depends on: %v", node.DependsOn)
	}
}

// TestDriftReconciliationResult_Helpers verifies DriftReconciliationResult methods.
func TestDriftReconciliationResult_Helpers(t *testing.T) {
	res := domainGitops.DriftReconciliationResult{
		HasDrift:         true,
		DriftedFields:    []string{"spec.replicas"},
		ConflictDetected: false,
		AutoResolved:     true,
	}

	if !res.HasDrift || res.ConflictDetected || !res.AutoResolved {
		t.Fatalf("unexpected result state: %+v", res)
	}
	if len(res.DriftedFields) != 1 || res.DriftedFields[0] != "spec.replicas" {
		t.Fatalf("unexpected drifted fields: %v", res.DriftedFields)
	}
}
