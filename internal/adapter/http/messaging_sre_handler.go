// Package http provides HTTP adapters and handlers for the k8sselfhost API.
package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	domainmessaging "github.com/datdt/k8sselfhost/internal/domain/messaging"
)

// MessagingSREHandler provides HTTP endpoints for Message Queue & Streaming SRE auto-tuning
// including Kafka consumer group cooperative rebalancing, RabbitMQ memory watermark auditing,
// and node Linux kernel sysctl dirty memory tuning.
type MessagingSREHandler struct {
	manager domainmessaging.MessageQueueSREManager
	logger  *zap.Logger
}

// NewMessagingSREHandler creates a new MessagingSREHandler instance.
func NewMessagingSREHandler(manager domainmessaging.MessageQueueSREManager, loggers ...*zap.Logger) *MessagingSREHandler {
	var l *zap.Logger
	if len(loggers) > 0 && loggers[0] != nil {
		l = loggers[0]
	} else {
		l = zap.NewNop()
	}
	return &MessagingSREHandler{
		manager: manager,
		logger:  l,
	}
}

// RegisterRoutes mounts messaging SRE management routes onto the provided chi router.
func (h *MessagingSREHandler) RegisterRoutes(r chi.Router) {
	r.Get("/kafka/{cluster_id}/groups", h.InspectKafkaConsumerGroups)
	r.Post("/kafka/{cluster_id}/groups/{group_id}/enforce-cooperative", h.EnforceKafkaCooperativePolicy)
	r.Get("/rabbitmq/{cluster_id}/audit", h.AuditRabbitMQMemoryAndTopology)
	r.Post("/kafka/nodes/{node_name}/sysctl-tune", h.ApplyKafkaKernelTuning)
}

// InspectKafkaConsumerGroups handles GET /api/v1/messaging/kafka/{cluster_id}/groups.
func (h *MessagingSREHandler) InspectKafkaConsumerGroups(w http.ResponseWriter, r *http.Request) {
	clusterID := strings.TrimSpace(chi.URLParam(r, "cluster_id"))
	if clusterID == "" {
		h.writeProblem(w, http.StatusBadRequest, "Bad Request", "cluster_id path parameter is required", "VALIDATION_ERROR", r.URL.Path)
		return
	}

	groups, err := h.manager.InspectKafkaConsumerGroups(r.Context(), clusterID)
	if errors.Is(err, domainmessaging.ErrKafkaRebalanceStorm) {
		h.logger.Warn("Kafka rebalance storm detected during group inspection", zap.String("cluster_id", clusterID), zap.Error(err))
		h.writeProblem(w, http.StatusInternalServerError, "Kafka Rebalance Storm Detected", err.Error(), domainmessaging.CodeKafkaRebalanceStorm, r.URL.Path)
		return
	}
	if err != nil {
		h.logger.Error("Failed to inspect Kafka consumer groups", zap.String("cluster_id", clusterID), zap.Error(err))
		h.writeProblem(w, http.StatusInternalServerError, "Inspection Error", err.Error(), "ERR_KAFKA_INSPECT_FAILED", r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(groups)
}

// EnforceKafkaCooperativePolicy handles POST /api/v1/messaging/kafka/{cluster_id}/groups/{group_id}/enforce-cooperative.
func (h *MessagingSREHandler) EnforceKafkaCooperativePolicy(w http.ResponseWriter, r *http.Request) {
	clusterID := strings.TrimSpace(chi.URLParam(r, "cluster_id"))
	if clusterID == "" {
		h.writeProblem(w, http.StatusBadRequest, "Bad Request", "cluster_id path parameter is required", "VALIDATION_ERROR", r.URL.Path)
		return
	}
	groupID := strings.TrimSpace(chi.URLParam(r, "group_id"))
	if groupID == "" {
		h.writeProblem(w, http.StatusBadRequest, "Bad Request", "group_id path parameter is required", "VALIDATION_ERROR", r.URL.Path)
		return
	}

	if err := h.manager.EnforceKafkaCooperativePolicy(r.Context(), clusterID, groupID); err != nil {
		h.logger.Error("Failed to enforce cooperative sticky policy",
			zap.String("cluster_id", clusterID),
			zap.String("group_id", groupID),
			zap.Error(err),
		)
		h.writeProblem(w, http.StatusInternalServerError, "Policy Enforcement Failed", err.Error(), "ERR_KAFKA_ENFORCE_FAILED", r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":     "enforced",
		"cluster_id": clusterID,
		"group_id":   groupID,
		"strategy":   domainmessaging.StrategyCooperativeSticky,
	})
}

// AuditRabbitMQMemoryAndTopology handles GET /api/v1/messaging/rabbitmq/{cluster_id}/audit.
func (h *MessagingSREHandler) AuditRabbitMQMemoryAndTopology(w http.ResponseWriter, r *http.Request) {
	clusterID := strings.TrimSpace(chi.URLParam(r, "cluster_id"))
	if clusterID == "" {
		h.writeProblem(w, http.StatusBadRequest, "Bad Request", "cluster_id path parameter is required", "VALIDATION_ERROR", r.URL.Path)
		return
	}

	report, err := h.manager.AuditRabbitMQMemoryAndTopology(r.Context(), clusterID)
	if errors.Is(err, domainmessaging.ErrRabbitMQFlowBlocked) {
		h.logger.Warn("RabbitMQ flow control blocked due to memory watermark limit",
			zap.String("cluster_id", clusterID),
			zap.Error(err),
		)
		h.writeProblem(w, http.StatusServiceUnavailable, "RabbitMQ Flow Blocked", err.Error(), domainmessaging.CodeRabbitMQFlowBlocked, r.URL.Path)
		return
	}
	if err != nil {
		h.logger.Error("Failed to audit RabbitMQ memory and topology", zap.String("cluster_id", clusterID), zap.Error(err))
		h.writeProblem(w, http.StatusInternalServerError, "RabbitMQ Audit Failed", err.Error(), "ERR_RABBITMQ_AUDIT_FAILED", r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(report)
}

// ApplyKafkaKernelTuning handles POST /api/v1/messaging/kafka/nodes/{node_name}/sysctl-tune.
func (h *MessagingSREHandler) ApplyKafkaKernelTuning(w http.ResponseWriter, r *http.Request) {
	nodeName := strings.TrimSpace(chi.URLParam(r, "node_name"))
	if nodeName == "" {
		h.writeProblem(w, http.StatusBadRequest, "Bad Request", "node_name path parameter is required", "VALIDATION_ERROR", r.URL.Path)
		return
	}

	if err := h.manager.ApplyKafkaKernelTuning(r.Context(), nodeName); err != nil {
		h.logger.Error("Failed to apply Linux kernel sysctl dirty tuning to node", zap.String("node_name", nodeName), zap.Error(err))
		h.writeProblem(w, http.StatusInternalServerError, "Kernel Tuning Failed", err.Error(), "ERR_KERNEL_TUNING_FAILED", r.URL.Path)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "tuned",
		"node_name": nodeName,
		"profile": map[string]string{
			"vm.dirty_background_ratio": "3",
			"vm.dirty_ratio":           "10",
			"vm.dirty_expire_centisecs": "1000",
		},
	})
}

func (h *MessagingSREHandler) writeProblem(w http.ResponseWriter, status int, title, detail, code, instance string) {
	problem := ProblemDetails{
		Type:     "about:blank",
		Title:    title,
		Status:   status,
		Detail:   detail,
		Instance: instance,
		Code:     code,
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(problem)
}
