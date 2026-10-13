package runtimeguard_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	domain "github.com/datdt/k8sselfhost/internal/domain/runtimeguard"
	usecase "github.com/datdt/k8sselfhost/internal/usecase/runtimeguard"
)

// FakeStorageProbe provides a stateful test probe for node storage telemetry.
type FakeStorageProbe struct {
	mu    sync.RWMutex
	nodes []domain.NodeStorageHealth
}

func (f *FakeStorageProbe) GetNodeStorageHealth(ctx context.Context) ([]domain.NodeStorageHealth, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	res := make([]domain.NodeStorageHealth, len(f.nodes))
	copy(res, f.nodes)
	return res, nil
}

func (f *FakeStorageProbe) SetNodes(nodes []domain.NodeStorageHealth) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nodes = nodes
}

// FakeContainerRuntimeOperator simulates crictl and containerd image management with safety checks.
type FakeContainerRuntimeOperator struct {
	mu               sync.Mutex
	activeImages     map[string][]string
	daemonSetImages  map[string][]string
	pruneCalls       []string
	protectedPassed  map[string][]string
	pruneReturnCount int
	pruneErr         error
}

func (f *FakeContainerRuntimeOperator) ListActiveImages(ctx context.Context, node string) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.activeImages[node], nil
}

func (f *FakeContainerRuntimeOperator) ListDaemonSetImages(ctx context.Context, node string) ([]string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.daemonSetImages[node], nil
}

func (f *FakeContainerRuntimeOperator) PruneDanglingImages(ctx context.Context, node string, protectedImages []string) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.pruneErr != nil {
		return 0, f.pruneErr
	}
	f.pruneCalls = append(f.pruneCalls, node)
	if f.protectedPassed == nil {
		f.protectedPassed = make(map[string][]string)
	}
	f.protectedPassed[node] = protectedImages
	return f.pruneReturnCount, nil
}

// FakeNodeTaintOperator records taint operations.
type FakeNodeTaintOperator struct {
	mu           sync.Mutex
	taintedNodes []string
}

func (f *FakeNodeTaintOperator) ApplyOutOfInodesTaint(ctx context.Context, node string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.taintedNodes = append(f.taintedNodes, node)
	return nil
}

// FakeAlertDispatcher records SRE alerts.
type FakeAlertDispatcher struct {
	mu     sync.Mutex
	alerts []string
}

func (f *FakeAlertDispatcher) EmitAlert(ctx context.Context, severity, code, message, node string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.alerts = append(f.alerts, fmt.Sprintf("%s|%s|%s|%s", severity, code, node, message))
	return nil
}

// FakeP2PPeerResolver simulates Spegel / Dragonfly DHT peer resolution.
type FakeP2PPeerResolver struct {
	delay   time.Duration
	failErr error
	peerURL string
}

