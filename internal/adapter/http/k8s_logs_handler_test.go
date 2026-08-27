package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/datdt/k8sselfhost/internal/infrastructure/cluster"
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
