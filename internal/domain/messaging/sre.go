// Package messaging defines domain entities, value objects, and interfaces
// for message queue and streaming SRE auto-tuning (Kafka, RabbitMQ, NATS).
package messaging

import (
	"context"
	"errors"
	"fmt"
	"strings"

	domainerrors "github.com/datdt/k8sselfhost/internal/pkg/errors"
)

// RFC 7807 error codes for message queue operations.
const (
	CodeKafkaRebalanceStorm = "ERR_KAFKA_REBALANCE_STORM"
	CodeRabbitMQFlowBlocked = "ERR_RABBITMQ_FLOW_BLOCKED"
)

// Standard Kafka assignment strategies.
const (
	StrategyCooperativeSticky = "org.apache.kafka.clients.consumer.CooperativeStickyAssignor"
	StrategyRangeAssignor     = "org.apache.kafka.clients.consumer.RangeAssignor"
	StrategyRoundRobin        = "org.apache.kafka.clients.consumer.RoundRobinAssignor"
	StrategySticky            = "org.apache.kafka.clients.consumer.StickyAssignor"
)

// Kafka consumer group health status values.
const (
	KafkaStatusHealthy          = "HEALTHY"
	KafkaStatusRebalancingStorm = "REBALANCING_STORM"
	KafkaStatusDegraded         = "DEGRADED"
)

// RabbitMQ health status values.
const (
	RabbitMQStatusHealthy     = "HEALTHY"
	RabbitMQStatusWarning     = "WARNING"
	RabbitMQStatusFlowBlocked = "FLOW_BLOCKED"
	RabbitMQStatusDegraded    = "DEGRADED"
)

// Domain sentinel errors.
var (
	ErrKafkaRebalanceStorm = errors.New("kafka consumer group is stuck in rebalance storm")
	ErrRabbitMQFlowBlocked = errors.New("rabbitmq node memory threshold exceeded, publishers flow blocked")
)

// NewErrKafkaRebalanceStorm creates an RFC 7807 compliant domain error for Kafka rebalance storm.
func NewErrKafkaRebalanceStorm(groupID string, totalLag int64, rebalances int) *domainerrors.DomainError {
	return &domainerrors.DomainError{
		Code:    CodeKafkaRebalanceStorm,
		Message: fmt.Sprintf("Kafka consumer group '%s' is stuck in rebalance storm (Lag: %d, Rebalances: %d).", groupID, totalLag, rebalances),
		Err:     ErrKafkaRebalanceStorm,
	}
}

// NewErrRabbitMQFlowBlocked creates an RFC 7807 compliant domain error for RabbitMQ flow control block.
func NewErrRabbitMQFlowBlocked(memoryPercentage float64) *domainerrors.DomainError {
	return &domainerrors.DomainError{
		Code:    CodeRabbitMQFlowBlocked,
		Message: fmt.Sprintf("RabbitMQ node memory threshold exceeded (Memory: %.2f%%). Publishers flow blocked.", memoryPercentage),
		Err:     ErrRabbitMQFlowBlocked,
	}
}

// KafkaGroupHealth represents consumer group health, topology, and rebalance state.
type KafkaGroupHealth struct {
	GroupID            string `json:"group_id"`
	Topic              string `json:"topic"`
	PartitionCount     int    `json:"partition_count"`
	ConsumerCount      int    `json:"consumer_count"`
	TotalLag           int64  `json:"total_lag"`
	AssignmentStrategy string `json:"assignment_strategy"`
	IsCooperative      bool   `json:"is_cooperative"`
	RebalanceCount24h  int    `json:"rebalance_count_24h"`
	Status             string `json:"status"` // HEALTHY, REBALANCING_STORM, DEGRADED
}

// Validate checks essential invariants for KafkaGroupHealth.
func (k *KafkaGroupHealth) Validate() error {
	if strings.TrimSpace(k.GroupID) == "" {
		return domainerrors.NewValidation("group_id", "group_id cannot be empty")
	}
	if k.PartitionCount < 0 {
		return domainerrors.NewValidation("partition_count", "partition_count cannot be negative")
	}
	if k.ConsumerCount < 0 {
		return domainerrors.NewValidation("consumer_count", "consumer_count cannot be negative")
	}
	if k.TotalLag < 0 {
		return domainerrors.NewValidation("total_lag", "total_lag cannot be negative")
	}
	return nil
}

// RabbitMQHealthReport represents broker memory metrics, connection states, and queue topology.
type RabbitMQHealthReport struct {
	ClusterName       string  `json:"cluster_name"`
	MemoryUsedBytes   int64   `json:"memory_used_bytes"`
	MemoryLimitBytes  int64   `json:"memory_limit_bytes"`
	MemoryPercentage  float64 `json:"memory_percentage"`
	FlowBlockedConns  int     `json:"flow_blocked_conns"`
	QuorumQueueCount  int     `json:"quorum_queue_count"`
	ClassicQueueCount int     `json:"classic_queue_count"`
	Status            string  `json:"status"` // HEALTHY, WARNING, FLOW_BLOCKED, DEGRADED
}

// Validate checks essential invariants for RabbitMQHealthReport.
func (r *RabbitMQHealthReport) Validate() error {
	if strings.TrimSpace(r.ClusterName) == "" {
		return domainerrors.NewValidation("cluster_name", "cluster_name cannot be empty")
	}
	if r.MemoryUsedBytes < 0 {
		return domainerrors.NewValidation("memory_used_bytes", "memory_used_bytes cannot be negative")
	}
	if r.MemoryLimitBytes <= 0 {
		return domainerrors.NewValidation("memory_limit_bytes", "memory_limit_bytes must be greater than zero")
	}
	if r.FlowBlockedConns < 0 {
		return domainerrors.NewValidation("flow_blocked_conns", "flow_blocked_conns cannot be negative")
	}
	return nil
}

// MessageQueueSREManager defines administrative and auto-tuning capabilities
// for distributed streaming and message queuing platforms.
type MessageQueueSREManager interface {
	InspectKafkaConsumerGroups(ctx context.Context, clusterID string) ([]KafkaGroupHealth, error)
	EnforceKafkaCooperativePolicy(ctx context.Context, clusterID, groupID string) error
	AuditRabbitMQMemoryAndTopology(ctx context.Context, clusterID string) (*RabbitMQHealthReport, error)
	ApplyKafkaKernelTuning(ctx context.Context, nodeName string) error
}
