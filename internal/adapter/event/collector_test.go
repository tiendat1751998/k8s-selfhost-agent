package event

import (
	"context"
	"testing"
)

func TestCollector_NilClientset_SafeReturn(t *testing.T) {
	collector := NewCollector(nil)
	data, err := collector.Collect(context.Background(), "default", "pod-1")
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if data == nil {
		t.Fatal("expected non-nil collected data")
	}
	if data.PodLogs == nil {
		t.Error("expected non-nil PodLogs map")
	}
}

func TestCollector_NilReceiver_SafeReturn(t *testing.T) {
	var collector *Collector
	data, err := collector.Collect(context.Background(), "default", "pod-1")
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if data == nil {
		t.Fatal("expected non-nil collected data")
	}
}

func TestCollector_HelperMethods_NilClientset(t *testing.T) {
	collector := NewCollector(nil)
	ctx := context.Background()
	data := &CollectedData{
		PodLogs: make(map[string]string),
	}

	if err := collector.collectPodLogs(ctx, "default", "pod-1", data); err != nil {
		t.Errorf("collectPodLogs failed: %v", err)
	}
	if logs, err := collector.getContainerLogs(ctx, "default", "pod-1", "c1", false); err != nil || logs != "" {
		t.Errorf("getContainerLogs failed: %v, %q", err, logs)
	}
	if err := collector.collectEvents(ctx, "default", "pod-1", data); err != nil {
		t.Errorf("collectEvents failed: %v", err)
	}
	if err := collector.collectPodDescribe(ctx, "default", "pod-1", data); err != nil {
		t.Errorf("collectPodDescribe failed: %v", err)
	}
	if err := collector.collectOwnerResources(ctx, "default", "pod-1", data); err != nil {
		t.Errorf("collectOwnerResources failed: %v", err)
	}
	if err := collector.collectDeploymentFromRS(ctx, "default", "rs-1", data); err != nil {
		t.Errorf("collectDeploymentFromRS failed: %v", err)
	}
	if err := collector.collectStatefulSet(ctx, "default", "sts-1", data); err != nil {
		t.Errorf("collectStatefulSet failed: %v", err)
	}
	if err := collector.collectServiceYAML(ctx, "default", "pod-1", data); err != nil {
		t.Errorf("collectServiceYAML failed: %v", err)
	}
	if err := collector.collectIngressYAML(ctx, "default", data); err != nil {
		t.Errorf("collectIngressYAML failed: %v", err)
	}
	if err := collector.collectNodeMetrics(ctx, "default", "pod-1", data); err != nil {
		t.Errorf("collectNodeMetrics failed: %v", err)
	}
}

func TestMatchesSelector_Match(t *testing.T) {
	podLabels := map[string]string{
		"app":     "nginx",
		"version": "v1",
		"env":     "prod",
	}
	selector := map[string]string{
		"app":     "nginx",
		"version": "v1",
	}

	if !matchesSelector(podLabels, selector) {
		t.Error("expected pod labels to match selector")
	}
}

func TestMatchesSelector_NoMatch(t *testing.T) {
	podLabels := map[string]string{
		"app":     "nginx",
		"version": "v1",
	}
	selector := map[string]string{
		"app":     "redis",
		"version": "v1",
	}

	if matchesSelector(podLabels, selector) {
		t.Error("expected pod labels to NOT match selector")
	}
}

func TestMatchesSelector_EmptySelector(t *testing.T) {
	podLabels := map[string]string{
		"app": "nginx",
	}

	if matchesSelector(podLabels, map[string]string{}) {
		t.Error("expected empty selector to not match")
	}
}

func TestMatchesSelector_NilSelector(t *testing.T) {
	podLabels := map[string]string{
		"app": "nginx",
	}

	if matchesSelector(podLabels, nil) {
		t.Error("expected nil selector to not match")
	}
}

func TestMatchesSelector_MissingLabel(t *testing.T) {
	podLabels := map[string]string{
		"app": "nginx",
	}
	selector := map[string]string{
		"app":     "nginx",
		"version": "v1",
	}

	if matchesSelector(podLabels, selector) {
		t.Error("expected missing label to cause no match")
	}
}
