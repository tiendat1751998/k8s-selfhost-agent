package mesh_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	domainmesh "github.com/datdt/k8sselfhost/internal/domain/mesh"
	usecasemesh "github.com/datdt/k8sselfhost/internal/usecase/mesh"
)

func TestSidecar_InjectHoldApplication(t *testing.T) {
	svc := usecasemesh.NewLifecycleService(usecasemesh.WithDefaultK8sVersion("1.27"))

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "orders-service",
			Namespace: "production",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "orders-app",
					Image: "orders-app:v1.0.0",
				},
				{
					Name:  "istio-proxy",
					Image: "docker.io/istio/proxyv2:1.18.0",
				},
			},
		},
	}

	podBytes, err := json.Marshal(pod)
	if err != nil {
		t.Fatalf("marshaling pod: %v", err)
	}

	ctx := context.Background()
	patchBytes, err := svc.InjectSidecarStartupGuard(ctx, podBytes)
	if err != nil {
		t.Fatalf("InjectSidecarStartupGuard returned error: %v", err)
	}

	patchStr := string(patchBytes)
	if !strings.Contains(patchStr, "holdApplicationUntilProxyStarts") || !strings.Contains(patchStr, "true") {
		t.Fatalf("expected patch to contain holdApplicationUntilProxyStarts: true, got: %s", patchStr)
	}
	if !strings.Contains(patchStr, "proxy.istio.io") {
		t.Fatalf("expected patch to target proxy.istio.io annotation, got: %s", patchStr)
	}
}

func TestSidecar_NativeSidecarK8s128(t *testing.T) {
	svc := usecasemesh.NewLifecycleService(usecasemesh.WithDefaultK8sVersion("1.28"))

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "payment-service",
			Namespace: "production",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "payment-app",
					Image: "payment:v2.1.0",
				},
				{
					Name:  "istio-proxy",
					Image: "docker.io/istio/proxyv2:1.20.0",
				},
			},
		},
	}

	podBytes, err := json.Marshal(pod)
	if err != nil {
		t.Fatalf("marshaling pod: %v", err)
	}

	ctx := context.Background()
	patchBytes, err := svc.InjectSidecarStartupGuard(ctx, podBytes)
	if err != nil {
		t.Fatalf("InjectSidecarStartupGuard returned error: %v", err)
	}

	patchStr := string(patchBytes)
	if !strings.Contains(patchStr, "initContainers") {
		t.Fatalf("expected patch to add/update initContainers, got: %s", patchStr)
	}
	if !strings.Contains(patchStr, "Always") {
		t.Fatalf("expected patch to set restartPolicy: Always, got: %s", patchStr)
	}
}

func TestSidecar_ControlPlaneExemption(t *testing.T) {
	svc := usecasemesh.NewLifecycleService()

	kubeSystemPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "coredns-5d78c9869d-abcd",
			Namespace: "kube-system",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "coredns",
					Image: "coredns:v1.10.1",
				},
			},
		},
	}

	podBytes, err := json.Marshal(kubeSystemPod)
	if err != nil {
		t.Fatalf("marshaling pod: %v", err)
	}

	patchBytes, err := svc.InjectSidecarStartupGuard(context.Background(), podBytes)
	if err != nil {
		t.Fatalf("unexpected error for exempt namespace: %v", err)
	}

	if string(patchBytes) != "[]" && len(patchBytes) != 0 {
		t.Fatalf("expected empty patch for control plane pod, got: %s", string(patchBytes))
	}
}

func TestWAF_RejectCatastrophicRegex(t *testing.T) {
	svc := usecasemesh.NewLifecycleService()

	catastrophicPatterns := []string{
		`(a+)+$`,
		`(x+)*`,
		`(?:.*=.*)+`,
		`([0-9]+)+`,
		`((a+))+`,
		`(a*)*`,
	}

	for _, pattern := range catastrophicPatterns {
		report, err := svc.VerifyWAFRegexSafety(pattern)
		if report == nil {
			t.Fatalf("expected report to be non-nil for pattern %q", pattern)
		}
		if report.Verdict != "REJECTED" {
			t.Errorf("expected pattern %q to be REJECTED, got %s", pattern, report.Verdict)
		}
		if !report.HasReDoSBacktrack {
			t.Errorf("expected HasReDoSBacktrack to be true for pattern %q", pattern)
		}
		if report.IsLinearDFA {
			t.Errorf("expected IsLinearDFA to be false for pattern %q", pattern)
		}
		if !errors.Is(err, domainmesh.ErrWAFNonLinearRegex) {
			t.Errorf("expected error ErrWAFNonLinearRegex for pattern %q, got %v", pattern, err)
		}
	}
}

