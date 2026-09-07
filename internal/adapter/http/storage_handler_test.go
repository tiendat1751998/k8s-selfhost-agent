package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/datdt/k8sselfhost/internal/adapter/http/middleware"
	"github.com/datdt/k8sselfhost/internal/domain/storage"
	domainerrors "github.com/datdt/k8sselfhost/internal/pkg/errors"
)

type mockVolumeService struct {
	volumes         []storage.DistributedVolume
	listErr         error
	expandErr       error
	snapshotErr     error
	lastExpanded    struct{ cluster, namespace, name string; size int64 }
	lastSnapshotted struct{ cluster, namespace, name string; req storage.VolumeSnapshotRequest }
}

func (m *mockVolumeService) ListVolumes(ctx context.Context, clusterID string) ([]storage.DistributedVolume, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	return m.volumes, nil
}

func (m *mockVolumeService) ExpandVolume(ctx context.Context, clusterID, namespace, name string, newSizeBytes int64) error {
	m.lastExpanded.cluster = clusterID
	m.lastExpanded.namespace = namespace
	m.lastExpanded.name = name
	m.lastExpanded.size = newSizeBytes
	return m.expandErr
}

func (m *mockVolumeService) CreateSnapshot(ctx context.Context, clusterID, namespace, name string, req storage.VolumeSnapshotRequest) (*storage.VolumeSnapshotResult, error) {
	m.lastSnapshotted.cluster = clusterID
	m.lastSnapshotted.namespace = namespace
	m.lastSnapshotted.name = name
	m.lastSnapshotted.req = req
	if m.snapshotErr != nil {
		return nil, m.snapshotErr
	}
	return &storage.VolumeSnapshotResult{
		SnapshotName: req.SnapshotName,
		VolumeName:   name,
		Status:       "Ready",
		CreatedAt:    "2026-09-03T18:00:00Z",
	}, nil
}

func setupStorageTestRouter(svc *mockVolumeService) (*StorageHandler, chi.Router) {
	var handler *StorageHandler
	if svc != nil {
		handler = NewStorageHandler(svc)
	} else {
		handler = NewStorageHandler(nil)
	}

	r := chi.NewRouter()
	r.Route("/k8s/{cluster}", func(sub chi.Router) {
		sub.With(middleware.RequireRolesForMutations("platform_admin", "tenant_admin", "operator")).Route("/storage/volumes", handler.RegisterRoutes)
	})

	return handler, r
}

func TestStorageHandler_ListVolumes(t *testing.T) {
	svc := &mockVolumeService{
		volumes: []storage.DistributedVolume{
			{
				Name:             "vol-lh",
				Namespace:        "default",
				StorageClass:     "longhorn",
				CapacityBytes:    10737418240,
				ReplicationCount: 3,
				HealthStatus:     "Healthy",
			},
		},
	}
	_, r := setupStorageTestRouter(svc)

	// Safe read-only GET should work for viewer role
	req := httptest.NewRequest(http.MethodGet, "/k8s/cluster-1/storage/volumes", nil)
	ctx := context.WithValue(req.Context(), middleware.UserRoleKey, "viewer")
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var res []storage.DistributedVolume
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if len(res) != 1 || res[0].Name != "vol-lh" {
		t.Errorf("unexpected volumes in response: %+v", res)
	}
}

func TestStorageHandler_ExpandVolume_Success(t *testing.T) {
	svc := &mockVolumeService{}
	_, r := setupStorageTestRouter(svc)

	body := `{"new_size_bytes": 21474836480, "namespace": "prod"}`
	req := httptest.NewRequest(http.MethodPost, "/k8s/cluster-1/storage/volumes/vol-mysql/expand", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), middleware.UserRoleKey, "operator")
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}

	if svc.lastExpanded.name != "vol-mysql" || svc.lastExpanded.size != 21474836480 || svc.lastExpanded.namespace != "prod" {
		t.Errorf("service did not receive expected arguments: %+v", svc.lastExpanded)
	}
}

func TestStorageHandler_ExpandVolume_RBAC(t *testing.T) {
	svc := &mockVolumeService{}
	_, r := setupStorageTestRouter(svc)

	body := `{"new_size_bytes": 21474836480}`
	req := httptest.NewRequest(http.MethodPost, "/k8s/cluster-1/storage/volumes/vol-mysql/expand", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	// Viewer role cannot mutate
	ctx := context.WithValue(req.Context(), middleware.UserRoleKey, "viewer")
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status 403 Forbidden for viewer, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestStorageHandler_ExpandVolume_Validation(t *testing.T) {
	svc := &mockVolumeService{}
	_, r := setupStorageTestRouter(svc)

	// Zero size
	body := `{"new_size_bytes": 0}`
	req := httptest.NewRequest(http.MethodPost, "/k8s/cluster-1/storage/volumes/vol-mysql/expand", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), middleware.UserRoleKey, "platform_admin")
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 for zero size, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestStorageHandler_ExpandVolume_NotFound(t *testing.T) {
	svc := &mockVolumeService{
		expandErr: domainerrors.NewNotFound("pvc", "missing-vol"),
	}
	_, r := setupStorageTestRouter(svc)

	body := `{"new_size_bytes": 21474836480}`
	req := httptest.NewRequest(http.MethodPost, "/k8s/cluster-1/storage/volumes/missing-vol/expand", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), middleware.UserRoleKey, "platform_admin")
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404 for missing volume, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestStorageHandler_CreateSnapshot_Success(t *testing.T) {
	svc := &mockVolumeService{}
	_, r := setupStorageTestRouter(svc)

	body := `{"snapshot_name": "snap-manual-1", "namespace": "prod"}`
	req := httptest.NewRequest(http.MethodPost, "/k8s/cluster-1/storage/volumes/vol-mysql/snapshot", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(req.Context(), middleware.UserRoleKey, "tenant_admin")
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status 201 Created, got %d: %s", rec.Code, rec.Body.String())
	}

	var res storage.VolumeSnapshotResult
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to decode snapshot result: %v", err)
	}

	if res.SnapshotName != "snap-manual-1" || res.VolumeName != "vol-mysql" || res.Status != "Ready" {
		t.Errorf("unexpected snapshot result: %+v", res)
	}
}

func TestStorageHandler_CreateSnapshot_RBAC(t *testing.T) {
	svc := &mockVolumeService{}
	_, r := setupStorageTestRouter(svc)

	body := `{"snapshot_name": "snap-manual-1"}`
	req := httptest.NewRequest(http.MethodPost, "/k8s/cluster-1/storage/volumes/vol-mysql/snapshot", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	// Viewer role cannot snapshot
	ctx := context.WithValue(req.Context(), middleware.UserRoleKey, "viewer")
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected status 403 Forbidden for viewer, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestStorageHandler_OfflineGraceful(t *testing.T) {
	_, r := setupStorageTestRouter(nil)

	endpoints := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/k8s/local/storage/volumes"},
		{http.MethodPost, "/k8s/local/storage/volumes/vol-1/expand"},
		{http.MethodPost, "/k8s/local/storage/volumes/vol-1/snapshot"},
	}

	for _, ep := range endpoints {
		req := httptest.NewRequest(ep.method, ep.path, bytes.NewBufferString(`{"new_size_bytes": 1024}`))
		req.Header.Set("Content-Type", "application/json")
		ctx := context.WithValue(req.Context(), middleware.UserRoleKey, "platform_admin")
		req = req.WithContext(ctx)

		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf("[%s %s] expected status 503 Service Unavailable, got %d", ep.method, ep.path, rec.Code)
		}
	}
}