func (f *FakeP2PPeerResolver) ResolveP2PPeer(ctx context.Context, imageURI string) (string, error) {
	if f.delay > 0 {
		select {
		case <-time.After(f.delay):
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	if f.failErr != nil {
		return "", f.failErr
	}
	return f.peerURL, nil
}

func TestAuditNodeInodes_Thresholds(t *testing.T) {
	probe := &FakeStorageProbe{}
	runtimeOp := &FakeContainerRuntimeOperator{
		activeImages:    map[string][]string{"node-warning": {"k8s.gcr.io/pause:3.9"}},
		daemonSetImages: map[string][]string{"node-warning": {"cilium/cilium:v1.14.2"}},
	}
	taintOp := &FakeNodeTaintOperator{}
	alertOp := &FakeAlertDispatcher{}
	p2pResolver := &FakeP2PPeerResolver{}

	mgr := usecase.NewResilienceManager(
		usecase.WithStorageProbe(probe),
		usecase.WithContainerRuntime(runtimeOp),
		usecase.WithNodeTainter(taintOp),
		usecase.WithAlertDispatcher(alertOp),
		usecase.WithP2PResolver(p2pResolver),
	)

	// Case 1: Healthy node (UsedInodesPct: 60%)
	probe.SetNodes([]domain.NodeStorageHealth{
		{
			NodeName:      "node-healthy",
			MountPoint:    domain.DefaultContainerdMountPoint,
			UsedInodesPct: 60.0,
		},
	})

	healths, err := mgr.AuditNodeInodes(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(healths) != 1 {
		t.Fatalf("expected 1 node, got %d", len(healths))
	}
	if len(runtimeOp.pruneCalls) != 0 {
		t.Errorf("expected 0 prunes for healthy node, got %d", len(runtimeOp.pruneCalls))
	}

	// Case 2: Warning node (UsedInodesPct: 76% >= 75%)
	probe.SetNodes([]domain.NodeStorageHealth{
		{
			NodeName:       "node-warning",
			MountPoint:     domain.DefaultContainerdMountPoint,
			UsedInodesPct:  76.0,
			DanglingImages: 8,
		},
	})

	healths, err = mgr.AuditNodeInodes(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(runtimeOp.pruneCalls) != 1 || runtimeOp.pruneCalls[0] != "node-warning" {
		t.Errorf("expected prune on node-warning, got calls: %v", runtimeOp.pruneCalls)
	}
	if len(taintOp.taintedNodes) != 0 {
		t.Errorf("expected no taints on 76%% usage, got: %v", taintOp.taintedNodes)
	}

	// Verify safety guardrail: active & daemonset images protected
	protected := runtimeOp.protectedPassed["node-warning"]
	if len(protected) != 2 {
		t.Errorf("expected 2 protected images, got %d: %v", len(protected), protected)
	}

	// Case 3: Critical node (UsedInodesPct: 88% >= 85%)
	probe.SetNodes([]domain.NodeStorageHealth{
		{
			NodeName:       "node-critical",
			MountPoint:     domain.DefaultContainerdMountPoint,
			UsedInodesPct:  88.0,
			DanglingImages: 5,
		},
	})

	_, err = mgr.AuditNodeInodes(context.Background())
	// At 85%+, taint is applied and P1 alert emitted
	if len(taintOp.taintedNodes) != 1 || taintOp.taintedNodes[0] != "node-critical" {
		t.Errorf("expected taint on node-critical, got: %v", taintOp.taintedNodes)
	}
	if len(alertOp.alerts) != 1 {
		t.Fatalf("expected 1 P1 alert, got %d", len(alertOp.alerts))
	}
	if !strings.Contains(alertOp.alerts[0], "ERR_CONTAINERD_INODE_SATURATION") {
		t.Errorf("expected alert with ERR_CONTAINERD_INODE_SATURATION, got: %s", alertOp.alerts[0])
	}
}

func TestSafetyGuardrail_NeverPruneActiveImages(t *testing.T) {
	runtimeOp := &FakeContainerRuntimeOperator{
		activeImages: map[string][]string{
			"worker-01": {"registry.k8s.io/kube-proxy:v1.28.2", "app/web:1.0"},
		},
		daemonSetImages: map[string][]string{
			"worker-01": {"datadog/agent:7.49.0"},
		},
	}

	mgr := usecase.NewResilienceManager(
		usecase.WithContainerRuntime(runtimeOp),
	)

	err := mgr.TriggerDanglingPrune(context.Background(), "worker-01")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	protected := runtimeOp.protectedPassed["worker-01"]
	expectedSet := map[string]bool{
		"registry.k8s.io/kube-proxy:v1.28.2": true,
		"app/web:1.0":                         true,
		"datadog/agent:7.49.0":                true,
	}

	for _, img := range protected {
		if !expectedSet[img] {
			t.Errorf("unexpected image protected: %s", img)
		}
	}
	if len(protected) != 3 {
		t.Errorf("expected 3 protected images, got %d", len(protected))
	}
}

func TestRegistryMirrorAdmissionRewriter(t *testing.T) {
	cfg := domain.RegistryMirrorConfig{
		PrimaryRegistry: "docker.io",
		MirrorURL:       "harbor.corp.local/proxy-cache",
		FallbackTimeout: 15 * time.Second,
		RateLimitFloor:  100,
	}

	mgr := usecase.NewResilienceManager()
	if err := mgr.ConfigureRegistryMirror(context.Background(), cfg); err != nil {
		t.Fatalf("failed to configure mirror: %v", err)
	}

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Unqualified single component",
			input:    "nginx:latest",
			expected: "harbor.corp.local/proxy-cache/library/nginx:latest",
		},
		{
			name:     "Unqualified with namespace",
			input:    "library/alpine:3.18",
			expected: "harbor.corp.local/proxy-cache/library/alpine:3.18",
		},
		{
			name:     "Qualified docker.io library",
			input:    "docker.io/library/ubuntu:22.04",
			expected: "harbor.corp.local/proxy-cache/library/ubuntu:22.04",
		},
		{
			name:     "Qualified index.docker.io custom org",
			input:    "index.docker.io/myorg/myapp:v2.1",
			expected: "harbor.corp.local/proxy-cache/myorg/myapp:v2.1",
		},
		{
			name:     "External registry untouched",
			input:    "quay.io/prometheus/node-exporter:v1.6.1",
			expected: "quay.io/prometheus/node-exporter:v1.6.1",
		},
		{
			name:     "Internal registry untouched",
			input:    "harbor.corp.local/apps/backend:v1.0",
			expected: "harbor.corp.local/apps/backend:v1.0",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mutated, err := mgr.MutateImageForMirror(tc.input)
			if err != nil {
				t.Fatalf("unexpected error mutating image: %v", err)
			}
			if mutated != tc.expected {
				t.Errorf("expected %s, got %s", tc.expected, mutated)
			}
		})
	}
}

func TestRegistryRateLimitFloorExceeded(t *testing.T) {
	cfg := domain.RegistryMirrorConfig{
		PrimaryRegistry: "docker.io",
		MirrorURL:       "harbor.corp.local/proxy-cache",
		FallbackTimeout: 15 * time.Second,
		RateLimitFloor:  5,
	}

	mgr := usecase.NewResilienceManager()
	_ = mgr.ConfigureRegistryMirror(context.Background(), cfg)

	// Simulate pull attempts hitting the ceiling
	for i := 0; i < 5; i++ {
		_, err := mgr.MutateImageForMirror("nginx:alpine")
		if err != nil {
			t.Fatalf("unexpected error at pull %d: %v", i, err)
		}
	}

	// 6th pull should exceed rate limit floor
	_, err := mgr.MutateImageForMirror("nginx:alpine")
	if !errors.Is(err, domain.ErrRegistryRateLimitExceeded) {
		t.Fatalf("expected ErrRegistryRateLimitExceeded, got: %v", err)
	}
}

func TestSpegelDragonflyP2PFallback_Within15s(t *testing.T) {
	// Case 1: P2P Peer succeeds quickly
	fastResolver := &FakeP2PPeerResolver{
		peerURL: "http://peer-03.dragonfly.local:65001/blobs/sha256:abcd",
	}

	mgrFast := usecase.NewResilienceManager(usecase.WithP2PResolver(fastResolver))
	resolved, err := mgrFast.ResolveImageWithP2PFallback(context.Background(), "docker.io/library/nginx:latest")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resolved != fastResolver.peerURL {
		t.Errorf("expected peer url %s, got %s", fastResolver.peerURL, resolved)
	}

	// Case 2: P2P Peer hangs or fails -> falls back within timeout to origin/mirror
	slowResolver := &FakeP2PPeerResolver{
		delay: 50 * time.Millisecond, // Simulating timeout with small test duration
	}

	mgrSlow := usecase.NewResilienceManager(
		usecase.WithP2PResolver(slowResolver),
		usecase.WithP2PTimeout(20*time.Millisecond), // Fast test timeout
	)

	resolved, err = mgrSlow.ResolveImageWithP2PFallback(context.Background(), "docker.io/library/nginx:latest")
	if err != nil {
		t.Fatalf("unexpected error on fallback: %v", err)
	}
	// Fallback must resolve to origin or internal mirror
	if !strings.Contains(resolved, "nginx:latest") {
		t.Errorf("expected fallback to contain origin image spec, got %s", resolved)
	}
}

func TestManifestValidation(t *testing.T) {
	mgr := usecase.NewResilienceManager()

	validURIs := []string{
		"docker.io/library/alpine:3.18",
		"nginx:1.25-alpine",
		"quay.io/coreos/etcd:v3.5.0",
		"internal.registry.io:5000/app/worker:sha256-abcdef",
	}
	for _, uri := range validURIs {
		valid, err := mgr.ValidateImageManifest(uri)
		if err != nil || !valid {
			t.Errorf("expected valid for %s, got valid=%v, err=%v", uri, valid, err)
		}
	}

	invalidURIs := []string{
		"",
		"   ",
		"invalid image with spaces",
		"http://docker.io/invalid-protocol",
		"docker.io/library/image@invalid-digest",
	}
	for _, uri := range invalidURIs {
		valid, err := mgr.ValidateImageManifest(uri)
		if valid || !errors.Is(err, domain.ErrManifestValidationFailed) {
			t.Errorf("expected ErrManifestValidationFailed for %s, got valid=%v, err=%v", uri, valid, err)
		}
	}
}

func TestConcurrentAuditAndCounterSafety(t *testing.T) {
	probe := &FakeStorageProbe{
		nodes: []domain.NodeStorageHealth{
			{NodeName: "n1", UsedInodesPct: 50.0},
			{NodeName: "n2", UsedInodesPct: 60.0},
		},
	}
	mgr := usecase.NewResilienceManager(usecase.WithStorageProbe(probe))

	const goroutines = 20
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			_, _ = mgr.AuditNodeInodes(context.Background())
			_, _ = mgr.ValidateImageManifest("nginx:latest")
		}()
	}

	wg.Wait()

	if mgr.InodeAuditCount() != uint64(goroutines*2) {
		t.Errorf("expected %d inode audits, got %d", goroutines*2, mgr.InodeAuditCount())
	}
}
