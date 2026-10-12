package network_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap/zaptest"
	admissionv1 "k8s.io/api/admission/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"

	usecasenet "github.com/datdt/k8sselfhost/internal/usecase/network"
)

func TestDNS_NDotsMutation(t *testing.T) {
	// 1. Normal pod without dnsConfig
	pod := corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "payment-service",
			Namespace: "production",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "web",
					Image: "nginx:latest",
				},
			},
		},
	}

	podBytes, err := json.Marshal(pod)
	if err != nil {
		t.Fatalf("Failed to marshal pod: %v", err)
	}

	reviewReq := admissionv1.AdmissionReview{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "admission.k8s.io/v1",
			Kind:       "AdmissionReview",
		},
		Request: &admissionv1.AdmissionRequest{
			UID: "705ab4f5-6393-11e8-b7cc-42010a800002",
			Object: runtime.RawExtension{
				Raw: podBytes,
			},
		},
	}

	reviewReqBytes, err := json.Marshal(reviewReq)
	if err != nil {
		t.Fatalf("Failed to marshal admission review request: %v", err)
	}

	// 2. Execute Webhook AdmissionReview
	respBytes, err := usecasenet.HandleDNSMutationAdmissionReview(reviewReqBytes)
	if err != nil {
		t.Fatalf("HandleDNSMutationAdmissionReview returned error: %v", err)
	}

	var reviewResp admissionv1.AdmissionReview
	if err := json.Unmarshal(respBytes, &reviewResp); err != nil {
		t.Fatalf("Failed to unmarshal admission review response: %v", err)
	}

	if reviewResp.Response == nil {
		t.Fatalf("Admission response is nil")
	}
	if !reviewResp.Response.Allowed {
		t.Errorf("Expected allowed=true, got false")
	}
	if reviewResp.Response.PatchType == nil || *reviewResp.Response.PatchType != admissionv1.PatchTypeJSONPatch {
		t.Fatalf("Expected PatchType=JSONPatch, got %v", reviewResp.Response.PatchType)
	}

	patchStr := string(reviewResp.Response.Patch)
	if !strings.Contains(patchStr, "ndots") || !strings.Contains(patchStr, `"2"`) {
		t.Errorf("Expected patch to contain ndots:2, got patch: %s", patchStr)
	}
	if !strings.Contains(patchStr, "single-request-reopen") {
		t.Errorf("Expected patch to contain single-request-reopen, got patch: %s", patchStr)
	}

	// 3. Test Pod that already has ndots:5 - should update to ndots:2
	val5 := "5"
	podWithNdots5 := pod.DeepCopy()
	podWithNdots5.Spec.DNSConfig = &corev1.PodDNSConfig{
		Options: []corev1.PodDNSConfigOption{
			{
				Name:  "ndots",
				Value: &val5,
			},
		},
	}

	patch5, mutated5, err5 := usecasenet.GenerateDNSConfigJSONPatch(podWithNdots5)
	if err5 != nil {
		t.Fatalf("GenerateDNSConfigJSONPatch error: %v", err5)
	}
	if !mutated5 {
		t.Fatalf("Expected mutated=true for ndots:5 pod")
	}
	if !strings.Contains(string(patch5), `"2"`) {
		t.Errorf("Expected ndots to be changed to 2, got: %s", string(patch5))
	}

	// 4. Test Idempotency: Pod that already has ndots:2 and single-request-reopen
	val2 := "2"
	podOptimized := pod.DeepCopy()
	podOptimized.Spec.DNSConfig = &corev1.PodDNSConfig{
		Options: []corev1.PodDNSConfigOption{
			{
				Name:  "ndots",
				Value: &val2,
			},
			{
				Name: "single-request-reopen",
			},
		},
	}

	_, mutatedOpt, errOpt := usecasenet.GenerateDNSConfigJSONPatch(podOptimized)
	if errOpt != nil {
		t.Fatalf("GenerateDNSConfigJSONPatch error on optimized pod: %v", errOpt)
	}
	if mutatedOpt {
		t.Errorf("Expected mutated=false for already optimized pod (idempotency)")
	}
}

