package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

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

