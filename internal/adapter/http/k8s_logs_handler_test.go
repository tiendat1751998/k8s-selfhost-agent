package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sfake "k8s.io/client-go/kubernetes/fake"

	"github.com/datdt/k8sselfhost/internal/infrastructure/cluster"
	"github.com/datdt/k8sselfhost/internal/pkg/tenancy"
)

func TestNewK8sLogsHandler(t *testing.T) {
	handler := NewK8sLogsHandler(nil, nil)
	if handler == nil {
		t.Fatal("expected non-nil K8sLogsHandler")
	}
}

func TestK8sLogsHandler_MissingPod(t *testing.T) {
	handler := NewK8sLogsHandler(nil, nil)

	r := chi.NewRouter()
	r.Get("/k8s/{cluster}/logs", handler.HandlePodLogs)

	req := httptest.NewRequest(http.MethodGet, "/k8s/test-cluster/logs", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 Bad Request, got %d", rec.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["error"] != "pod is required" {
		t.Fatalf("expected error 'pod is required', got %q", resp["error"])
	}
}

func TestK8sLogsHandler_K8sUnavailable(t *testing.T) {
	handler := NewK8sLogsHandler(nil, nil)

	r := chi.NewRouter()
	r.Get("/k8s/{cluster}/logs/{pod}", handler.HandlePodLogs)

	req := httptest.NewRequest(http.MethodGet, "/k8s/test-cluster/logs/my-pod", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503 Service Unavailable, got %d", rec.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["code"] != "K8S_UNAVAILABLE" {
		t.Fatalf("expected code 'K8S_UNAVAILABLE', got %q", resp["code"])
	}
}

func TestK8sLogsHandler_WithClientManager(t *testing.T) {
	cm := cluster.NewClientManager(nil)
	handler := NewK8sLogsHandler(nil, cm)

	r := chi.NewRouter()
	r.Get("/k8s/{cluster}/pods/{pod}/logs", handler.HandlePodLogs)

	req := httptest.NewRequest(http.MethodGet, "/k8s/non-existent-cluster/pods/my-pod/logs?follow=true&tailLines=50", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable && rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected error status, got %d", rec.Code)
	}
}

func TestK8sLogsHandler_NamespaceAuthorization(t *testing.T) {
	handler := NewK8sLogsHandler(nil, nil)

	r := chi.NewRouter()
	r.Get("/k8s/{cluster}/pods/{pod}/logs", handler.HandlePodLogs)

	tenantCtx := tenancy.WithUserRole(context.Background(), "tenant_admin")
	tenantCtx = tenancy.WithTenantID(tenantCtx, "tenant-a")

	adminCtx := tenancy.WithUserRole(context.Background(), "platform_admin")
	adminCtx = tenancy.WithTenantID(adminCtx, "platform")

	t.Run("tenant-a rejected with 403 on kube-system logs", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/k8s/local/pods/my-pod/logs?ns=kube-system", nil).WithContext(tenantCtx)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403 Forbidden on kube-system logs, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("tenant-a rejected with 403 on other tenant logs", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/k8s/local/pods/my-pod/logs?ns=tenant-b", nil).WithContext(tenantCtx)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("expected 403 Forbidden on tenant-b logs, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("platform_admin allowed on kube-system logs", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/k8s/local/pods/my-pod/logs?ns=kube-system", nil).WithContext(adminCtx)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code == http.StatusForbidden {
			t.Fatalf("expected platform_admin NOT to get 403 on kube-system logs, got 403")
		}
	})
}

