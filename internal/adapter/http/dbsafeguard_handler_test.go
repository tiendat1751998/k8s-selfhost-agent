package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datdt/k8sselfhost/internal/domain/dbsafeguard"
)

type mockDBSafeguardEngine struct {
	xidHealth *dbsafeguard.PostgresXIDHealth
	xidErr    error

	terminateErr error
	killedMaxAge time.Duration

	replHealth *dbsafeguard.RedisReplicationHealth
	replErr    error

	poisonPillErr error
}

func (m *mockDBSafeguardEngine) CheckPostgresXIDHealth(ctx context.Context) (*dbsafeguard.PostgresXIDHealth, error) {
	if m.xidErr != nil {
		return m.xidHealth, m.xidErr
	}
	return m.xidHealth, nil
}

func (m *mockDBSafeguardEngine) TerminateBlockingTransactions(ctx context.Context, maxAge time.Duration) error {
	m.killedMaxAge = maxAge
	return m.terminateErr
}

func (m *mockDBSafeguardEngine) TuneAutovacuumAggressiveness(ctx context.Context) error {
	return nil
}

func (m *mockDBSafeguardEngine) AuditRedisReplicationBuffer(ctx context.Context) (*dbsafeguard.RedisReplicationHealth, error) {
	if m.replErr != nil {
		return m.replHealth, m.replErr
	}
	return m.replHealth, nil
}

func (m *mockDBSafeguardEngine) HandleKafkaPoisonPill(ctx context.Context, topic string, partition int32, offset int64, payload []byte, err error) error {
	return m.poisonPillErr
}

func setupDBSafeguardTestRouter(engine dbsafeguard.DatabaseResilienceEngine) *chi.Mux {
	handler := NewDBSafeguardHandler(engine)
	r := chi.NewRouter()
	r.Route("/api/v1/db", handler.RegisterRoutes)
	return r
}

func TestDBSafeguardHandler_GetPostgresXIDHealth_Healthy(t *testing.T) {
	health, err := dbsafeguard.NewPostgresXIDHealth("k8s_prod", 400_000_000, 2*time.Minute, 101)
	require.NoError(t, err)

	mockEngine := &mockDBSafeguardEngine{xidHealth: health}
	r := setupDBSafeguardTestRouter(mockEngine)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/db/postgres/xid-health", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var resp dbsafeguard.PostgresXIDHealth
	err = json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "k8s_prod", resp.DatabaseName)
	assert.Equal(t, int64(400_000_000), resp.MaxXIDAge)
	assert.False(t, resp.IsEmergencyFreeze)
}

func TestDBSafeguardHandler_GetPostgresXIDHealth_WraparoundRisk_RFC7807(t *testing.T) {
	mockEngine := &mockDBSafeguardEngine{
		xidErr: dbsafeguard.ErrDBXIDWraparoundRisk,
	}
	r := setupDBSafeguardTestRouter(mockEngine)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/db/postgres/xid-health", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Equal(t, "application/problem+json", rec.Header().Get("Content-Type"))

	var prob dbsafeguard.RFC7807ProblemDetail
	err := json.Unmarshal(rec.Body.Bytes(), &prob)
	require.NoError(t, err)
	assert.Equal(t, "ERR_DB_XID_WRAPAROUND_RISK", prob.Code)
	assert.Equal(t, 500, prob.Status)
	assert.Equal(t, "/api/v1/db/postgres/xid-health", prob.Instance)
}

func TestDBSafeguardHandler_KillIdleTransactions(t *testing.T) {
	mockEngine := &mockDBSafeguardEngine{}
	r := setupDBSafeguardTestRouter(mockEngine)

	t.Run("Default max age", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/db/postgres/kill-idle-tx", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, 1800*time.Second, mockEngine.killedMaxAge)
	})

	t.Run("Custom max age", func(t *testing.T) {
		body := bytes.NewBufferString(`{"max_age_seconds": 600}`)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/db/postgres/kill-idle-tx", body)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, 600*time.Second, mockEngine.killedMaxAge)
	})

	t.Run("Engine error", func(t *testing.T) {
		mockEngine.terminateErr = errors.New("db connection failure")
		req := httptest.NewRequest(http.MethodPost, "/api/v1/db/postgres/kill-idle-tx", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusInternalServerError, rec.Code)
	})
}

func TestDBSafeguardHandler_GetRedisReplHealth_Healthy(t *testing.T) {
	repl, err := dbsafeguard.NewRedisReplicationHealth("redis-master:6379", 3, 134217728, 4096, 0, false)
	require.NoError(t, err)

	mockEngine := &mockDBSafeguardEngine{replHealth: repl}
	r := setupDBSafeguardTestRouter(mockEngine)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/db/redis/repl-health", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var resp dbsafeguard.RedisReplicationHealth
	err = json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "redis-master:6379", resp.MasterHost)
	assert.False(t, resp.IsFullResyncLooping)
}

func TestDBSafeguardHandler_GetRedisReplHealth_ResyncStorm_RFC7807(t *testing.T) {
	mockEngine := &mockDBSafeguardEngine{
		replErr: dbsafeguard.ErrRedisResyncStormLoop,
	}
	r := setupDBSafeguardTestRouter(mockEngine)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/db/redis/repl-health", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Equal(t, "application/problem+json", rec.Header().Get("Content-Type"))

	var prob dbsafeguard.RFC7807ProblemDetail
	err := json.Unmarshal(rec.Body.Bytes(), &prob)
	require.NoError(t, err)
	assert.Equal(t, "ERR_REDIS_RESYNC_STORM_LOOP", prob.Code)
	assert.Equal(t, 500, prob.Status)
}

func TestDBSafeguardHandler_HandleKafkaPoisonPill_RFC7807_422(t *testing.T) {
	mockEngine := &mockDBSafeguardEngine{
		poisonPillErr: dbsafeguard.ErrKafkaPoisonPillDiverted,
	}
	r := setupDBSafeguardTestRouter(mockEngine)

	payload := `{"topic":"billing.events","partition":1,"offset":7788,"payload":"invalid-binary","error_reason":"corrupt avro magic byte"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/db/kafka/handle-poison-pill", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	assert.Equal(t, "application/problem+json", rec.Header().Get("Content-Type"))

	var prob dbsafeguard.RFC7807ProblemDetail
	err := json.Unmarshal(rec.Body.Bytes(), &prob)
	require.NoError(t, err)
	assert.Equal(t, "ERR_KAFKA_POISON_PILL_DIVERTED", prob.Code)
	assert.Equal(t, 422, prob.Status)
}

func TestDBSafeguardHandler_HandleKafkaPoisonPill_ValidationFailure_400(t *testing.T) {
	mockEngine := &mockDBSafeguardEngine{}
	r := setupDBSafeguardTestRouter(mockEngine)

	// Missing topic and error_reason
	payload := `{"topic":"","partition":-1,"offset":0,"payload":"","error_reason":""}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/db/kafka/handle-poison-pill", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}
