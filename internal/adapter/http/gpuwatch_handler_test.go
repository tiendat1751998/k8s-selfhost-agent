package http_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	adapterhttp "github.com/datdt/k8sselfhost/internal/adapter/http"
	"github.com/datdt/k8sselfhost/internal/domain/gpuwatch"
	usecase "github.com/datdt/k8sselfhost/internal/usecase/gpuwatch"
)

type mockClusterManager struct {
	cards       []gpuwatch.GPUCardTelemetry
	taintedNode string
	taintedXID  int
	evictedPod  string
	ncclHang    bool
}

func (m *mockClusterManager) ScanNodeGPUs(ctx context.Context, node string) ([]gpuwatch.GPUCardTelemetry, error) {
	return m.cards, nil
}

func (m *mockClusterManager) TaintFaultyGPUNode(ctx context.Context, node string, pciAddr string, xid int) error {
	m.taintedNode = node
	m.taintedXID = xid
	return nil
}

func (m *mockClusterManager) EvictZombiePod(ctx context.Context, podName, namespace string) error {
	m.evictedPod = namespace + "/" + podName
	return nil
}

func (m *mockClusterManager) CheckNCCLProgress(ctx context.Context, jobID string, maxIdleDuration time.Duration) (bool, error) {
	return m.ncclHang, nil
}

type mockKVSwapper struct {
	offloaded int
}

func (m *mockKVSwapper) OffloadKVCacheToHostRAM(ctx context.Context, workloadID string, numBlocks int) (int, error) {
	m.offloaded = numBlocks
	return numBlocks, nil
}

func setupGPUWatchTestRouter(cm *mockClusterManager, swapper *mockKVSwapper) *chi.Mux {
	service := usecase.NewGPUManagerService(cm, swapper)
	handler := adapterhttp.NewGPUWatchHandler(service)

	r := chi.NewRouter()
	r.Route("/api/v1/gpu", func(sub chi.Router) {
		handler.RegisterRoutes(sub)
	})
	return r
}

func TestGPUWatchHandler_GetNodeTelemetry(t *testing.T) {
	cm := &mockClusterManager{
		cards: []gpuwatch.GPUCardTelemetry{
			{
				Index:           0,
				PCIAddress:      "0000:01:00.0",
				Model:           "NVIDIA H100",
				VRAMTotalBytes:  80000000000,
				VRAMUsedBytes:   40000000000,
				VRAMUsagePct:    50.0,
				KVCacheUsagePct: 60.0,
				TemperatureC:    52,
				HealthState:     gpuwatch.GPUHealthy,
			},
		},
	}
	r := setupGPUWatchTestRouter(cm, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/gpu/nodes/worker-gpu-1/telemetry", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /telemetry code = %d; want %d. Body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse JSON response: %v", err)
	}
	if resp["node"] != "worker-gpu-1" {
		t.Errorf("resp.node = %v; want worker-gpu-1", resp["node"])
	}
}

func TestGPUWatchHandler_TaintFaultyNode_CriticalFault(t *testing.T) {
	cm := &mockClusterManager{}
	r := setupGPUWatchTestRouter(cm, nil)

	body := map[string]interface{}{
		"dmesg_log": "NVRM: Xid (PCI:0000:01:00.0): 31, pid=123, MMU Fault: ENGINE GRAPHICS",
		"affected_pods": []map[string]string{
			{"name": "trainer-rank0", "namespace": "ai-train"},
		},
	}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/gpu/nodes/node-h100-fail/taint-faulty", bytes.NewReader(b))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("POST /taint-faulty code = %d; want %d. Body: %s", rec.Code, http.StatusInternalServerError, rec.Body.String())
	}

	var prob adapterhttp.ProblemDetails
	if err := json.Unmarshal(rec.Body.Bytes(), &prob); err != nil {
		t.Fatalf("Failed to parse problem details: %v", err)
	}
	if prob.Code != gpuwatch.CodeGPUXIDHardwareFault {
		t.Errorf("prob.Code = %s; want %s", prob.Code, gpuwatch.CodeGPUXIDHardwareFault)
	}
	if cm.taintedNode != "node-h100-fail" || cm.taintedXID != 31 {
		t.Errorf("Expected node tainted with XID 31; got node=%s, xid=%d", cm.taintedNode, cm.taintedXID)
	}
	if cm.evictedPod != "ai-train/trainer-rank0" {
		t.Errorf("Expected evictedPod = ai-train/trainer-rank0; got %s", cm.evictedPod)
	}
}