func TestWorkloadLogs_DeploymentResolution(t *testing.T) {
	fakeClient := k8sfake.NewSimpleClientset(
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "ingress-traefik",
				Namespace: "default",
			},
			Spec: appsv1.DeploymentSpec{
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"app": "traefik"},
				},
			},
		},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "ingress-traefik-abc",
				Namespace: "default",
				Labels:    map[string]string{"app": "traefik"},
			},
			Status: corev1.PodStatus{Phase: corev1.PodRunning},
		},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "ingress-traefik-def",
				Namespace: "default",
				Labels:    map[string]string{"app": "traefik"},
			},
			Status: corev1.PodStatus{Phase: corev1.PodRunning},
		},
	)

	handler := NewK8sLogsHandler(fakeClient, nil)
	r := chi.NewRouter()
	r.Get("/k8s/{cluster}/workloads/{kind}/{name}/logs", handler.HandleWorkloadLogs)
	r.Get("/k8s/{cluster}/deployments/{name}/logs", handler.HandleWorkloadLogs)

	req := httptest.NewRequest(http.MethodGet, "/k8s/local/workloads/deployments/ingress-traefik/logs?namespace=default", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Workload   string   `json:"workload"`
		Namespace  string   `json:"namespace"`
		Pods       []string `json:"pods"`
		Logs       string   `json:"logs"`
		TotalLines int      `json:"total_lines"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Workload != "ingress-traefik" {
		t.Errorf("expected workload 'ingress-traefik', got %q", resp.Workload)
	}
	if resp.Namespace != "default" {
		t.Errorf("expected namespace 'default', got %q", resp.Namespace)
	}
	if len(resp.Pods) != 2 {
		t.Errorf("expected 2 pods, got %d (%v)", len(resp.Pods), resp.Pods)
	}
	if !strings.Contains(resp.Logs, "[ingress-traefik-abc]") || !strings.Contains(resp.Logs, "[ingress-traefik-def]") {
		t.Errorf("expected logs to contain pod prefixes, got %q", resp.Logs)
	}
	if resp.TotalLines <= 0 {
		t.Errorf("expected total_lines > 0, got %d", resp.TotalLines)
	}

	// Also verify the /deployments/{name}/logs shortcut route
	reqDeploy := httptest.NewRequest(http.MethodGet, "/k8s/local/deployments/ingress-traefik/logs?namespace=default", nil)
	recDeploy := httptest.NewRecorder()
	r.ServeHTTP(recDeploy, reqDeploy)
	if recDeploy.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK on deployments shortcut route, got %d", recDeploy.Code)
	}
}

func TestWorkloadLogs_SpecificPod(t *testing.T) {
	fakeClient := k8sfake.NewSimpleClientset(
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "ingress-traefik",
				Namespace: "default",
			},
			Spec: appsv1.DeploymentSpec{
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"app": "traefik"},
				},
			},
		},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "ingress-traefik-abc",
				Namespace: "default",
				Labels:    map[string]string{"app": "traefik"},
			},
			Status: corev1.PodStatus{Phase: corev1.PodRunning},
		},
	)

	handler := NewK8sLogsHandler(fakeClient, nil)
	r := chi.NewRouter()
	r.Get("/k8s/{cluster}/workloads/{kind}/{name}/logs", handler.HandleWorkloadLogs)

	req := httptest.NewRequest(http.MethodGet, "/k8s/local/workloads/deployments/ingress-traefik/logs?namespace=default&pod=ingress-traefik-abc", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Workload   string   `json:"workload"`
		Namespace  string   `json:"namespace"`
		Pods       []string `json:"pods"`
		Logs       string   `json:"logs"`
		TotalLines int      `json:"total_lines"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Workload != "ingress-traefik" {
		t.Errorf("expected workload 'ingress-traefik', got %q", resp.Workload)
	}
	if len(resp.Pods) != 1 || resp.Pods[0] != "ingress-traefik-abc" {
		t.Errorf("expected pods to contain only 'ingress-traefik-abc', got %v", resp.Pods)
	}
	if resp.Logs == "" {
		t.Errorf("expected non-empty logs for specific pod")
	}
}

