package messaging

import (
	"sync"
	"testing"
)

func TestKafkaGroupHealth_Validation(t *testing.T) {
	tests := []struct {
		name               string
		groupID            string
		topic              string
		partitionCount     int
		consumerCount      int
		totalLag           int64
		assignmentStrategy string
		isCooperative      bool
		rebalanceCount24h  int
		expectErr          bool
		expectedStatus     KafkaGroupStatus
	}{
		{
			name:               "Valid cooperative healthy group",
			groupID:            "order-processing-group",
			topic:              "orders.v1",
			partitionCount:     12,
			consumerCount:      12,
			totalLag:           50,
			assignmentStrategy: CooperativeStickyAssignor,
			isCooperative:      true,
			rebalanceCount24h:  2,
			expectErr:          false,
			expectedStatus:     KafkaGroupStatusHealthy,
		},
		{
			name:               "Valid group with high rebalances - storm status",
			groupID:            "payment-group",
			topic:              "payments.v1",
			partitionCount:     8,
			consumerCount:      8,
			totalLag:           5000,
			assignmentStrategy: RangeAssignor,
			isCooperative:      false,
			rebalanceCount24h:  45,
			expectErr:          false,
			expectedStatus:     KafkaGroupStatusRebalanceStorm,
		},
		{
			name:               "Valid group with warning status due to lag and non-cooperative",
			groupID:            "analytics-group",
			topic:              "events.v1",
			partitionCount:     6,
			consumerCount:      6,
			totalLag:           15000,
			assignmentStrategy: RoundRobinAssignor,
			isCooperative:      false,
			rebalanceCount24h:  8,
			expectErr:          false,
			expectedStatus:     KafkaGroupStatusWarning,
		},
		{
			name:               "Missing group ID",
			groupID:            "",
			topic:              "orders.v1",
			partitionCount:     12,
			consumerCount:      12,
			totalLag:           50,
			assignmentStrategy: CooperativeStickyAssignor,
			isCooperative:      true,
			rebalanceCount24h:  0,
			expectErr:          true,
		},
		{
			name:               "Missing topic",
			groupID:            "order-group",
			topic:              "",
			partitionCount:     12,
			consumerCount:      12,
			totalLag:           50,
			assignmentStrategy: CooperativeStickyAssignor,
			isCooperative:      true,
			rebalanceCount24h:  0,
			expectErr:          true,
		},
		{
			name:               "Negative partition count",
			groupID:            "order-group",
			topic:              "orders.v1",
			partitionCount:     -1,
			consumerCount:      1,
			totalLag:           0,
			assignmentStrategy: CooperativeStickyAssignor,
			isCooperative:      true,
			rebalanceCount24h:  0,
			expectErr:          true,
		},
		{
			name:               "Negative consumer count",
			groupID:            "order-group",
			topic:              "orders.v1",
			partitionCount:     1,
			consumerCount:      -1,
			totalLag:           0,
			assignmentStrategy: CooperativeStickyAssignor,
			isCooperative:      true,
			rebalanceCount24h:  0,
			expectErr:          true,
		},
		{
			name:               "Negative total lag",
			groupID:            "order-group",
			topic:              "orders.v1",
			partitionCount:     1,
			consumerCount:      1,
			totalLag:           -10,
			assignmentStrategy: CooperativeStickyAssignor,
			isCooperative:      true,
			rebalanceCount24h:  0,
			expectErr:          true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			health, err := NewKafkaGroupHealth(
				tt.groupID,
				tt.topic,
				tt.partitionCount,
				tt.consumerCount,
				tt.totalLag,
				tt.assignmentStrategy,
				tt.isCooperative,
				tt.rebalanceCount24h,
			)

			if tt.expectErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if health.Status != tt.expectedStatus {
				t.Errorf("expected status %s, got %s", tt.expectedStatus, health.Status)
			}
			if health.GroupID != tt.groupID {
				t.Errorf("expected groupID %s, got %s", tt.groupID, health.GroupID)
			}
			if health.Topic != tt.topic {
				t.Errorf("expected topic %s, got %s", tt.topic, health.Topic)
			}
		})
	}
}

