package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"k8s.io/client-go/tools/remotecommand"

	"github.com/datdt/k8sselfhost/internal/infrastructure/cluster"
)

func TestNewK8sExecHandler(t *testing.T) {
	handler := NewK8sExecHandler(nil, nil)
	if handler == nil {
		t.Fatal("expected non-nil K8sExecHandler")
	}
}

func TestK8sExecHandler_MissingPod(t *testing.T) {
	handler := NewK8sExecHandler(nil, nil)

	r := chi.NewRouter()
	r.HandleFunc("/k8s/{cluster}/exec", handler.HandleExec)

	req := httptest.NewRequest(http.MethodGet, "/k8s/test-cluster/exec", nil)
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

func TestK8sExecHandler_K8sUnavailable(t *testing.T) {
	handler := NewK8sExecHandler(nil, nil)

	r := chi.NewRouter()
	r.HandleFunc("/k8s/{cluster}/exec", handler.HandleExec)

	req := httptest.NewRequest(http.MethodGet, "/k8s/test-cluster/exec?pod=test-pod", nil)
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

func TestTerminalSizeQueue(t *testing.T) {
	queue := newTerminalSizeQueue()

	// Push resize event
	queue.resizeChan <- remotecommand.TerminalSize{Width: 120, Height: 40}

	size := queue.Next()
	if size == nil {
		t.Fatal("expected non-nil TerminalSize")
	}
	if size.Width != 120 || size.Height != 40 {
		t.Fatalf("expected 120x40, got %dx%d", size.Width, size.Height)
	}

	// Close queue
	close(queue.resizeChan)
	sizeAfterClose := queue.Next()
	if sizeAfterClose != nil {
		t.Fatalf("expected nil after close, got %v", sizeAfterClose)
	}
}

func TestWsExecMessageParsing(t *testing.T) {
	rawResize := []byte(`{"type":"resize","cols":100,"rows":30}`)
	var msg wsExecMessage
	if err := json.Unmarshal(rawResize, &msg); err != nil {
		t.Fatalf("failed to unmarshal resize message: %v", err)
	}
	if msg.Type != "resize" || msg.Cols != 100 || msg.Rows != 30 {
		t.Fatalf("unexpected parsed msg: %+v", msg)
	}

	rawStdin := []byte(`{"type":"stdin","data":"echo hello\n"}`)
	var stdinMsg wsExecMessage
	if err := json.Unmarshal(rawStdin, &stdinMsg); err != nil {
		t.Fatalf("failed to unmarshal stdin message: %v", err)
	}
	if stdinMsg.Type != "stdin" || stdinMsg.Data != "echo hello\n" {
		t.Fatalf("unexpected parsed stdin msg: %+v", stdinMsg)
	}
}

func TestK8sExecHandler_WithClientManager(t *testing.T) {
	cm := cluster.NewClientManager(nil)
	handler := NewK8sExecHandler(nil, cm)

	r := chi.NewRouter()
	r.HandleFunc("/k8s/{cluster}/exec", handler.HandleExec)

	req := httptest.NewRequest(http.MethodGet, "/k8s/non-existent-cluster/exec?pod=my-pod", nil)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	// Since fleetRepo is nil and cluster doesn't exist, it should return 503 or 500 error
	if rec.Code != http.StatusServiceUnavailable && rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected error status, got %d", rec.Code)
	}
}
