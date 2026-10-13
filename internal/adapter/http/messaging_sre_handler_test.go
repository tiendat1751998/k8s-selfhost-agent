package http_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	adapthttp "github.com/datdt/k8sselfhost/internal/adapter/http"
	"github.com/datdt/k8sselfhost/internal/adapter/http/middleware"
	domainmessaging "github.com/datdt/k8sselfhost/internal/domain/messaging"
	"github.com/datdt/k8sselfhost/internal/pkg/health"
	usecasemessaging "github.com/datdt/k8sselfhost/internal/usecase/messaging"
)

func TestMessagingHandler_Endpoints(t *testing.T) {
	svc := usecasemessaging.NewService(usecasemessaging.WithLogger(zap.NewNop()))
	handler := adapthttp.NewMessagingSREHandler(svc, zap.NewNop())

	r := chi.NewRouter()
	handler.RegisterRoutes(r)

	// Seed Kafka group
	svc.SeedKafkaGroup("k-prod", usecasemessaging.KafkaConsumerGroupRaw{
		GroupID:            "events-consumer",
		Topic:              "system.events",
		PartitionCount:     8,
		ConsumerCount:      4,
		TotalLag:           50,
		AssignmentStrategy: domainmessaging.StrategyRangeAssignor,
		RebalanceCount24h:  1,
		State:              "Stable",
	})

	// 1. GET /kafka/{cluster_id}/groups
	t.Run("GET kafka groups healthy", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/kafka/k-prod/groups", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var groups []domainmessaging.KafkaGroupHealth
		err := json.NewDecoder(rec.Body).Decode(&groups)
		require.NoError(t, err)
		require.Len(t, groups, 1)
		assert.Equal(t, "events-consumer", groups[0].GroupID)
		assert.Equal(t, domainmessaging.KafkaStatusDegraded, groups[0].Status) // RangeAssignor
	})

	// 2. POST /kafka/{cluster_id}/groups/{group_id}/enforce-cooperative
	t.Run("POST kafka enforce cooperative", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/kafka/k-prod/groups/events-consumer/enforce-cooperative", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var resp map[string]interface{}
		err := json.NewDecoder(rec.Body).Decode(&resp)
		require.NoError(t, err)
		assert.Equal(t, "enforced", resp["status"])
		assert.Equal(t, "events-consumer", resp["group_id"])

		// Verify that inspection now returns cooperative & healthy
		inspectReq := httptest.NewRequest(http.MethodGet, "/kafka/k-prod/groups", nil)
		inspectRec := httptest.NewRecorder()
		r.ServeHTTP(inspectRec, inspectReq)

		var groups []domainmessaging.KafkaGroupHealth
		err = json.NewDecoder(inspectRec.Body).Decode(&groups)
		require.NoError(t, err)
		require.Len(t, groups, 1)
		assert.True(t, groups[0].IsCooperative)
		assert.Equal(t, domainmessaging.KafkaStatusHealthy, groups[0].Status)
	})

	// 3. GET /rabbitmq/{cluster_id}/audit - Healthy
	t.Run("GET rabbitmq audit healthy", func(t *testing.T) {
		svc.SeedRabbitMQCluster(usecasemessaging.RabbitMQClusterRaw{
			ClusterName:       "rmq-staging",
			MemoryUsedBytes:   2 * 1024 * 1024 * 1024,
			MemoryLimitBytes:  8 * 1024 * 1024 * 1024,
			FlowBlockedConns:  0,
			QuorumQueueCount:  15,
			ClassicQueueCount: 2,
		})

		req := httptest.NewRequest(http.MethodGet, "/rabbitmq/rmq-staging/audit", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var report domainmessaging.RabbitMQHealthReport
		err := json.NewDecoder(rec.Body).Decode(&report)
		require.NoError(t, err)
		assert.Equal(t, "rmq-staging", report.ClusterName)
		assert.Equal(t, domainmessaging.RabbitMQStatusHealthy, report.Status)
	})

	// 4. GET /rabbitmq/{cluster_id}/audit - Flow Blocked (RFC 7807 503)
	t.Run("GET rabbitmq audit flow blocked 503", func(t *testing.T) {
		svc.SeedRabbitMQCluster(usecasemessaging.RabbitMQClusterRaw{
			ClusterName:       "rmq-deadlock-cluster",
			MemoryUsedBytes:   9000000000,
			MemoryLimitBytes:  10000000000,
			FlowBlockedConns:  2,
			QuorumQueueCount:  0,
			ClassicQueueCount: 20,
		})

		req := httptest.NewRequest(http.MethodGet, "/rabbitmq/rmq-deadlock-cluster/audit", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
		assert.Equal(t, "application/problem+json", rec.Header().Get("Content-Type"))

		var problem adapthttp.ProblemDetails
		err := json.NewDecoder(rec.Body).Decode(&problem)
		require.NoError(t, err)
		assert.Equal(t, domainmessaging.CodeRabbitMQFlowBlocked, problem.Code)
		assert.Contains(t, problem.Detail, "RabbitMQ node memory threshold exceeded")
	})

	// 5. POST /kafka/nodes/{node_name}/sysctl-tune
	t.Run("POST kafka sysctl-tune node", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/kafka/nodes/node-worker-streaming-01/sysctl-tune", nil)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusOK, rec.Code)
		var resp map[string]interface{}
		err := json.NewDecoder(rec.Body).Decode(&resp)
		require.NoError(t, err)
		assert.Equal(t, "tuned", resp["status"])
		assert.Equal(t, "node-worker-streaming-01", resp["node_name"])
	})
}

func TestRouter_MessagingSREIntegration(t *testing.T) {
	svc := usecasemessaging.NewService(usecasemessaging.WithLogger(zap.NewNop()))
	handler := adapthttp.NewMessagingSREHandler(svc, zap.NewNop())

	healthHandler := health.NewHandler(5 * time.Second)
	platform := &adapthttp.PlatformHandlers{
		MessagingSRE: handler,
	}

	router := adapthttp.NewRouterWithWS(healthHandler, nil, platform)

	adminToken, err := middleware.GenerateJWT("admin-user", "platform_admin", "default-tenant")
	require.NoError(t, err)

	svc.SeedKafkaGroup("cluster-cloud", usecasemessaging.KafkaConsumerGroupRaw{
		GroupID:            "payment-consumers",
		Topic:              "payments",
		PartitionCount:     12,
		ConsumerCount:      6,
		TotalLag:           10,
		AssignmentStrategy: domainmessaging.StrategyCooperativeSticky,
		RebalanceCount24h:  0,
		State:              "Stable",
	})

	// 1. GET /api/v1/messaging/kafka/{cluster_id}/groups
	req := httptest.NewRequest(http.MethodGet, "/api/v1/messaging/kafka/cluster-cloud/groups", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	var groups []domainmessaging.KafkaGroupHealth
	err = json.NewDecoder(rec.Body).Decode(&groups)
	require.NoError(t, err)
	require.Len(t, groups, 1)
	assert.Equal(t, "payment-consumers", groups[0].GroupID)

	// 2. POST /api/v1/messaging/kafka/nodes/{node_name}/sysctl-tune
	tuneReq := httptest.NewRequest(http.MethodPost, "/api/v1/messaging/kafka/nodes/kafka-broker-01/sysctl-tune", nil)
	tuneReq.Header.Set("Authorization", "Bearer "+adminToken)
	tuneRec := httptest.NewRecorder()
	router.ServeHTTP(tuneRec, tuneReq)

	assert.Equal(t, http.StatusOK, tuneRec.Code)
}