func TestRabbitMQHealthReport_Validation(t *testing.T) {
	tests := []struct {
		name              string
		clusterName       string
		memoryUsedBytes   uint64
		memoryLimitBytes  uint64
		flowBlockedConns  int
		quorumQueueCount  int
		classicQueueCount int
		expectErr         bool
		expectedStatus    RabbitMQStatus
		expectedMinPct    float64
	}{
		{
			name:              "Healthy RabbitMQ cluster (<70% memory, no blocked flows)",
			clusterName:       "prod-rabbit-01",
			memoryUsedBytes:   5 * 1024 * 1024 * 1024,  // 5 GB
			memoryLimitBytes:  10 * 1024 * 1024 * 1024, // 10 GB
			flowBlockedConns:  0,
			quorumQueueCount:  20,
			classicQueueCount: 0,
			expectErr:         false,
			expectedStatus:    RabbitMQStatusHealthy,
			expectedMinPct:    50.0,
		},
		{
			name:              "Warning RabbitMQ cluster (70-85% memory)",
			clusterName:       "prod-rabbit-02",
			memoryUsedBytes:   75 * 1024 * 1024 * 1024,  // 75 GB
			memoryLimitBytes:  100 * 1024 * 1024 * 1024, // 100 GB
			flowBlockedConns:  0,
			quorumQueueCount:  15,
			classicQueueCount: 5,
			expectErr:         false,
			expectedStatus:    RabbitMQStatusWarning,
			expectedMinPct:    75.0,
		},
		{
			name:              "High watermark RabbitMQ cluster (>85% memory)",
			clusterName:       "prod-rabbit-03",
			memoryUsedBytes:   88 * 1024 * 1024 * 1024,  // 88 GB
			memoryLimitBytes:  100 * 1024 * 1024 * 1024, // 100 GB
			flowBlockedConns:  12,
			quorumQueueCount:  10,
			classicQueueCount: 10,
			expectErr:         false,
			expectedStatus:    RabbitMQStatusHighWatermark,
			expectedMinPct:    88.0,
		},
		{
			name:              "Critical RabbitMQ cluster with blocked flows even if memory under 85%",
			clusterName:       "prod-rabbit-04",
			memoryUsedBytes:   60 * 1024 * 1024 * 1024,  // 60 GB
			memoryLimitBytes:  100 * 1024 * 1024 * 1024, // 100 GB
			flowBlockedConns:  4,
			quorumQueueCount:  5,
			classicQueueCount: 20,
			expectErr:         false,
			expectedStatus:    RabbitMQStatusCritical,
			expectedMinPct:    60.0,
		},
		{
			name:              "Missing cluster name",
			clusterName:       "",
			memoryUsedBytes:   1024,
			memoryLimitBytes:  2048,
			flowBlockedConns:  0,
			quorumQueueCount:  1,
			classicQueueCount: 0,
			expectErr:         true,
		},
		{
			name:              "Zero memory limit",
			clusterName:       "rabbit-zero",
			memoryUsedBytes:   1024,
			memoryLimitBytes:  0,
			flowBlockedConns:  0,
			quorumQueueCount:  1,
			classicQueueCount: 0,
			expectErr:         true,
		},
		{
			name:              "Negative flow blocked connections",
			clusterName:       "rabbit-neg",
			memoryUsedBytes:   1024,
			memoryLimitBytes:  2048,
			flowBlockedConns:  -1,
			quorumQueueCount:  1,
			classicQueueCount: 0,
			expectErr:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			report, err := NewRabbitMQHealthReport(
				tt.clusterName,
				tt.memoryUsedBytes,
				tt.memoryLimitBytes,
				tt.flowBlockedConns,
				tt.quorumQueueCount,
				tt.classicQueueCount,
			)

			if tt.expectErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if report.Status != tt.expectedStatus {
				t.Errorf("expected status %s, got %s", tt.expectedStatus, report.Status)
			}
			if report.MemoryPercentage < tt.expectedMinPct-0.1 || report.MemoryPercentage > tt.expectedMinPct+0.1 {
				t.Errorf("expected memory percentage ~%f, got %f", tt.expectedMinPct, report.MemoryPercentage)
			}
		})
	}
}

func TestDefaultKafkaKernelProfile(t *testing.T) {
	profile := DefaultKafkaKernelProfile()

	if profile.DirtyBackgroundRatio != 3 {
		t.Errorf("expected DirtyBackgroundRatio=3, got %d", profile.DirtyBackgroundRatio)
	}
	if profile.DirtyRatio != 10 {
		t.Errorf("expected DirtyRatio=10, got %d", profile.DirtyRatio)
	}
	if profile.DirtyExpireCentisecs != 1000 {
		t.Errorf("expected DirtyExpireCentisecs=1000, got %d", profile.DirtyExpireCentisecs)
	}
}

func TestPaddedThroughputMetric_Concurrency(t *testing.T) {
	metrics := NewPaddedThroughputMetric()

	var wg sync.WaitGroup
	workers := 50
	iterations := 1000

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				metrics.IncTotalInspections()
				metrics.IncActiveOperations()
				metrics.DecActiveOperations()
				metrics.IncRebalanceStormsMitigated()
				metrics.IncWatermarkRescuesTriggered()
			}
		}()
	}

	wg.Wait()

	expectedCount := uint64(workers * iterations)
	if metrics.TotalInspections() != expectedCount {
		t.Errorf("expected inspections %d, got %d", expectedCount, metrics.TotalInspections())
	}
	if metrics.RebalanceStormsMitigated() != expectedCount {
		t.Errorf("expected mitigated %d, got %d", expectedCount, metrics.RebalanceStormsMitigated())
	}
	if metrics.WatermarkRescuesTriggered() != expectedCount {
		t.Errorf("expected rescues %d, got %d", expectedCount, metrics.WatermarkRescuesTriggered())
	}
	if metrics.ActiveOperations() != 0 {
		t.Errorf("expected active ops 0, got %d", metrics.ActiveOperations())
	}
}

func TestPaddedSREQueue_BoundedRingBuffer(t *testing.T) {
	q := NewPaddedSREQueue(8)
	if q.Capacity() != 8 {
		t.Fatalf("expected capacity 8, got %d", q.Capacity())
	}

	for i := 0; i < 8; i++ {
		ok := q.TryPush(SREAuditRecord{
			Timestamp: 1000 + int64(i),
			Action:    "cooperative_assignment_enforced",
			Target:    "group-alpha",
		})
		if !ok {
			t.Fatalf("expected push %d to succeed", i)
		}
	}

	overflow := q.TryPush(SREAuditRecord{
		Timestamp: 2000,
		Action:    "drop",
		Target:    "group-overflow",
	})
	if overflow {
		t.Fatalf("expected queue full backpressure rejection, got success")
	}

	for i := 0; i < 8; i++ {
		rec, ok := q.TryPop()
		if !ok {
			t.Fatalf("expected pop %d to succeed", i)
		}
		if rec.Timestamp != 1000+int64(i) {
			t.Errorf("expected timestamp %d, got %d", 1000+int64(i), rec.Timestamp)
		}
	}

	_, ok := q.TryPop()
	if ok {
		t.Fatalf("expected empty queue pop to return false")
	}
}
