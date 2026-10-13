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
	domain "github.com/datdt/k8sselfhost/internal/domain/vfswatchdog"
)

// mockVFSWatchdog implements domain.VFSWatchdog for HTTP handler unit tests.
type mockVFSWatchdog struct {
	scanInodesFunc          func(ctx context.Context) ([]domain.InodeReport, error)
	findTopConsumersFunc    func(ctx context.Context, mountPoint string, limit int) ([]string, error)
	detectDStateFunc        func(ctx context.Context, maxDuration time.Duration) ([]domain.DStateProcessInfo, error)
	remountOrUnmountFunc    func(ctx context.Context, mountPoint string) error
	enforcePIDLimitsFunc    func(ctx context.Context, maxPIDsPerPod int) error
}

func (m *mockVFSWatchdog) ScanInodes(ctx context.Context) ([]domain.InodeReport, error) {
	if m.scanInodesFunc != nil {
		return m.scanInodesFunc(ctx)
	}
	return nil, nil
}

func (m *mockVFSWatchdog) FindTopInodeConsumers(ctx context.Context, mountPoint string, limit int) ([]string, error) {
	if m.findTopConsumersFunc != nil {
		return m.findTopConsumersFunc(ctx, mountPoint, limit)
	}
	return nil, nil
}

func (m *mockVFSWatchdog) DetectDStateProcesses(ctx context.Context, maxDuration time.Duration) ([]domain.DStateProcessInfo, error) {
	if m.detectDStateFunc != nil {
		return m.detectDStateFunc(ctx, maxDuration)
	}
	return nil, nil
}

func (m *mockVFSWatchdog) RemountOrUnmountHungNFS(ctx context.Context, mountPoint string) error {
	if m.remountOrUnmountFunc != nil {
		return m.remountOrUnmountFunc(ctx, mountPoint)
	}
	return nil
}

func (m *mockVFSWatchdog) EnforcePIDLimits(ctx context.Context, maxPIDsPerPod int) error {
	if m.enforcePIDLimitsFunc != nil {
		return m.enforcePIDLimitsFunc(ctx, maxPIDsPerPod)
	}
	return nil
}

func setupVFSTestRouter(watchdog domain.VFSWatchdog) http.Handler {
	r := chi.NewRouter()
	handler := adapterhttp.NewVFSWatchdogHandler(watchdog)
	handler.RegisterAPIRoutes(r)
	return r
}

func TestVFSWatchdogHandler_GetInodeReport_Success(t *testing.T) {
	mock := &mockVFSWatchdog{
		scanInodesFunc: func(ctx context.Context) ([]domain.InodeReport, error) {
			return []domain.InodeReport{
				{
					MountPoint:      "/",
					TotalInodes:     1000000,
					FreeInodes:      800000,
					UsagePercentage: 20.0,
				},
			}, nil
		},
	}

	router := setupVFSTestRouter(mock)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/vfs/inodes/report", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if count, ok := resp["count"].(float64); !ok || int(count) != 1 {
		t.Errorf("expected count 1, got %v", resp["count"])
	}
}

func TestVFSWatchdogHandler_GetInodeReport_Exhaustion507(t *testing.T) {
	mock := &mockVFSWatchdog{
		scanInodesFunc: func(ctx context.Context) ([]domain.InodeReport, error) {
			return nil, domain.ErrVFSInodeExhaustion
		},
	}

	router := setupVFSTestRouter(mock)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/vfs/inodes/report", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	// Inode exhaustion returns HTTP 507 Insufficient Storage (RFC 7807)
	if rec.Code != http.StatusInsufficientStorage {
		t.Fatalf("expected status 507 Insufficient Storage, got %d", rec.Code)
	}

	var problem map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &problem); err != nil {
		t.Fatalf("failed to decode problem details: %v", err)
	}

	if code, ok := problem["code"].(string); !ok || code != domain.CodeVFSInodeExhaustion {
		t.Errorf("expected error code %s, got %v", domain.CodeVFSInodeExhaustion, problem["code"])
	}
}

func TestVFSWatchdogHandler_GetTopInodeCulprits_Validation(t *testing.T) {
	mock := &mockVFSWatchdog{}
	router := setupVFSTestRouter(mock)

	// Missing mount parameter
	req := httptest.NewRequest(http.MethodGet, "/api/v1/vfs/inodes/top-culprits", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for missing mount parameter, got %d", rec.Code)
	}

	// Invalid limit parameter
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/vfs/inodes/top-culprits?mount=/&limit=invalid", nil)
	rec2 := httptest.NewRecorder()
	router.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for invalid limit, got %d", rec2.Code)
	}
}

