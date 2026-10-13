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
	domainconsensus "github.com/datdt/k8sselfhost/internal/domain/consensus"
)

// fakeConsensusCoordinator implements domainconsensus.ConsensusCoordinator for handler tests.
type fakeConsensusCoordinator struct {
	defragErr   error
	acquireErr  error
	validateErr error
	flushErr    error

	lastToken *domainconsensus.FencingToken
	validRes  bool
}

func (f *fakeConsensusCoordinator) AcquireLockWithFencing(
	ctx context.Context,
	key, ownerID string,
	ttl time.Duration,
) (*domainconsensus.FencingToken, error) {
	if f.acquireErr != nil {
		return nil, f.acquireErr
	}
	tok, err := domainconsensus.NewFencingToken(key, ownerID, 10, time.Now().UTC().Add(ttl))
	f.lastToken = tok
	return tok, err
}

func (f *fakeConsensusCoordinator) ValidateFencingToken(ctx context.Context, key string, token int64) (bool, error) {
	if f.validateErr != nil {
		return false, f.validateErr
	}
	return f.validRes, nil
}

func (f *fakeConsensusCoordinator) SafeDefragCluster(ctx context.Context, endpoints []string) error {
	return f.defragErr
}

func (f *fakeConsensusCoordinator) FlushPgBouncerStaleConnections(ctx context.Context, poolerAddr, newMasterAddr string) error {
	return f.flushErr
}

func setupConsensusRouter(coordinator domainconsensus.ConsensusCoordinator) http.Handler {
	r := chi.NewRouter()
	h := adapterhttp.NewConsensusHandler(coordinator)
	h.RegisterRoutes(r)
	return r
}

func TestConsensusHandler_SafeDefrag_Success(t *testing.T) {
	fake := &fakeConsensusCoordinator{}
	router := setupConsensusRouter(fake)

	body := []byte(`{"endpoints": ["http://10.0.0.1:2379", "http://10.0.0.2:2379", "http://10.0.0.3:2379"]}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/consensus/etcd/safe-defrag", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d (body: %s)", rec.Code, rec.Body.String())
	}
}

func TestConsensusHandler_SafeDefrag_QuorumLost(t *testing.T) {
	fake := &fakeConsensusCoordinator{
		defragErr: domainconsensus.ErrConsensusQuorumLost,
	}
	router := setupConsensusRouter(fake)

	body := []byte(`{"endpoints": ["http://10.0.0.1:2379"]}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/consensus/etcd/safe-defrag", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 Service Unavailable, got %d", rec.Code)
	}
	if rec.Header().Get("Content-Type") != "application/problem+json" {
		t.Errorf("expected application/problem+json, got %s", rec.Header().Get("Content-Type"))
	}
}

func TestConsensusHandler_SafeDefrag_LeaderBlocked(t *testing.T) {
	fake := &fakeConsensusCoordinator{
		defragErr: domainconsensus.ErrEtcdDefragLeaderBlocked,
	}
	router := setupConsensusRouter(fake)

	body := []byte(`{"endpoints": ["http://10.0.0.1:2379"]}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/consensus/etcd/safe-defrag", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409 Conflict, got %d", rec.Code)
	}
}

func TestConsensusHandler_AcquireFencingLock_Success(t *testing.T) {
	fake := &fakeConsensusCoordinator{}
	router := setupConsensusRouter(fake)

	body := []byte(`{"key": "leader/patroni/primary", "owner_id": "worker-1", "ttl_ms": 5000}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/consensus/locks/acquire-fencing", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp domainconsensus.FencingToken
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.LockKey != "leader/patroni/primary" || resp.OwnerID != "worker-1" || resp.Generation != 10 {
		t.Errorf("unexpected fencing token: %+v", resp)
	}
}

func TestConsensusHandler_AcquireFencingLock_InvalidInput(t *testing.T) {
	fake := &fakeConsensusCoordinator{}
	router := setupConsensusRouter(fake)

	body := []byte(`{"key": "", "owner_id": "worker-1", "ttl_ms": 5000}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/consensus/locks/acquire-fencing", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for empty key, got %d", rec.Code)
	}
}

func TestConsensusHandler_ValidateFencingToken_Success(t *testing.T) {
	fake := &fakeConsensusCoordinator{validRes: true}
	router := setupConsensusRouter(fake)

	body := []byte(`{"key": "leader/patroni/primary", "generation": 10}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/consensus/locks/validate-fencing", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}
}

func TestConsensusHandler_ValidateFencingToken_Stale(t *testing.T) {
	fake := &fakeConsensusCoordinator{
		validateErr: domainconsensus.ErrFencingTokenStale,
	}
	router := setupConsensusRouter(fake)

	body := []byte(`{"key": "leader/patroni/primary", "generation": 5}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/consensus/locks/validate-fencing", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusPreconditionFailed {
		t.Fatalf("expected 412 Precondition Failed, got %d (body: %s)", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Content-Type") != "application/problem+json" {
		t.Errorf("expected application/problem+json, got %s", rec.Header().Get("Content-Type"))
	}
}

func TestConsensusHandler_FlushPgBouncerStale_Success(t *testing.T) {
	fake := &fakeConsensusCoordinator{}
	router := setupConsensusRouter(fake)

	body := []byte(`{"pooler_addr": "pgbouncer:6432", "new_master_addr": "postgres-primary:5432"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/consensus/pgbouncer/flush-stale", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}
}

func TestConsensusHandler_FlushPgBouncerStale_InvalidInput(t *testing.T) {
	fake := &fakeConsensusCoordinator{}
	router := setupConsensusRouter(fake)

	body := []byte(`{"pooler_addr": "", "new_master_addr": "postgres-primary:5432"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/consensus/pgbouncer/flush-stale", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request, got %d", rec.Code)
	}
}
