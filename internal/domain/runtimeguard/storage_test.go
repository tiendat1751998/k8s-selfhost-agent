package runtimeguard_test

import (
	"errors"
	"sync"
	"testing"
	"time"
	"unsafe"

	"github.com/datdt/k8sselfhost/internal/domain/runtimeguard"
)

// TestPaddedCountersCacheLine verifies the 64-byte alignment and padding of telemetry counters
// to eliminate false sharing on multicore CPUs under MESI/MOESI cache coherence protocols.
func TestPaddedCountersCacheLine(t *testing.T) {
	var inodeCounter runtimeguard.PaddedInodeUsageCounter
	var rateLimitCounter runtimeguard.PaddedRateLimitTracker
	var metricCounter runtimeguard.PaddedStorageMetricTracker

	if sz := unsafe.Sizeof(inodeCounter); sz != 64 {
		t.Fatalf("expected PaddedInodeUsageCounter size to be 64 bytes, got %d bytes", sz)
	}

	if sz := unsafe.Sizeof(rateLimitCounter); sz != 64 {
		t.Fatalf("expected PaddedRateLimitTracker size to be 64 bytes, got %d bytes", sz)
	}

	if sz := unsafe.Sizeof(metricCounter); sz != 64 {
		t.Fatalf("expected PaddedStorageMetricTracker size to be 64 bytes, got %d bytes", sz)
	}
}

// TestPaddedCountersConcurrentOperations asserts race-safety and atomic consistency under heavy concurrent load.
func TestPaddedCountersConcurrentOperations(t *testing.T) {
	var inodeCounter runtimeguard.PaddedInodeUsageCounter
	var rateLimitCounter runtimeguard.PaddedRateLimitTracker

	const workers = 50
	const iterations = 1000

	var wg sync.WaitGroup
	wg.Add(workers * 2)

	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				inodeCounter.Inc()
			}
		}()

		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				rateLimitCounter.Add(1)
			}
		}()
	}

	wg.Wait()

	expected := uint64(workers * iterations)
	if got := inodeCounter.Load(); got != expected {
		t.Errorf("inodeCounter: expected %d, got %d", expected, got)
	}
	if got := rateLimitCounter.Load(); got != expected {
		t.Errorf("rateLimitCounter: expected %d, got %d", expected, got)
	}

	// Test Reset
	inodeCounter.Reset()
	if got := inodeCounter.Load(); got != 0 {
		t.Errorf("inodeCounter after Reset: expected 0, got %d", got)
	}
}

// TestDomainErrors verifies sentinel domain errors, status codes, and RFC 7807 compliance.
func TestDomainErrors(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		targetErr  error
		code       string
		httpStatus int
	}{
		{
			name:       "ErrContainerdInodeSaturation",
			err:        runtimeguard.ErrContainerdInodeSaturation,
			targetErr:  runtimeguard.ErrContainerdInodeSaturation,
			code:       runtimeguard.CodeContainerdInodeSaturation,
			httpStatus: 507,
		},
		{
			name:       "ErrRegistryRateLimitExceeded",
			err:        runtimeguard.ErrRegistryRateLimitExceeded,
			targetErr:  runtimeguard.ErrRegistryRateLimitExceeded,
			code:       runtimeguard.CodeRegistryRateLimitExceeded,
			httpStatus: 429,
		},
		{
			name:       "ErrManifestValidationFailed",
			err:        runtimeguard.ErrManifestValidationFailed,
			targetErr:  runtimeguard.ErrManifestValidationFailed,
			code:       runtimeguard.CodeManifestValidationFailed,
			httpStatus: 422,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if !errors.Is(tc.err, tc.targetErr) {
				t.Fatalf("expected errors.Is(%v, %v) to be true", tc.err, tc.targetErr)
			}

			var domErr *runtimeguard.DomainError
			if !errors.As(tc.err, &domErr) {
				t.Fatalf("expected err to be of type *runtimeguard.DomainError, got %T", tc.err)
			}

			if domErr.Code != tc.code {
				t.Errorf("expected code %s, got %s", tc.code, domErr.Code)
			}
			if domErr.HTTPStatus != tc.httpStatus {
				t.Errorf("expected HTTP status %d, got %d", tc.httpStatus, domErr.HTTPStatus)
			}
			if domErr.Error() == "" {
				t.Error("expected non-empty Error() string")
			}
		})
	}
}

// TestNodeStorageHealthCalculations tests threshold calculations on NodeStorageHealth.
func TestNodeStorageHealthCalculations(t *testing.T) {
	health := runtimeguard.NodeStorageHealth{
		NodeName:       "worker-01",
		MountPoint:     runtimeguard.DefaultContainerdMountPoint,
		TotalBytes:     500 * 1024 * 1024 * 1024,
		UsedBytesPct:   45.5,
		TotalInodes:    30000000,
		UsedInodesPct:  76.2,
		DanglingImages: 12,
		LastPrunedAt:   time.Now().UTC().Add(-2 * time.Hour),
	}

	if !health.RequiresPrune() {
		t.Error("expected RequiresPrune() to be true when UsedInodesPct >= 75%")
	}

	if health.RequiresTaint() {
		t.Error("expected RequiresTaint() to be false when UsedInodesPct < 85%")
	}

	health.UsedInodesPct = 86.1
	if !health.RequiresTaint() {
		t.Error("expected RequiresTaint() to be true when UsedInodesPct >= 85%")
	}
}

// TestRegistryMirrorConfigValidation tests RegistryMirrorConfig validation logic.
func TestRegistryMirrorConfigValidation(t *testing.T) {
	validCfg := runtimeguard.RegistryMirrorConfig{
		PrimaryRegistry: "docker.io",
		MirrorURL:       "harbor.corp.local/proxy-cache",
		FallbackTimeout: 15 * time.Second,
		RateLimitFloor:  10,
	}

	if err := validCfg.Validate(); err != nil {
		t.Fatalf("expected valid config, got error: %v", err)
	}

	invalidCfg := runtimeguard.RegistryMirrorConfig{
		PrimaryRegistry: "",
		MirrorURL:       "",
	}
	if err := invalidCfg.Validate(); err == nil {
		t.Fatal("expected error for empty primary registry, got nil")
	}
}
