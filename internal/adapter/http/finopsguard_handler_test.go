package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datdt/k8sselfhost/internal/domain/finopsguard"
)

type mockFinOpsEngine struct {
	unhintedCount int
	auditErr      error

	injectErr error

	orphans []finopsguard.OrphanCloudResource
	scanErr error

	reapErr error
}

func (m *mockFinOpsEngine) AuditTopologyHints(ctx context.Context) (int, error) {
	if m.auditErr != nil {
		return m.unhintedCount, m.auditErr
	}
	return m.unhintedCount, nil
}

func (m *mockFinOpsEngine) InjectTopologyAwareHints(ctx context.Context, namespace, serviceName string) error {
	return m.injectErr
}

func (m *mockFinOpsEngine) ScanOrphanCloudResources(ctx context.Context) ([]finopsguard.OrphanCloudResource, error) {
	if m.scanErr != nil {
		return nil, m.scanErr
	}
	return m.orphans, nil
}

func (m *mockFinOpsEngine) ReapApprovedResource(ctx context.Context, resourceID string) error {
	return m.reapErr
}

func (m *mockFinOpsEngine) DispatchTelegramCostReport(ctx context.Context, orphans []finopsguard.OrphanCloudResource) error {
	return nil
}

func setupFinOpsTestRouter(engine finopsguard.FinOpsAutomationEngine) *chi.Mux {
	handler := NewFinOpsGuardHandler(engine)
	r := chi.NewRouter()
	r.Route("/api/v1/finops", handler.RegisterRoutes)
	return r
}

func TestFinOpsGuardHandler_AuditTopologyHints_WasteDetected_409_RFC7807(t *testing.T) {
	t.Parallel()

	engine := &mockFinOpsEngine{
		unhintedCount: 3,
		auditErr:      finopsguard.ErrFinOpsCrossAZWasteDetected,
	}
	router := setupFinOpsTestRouter(engine)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/finops/services/topology-audit", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusConflict, rr.Code)
	assert.Contains(t, rr.Header().Get("Content-Type"), "application/problem+json")

	var prob finopsguard.RFC7807ProblemDetail
	err := json.NewDecoder(rr.Body).Decode(&prob)
	require.NoError(t, err)
	assert.Equal(t, "ERR_FINOPS_CROSS_AZ_WASTE_DETECTED", prob.Code)
	assert.Equal(t, http.StatusConflict, prob.Status)
}

func TestFinOpsGuardHandler_AuditTopologyHints_Optimal_200(t *testing.T) {
	t.Parallel()

	engine := &mockFinOpsEngine{
		unhintedCount: 0,
		auditErr:      nil,
	}
	router := setupFinOpsTestRouter(engine)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/finops/services/topology-audit", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp map[string]interface{}
	err := json.NewDecoder(rr.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, float64(0), resp["unhinted_services_count"])
}

func TestFinOpsGuardHandler_InjectTopologyAwareHints_200(t *testing.T) {
	t.Parallel()

	engine := &mockFinOpsEngine{
		injectErr: nil,
	}
	router := setupFinOpsTestRouter(engine)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/finops/services/prod/payment/inject-hints", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp map[string]interface{}
	err := json.NewDecoder(rr.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, "success", resp["status"])
	assert.Equal(t, "prod", resp["namespace"])
	assert.Equal(t, "payment", resp["service"])
}

func TestFinOpsGuardHandler_ScanOrphanResources_200(t *testing.T) {
	t.Parallel()

	engine := &mockFinOpsEngine{
		orphans: []finopsguard.OrphanCloudResource{
			{
				ResourceID:              "vol-0123456",
				Type:                    finopsguard.ResourceEBSVolume,
				CloudProvider:           "aws",
				OriginalNamespace:       "analytics",
				IdleSince:               time.Now().Add(-50 * time.Hour),
				EstimatedMonthlyCostUSD: 140.0,
				ApprovalStatus:          finopsguard.ApprovalStatusPending,
			},
		},
	}
	router := setupFinOpsTestRouter(engine)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/finops/resources/orphans", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp struct {
		Data  []finopsguard.OrphanCloudResource `json:"data"`
		Count int                               `json:"count"`
	}
	err := json.NewDecoder(rr.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, 1, resp.Count)
	assert.Equal(t, "vol-0123456", resp.Data[0].ResourceID)
}

func TestFinOpsGuardHandler_ReapResource_Unapproved_400_RFC7807(t *testing.T) {
	t.Parallel()

	engine := &mockFinOpsEngine{
		reapErr: finopsguard.ErrResourceNotApproved,
	}
	router := setupFinOpsTestRouter(engine)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/finops/resources/vol-0123456/reap", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Header().Get("Content-Type"), "application/problem+json")

	var prob finopsguard.RFC7807ProblemDetail
	err := json.NewDecoder(rr.Body).Decode(&prob)
	require.NoError(t, err)
	assert.Equal(t, "ERR_RESOURCE_NOT_APPROVED", prob.Code)
	assert.Equal(t, http.StatusBadRequest, prob.Status)
}

func TestFinOpsGuardHandler_ReapResource_SnapshotFailed_500_RFC7807(t *testing.T) {
	t.Parallel()

	engine := &mockFinOpsEngine{
		reapErr: finopsguard.ErrSnapshotGenerationFailed,
	}
	router := setupFinOpsTestRouter(engine)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/finops/resources/vol-0123456/reap", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.Contains(t, rr.Header().Get("Content-Type"), "application/problem+json")

	var prob finopsguard.RFC7807ProblemDetail
	err := json.NewDecoder(rr.Body).Decode(&prob)
	require.NoError(t, err)
	assert.Equal(t, "ERR_SNAPSHOT_GENERATION_FAILED", prob.Code)
	assert.Equal(t, http.StatusInternalServerError, prob.Status)
}

func TestFinOpsGuardHandler_ReapResource_Success_200(t *testing.T) {
	t.Parallel()

	engine := &mockFinOpsEngine{
		reapErr: nil,
	}
	router := setupFinOpsTestRouter(engine)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/finops/resources/vol-0123456/reap", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp map[string]interface{}
	err := json.NewDecoder(rr.Body).Decode(&resp)
	require.NoError(t, err)
	assert.Equal(t, "reaped", resp["status"])
	assert.Equal(t, "vol-0123456", resp["resource_id"])
}
