package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/datdt/k8sselfhost/internal/adapter/http/middleware"
	domain "github.com/datdt/k8sselfhost/internal/domain/messaging"
)

type mockMessageQueueSREManager struct {
	groups      []domain.KafkaGroupHealth
	report      *domain.RabbitMQHealthReport
	inspectErr  error
	enforceErr  error
	auditErr    error
	kernelErr   error
	enforcedGrp string
	tunedNode   string
}

func (m *mockMessageQueueSREManager) InspectKafkaConsumerGroups(ctx context.Context, clusterID string) ([]domain.KafkaGroupHealth, error) {
	if m.inspectErr != nil {
		return nil, m.inspectErr
	}
	return m.groups, nil
}

func (m *mockMessageQueueSREManager) EnforceKafkaCooperativePolicy(ctx context.Context, clusterID, groupID string) error {
	if m.enforceErr != nil {
		return m.enforceErr
	}
	m.enforcedGrp = groupID
	return nil
}

func (m *mockMessageQueueSREManager) AuditRabbitMQMemoryAndTopology(ctx context.Context, clusterID string) (*domain.RabbitMQHealthReport, error) {
	if m.auditErr != nil {
		return nil, m.auditErr
	}
	return m.report, nil
}

func (m *mockMessageQueueSREManager) ApplyKafkaKernelTuning(ctx context.Context, nodeName string) error {
	if m.kernelErr != nil {
		return m.kernelErr
	}
	m.tunedNode = nodeName
	return nil
}

func setupMessagingTestRouter(mgr domain.MessageQueueSREManager) *chi.Mux {
	r := chi.NewRouter()
	handler := NewMessagingHandler(mgr)
	handler.WireToRouter(r)
	return r
}

func withUserRole(req *http.Request, role string) *http.Request {
	ctx := context.WithValue(req.Context(), middleware.UserRoleKey, role)
	return req.WithContext(ctx)
}

func TestMessagingHandler_InspectKafkaConsumerGroups(t *testing.T) {
	gh, _ := domain.NewKafkaGroupHealth("order-grp", "orders.v1", 12, 12, 100, domain.CooperativeStickyAssignor, true, 2)
	mgr := &mockMessageQueueSREManager{
		groups: []domain.KafkaGroupHealth{*gh},
	}
	router := setupMessagingTestRouter(mgr)

	t.Run("Success", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/clusters/cluster-01/messaging/kafka/consumer-groups", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d. Body: %s", rec.Code, rec.Body.String())
		}

		var resp struct {
			ClusterID string                    `json:"cluster_id"`
			Groups    []domain.KafkaGroupHealth `json:"groups"`
		}
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if len(resp.Groups) != 1 || resp.Groups[0].GroupID != "order-grp" {
			t.Errorf("unexpected groups response: %+v", resp.Groups)
		}
	})

	t.Run("Internal error from manager", func(t *testing.T) {
		errMgr := &mockMessageQueueSREManager{inspectErr: errors.New("backend timeout")}
		errRouter := setupMessagingTestRouter(errMgr)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/clusters/cluster-01/messaging/kafka/consumer-groups", nil)
		rec := httptest.NewRecorder()
		errRouter.ServeHTTP(rec, req)

		if rec.Code != http.StatusInternalServerError {
			t.Errorf("expected 500 InternalServerError, got %d", rec.Code)
		}
	})
}

func TestMessagingHandler_EnforceKafkaCooperativePolicy(t *testing.T) {
	mgr := &mockMessageQueueSREManager{}
	router := setupMessagingTestRouter(mgr)

	t.Run("Unauthorized without role", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/clusters/cluster-01/messaging/kafka/consumer-groups/order-grp/enforce-cooperative", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 Unauthorized, got %d", rec.Code)
		}
	})

	t.Run("Forbidden with viewer role", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/clusters/cluster-01/messaging/kafka/consumer-groups/order-grp/enforce-cooperative", nil)
		req = withUserRole(req, "viewer")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Errorf("expected 403 Forbidden, got %d", rec.Code)
		}
	})

	t.Run("Success with platform_admin role", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/clusters/cluster-01/messaging/kafka/consumer-groups/order-grp/enforce-cooperative", nil)
		req = withUserRole(req, "platform_admin")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d. Body: %s", rec.Code, rec.Body.String())
		}
		if mgr.enforcedGrp != "order-grp" {
			t.Errorf("expected enforced group 'order-grp', got '%s'", mgr.enforcedGrp)
		}
	})
}

func TestMessagingHandler_AuditRabbitMQ(t *testing.T) {
	report, _ := domain.NewRabbitMQHealthReport("rabbit-cluster", 40*1024, 100*1024, 0, 10, 0)
	mgr := &mockMessageQueueSREManager{report: report}
	router := setupMessagingTestRouter(mgr)

	t.Run("Success", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/clusters/cluster-01/messaging/rabbitmq/health", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d. Body: %s", rec.Code, rec.Body.String())
		}

		var resp domain.RabbitMQHealthReport
		if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if resp.ClusterName != "rabbit-cluster" || resp.Status != domain.RabbitMQStatusHealthy {
			t.Errorf("unexpected report: %+v", resp)
		}
	})
}

func TestMessagingHandler_ApplyKafkaKernelTuning(t *testing.T) {
	mgr := &mockMessageQueueSREManager{}
	router := setupMessagingTestRouter(mgr)

	t.Run("Unauthorized without role", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/clusters/cluster-01/messaging/kafka/nodes/worker-node-01/kernel-tune", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("expected 401 Unauthorized, got %d", rec.Code)
		}
	})

	t.Run("Success with tenant_admin role", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/clusters/cluster-01/messaging/kafka/nodes/worker-node-01/kernel-tune", nil)
		req = withUserRole(req, "tenant_admin")
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d. Body: %s", rec.Code, rec.Body.String())
		}
		if mgr.tunedNode != "worker-node-01" {
			t.Errorf("expected tuned node 'worker-node-01', got '%s'", mgr.tunedNode)
		}
	})
}
