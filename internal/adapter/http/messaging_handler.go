package http

import (
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/datdt/k8sselfhost/internal/adapter/http/middleware"
	domain "github.com/datdt/k8sselfhost/internal/domain/messaging"
)

// MessagingHandler provides REST endpoints for Kafka, RabbitMQ, and host kernel SRE operations.
type MessagingHandler struct {
	manager domain.MessageQueueSREManager
}

// NewMessagingHandler constructs a new MessagingHandler with the given domain SRE manager.
func NewMessagingHandler(manager domain.MessageQueueSREManager) *MessagingHandler {
	return &MessagingHandler{
		manager: manager,
	}
}

// RegisterRoutes registers the messaging endpoints under a cluster-scoped parent router.
func (h *MessagingHandler) RegisterRoutes(r chi.Router) {
	r.Get("/kafka/consumer-groups", h.InspectKafkaConsumerGroups)
	r.Get("/rabbitmq/health", h.AuditRabbitMQ)
	r.Get("/tenants/{tenantId}/backlog-alerts", h.GetTenantBacklogAlerts)

	r.With(middleware.RBACMiddleware("platform_admin", "tenant_admin")).Group(func(mut chi.Router) {
		mut.Post("/kafka/consumer-groups/{groupId}/enforce-cooperative", h.EnforceKafkaCooperativePolicy)
		mut.Post("/kafka/nodes/{nodeName}/kernel-tune", h.ApplyKafkaKernelTuning)
	})
}

// WireToRouter attaches cluster routes under /api/v1/clusters/{clusterId}/messaging onto a root router.
func (h *MessagingHandler) WireToRouter(r chi.Router) {
	r.Route("/api/v1/clusters/{clusterId}/messaging", func(sub chi.Router) {
		h.RegisterRoutes(sub)
	})
}

func extractGroupID(r *http.Request) string {
	if id := strings.TrimSpace(chi.URLParam(r, "groupId")); id != "" {
		return id
	}
	if id := strings.TrimSpace(chi.URLParam(r, "group_id")); id != "" {
		return id
	}
	if id := strings.TrimSpace(r.URL.Query().Get("groupId")); id != "" {
		return id
	}
	return strings.TrimSpace(r.URL.Query().Get("group_id"))
}

func extractNodeName(r *http.Request) string {
	if name := strings.TrimSpace(chi.URLParam(r, "nodeName")); name != "" {
		return name
	}
	if name := strings.TrimSpace(chi.URLParam(r, "node_name")); name != "" {
		return name
	}
	if name := strings.TrimSpace(r.URL.Query().Get("nodeName")); name != "" {
		return name
	}
	return strings.TrimSpace(r.URL.Query().Get("node_name"))
}

// InspectKafkaConsumerGroups handles GET /api/v1/clusters/{clusterId}/messaging/kafka/consumer-groups
func (h *MessagingHandler) InspectKafkaConsumerGroups(w http.ResponseWriter, r *http.Request) {
	if h.manager == nil {
		writeError(w, http.StatusServiceUnavailable, "messaging SRE manager not configured", nil)
		return
	}

	clusterID := extractClusterID(r)
	if clusterID == "" {
		writeError(w, http.StatusBadRequest, "clusterId parameter is required", nil)
		return
	}

	groups, err := h.manager.InspectKafkaConsumerGroups(r.Context(), clusterID)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidClusterID) {
			writeError(w, http.StatusBadRequest, err.Error(), err)
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to inspect kafka consumer groups", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"cluster_id": clusterID,
		"groups":     groups,
	})
}

// EnforceKafkaCooperativePolicy handles POST /api/v1/clusters/{clusterId}/messaging/kafka/consumer-groups/{groupId}/enforce-cooperative
func (h *MessagingHandler) EnforceKafkaCooperativePolicy(w http.ResponseWriter, r *http.Request) {
	if h.manager == nil {
		writeError(w, http.StatusServiceUnavailable, "messaging SRE manager not configured", nil)
		return
	}

	clusterID := extractClusterID(r)
	if clusterID == "" {
		writeError(w, http.StatusBadRequest, "clusterId parameter is required", nil)
		return
	}

	groupID := extractGroupID(r)
	if groupID == "" {
		writeError(w, http.StatusBadRequest, "groupId parameter is required", nil)
		return
	}

	err := h.manager.EnforceKafkaCooperativePolicy(r.Context(), clusterID, groupID)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidGroupID) || errors.Is(err, domain.ErrInvalidClusterID) {
			writeError(w, http.StatusBadRequest, err.Error(), err)
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to enforce cooperative policy", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"cluster_id": clusterID,
		"group_id":   groupID,
		"status":     "COOPERATIVE_POLICY_ENFORCED",
		"assignor":   domain.CooperativeStickyAssignor,
	})
}

// AuditRabbitMQ handles GET /api/v1/clusters/{clusterId}/messaging/rabbitmq/health
func (h *MessagingHandler) AuditRabbitMQ(w http.ResponseWriter, r *http.Request) {
	if h.manager == nil {
		writeError(w, http.StatusServiceUnavailable, "messaging SRE manager not configured", nil)
		return
	}

	clusterID := extractClusterID(r)
	if clusterID == "" {
		writeError(w, http.StatusBadRequest, "clusterId parameter is required", nil)
		return
	}

	report, err := h.manager.AuditRabbitMQMemoryAndTopology(r.Context(), clusterID)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidClusterID) {
			writeError(w, http.StatusBadRequest, err.Error(), err)
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to audit rabbitmq cluster", err)
		return
	}

	writeJSON(w, http.StatusOK, report)
}

// ApplyKafkaKernelTuning handles POST /api/v1/clusters/{clusterId}/messaging/kafka/nodes/{nodeName}/kernel-tune
func (h *MessagingHandler) ApplyKafkaKernelTuning(w http.ResponseWriter, r *http.Request) {
	if h.manager == nil {
		writeError(w, http.StatusServiceUnavailable, "messaging SRE manager not configured", nil)
		return
	}

	nodeName := extractNodeName(r)
	if nodeName == "" {
		writeError(w, http.StatusBadRequest, "nodeName parameter is required", nil)
		return
	}

	err := h.manager.ApplyKafkaKernelTuning(r.Context(), nodeName)
	if err != nil {
		if errors.Is(err, domain.ErrInvalidNodeName) {
			writeError(w, http.StatusBadRequest, err.Error(), err)
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to apply kafka kernel tuning", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"node_name": nodeName,
		"status":    "KERNEL_PROFILE_APPLIED",
		"profile":   domain.DefaultKafkaKernelProfile(),
	})
}

// GetTenantBacklogAlerts handles GET /api/v1/messaging/tenants/{tenantId}/backlog-alerts
func (h *MessagingHandler) GetTenantBacklogAlerts(w http.ResponseWriter, r *http.Request) {
	tenantID := strings.TrimSpace(chi.URLParam(r, "tenantId"))
	if tenantID == "" {
		writeError(w, http.StatusBadRequest, "tenantId parameter is required", nil)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"tenant_id": tenantID,
		"alerts":    []interface{}{},
	})
}
