package kubernetes

import (
	"context"
	"testing"

	"k8s.io/client-go/kubernetes/fake"
)

func TestKubeletVolumeMetricsProvider_NilClient(t *testing.T) {
	provider := NewKubeletVolumeMetricsProvider()
	used, err := provider.GetVolumeMetrics(context.Background(), nil, "default", "pvc-1")
	if err == nil {
		t.Fatal("expected error for nil client")
	}
	if used != nil {
		t.Errorf("expected nil used bytes, got %v", used)
	}
}

func TestKubeletVolumeMetricsProvider_FakeClient_GracefulFallback(t *testing.T) {
	provider := NewKubeletVolumeMetricsProvider()
	fakeClient := fake.NewSimpleClientset()
	used, err := provider.GetVolumeMetrics(context.Background(), fakeClient, "default", "pvc-1")
	if err == nil {
		t.Fatal("expected error for fake client without live metrics server")
	}
	if used != nil {
		t.Errorf("expected nil used bytes on error, got %v", used)
	}
}