func TestWAF_RejectLookaroundAndBackreference(t *testing.T) {
	svc := usecasemesh.NewLifecycleService()

	nonLinearPatterns := []string{
		`([a-z]+)\1`,
		`(?=abc).*`,
		`.*(?!bad)`,
		`(?<=foo)bar`,
		`(?<!foo)bar`,
	}

	for _, pattern := range nonLinearPatterns {
		report, err := svc.VerifyWAFRegexSafety(pattern)
		if report == nil {
			t.Fatalf("expected report to be non-nil for pattern %q", pattern)
		}
		if report.Verdict != "REJECTED" {
			t.Errorf("expected pattern %q to be REJECTED, got %s", pattern, report.Verdict)
		}
		if !errors.Is(err, domainmesh.ErrWAFNonLinearRegex) {
			t.Errorf("expected ErrWAFNonLinearRegex for pattern %q, got %v", pattern, err)
		}
	}
}

func TestWAF_AcceptLinearRegex(t *testing.T) {
	svc := usecasemesh.NewLifecycleService()

	linearPatterns := []string{
		`^[a-zA-Z0-9_\-\.]+@[a-zA-Z0-9\-]+\.[a-zA-Z0-9\-\.]+$`,
		`^/api/v1/[a-z0-9_-]+$`,
		`Bearer [A-Za-z0-9\-_=]+`,
		`^https?://[a-zA-Z0-9.-]+(:[0-9]+)?(/.*)?$`,
	}

	for _, pattern := range linearPatterns {
		report, err := svc.VerifyWAFRegexSafety(pattern)
		if err != nil {
			t.Fatalf("expected nil error for valid linear pattern %q, got: %v", pattern, err)
		}
		if report.Verdict != "APPROVED" {
			t.Errorf("expected pattern %q to be APPROVED, got %s", pattern, report.Verdict)
		}
		if !report.IsLinearDFA {
			t.Errorf("expected IsLinearDFA=true for pattern %q", pattern)
		}
		if report.HasReDoSBacktrack {
			t.Errorf("expected HasReDoSBacktrack=false for pattern %q", pattern)
		}
	}
}

func TestWAF_ExecutionTimeoutStress(t *testing.T) {
	// Configure with 1ns execution timeout to verify hard cutoff
	svc := usecasemesh.NewLifecycleService(usecasemesh.WithWAFTimeout(1 * time.Nanosecond))

	pattern := `^[a-zA-Z0-9]+.*$`
	report, err := svc.VerifyWAFRegexSafety(pattern)
	if report == nil {
		t.Fatal("expected report to be non-nil")
	}
	if report.Verdict != "REJECTED" {
		t.Fatalf("expected timeout stress to yield REJECTED, got %s", report.Verdict)
	}
	if err == nil {
		t.Fatal("expected error on timeout")
	}
}

func TestConfigureGracefulTLSDrain_Validation(t *testing.T) {
	svc := usecasemesh.NewLifecycleService()

	if err := svc.ConfigureGracefulTLSDrain(context.Background(), "", "app", 10*time.Second); err == nil {
		t.Error("expected error on empty namespace")
	}
	if err := svc.ConfigureGracefulTLSDrain(context.Background(), "prod", "", 10*time.Second); err == nil {
		t.Error("expected error on empty deployment")
	}
	if err := svc.ConfigureGracefulTLSDrain(context.Background(), "prod", "app", 0); err == nil {
		t.Error("expected error on non-positive drain duration")
	}
	if err := svc.ConfigureGracefulTLSDrain(context.Background(), "prod", "app", 30*time.Second); err != nil {
		t.Errorf("unexpected error with nil client and valid args: %v", err)
	}
}

func BenchmarkWAFRegexVerification(b *testing.B) {
	svc := usecasemesh.NewLifecycleService()
	pattern := `^[a-zA-Z0-9_\-\.]+@[a-zA-Z0-9\-]+\.[a-zA-Z0-9\-\.]+$`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = svc.VerifyWAFRegexSafety(pattern)
	}
}
