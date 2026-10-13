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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	adapterHTTP "github.com/datdt/k8sselfhost/internal/adapter/http"
	domainCluster "github.com/datdt/k8sselfhost/internal/domain/cluster"
)

// mockMultiDCFencingManager implements domainCluster.MultiDCFencingManager for HTTP testing.
type mockMultiDCFencingManager struct {
	stonithReport *domainCluster.NodeFencingReport
	stonithErr    error

	readinessStatus *domainCluster.FailoverReadinessStatus
	readinessErr    error

	imdsErr error
}

func (m *mockMultiDCFencingManager) ExecuteSTONITH(ctx context.Context, targetNodeID, dc string) (*domainCluster.NodeFencingReport, error) {
	if m.stonithErr != nil {
		return nil, m.stonithErr
	}
	if m.stonithReport != nil {
		return m.stonithReport, nil
	}
	return domainCluster.NewNodeFencingReport(targetNodeID, dc, domainCluster.IPMI_POWER_OFF, true, time.Now().UTC(), 500*time.Millisecond, []string{"Fenced"}), nil
}

func (m *mockMultiDCFencingManager) ValidateFailoverReadiness(ctx context.Context, clusterID string) (*domainCluster.FailoverReadinessStatus, error) {
	if m.readinessErr != nil {
		return m.readinessStatus, m.readinessErr
	}
	if m.readinessStatus != nil {
		return m.readinessStatus, nil
	}
	return domainCluster.NewFailoverReadinessStatus(true, true, 5*time.Millisecond, true, ""), nil
}

func (m *mockMultiDCFencingManager) EnforceIMDSv2Security(ctx context.Context, clusterID string) error {
	return m.imdsErr
}

func setupFencingTestRouter(manager domainCluster.MultiDCFencingManager) *chi.Mux {
	r := chi.NewRouter()
	handler := adapterHTTP.NewFencingHandler(manager, zap.NewNop())
	handler.WireToRouter(r)
	return r
}

func TestFencingHandler_ExecuteSTONITH_Success(t *testing.T) {
	mockManager := &mockMultiDCFencingManager{
		stonithReport: domainCluster.NewNodeFencingReport("node-1", "dc-east", domainCluster.IPMI_POWER_OFF, true, time.Now().UTC(), 300*time.Millisecond, []string{"Power off confirmed"}),
	}
	router := setupFencingTestRouter(mockManager)

	body := `{"target_node_id":"node-1","datacenter":"dc-east"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/clusters/prod-cluster/fencing/stonith", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp domainCluster.NodeFencingReport
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "node-1", resp.TargetNodeID)
	assert.Equal(t, "dc-east", resp.Datacenter)
	assert.True(t, resp.IsFenced)
}

func TestFencingHandler_ExecuteSTONITH_MissingNode_Returns400(t *testing.T) {
	mockManager := &mockMultiDCFencingManager{}
	router := setupFencingTestRouter(mockManager)

	body := `{"datacenter":"dc-east"}` // missing target_node_id
	req := httptest.NewRequest(http.MethodPost, "/api/v1/clusters/prod-cluster/fencing/stonith", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestFencingHandler_GetFailoverReadiness_Success(t *testing.T) {
	mockManager := &mockMultiDCFencingManager{
		readinessStatus: domainCluster.NewFailoverReadinessStatus(true, true, 4*time.Millisecond, true, ""),
	}
	router := setupFencingTestRouter(mockManager)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/clusters/prod-cluster/fencing/readiness", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp domainCluster.FailoverReadinessStatus
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.True(t, resp.CanPromote)
	assert.True(t, resp.OldPrimaryFenced)
	assert.True(t, resp.GraceDelayPassed)
}

func TestFencingHandler_GetFailoverReadiness_NotFenced_Returns409(t *testing.T) {
	mockManager := &mockMultiDCFencingManager{
		readinessStatus: domainCluster.NewFailoverReadinessStatus(false, false, 10*time.Millisecond, true, "Old primary node 'primary-01' is not confirmed fenced. Promoting replica risks split-brain writes."),
		readinessErr:    domainCluster.ErrFailoverPrimaryNotFenced,
	}
	router := setupFencingTestRouter(mockManager)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/clusters/prod-cluster/fencing/readiness", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Contains(t, resp["error"], "old primary is not confirmed fenced")
}

func TestFencingHandler_GetFailoverReadiness_GracePeriodActive_Returns409(t *testing.T) {
	mockManager := &mockMultiDCFencingManager{
		readinessStatus: domainCluster.NewFailoverReadinessStatus(false, false, 2*time.Millisecond, false, "Grace period active for node 'primary-01'. Transient network hiccup protection in progress."),
		readinessErr:    domainCluster.ErrGracePeriodActive,
	}
	router := setupFencingTestRouter(mockManager)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/clusters/prod-cluster/fencing/readiness", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Contains(t, resp["error"], "transient grace period active")
}

func TestFencingHandler_EnforceIMDSv2_Success(t *testing.T) {
	mockManager := &mockMultiDCFencingManager{}
	router := setupFencingTestRouter(mockManager)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/clusters/prod-cluster/security/imdsv2", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "enforced", resp["status"])
	assert.Equal(t, "IMDSv2", resp["metadata_version"])
	assert.Equal(t, "block-imds-metadata-access", resp["network_policy"])
}

func TestFencingHandler_EnforceIMDSv2_InsecureAccessBlocked_Returns403(t *testing.T) {
	mockManager := &mockMultiDCFencingManager{
		imdsErr: domainCluster.ErrIMDSAccessBlocked,
	}
	router := setupFencingTestRouter(mockManager)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/clusters/prod-cluster/security/imdsv2", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Contains(t, resp["error"], "insecure metadata access detected")
}

func TestFencingHandler_WireToRouter_AlternativeClusterPath(t *testing.T) {
	mockManager := &mockMultiDCFencingManager{}
	router := setupFencingTestRouter(mockManager)

	// Verify route is also accessible via /clusters/{clusterId}/fencing/readiness
	req := httptest.NewRequest(http.MethodGet, "/clusters/secondary-dc/fencing/readiness", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}