func TestWorkloadLogs_ZeroReadyPods(t *testing.T) {
	fakeClient := k8sfake.NewSimpleClientset(
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "zero-pod-deploy",
				Namespace: "default",
			},
			Spec: appsv1.DeploymentSpec{
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"app": "zero"},
				},
			},
		},
	)

	handler := NewK8sLogsHandler(fakeClient, nil)
	r := chi.NewRouter()
	r.Get("/k8s/{cluster}/workloads/{kind}/{name}/logs", handler.HandleWorkloadLogs)

	req := httptest.NewRequest(http.MethodGet, "/k8s/local/workloads/deployments/zero-pod-deploy?namespace=default", nil)
	// Missing /logs path in URL -> test with /logs
	req = httptest.NewRequest(http.MethodGet, "/k8s/local/workloads/deployments/zero-pod-deploy/logs?namespace=default", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK for 0 ready pods, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Workload   string   `json:"workload"`
		Namespace  string   `json:"namespace"`
		Pods       []string `json:"pods"`
		Logs       string   `json:"logs"`
		TotalLines int      `json:"total_lines"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(resp.Pods) != 0 {
		t.Errorf("expected 0 pods, got %v", resp.Pods)
	}
	if resp.Logs != "" {
		t.Errorf("expected empty logs string, got %q", resp.Logs)
	}
	if resp.TotalLines != 0 {
		t.Errorf("expected total_lines == 0, got %d", resp.TotalLines)
	}
}

func TestWorkloadLogs_NotFound(t *testing.T) {
	fakeClient := k8sfake.NewSimpleClientset()
	handler := NewK8sLogsHandler(fakeClient, nil)

	r := chi.NewRouter()
	r.Get("/k8s/{cluster}/workloads/{kind}/{name}/logs", handler.HandleWorkloadLogs)

	req := httptest.NewRequest(http.MethodGet, "/k8s/local/workloads/deployments/nonexistent-deploy/logs?namespace=default", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404 Not Found, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestWorkloadLogs_NamespaceRBAC(t *testing.T) {
	fakeClient := k8sfake.NewSimpleClientset()
	handler := NewK8sLogsHandler(fakeClient, nil)

	r := chi.NewRouter()
	r.Get("/k8s/{cluster}/workloads/{kind}/{name}/logs", handler.HandleWorkloadLogs)

	tenantCtx := tenancy.WithUserRole(context.Background(), "tenant_admin")
	tenantCtx = tenancy.WithTenantID(tenantCtx, "tenant-a")

	req := httptest.NewRequest(http.MethodGet, "/k8s/local/workloads/deployments/dep-1/logs?namespace=kube-system", nil).WithContext(tenantCtx)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for tenant accessing kube-system, got %d", rec.Code)
	}
}

func TestWorkloadLogs_StatefulSetAndDaemonSet(t *testing.T) {
	fakeClient := k8sfake.NewSimpleClientset(
		&appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "redis-cluster",
				Namespace: "default",
			},
			Spec: appsv1.StatefulSetSpec{
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"app": "redis"},
				},
			},
		},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "redis-cluster-0",
				Namespace: "default",
				Labels:    map[string]string{"app": "redis"},
			},
			Status: corev1.PodStatus{Phase: corev1.PodRunning},
		},
		&appsv1.DaemonSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "fluentd",
				Namespace: "default",
			},
			Spec: appsv1.DaemonSetSpec{
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"app": "fluentd"},
				},
			},
		},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "fluentd-node1",
				Namespace: "default",
				Labels:    map[string]string{"app": "fluentd"},
			},
			Status: corev1.PodStatus{Phase: corev1.PodRunning},
		},
	)

	handler := NewK8sLogsHandler(fakeClient, nil)
	r := chi.NewRouter()
	r.Get("/k8s/{cluster}/workloads/{kind}/{name}/logs", handler.HandleWorkloadLogs)

	t.Run("statefulset resolution", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/k8s/local/workloads/statefulsets/redis-cluster/logs?namespace=default", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK for statefulset, got %d: %s", rec.Code, rec.Body.String())
		}
		var resp struct {
			Pods []string `json:"pods"`
		}
		_ = json.NewDecoder(rec.Body).Decode(&resp)
		if len(resp.Pods) != 1 || resp.Pods[0] != "redis-cluster-0" {
			t.Errorf("expected pod redis-cluster-0, got %v", resp.Pods)
		}
	})

	t.Run("daemonset resolution", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/k8s/local/workloads/daemonsets/fluentd/logs?namespace=default", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK for daemonset, got %d: %s", rec.Code, rec.Body.String())
		}
		var resp struct {
			Pods []string `json:"pods"`
		}
		_ = json.NewDecoder(rec.Body).Decode(&resp)
		if len(resp.Pods) != 1 || resp.Pods[0] != "fluentd-node1" {
			t.Errorf("expected pod fluentd-node1, got %v", resp.Pods)
		}
	})
}

func TestWorkloadLogs_FollowStreaming(t *testing.T) {
	fakeClient := k8sfake.NewSimpleClientset(
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "streaming-deploy",
				Namespace: "default",
			},
			Spec: appsv1.DeploymentSpec{
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"app": "streaming"},
				},
			},
		},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "streaming-pod-1",
				Namespace: "default",
				Labels:    map[string]string{"app": "streaming"},
			},
			Status: corev1.PodStatus{Phase: corev1.PodRunning},
		},
	)

	handler := NewK8sLogsHandler(fakeClient, nil)
	r := chi.NewRouter()
	r.Get("/k8s/{cluster}/workloads/{kind}/{name}/logs", handler.HandleWorkloadLogs)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := httptest.NewRequest(http.MethodGet, "/k8s/local/workloads/deployments/streaming-deploy/logs?namespace=default&follow=true&tailLines=5000", nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}
	if rec.Header().Get("Content-Type") != "text/event-stream" {
		t.Errorf("expected Content-Type text/event-stream, got %q", rec.Header().Get("Content-Type"))
	}
	body := rec.Body.String()
	if !strings.Contains(body, "[streaming-pod-1]") {
		t.Errorf("expected SSE body to contain [streaming-pod-1], got %q", body)
	}
}

