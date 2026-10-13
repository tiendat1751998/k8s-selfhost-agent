package messaging_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/datdt/k8sselfhost/internal/domain/messaging"
)

func TestKafkaGroupHealth_DomainErrors(t *testing.T) {
	err := messaging.NewErrKafkaRebalanceStorm("orders-consumer-group", 15420, 5)
	assert.NotNil(t, err)
	assert.Equal(t, messaging.CodeKafkaRebalanceStorm, err.Code)
	assert.Contains(t, err.Message, "orders-consumer-group")
	assert.Contains(t, err.Message, "15420")
	assert.Contains(t, err.Message, "5")
	assert.True(t, errors.Is(err, messaging.ErrKafkaRebalanceStorm))
}

func TestRabbitMQHealthReport_DomainErrors(t *testing.T) {
	err := messaging.NewErrRabbitMQFlowBlocked(92.45)
	assert.NotNil(t, err)
	assert.Equal(t, messaging.CodeRabbitMQFlowBlocked, err.Code)
	assert.Contains(t, err.Message, "92.45%")
	assert.True(t, errors.Is(err, messaging.ErrRabbitMQFlowBlocked))
}

func TestDomainEntities_DefaultsAndStatus(t *testing.T) {
	kGroup := messaging.KafkaGroupHealth{
		GroupID:            "telemetry-processor",
		Topic:              "events.metrics",
		PartitionCount:     12,
		ConsumerCount:      6,
		TotalLag:           0,
		AssignmentStrategy: messaging.StrategyCooperativeSticky,
		IsCooperative:      true,
		RebalanceCount24h:  0,
		Status:             messaging.KafkaStatusHealthy,
	}

	assert.True(t, kGroup.IsCooperative)
	assert.Equal(t, messaging.KafkaStatusHealthy, kGroup.Status)

	rmqReport := messaging.RabbitMQHealthReport{
		ClusterName:       "prod-rmq-cluster",
		MemoryUsedBytes:   8 * 1024 * 1024 * 1024,
		MemoryLimitBytes:  16 * 1024 * 1024 * 1024,
		MemoryPercentage:  50.0,
		FlowBlockedConns:  0,
		QuorumQueueCount:  20,
		ClassicQueueCount: 0,
		Status:            messaging.RabbitMQStatusHealthy,
	}

	assert.Equal(t, 50.0, rmqReport.MemoryPercentage)
	assert.Equal(t, 0, rmqReport.FlowBlockedConns)
	assert.Equal(t, messaging.RabbitMQStatusHealthy, rmqReport.Status)
}
