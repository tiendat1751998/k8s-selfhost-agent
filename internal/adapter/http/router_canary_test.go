package http_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	adapthttp "github.com/datdt/k8sselfhost/internal/adapter/http"
	"github.com/datdt/k8sselfhost/internal/adapter/http/middleware"
	"github.com/datdt/k8sselfhost/internal/domain/promotion"
	"github.com/datdt/k8sselfhost/internal/pkg/health"
	usecasePromotion "github.com/datdt/k8sselfhost/internal/usecase/promotion"
)

func TestRouter_CanaryProgressiveDeliveryWorkflow(t *testing.T) {
	healthHandler := health.NewHandler(5 * time.Second)
	engine := usecasePromotion.NewCanaryEngine()
	canaryHandler := adapthttp.NewCanaryHandler(engine)

	platform := &adapthttp.PlatformHandlers{
		Canary: canaryHandler,
	}

	router := adapthttp.NewRouterWithWS(healthHandler, nil, platform)

	adminToken, err := middleware.GenerateJWT("admin-user", "platform_admin", "default-tenant")
	require.NoError(t, err)

	// 1. Start Canary Rollout
	startPayload := map[string]interface{}{
		"workload":  "checkout-engine",
		"namespace": "production",
		"new_image": "checkout:v2.0.0",
		"steps": []map[string]interface{}{
			{"traffic_weight_percentage": 10, "evaluation_duration": "5m"},
			{"traffic_weight_percentage": 25, "evaluation_duration": "5m"},
			{"traffic_weight_percentage": 50, "evaluation_duration": "5m"},
			{"traffic_weight_percentage": 100, "evaluation_duration": "5m"},
		},
	}
	body, _ := json.Marshal(startPayload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/promotions/canary/start", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+adminToken)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)
	var exec promotion.CanaryReleaseExecution
	err = json.Unmarshal(rec.Body.Bytes(), &exec)
	require.NoError(t, err)
	assert.NotEmpty(t, exec.ReleaseID)
	assert.Equal(t, 10, exec.CurrentWeight)
	assert.Equal(t, promotion.CanaryStatusProgressing, exec.Status)

	// Set healthy baseline metrics for evaluation
	engine.SetMetricsForWorkload("checkout-engine", "production", 0.001, 0.001, 50*time.Millisecond, 50*time.Millisecond)

	// 2. Evaluate Canary (Healthy)
	reqEval := httptest.NewRequest(http.MethodPost, "/api/v1/promotions/canary/"+exec.ReleaseID+"/evaluate", nil)
	reqEval.Header.Set("Authorization", "Bearer "+adminToken)
	recEval := httptest.NewRecorder()
	router.ServeHTTP(recEval, reqEval)

	require.Equal(t, http.StatusOK, recEval.Code)
	var comp promotion.CanaryMetricsComparison
	err = json.Unmarshal(recEval.Body.Bytes(), &comp)
	require.NoError(t, err)
	assert.Equal(t, promotion.VerdictProceed, comp.ResultVerdict)

	// 3. Promote Next Step (10% -> 25%)
	reqPromote := httptest.NewRequest(http.MethodPost, "/api/v1/promotions/canary/"+exec.ReleaseID+"/promote", nil)
	reqPromote.Header.Set("Authorization", "Bearer "+adminToken)
	recPromote := httptest.NewRecorder()
	router.ServeHTTP(recPromote, reqPromote)

	require.Equal(t, http.StatusOK, recPromote.Code)

	// Verify weight is now 25%
	reqGet := httptest.NewRequest(http.MethodGet, "/api/v1/promotions/canary/"+exec.ReleaseID, nil)
	reqGet.Header.Set("Authorization", "Bearer "+adminToken)
	recGet := httptest.NewRecorder()
	router.ServeHTTP(recGet, reqGet)
	require.Equal(t, http.StatusOK, recGet.Code)
	var updatedExec promotion.CanaryReleaseExecution
	err = json.Unmarshal(recGet.Body.Bytes(), &updatedExec)
	require.NoError(t, err)
	assert.Equal(t, 25, updatedExec.CurrentWeight)

	// 4. Inject Anomaly: Canary Error Rate Delta > 0.005
	engine.SetMetricsForWorkload("checkout-engine", "production", 0.001, 0.015, 60*time.Millisecond, 65*time.Millisecond)

	reqEvalAnomaly := httptest.NewRequest(http.MethodPost, "/api/v1/promotions/canary/"+exec.ReleaseID+"/evaluate", nil)
	reqEvalAnomaly.Header.Set("Authorization", "Bearer "+adminToken)
	recEvalAnomaly := httptest.NewRecorder()
	router.ServeHTTP(recEvalAnomaly, reqEvalAnomaly)

	require.Equal(t, http.StatusOK, recEvalAnomaly.Code)
	var compAnomaly promotion.CanaryMetricsComparison
	err = json.Unmarshal(recEvalAnomaly.Body.Bytes(), &compAnomaly)
	require.NoError(t, err)
	assert.Equal(t, promotion.VerdictAbort, compAnomaly.ResultVerdict)

	// 5. Subsequent Promote MUST return HTTP 422 with RFC 7807 ERR_CANARY_ABORTED_HIGH_ERRORS
	reqPromoteAborted := httptest.NewRequest(http.MethodPost, "/api/v1/promotions/canary/"+exec.ReleaseID+"/promote", nil)
	reqPromoteAborted.Header.Set("Authorization", "Bearer "+adminToken)
	recPromoteAborted := httptest.NewRecorder()
	router.ServeHTTP(recPromoteAborted, reqPromoteAborted)

	assert.Equal(t, http.StatusUnprocessableEntity, recPromoteAborted.Code)
	assert.Equal(t, "application/problem+json", recPromoteAborted.Header().Get("Content-Type"))
	var problem struct {
		Code   string `json:"code"`
		Status int    `json:"status"`
	}
	err = json.Unmarshal(recPromoteAborted.Body.Bytes(), &problem)
	require.NoError(t, err)
	assert.Equal(t, 422, problem.Status)
	assert.Equal(t, promotion.ErrCodeCanaryAbortedHighErrors, problem.Code)
}