func TestVFSWatchdogHandler_GetTopInodeCulprits_Success(t *testing.T) {
	mock := &mockVFSWatchdog{
		findTopConsumersFunc: func(ctx context.Context, mountPoint string, limit int) ([]string, error) {
			return []string{"/tmp/cache", "/tmp/artifacts"}, nil
		},
	}

	router := setupVFSTestRouter(mock)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/vfs/inodes/top-culprits?mount=/tmp&limit=5", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if mount, ok := resp["mount_point"].(string); !ok || mount != "/tmp" {
		t.Errorf("expected mount_point /tmp, got %v", resp["mount_point"])
	}
}

func TestVFSWatchdogHandler_GetDStateProcesses_Deadlock500(t *testing.T) {
	mock := &mockVFSWatchdog{
		detectDStateFunc: func(ctx context.Context, maxDuration time.Duration) ([]domain.DStateProcessInfo, error) {
			return nil, domain.ErrVFSDStateDeadlock
		},
	}

	router := setupVFSTestRouter(mock)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/vfs/processes/d-state?max_duration=5m", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500 for D-state deadlock, got %d", rec.Code)
	}

	var problem map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &problem); err != nil {
		t.Fatalf("failed to decode problem details: %v", err)
	}

	if code, ok := problem["code"].(string); !ok || code != domain.CodeVFSDStateDeadlock {
		t.Errorf("expected error code %s, got %v", domain.CodeVFSDStateDeadlock, problem["code"])
	}
}

func TestVFSWatchdogHandler_GetDStateProcesses_Success(t *testing.T) {
	mock := &mockVFSWatchdog{
		detectDStateFunc: func(ctx context.Context, maxDuration time.Duration) ([]domain.DStateProcessInfo, error) {
			return []domain.DStateProcessInfo{
				{
					PID:          1234,
					Command:      "dd",
					WaitDuration: 10 * time.Second,
					WchanSymbol:  "io_schedule",
					MountPoint:   "/dev/sda",
				},
			}, nil
		},
	}

	router := setupVFSTestRouter(mock)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/vfs/processes/d-state?max_duration=1m", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
}

func TestVFSWatchdogHandler_LazyUnmount_SuccessAndFailure(t *testing.T) {
	var unmountedTarget string
	mock := &mockVFSWatchdog{
		remountOrUnmountFunc: func(ctx context.Context, mountPoint string) error {
			if mountPoint == "/mnt/error" {
				return domain.ErrNFSUnmountFailed
			}
			unmountedTarget = mountPoint
			return nil
		},
	}

	router := setupVFSTestRouter(mock)

	// Success case
	body := bytes.NewBufferString(`{"mount_point": "/mnt/nfs_hung"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/vfs/mounts/lazy-unmount", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	if unmountedTarget != "/mnt/nfs_hung" {
		t.Errorf("expected unmounted target /mnt/nfs_hung, got %s", unmountedTarget)
	}

	// Failure case
	failBody := bytes.NewBufferString(`{"mount_point": "/mnt/error"}`)
	reqFail := httptest.NewRequest(http.MethodPost, "/api/v1/vfs/mounts/lazy-unmount", failBody)
	reqFail.Header.Set("Content-Type", "application/json")
	recFail := httptest.NewRecorder()

	router.ServeHTTP(recFail, reqFail)

	if recFail.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500 on unmount failure, got %d", recFail.Code)
	}

	// Missing body parameter
	badBody := bytes.NewBufferString(`{"mount_point": ""}`)
	reqBad := httptest.NewRequest(http.MethodPost, "/api/v1/vfs/mounts/lazy-unmount", badBody)
	recBad := httptest.NewRecorder()

	router.ServeHTTP(recBad, reqBad)

	if recBad.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 on empty mount_point, got %d", recBad.Code)
	}
}

func TestVFSWatchdogHandler_EnforcePIDLimits(t *testing.T) {
	mock := &mockVFSWatchdog{
		enforcePIDLimitsFunc: func(ctx context.Context, maxPIDsPerPod int) error {
			if maxPIDsPerPod < 100 {
				return domain.ErrPIDQuotaExceeded
			}
			return nil
		},
	}

	router := setupVFSTestRouter(mock)

	// Success case
	body := bytes.NewBufferString(`{"max_pids_per_pod": 500}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/vfs/pids/enforce", body)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	// Quota exceeded case
	exceededBody := bytes.NewBufferString(`{"max_pids_per_pod": 50}`)
	reqExceeded := httptest.NewRequest(http.MethodPost, "/api/v1/vfs/pids/enforce", exceededBody)
	recExceeded := httptest.NewRecorder()

	router.ServeHTTP(recExceeded, reqExceeded)

	if recExceeded.Code != http.StatusTooManyRequests {
		t.Fatalf("expected status 429 Too Many Requests, got %d", recExceeded.Code)
	}
}
