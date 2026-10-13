// Package messaging defines domain models, telemetry metrics, and SRE contracts
// for enterprise messaging systems (Kafka, RabbitMQ, NATS).
package messaging

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
)

/*
HARDWARE BOTTLENECK & FAILURE MODE SRE ENGINEERING NOTE:
1. NIC Interrupt Moderation & PCIe DMA:
   At 25GbE/100GbE line rates, messaging ingestion saturates CPU cores if per-packet
   IRQs are handled synchronously. Adaptive interrupt moderation (ethtool -C adaptive-rx on)
   and PCIe ring buffer coalescing prevent kernel softirq storms. Socket buffer misconfigurations
   (SO_SNDBUF, SO_RCVBUF) starve the NIC DMA engine, causing packet drops in kernel ring buffers.
2. CPU Cache-Line Bouncing & MESI/MOESI Invalidation:
   When multiple CPU cores concurrently update shared atomic metrics, placing adjacent
   atomic variables on the same 64-byte cache line triggers false sharing. Cache lines bounce
   across L1/L2 caches via interconnects (UPI/Infinity Fabric), costing 40-100 cycles per atomic write.
   Structs here enforce 64-byte cache-line padding (_ [56]byte) between independent atomic counters.
3. Go GC P99 Latency Tails:
   High-frequency lag polling and consumer metadata allocations trigger Go concurrent mark-and-sweep
   GC pauses and write barrier overhead, elevating P99 latency past 50ms. Zero-allocation ring
   buffers and bounded queues prevent GC heap churn.
4. Kafka Consumer Group Rebalance Storms:
   Consumers using Eager Assignors (RangeAssignor, RoundRobinAssignor) trigger 'stop-the-world'
   partition revocations across ALL consumers upon any single heartbeat timeout or pod restart.
   This initiates cascading rebalance loops (CompletingRebalance <-> PreparingRebalance). Enforcing
   CooperativeStickyAssignor and static membership (group.instance.id) limits rebalancing strictly
   to migrating partitions.
5. RabbitMQ High-Watermark Socket Deadlocks:
   When memory usage exceeds the alarm threshold (>85%), RabbitMQ disables TCP socket reads on
   publishers via Erlang flow control. If publishers do not honor backpressure or share connections
   with consumers, socket receive buffers fill up, triggering deadlocks.
6. Linux Dirty-Page Flush Stalls:
   Default Linux dirty_ratio (20%) on a 256GB RAM Kafka broker allows 51GB of dirty pages.
   When kswapd flushes 51GB to NVMe/EBS, it monopolizes disk I/O, stalling synchronous fsync and
   replication RPCs. Tuning vm.dirty_background_ratio=3, vm.dirty_ratio=10, and
   vm.dirty_expire_centisecs=1000 enforces continuous micro-flushing and avoids I/O stalls.
*/

// Domain sentinel errors.
var (
	ErrInvalidClusterID             = errors.New("invalid cluster id: must not be empty or exceed 128 characters")
	ErrInvalidGroupID               = errors.New("invalid consumer group id: must not be empty")
	ErrInvalidNodeName              = errors.New("invalid node name: must not be empty")
	ErrRebalanceStormActive         = errors.New("kafka rebalance storm detected: frequent rebalancing in progress")
	ErrRabbitMQMemoryCritical       = errors.New("rabbitmq memory usage exceeds critical threshold (>85%)")
	ErrRabbitMQFlowBlocked          = errors.New("rabbitmq connections are flow-blocked due to memory alarms")
	ErrKernelTuningExecutionFailed  = errors.New("failed to apply kernel tuning profile")
	ErrConsumerGroupNotFound        = errors.New("kafka consumer group not found")
	ErrClusterUnavailable           = errors.New("messaging cluster unavailable")
)

// Standard assignor strategies for Apache Kafka.
const (
	CooperativeStickyAssignor = "org.apache.kafka.clients.consumer.CooperativeStickyAssignor"
	RangeAssignor             = "org.apache.kafka.clients.consumer.RangeAssignor"
	RoundRobinAssignor        = "org.apache.kafka.clients.consumer.RoundRobinAssignor"
	StickyAssignor            = "org.apache.kafka.clients.consumer.StickyAssignor"
)

// KafkaGroupStatus represents the health classification of a consumer group.
type KafkaGroupStatus string

const (
	KafkaGroupStatusHealthy        KafkaGroupStatus = "HEALTHY"
	KafkaGroupStatusWarning        KafkaGroupStatus = "WARNING"
	KafkaGroupStatusCritical       KafkaGroupStatus = "CRITICAL"
	KafkaGroupStatusRebalanceStorm KafkaGroupStatus = "REBALANCE_STORM"
)

// RabbitMQStatus represents the health classification of a RabbitMQ broker/cluster.
type RabbitMQStatus string