func TestGPUWatchHandler_TaintFaultyNode_NonCritical_Guardrail(t *testing.T) {
	cm := &mockClusterManager{}
	r := setupGPUWatchTestRouter(cm, nil)

	body := map[string]interface{}{
		"dmesg_log": "NVRM: Xid (PCI:0000:01:00.0): 13, pid=123, Graphics Exception",
		"affected_pods": []map[string]string{
			{"name": "trainer-rank0", "namespace": "ai-train"},
		},
	}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/gpu/nodes/node-h100-warning/taint-faulty", bytes.NewReader(b))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("POST /taint-faulty non-critical code = %d; want %d. Body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if cm.taintedNode != "" {
		t.Errorf("Guardrail failed: node was tainted: %s", cm.taintedNode)
	}
	if cm.evictedPod != "" {
		t.Errorf("Guardrail failed: pod was evicted: %s", cm.evictedPod)
	}
}

func TestGPUWatchHandler_CheckKVCache_Preemption(t *testing.T) {
	swapper := &mockKVSwapper{}
	r := setupGPUWatchTestRouter(&mockClusterManager{}, swapper)

	// High pressure >= 85%: triggers 429
	highBody := map[string]interface{}{
		"workload_id":  "vllm-mistral",
		"usage_pct":    91.2,
		"total_blocks": 2048,
	}
	bHigh, _ := json.Marshal(highBody)

	reqHigh := httptest.NewRequest(http.MethodPost, "/api/v1/gpu/inference/kv-cache-check", bytes.NewReader(bHigh))
	recHigh := httptest.NewRecorder()
	r.ServeHTTP(recHigh, reqHigh)

	if recHigh.Code != http.StatusTooManyRequests {
		t.Fatalf("POST /kv-cache-check code = %d; want %d (429). Body: %s", recHigh.Code, http.StatusTooManyRequests, recHigh.Body.String())
	}

	var prob adapterhttp.ProblemDetails
	if err := json.Unmarshal(recHigh.Body.Bytes(), &prob); err != nil {
		t.Fatalf("Failed to parse ProblemDetails: %v", err)
	}
	if prob.Code != gpuwatch.CodeKVCacheOOMRisk {
		t.Errorf("prob.Code = %s; want %s", prob.Code, gpuwatch.CodeKVCacheOOMRisk)
	}
	if swapper.offloaded <= 0 {
		t.Errorf("Expected blocks offloaded; got %d", swapper.offloaded)
	}

	// Safe pressure < 85%: 200 OK
	safeBody := map[string]interface{}{
		"workload_id":  "vllm-mistral",
		"usage_pct":    65.0,
		"total_blocks": 2048,
	}
	bSafe, _ := json.Marshal(safeBody)

	reqSafe := httptest.NewRequest(http.MethodPost, "/api/v1/gpu/inference/kv-cache-check", bytes.NewReader(bSafe))
	recSafe := httptest.NewRecorder()
	r.ServeHTTP(recSafe, reqSafe)

	if recSafe.Code != http.StatusOK {
		t.Fatalf("POST /kv-cache-check safe code = %d; want 200. Body: %s", recSafe.Code, recSafe.Body.String())
	}
}

func TestGPUWatchHandler_CheckNCCLProgress(t *testing.T) {
	cm := &mockClusterManager{
		ncclHang: true,
	}
	r := setupGPUWatchTestRouter(cm, nil)

	// Deadlocked job (> 5m idle)
	body := map[string]interface{}{
		"idle_duration_sec": 360,
		"namespace":         "ai-train",
		"pod_name":          "megatron-worker-1",
	}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/gpu/jobs/job-llama-64nodes/nccl-progress", bytes.NewReader(b))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusGatewayTimeout {
		t.Fatalf("POST /nccl-progress hang code = %d; want %d. Body: %s", rec.Code, http.StatusGatewayTimeout, rec.Body.String())
	}

	var prob adapterhttp.ProblemDetails
	if err := json.Unmarshal(rec.Body.Bytes(), &prob); err != nil {
		t.Fatalf("Failed to parse ProblemDetails: %v", err)
	}
	if prob.Code != gpuwatch.CodeNCCLHangDetected {
		t.Errorf("prob.Code = %s; want %s", prob.Code, gpuwatch.CodeNCCLHangDetected)
	}
	if cm.evictedPod != "ai-train/megatron-worker-1" {
		t.Errorf("Expected evicted worker pod; got %s", cm.evictedPod)
	}
}
