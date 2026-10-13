package messaging_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	domainmessaging "github.com/datdt/k8sselfhost/internal/domain/messaging"
	usecasemessaging "github.com/datdt/k8sselfhost/internal/usecase/messaging"
)

func TestKafka_DetectRebalanceStorm(t *testing.T) {
	ctx := context.Background()
	svc := usecasemessaging.NewService(usecasemessaging.WithLogger(zap.NewNop()))

	// Register test groups
	svc.SeedKafkaGroup("cluster-alpha", usecasemessaging.KafkaConsumerGroupRaw{
		GroupID:            "order-stream-processor",
		Topic:              "orders.v1",
		PartitionCount:     16,
		ConsumerCount:      8,
		TotalLag:           25000,
		AssignmentStrategy: domainmessaging.StrategyRangeAssignor,
		RebalanceCount24h:  14,
		State:              "PreparingRebalance",
	})

	svc.SeedKafkaGroup("cluster-alpha", usecasemessaging.KafkaConsumerGroupRaw{
		GroupID:            "stable-payment-consumer",
		Topic:              "payments.v1",
		PartitionCount:     8,
		ConsumerCount:      4,
		TotalLag:           10,
		AssignmentStrategy: domainmessaging.StrategyCooperativeSticky,
		RebalanceCount24h:  0,
		State:              "Stable",
	})

	groups, err := svc.InspectKafkaConsumerGroups(ctx, "cluster-alpha")
	require.NoError(t, err)
	require.Len(t, groups, 2)

	var stormGroup, stableGroup *domainmessaging.KafkaGroupHealth
	for i := range groups {
		if groups[i].GroupID == "order-stream-processor" {
			stormGroup = &groups[i]
		}
		if groups[i].GroupID == "stable-payment-consumer" {
			stableGroup = &groups[i]
		}
	}

	require.NotNil(t, stormGroup)
	assert.Equal(t, domainmessaging.KafkaStatusRebalancingStorm, stormGroup.Status)
	assert.False(t, stormGroup.IsCooperative)

	require.NotNil(t, stableGroup)
	assert.Equal(t, domainmessaging.KafkaStatusHealthy, stableGroup.Status)
	assert.True(t, stableGroup.IsCooperative)
}

func TestKafka_InjectCooperativeConfig(t *testing.T) {
	ctx := context.Background()
	svc := usecasemessaging.NewService(usecasemessaging.WithLogger(zap.NewNop()))

	svc.SeedKafkaGroup("cluster-prod", usecasemessaging.KafkaConsumerGroupRaw{
		GroupID:            "checkout-worker-group",
		Topic:              "checkout.events",
		PartitionCount:     24,
		ConsumerCount:      12,
		TotalLag:           4000,
		AssignmentStrategy: domainmessaging.StrategyRoundRobin,
		RebalanceCount24h:  5,
		State:              "CompletingRebalance",
		DeploymentName:     "checkout-worker",
	})

	// Before enforcement: group is degraded or rebalancing
	groupsBefore, err := svc.InspectKafkaConsumerGroups(ctx, "cluster-prod")
	require.NoError(t, err)
	require.Len(t, groupsBefore, 1)
	assert.False(t, groupsBefore[0].IsCooperative)

	// Enforce CooperativeStickyAssignor policy
	err = svc.EnforceKafkaCooperativePolicy(ctx, "cluster-prod", "checkout-worker-group")
	require.NoError(t, err)

	// After enforcement: verify assignment strategy & static membership & status
	groupsAfter, err := svc.InspectKafkaConsumerGroups(ctx, "cluster-prod")
	require.NoError(t, err)
	require.Len(t, groupsAfter, 1)
	assert.True(t, groupsAfter[0].IsCooperative)
	assert.Equal(t, domainmessaging.StrategyCooperativeSticky, groupsAfter[0].AssignmentStrategy)

	// Verify internal configuration record
	config, ok := svc.GetGroupConfig("cluster-prod", "checkout-worker-group")
	require.True(t, ok)
	assert.Equal(t, domainmessaging.StrategyCooperativeSticky, config.AssignmentStrategy)
	assert.NotEmpty(t, config.StaticInstanceID)
	assert.True(t, config.RollingRestart)
}

func TestRabbitMQ_DetectMemoryAlarm(t *testing.T) {
	ctx := context.Background()
	svc := usecasemessaging.NewService(usecasemessaging.WithLogger(zap.NewNop()))

	// Mock RabbitMQ with 92% memory usage
	svc.SeedRabbitMQCluster(usecasemessaging.RabbitMQClusterRaw{
		ClusterName:       "rmq-ecommerce-prod",
		MemoryUsedBytes:   9200000000,
		MemoryLimitBytes:  10000000000,
		FlowBlockedConns:  4,
		QuorumQueueCount:  5,
		ClassicQueueCount: 30,
	})

	report, err := svc.AuditRabbitMQMemoryAndTopology(ctx, "rmq-ecommerce-prod")
	require.Error(t, err)
	assert.True(t, errors.Is(err, domainmessaging.ErrRabbitMQFlowBlocked))

	require.NotNil(t, report)
	assert.Equal(t, 92.0, report.MemoryPercentage)
	assert.Equal(t, 4, report.FlowBlockedConns)
	assert.Equal(t, domainmessaging.RabbitMQStatusFlowBlocked, report.Status)

	// Verify backpressure was signaled
	assert.True(t, svc.IsRabbitMQBackpressureActive("rmq-ecommerce-prod"))
}

func TestKafka_KernelTuningProfile(t *testing.T) {
	ctx := context.Background()
	svc := usecasemessaging.NewService(usecasemessaging.WithLogger(zap.NewNop()))

	err := svc.ApplyKafkaKernelTuning(ctx, "k8s-node-kafka-worker-01")
	require.NoError(t, err)

	profile, ok := svc.GetNodeSysctlProfile("k8s-node-kafka-worker-01")
	require.True(t, ok)
	assert.Equal(t, "3", profile["vm.dirty_background_ratio"])
	assert.Equal(t, "10", profile["vm.dirty_ratio"])
	assert.Equal(t, "1000", profile["vm.dirty_expire_centisecs"])
}

func BenchmarkInspectKafkaConsumerGroups(b *testing.B) {
	ctx := context.Background()
	svc := usecasemessaging.NewService(usecasemessaging.WithLogger(zap.NewNop()))

	for i := 0; i < 50; i++ {
		svc.SeedKafkaGroup("bench-cluster", usecasemessaging.KafkaConsumerGroupRaw{
			GroupID:            "bench-group",
			Topic:              "bench.events",
			PartitionCount:     32,
			ConsumerCount:      16,
			TotalLag:           100,
			AssignmentStrategy: domainmessaging.StrategyCooperativeSticky,
			RebalanceCount24h:  0,
			State:              "Stable",
		})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := svc.InspectKafkaConsumerGroups(ctx, "bench-cluster")
		if err != nil {
			b.Fatalf("unexpected error: %v", err)
		}
	}
}