const (
	RabbitMQStatusHealthy       RabbitMQStatus = "HEALTHY"
	RabbitMQStatusWarning       RabbitMQStatus = "WARNING"
	RabbitMQStatusCritical      RabbitMQStatus = "CRITICAL"
	RabbitMQStatusHighWatermark RabbitMQStatus = "HIGH_WATERMARK"
)

// KafkaGroupHealth contains consumer group topology, lag, and assignment telemetry.
type KafkaGroupHealth struct {
	GroupID            string           `json:"group_id"`
	Topic              string           `json:"topic"`
	PartitionCount     int              `json:"partition_count"`
	ConsumerCount      int              `json:"consumer_count"`
	TotalLag           int64            `json:"total_lag"`
	AssignmentStrategy string           `json:"assignment_strategy"`
	IsCooperative      bool             `json:"is_cooperative"`
	RebalanceCount24h  int              `json:"rebalance_count_24h"`
	Status             KafkaGroupStatus `json:"status"`
}

// NewKafkaGroupHealth validates invariants and constructs a KafkaGroupHealth record.
func NewKafkaGroupHealth(
	groupID string,
	topic string,
	partitionCount int,
	consumerCount int,
	totalLag int64,
	assignmentStrategy string,
	isCooperative bool,
	rebalanceCount24h int,
) (*KafkaGroupHealth, error) {
	if groupID == "" {
		return nil, fmt.Errorf("%w: groupID is required", ErrInvalidGroupID)
	}
	if topic == "" {
		return nil, errors.New("topic is required")
	}
	if partitionCount < 0 {
		return nil, errors.New("partitionCount cannot be negative")
	}
	if consumerCount < 0 {
		return nil, errors.New("consumerCount cannot be negative")
	}
	if totalLag < 0 {
		return nil, errors.New("totalLag cannot be negative")
	}
	if rebalanceCount24h < 0 {
		return nil, errors.New("rebalanceCount24h cannot be negative")
	}

	status := KafkaGroupStatusHealthy
	if rebalanceCount24h > 20 {
		status = KafkaGroupStatusRebalanceStorm
	} else if !isCooperative && (totalLag > 10000 || rebalanceCount24h > 5) {
		status = KafkaGroupStatusWarning
	} else if totalLag > 50000 {
		status = KafkaGroupStatusCritical
	}

	return &KafkaGroupHealth{
		GroupID:            groupID,
		Topic:              topic,
		PartitionCount:     partitionCount,
		ConsumerCount:      consumerCount,
		TotalLag:           totalLag,
		AssignmentStrategy: assignmentStrategy,
		IsCooperative:      isCooperative,
		RebalanceCount24h:  rebalanceCount24h,
		Status:             status,
	}, nil
}

// RabbitMQHealthReport contains memory, queue architecture, and flow control telemetry.
type RabbitMQHealthReport struct {
	ClusterName       string         `json:"cluster_name"`
	MemoryUsedBytes   uint64         `json:"memory_used_bytes"`
	MemoryLimitBytes  uint64         `json:"memory_limit_bytes"`
	MemoryPercentage  float64        `json:"memory_percentage"`
	FlowBlockedConns  int            `json:"flow_blocked_conns"`
	QuorumQueueCount  int            `json:"quorum_queue_count"`
	ClassicQueueCount int            `json:"classic_queue_count"`
	Status            RabbitMQStatus `json:"status"`
}

// NewRabbitMQHealthReport constructs and validates a RabbitMQ health report.
func NewRabbitMQHealthReport(
	clusterName string,
	memoryUsedBytes uint64,
	memoryLimitBytes uint64,
	flowBlockedConns int,
	quorumQueueCount int,
	classicQueueCount int,
) (*RabbitMQHealthReport, error) {
	if clusterName == "" {
		return nil, fmt.Errorf("%w: clusterName is required", ErrInvalidClusterID)
	}
	if memoryLimitBytes == 0 {
		return nil, errors.New("memoryLimitBytes must be greater than zero")
	}
	if flowBlockedConns < 0 {
		return nil, errors.New("flowBlockedConns cannot be negative")
	}
	if quorumQueueCount < 0 || classicQueueCount < 0 {
		return nil, errors.New("queue counts cannot be negative")
	}

	memPct := (float64(memoryUsedBytes) / float64(memoryLimitBytes)) * 100.0

	status := RabbitMQStatusHealthy
	if memPct >= 85.0 {
		status = RabbitMQStatusHighWatermark
	} else if flowBlockedConns > 0 {
		status = RabbitMQStatusCritical
	} else if memPct >= 70.0 || classicQueueCount > quorumQueueCount {
		status = RabbitMQStatusWarning
	}

	return &RabbitMQHealthReport{
		ClusterName:       clusterName,
		MemoryUsedBytes:   memoryUsedBytes,
		MemoryLimitBytes:  memoryLimitBytes,
		MemoryPercentage:  memPct,
		FlowBlockedConns:  flowBlockedConns,
		QuorumQueueCount:  quorumQueueCount,
		ClassicQueueCount: classicQueueCount,
		Status:            status,
	}, nil
}