func TestDNS_InjectDNSConfigOptimization(t *testing.T) {
	ctx := context.Background()
	logger := zaptest.NewLogger(t)

	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cart-service",
			Namespace: "production",
		},
		Spec: appsv1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "cart", Image: "cart:v1"},
					},
				},
			},
		},
	}

	k8sClient := fake.NewSimpleClientset(deploy)
	usecase := usecasenet.NewNetworkUsecase(k8sClient, nil, nil, nil, logger)

	err := usecase.InjectDNSConfigOptimization(ctx, "cluster-1", "production")
	if err != nil {
		t.Fatalf("InjectDNSConfigOptimization returned error: %v", err)
	}

	updatedDeploy, err := k8sClient.AppsV1().Deployments("production").Get(ctx, "cart-service", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get updated deployment: %v", err)
	}

	dnsConfig := updatedDeploy.Spec.Template.Spec.DNSConfig
	if dnsConfig == nil {
		t.Fatalf("DNSConfig is nil on updated deployment")
	}

	hasNdots2 := false
	hasReopen := false
	for _, opt := range dnsConfig.Options {
		if opt.Name == "ndots" && opt.Value != nil && *opt.Value == "2" {
			hasNdots2 = true
		}
		if opt.Name == "single-request-reopen" {
			hasReopen = true
		}
	}

	if !hasNdots2 {
		t.Errorf("Updated deployment missing ndots:2 option, options: %+v", dnsConfig.Options)
	}
	if !hasReopen {
		t.Errorf("Updated deployment missing single-request-reopen option, options: %+v", dnsConfig.Options)
	}
}

func TestDNS_CheckDNSHealth(t *testing.T) {
	ctx := context.Background()
	logger := zaptest.NewLogger(t)

	replicas := int32(3)
	coreDNSDeploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "coredns",
			Namespace: "kube-system",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
		},
		Status: appsv1.DeploymentStatus{
			ReadyReplicas: 3,
		},
	}

	nodeLocalDS := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "node-local-dns",
			Namespace: "kube-system",
		},
		Status: appsv1.DaemonSetStatus{
			NumberReady: 3,
		},
	}

	k8sClient := fake.NewSimpleClientset(coreDNSDeploy, nodeLocalDS)
	metricsProvider := usecasenet.NewDefaultDNSMetricsProvider()
	usecase := usecasenet.NewNetworkUsecase(k8sClient, nil, nil, metricsProvider, logger)

	// Scenario 1: Critical Latency > 100ms -> Alert Triggered
	metricsProvider.SetMetrics("cluster-1", 125*time.Millisecond, 0.06)

	report, err := usecase.CheckDNSHealth(ctx, "cluster-1")
	if err != nil {
		t.Fatalf("CheckDNSHealth returned error: %v", err)
	}

	if !report.AlertTriggered {
		t.Errorf("Expected AlertTriggered=true for 125ms latency, got false")
	}
	if report.CoreDNSReplicas != 3 {
		t.Errorf("CoreDNSReplicas = %d, want 3", report.CoreDNSReplicas)
	}
	if !report.NodeLocalActive {
		t.Errorf("NodeLocalActive = false, want true")
	}
	if report.P99Latency != 125*time.Millisecond {
		t.Errorf("P99Latency = %v, want 125ms", report.P99Latency)
	}

	// Scenario 2: Healthy Latency 4ms -> No Alert
	metricsProvider.SetMetrics("cluster-1", 4*time.Millisecond, 0.001)

	reportHealthy, err := usecase.CheckDNSHealth(ctx, "cluster-1")
	if err != nil {
		t.Fatalf("CheckDNSHealth returned error: %v", err)
	}

	if reportHealthy.AlertTriggered {
		t.Errorf("Expected AlertTriggered=false for healthy metrics, got true")
	}
}

func TestDNS_ConcurrencySafety(t *testing.T) {
	ctx := context.Background()
	logger := zaptest.NewLogger(t)

	replicas := int32(2)
	coreDNSDeploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "coredns", Namespace: "kube-system"},
		Spec:       appsv1.DeploymentSpec{Replicas: &replicas},
		Status:     appsv1.DeploymentStatus{ReadyReplicas: 2},
	}

	k8sClient := fake.NewSimpleClientset(coreDNSDeploy)
	metricsProvider := usecasenet.NewDefaultDNSMetricsProvider()
	usecase := usecasenet.NewNetworkUsecase(k8sClient, nil, nil, metricsProvider, logger)

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			metricsProvider.SetMetrics("cluster-1", time.Duration(workerID)*time.Millisecond, 0.01)
			_, _ = usecase.CheckDNSHealth(ctx, "cluster-1")

			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("pod-%d", workerID)},
			}
			_, _, _ = usecasenet.GenerateDNSConfigJSONPatch(pod)
		}(i)
	}
	wg.Wait()
}
