package http_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	adapthttp "github.com/datdt/k8sselfhost/internal/adapter/http"
	"github.com/datdt/k8sselfhost/internal/domain/promotion"
	usecasePromotion "github.com/datdt/k8sselfhost/internal/usecase/promotion"
)

func setupCanaryTestRouter(engine *usecasePromotion.CanaryEngine) *chi.Mux {
	r := chi.NewRouter()
	handler := adapthttp.NewCanaryHandler(engine)
	r.Route("/api/v1/promotions/canary", handler.RegisterRoutes)
	return r
}

func TestCanaryHandler_StartCanary_Success(t *testing.T) {
	engine := usecasePromotion.NewCanaryEngine()
	router := setupCanaryTestRouter(engine)

	body := map[string]interface{}{
		"workload":  "billing-service",
		"namespace": "production",
		"new_image": "billing-service:v2.5.0",
	}
	raw, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/promotions/canary/start", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusCreated, rec.Code)
	var resp promotion.CanaryReleaseExecution
	err := json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.NotEmpty(t, resp.ReleaseID)
	assert.Equal(t, "billing-service", resp.WorkloadName)
	assert.Equal(t, 10, resp.CurrentWeight)
	assert.Equal(t, promotion.CanaryStatusProgressing, resp.Status)
}

func TestCanaryHandler_StartCanary_ValidationFailure(t *testing.T) {
	engine := usecasePromotion.NewCanaryEngine()
	router := setupCanaryTestRouter(engine)

	// Missing workload
	body := map[string]interface{}{
		"namespace": "production",
		"new_image": "app:v1",
	}
	raw, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/promotions/canary/start", bytes.NewReader(raw))
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestCanaryHandler_EvaluateCanary(t *testing.T) {
	engine := usecasePromotion.NewCanaryEngine()
	router := setupCanaryTestRouter(engine)

	exec, err := engine.StartCanaryRelease(t.Context(), "user-service", "prod", "user:v2", nil)
	require.NoError(t, err)

	// Set healthy baseline metrics for evaluation
	engine.SetMetricsForWorkload("user-service", "prod", 0.001, 0.001, 50*time.Millisecond, 50*time.Millisecond)

	// Normal healthy evaluation
	req := httptest.NewRequest(http.MethodPost, "/api/v1/promotions/canary/"+exec.ReleaseID+"/evaluate", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var metrics promotion.CanaryMetricsComparison
	err = json.Unmarshal(rec.Body.Bytes(), &metrics)
	require.NoError(t, err)
	assert.Equal(t, promotion.VerdictProceed, metrics.ResultVerdict)

	// High error rate -> ABORT evaluation
	engine.SetMetricsForWorkload("user-service", "prod", 0.001, 0.02, 50*time.Millisecond, 50*time.Millisecond)

	reqAbort := httptest.NewRequest(http.MethodPost, "/api/v1/promotions/canary/"+exec.ReleaseID+"/evaluate", nil)
	recAbort := httptest.NewRecorder()
	router.ServeHTTP(recAbort, reqAbort)

	assert.Equal(t, http.StatusOK, recAbort.Code)
	var metricsAbort promotion.CanaryMetricsComparison
	err = json.Unmarshal(recAbort.Body.Bytes(), &metricsAbort)
	require.NoError(t, err)
	assert.Equal(t, promotion.VerdictAbort, metricsAbort.ResultVerdict)
}

func TestCanaryHandler_PromoteCanary_SuccessAndAbortGuard(t *testing.T) {
	engine := usecasePromotion.NewCanaryEngine()
	router := setupCanaryTestRouter(engine)

	exec, err := engine.StartCanaryRelease(t.Context(), "cart-service", "prod", "cart:v1", nil)
	require.NoError(t, err)

	// Promote step 1 -> step 2
	req := httptest.NewRequest(http.MethodPost, "/api/v1/promotions/canary/"+exec.ReleaseID+"/promote", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var resp map[string]interface{}
	err = json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "promoted", resp["status"])

	// Now abort rollout
	err = engine.AbortCanaryRollback(t.Context(), exec.ReleaseID, "high errors")
	require.NoError(t, err)

	// Promote on aborted rollout must return 422 Unprocessable Entity
	reqAfterAbort := httptest.NewRequest(http.MethodPost, "/api/v1/promotions/canary/"+exec.ReleaseID+"/promote", nil)
	recAfterAbort := httptest.NewRecorder()
	router.ServeHTTP(recAfterAbort, reqAfterAbort)

	assert.Equal(t, http.StatusUnprocessableEntity, recAfterAbort.Code)
	assert.Equal(t, "application/problem+json", recAfterAbort.Header().Get("Content-Type"))

	var problem struct {
		Code   string `json:"code"`
		Status int    `json:"status"`
	}
	err = json.Unmarshal(recAfterAbort.Body.Bytes(), &problem)
	require.NoError(t, err)
	assert.Equal(t, 422, problem.Status)
	assert.Equal(t, promotion.ErrCodeCanaryAbortedHighErrors, problem.Code)
}

func TestCanaryHandler_AbortCanary_Manual(t *testing.T) {
	engine := usecasePromotion.NewCanaryEngine()
	router := setupCanaryTestRouter(engine)

	exec, err := engine.StartCanaryRelease(t.Context(), "checkout-service", "prod", "checkout:v2", nil)
	require.NoError(t, err)

	body := map[string]string{"reason": "manual rollback trigger"}
	raw, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/promotions/canary/"+exec.ReleaseID+"/abort", bytes.NewReader(raw))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var resp map[string]string
	err = json.Unmarshal(rec.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "aborted", resp["status"])
	assert.Equal(t, "manual rollback trigger", resp["reason"])
}

func TestCanaryHandler_NotFound(t *testing.T) {
	engine := usecasePromotion.NewCanaryEngine()
	router := setupCanaryTestRouter(engine)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/promotions/canary/non-existent-id/promote", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusNotFound, rec.Code)
}