// KernelSysctlProfile holds OS dirty page tuning parameters to prevent disk I/O stalls.
type KernelSysctlProfile struct {
	DirtyBackgroundRatio int `json:"vm.dirty_background_ratio"`
	DirtyRatio           int `json:"vm.dirty_ratio"`
	DirtyExpireCentisecs int `json:"vm.dirty_expire_centisecs"`
}

// DefaultKafkaKernelProfile returns the production sysctl profile for Kafka brokers.
func DefaultKafkaKernelProfile() KernelSysctlProfile {
	return KernelSysctlProfile{
		DirtyBackgroundRatio: 3,
		DirtyRatio:           10,
		DirtyExpireCentisecs: 1000,
	}
}

// PaddedThroughputMetric provides lock-free, cache-line padded operational metrics.
// 64-byte padding prevents MESI/MOESI cache line bouncing across concurrent CPU cores.
type PaddedThroughputMetric struct {
	activeOperations atomic.Int64
	_                [56]byte

	totalInspections atomic.Uint64
	_                [56]byte

	rebalanceStormsMitigated atomic.Uint64
	_                        [56]byte

	watermarkRescuesTriggered atomic.Uint64
	_                         [56]byte
}

// NewPaddedThroughputMetric allocates a new cache-line padded metric structure.
func NewPaddedThroughputMetric() *PaddedThroughputMetric {
	return &PaddedThroughputMetric{}
}

func (m *PaddedThroughputMetric) IncActiveOperations()          { m.activeOperations.Add(1) }
func (m *PaddedThroughputMetric) DecActiveOperations()          { m.activeOperations.Add(-1) }
func (m *PaddedThroughputMetric) ActiveOperations() int64       { return m.activeOperations.Load() }
func (m *PaddedThroughputMetric) IncTotalInspections()          { m.totalInspections.Add(1) }
func (m *PaddedThroughputMetric) TotalInspections() uint64      { return m.totalInspections.Load() }
func (m *PaddedThroughputMetric) IncRebalanceStormsMitigated()  { m.rebalanceStormsMitigated.Add(1) }
func (m *PaddedThroughputMetric) RebalanceStormsMitigated() uint64 {
	return m.rebalanceStormsMitigated.Load()
}
func (m *PaddedThroughputMetric) IncWatermarkRescuesTriggered() {
	m.watermarkRescuesTriggered.Add(1)
}
func (m *PaddedThroughputMetric) WatermarkRescuesTriggered() uint64 {
	return m.watermarkRescuesTriggered.Load()
}

// SREAuditRecord captures an SRE action for high-throughput bounded logging.
type SREAuditRecord struct {
	Timestamp int64  `json:"timestamp"`
	Action    string `json:"action"`
	Target    string `json:"target"`
}

// PaddedSREQueue is a bounded lock-free/low-contention ring buffer for SRE telemetry.
type PaddedSREQueue struct {
	head     atomic.Uint64
	_        [56]byte
	tail     atomic.Uint64
	_        [56]byte
	capacity uint64
	buffer   []SREAuditRecord
}

// NewPaddedSREQueue initializes a bounded ring buffer of given capacity.
func NewPaddedSREQueue(capacity uint64) *PaddedSREQueue {
	if capacity == 0 {
		capacity = 1024
	}
	return &PaddedSREQueue{
		capacity: capacity,
		buffer:   make([]SREAuditRecord, capacity),
	}
}

func (q *PaddedSREQueue) Capacity() uint64 { return q.capacity }

// TryPush enqueues a record or returns false if capacity would be exceeded (backpressure).
func (q *PaddedSREQueue) TryPush(rec SREAuditRecord) bool {
	tail := q.tail.Load()
	head := q.head.Load()
	if tail-head >= q.capacity {
		return false
	}
	q.buffer[tail%q.capacity] = rec
	q.tail.Add(1)
	return true
}

// TryPop retrieves a record or returns false if empty.
func (q *PaddedSREQueue) TryPop() (SREAuditRecord, bool) {
	head := q.head.Load()
	tail := q.tail.Load()
	if head >= tail {
		return SREAuditRecord{}, false
	}
	rec := q.buffer[head%q.capacity]
	q.head.Add(1)
	return rec, true
}

// MessageQueueSREManager defines the domain port for message queue SRE auto-tuning.
type MessageQueueSREManager interface {
	InspectKafkaConsumerGroups(ctx context.Context, clusterID string) ([]KafkaGroupHealth, error)
	EnforceKafkaCooperativePolicy(ctx context.Context, clusterID, groupID string) error
	AuditRabbitMQMemoryAndTopology(ctx context.Context, clusterID string) (*RabbitMQHealthReport, error)
	ApplyKafkaKernelTuning(ctx context.Context, nodeName string) error
}
