package finopsguard

import (
	"context"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"
)

// CloudResourceType defines the categorization of tracked infrastructure resources.
type CloudResourceType string

const (
	ResourceEBSVolume    CloudResourceType = "EBSVolume"
	ResourceLoadBalancer CloudResourceType = "LoadBalancer"
	ResourceElasticIP    CloudResourceType = "ElasticIP"
	ResourceNATGateway   CloudResourceType = "NATGateway"
)

// ApprovalStatus represents the operational approval lifecycle for resource remediation.
type ApprovalStatus string

const (
	ApprovalStatusPending  ApprovalStatus = "pending"
	ApprovalStatusApproved ApprovalStatus = "approved"
	ApprovalStatusRejected ApprovalStatus = "rejected"
	ApprovalStatusReaped   ApprovalStatus = "reaped"
)

// OrphanCloudResource models an unattached, idle, or abandoned cloud infrastructure asset.
type OrphanCloudResource struct {
	ResourceID              string            `json:"resource_id"`
	Type                    CloudResourceType `json:"type"`
	CloudProvider           string            `json:"cloud_provider"`
	ClusterTag              string            `json:"cluster_tag"`
	OriginalNamespace       string            `json:"original_namespace"`
	IdleSince               time.Time         `json:"idle_since"`
	EstimatedMonthlyCostUSD float64           `json:"estimated_monthly_cost_usd"`
	ApprovalStatus          ApprovalStatus    `json:"approval_status"`
	HasSnapshot             bool              `json:"has_snapshot"`
	SnapshotID              string            `json:"snapshot_id,omitempty"`
}

// IsIdleBeyond returns true if the resource has been idle longer than the specified threshold.
func (r OrphanCloudResource) IsIdleBeyond(threshold time.Duration) bool {
	if r.IdleSince.IsZero() {
		return false
	}
	return time.Since(r.IdleSince) >= threshold
}

// IsApproved returns true if the resource has been vetted and approved for safe reaping.
func (r OrphanCloudResource) IsApproved() bool {
	return r.ApprovalStatus == ApprovalStatusApproved
}

// RFC7807ProblemDetail defines the RFC 7807 problem details specification for domain errors.
type RFC7807ProblemDetail struct {
	Type     string `json:"type,omitempty"`
	Title    string `json:"title"`
	Status   int    `json:"status"`
	Detail   string `json:"detail"`
	Instance string `json:"instance,omitempty"`
	Code     string `json:"code"`
}

// Error implements the standard Go error interface.
func (p RFC7807ProblemDetail) Error() string {
	return fmt.Sprintf("[%s] %s: %s (status %d)", p.Code, p.Title, p.Detail, p.Status)
}

// Standard domain error instances adhering to RFC 7807 specifications.
var (
	ErrFinOpsCrossAZWasteDetected = &RFC7807ProblemDetail{
		Type:   "https://k8sselfhost.io/finops/errors/ERR_FINOPS_CROSS_AZ_WASTE_DETECTED",
		Title:  "Cross-AZ Network Egress Waste Detected",
		Status: http.StatusConflict, // 409
		Detail: "service spans multiple availability zones without topology-aware hints, generating cross-AZ egress charges",
		Code:   "ERR_FINOPS_CROSS_AZ_WASTE_DETECTED",
	}

	ErrResourceNotApproved = &RFC7807ProblemDetail{
		Type:   "https://k8sselfhost.io/finops/errors/ERR_RESOURCE_NOT_APPROVED",
		Title:  "Resource Not Approved For Reaping",
		Status: http.StatusBadRequest, // 400
		Detail: "cloud orphan resource must be approved by SRE before reaping",
		Code:   "ERR_RESOURCE_NOT_APPROVED",
	}

	ErrSnapshotGenerationFailed = &RFC7807ProblemDetail{
		Type:   "https://k8sselfhost.io/finops/errors/ERR_SNAPSHOT_GENERATION_FAILED",
		Title:  "Snapshot Generation Failed",
		Status: http.StatusInternalServerError, // 500
		Detail: "failed to generate ephemeral backup snapshot before volume reaping; deletion aborted for safety",
		Code:   "ERR_SNAPSHOT_GENERATION_FAILED",
	}

	ErrResourceNotFound = &RFC7807ProblemDetail{
		Type:   "https://k8sselfhost.io/finops/errors/ERR_RESOURCE_NOT_FOUND",
		Title:  "Resource Not Found",
		Status: http.StatusNotFound, // 404
		Detail: "target resource ID was not found in scanned inventory",
		Code:   "ERR_RESOURCE_NOT_FOUND",
	}
)

// FinOpsTelemetryTracker provides high-throughput, atomic telemetry counters for cluster-wide
// FinOps scanning and remediation workloads.
//
// ARCHITECTURAL HARDWARE NOTE & CACHE LINE PADDING:
// To eliminate false sharing across CPU core cache lines under high-concurrency multi-socket NUMA
// scanning (MESI/MOESI cache coherency protocols), each atomic counter is isolated with a 64-byte
// cache-line padding boundary (_ [8]uint64, 8 * 8 = 64 bytes).
// Without padding, concurrent writes to adjacent atomic variables force continuous cache line
// invalidation cycles (bus snooping / invalidation storms), severely degrading throughput.
//
// PHYSICAL HARDWARE BOTTLENECK & REAL-WORLD FAILURE ANALYSIS:
// 1. Cross-AZ Fiber Propagation Latency (~1-2ms RTT):
//    Inter-AZ communication crosses physical metropolitan dark fiber links adding 1-2ms round-trip
//    latency compared to <0.1ms intra-AZ communication. High-frequency microservices performing
//    hundreds of RPC hops compound this latency exponentially if traffic leaves the local AZ.
// 2. Cloud Provider Cross-AZ Data Transfer Fees ($0.01/GB each direction):
//    AWS/GCP/Azure bill $0.01/GB for cross-AZ ingress and egress ($0.02/GB effective roundtrip).
//    In high-throughput distributed systems (e.g., Kafka, Elasticsearch, database replication),
//    unhinted routing causes a cross-AZ traffic explosion, generating tens of thousands in unexpected egress bills.
// 3. EBS/Disk Snapshot Creation I/O Performance Impact:
//    Initiating an EBS volume snapshot triggers copy-on-write (COW) tracking. In busy I/O systems,
//    initial snapshot block freezes cause block queue depth spikes and volume latency degradation.
//    Reaping must ensure snapshot completion without saturating underlying disk channels.
// 4. Cloud API Rate Limit Throttling:
//    Cloud provider control planes enforce aggressive token bucket rate limits (e.g., AWS EC2 DescribeVolumes
//    capped at ~20 TPS). Uncontrolled batch scanning leads to API throttling (RequestLimitExceeded),
//    starving deployment controllers and critical health checks.
// 5. Traffic Skew Failure Mode with Topology Hints:
//    If topology hints ('service.kubernetes.io/topology-mode: Auto') are applied naively when endpoint
//    distribution is asymmetrical (e.g. 1 pod in AZ-a, 9 pods in AZ-b), intra-AZ routing forces 100% of
//    AZ-a traffic to that single pod, causing CPU saturation, memory exhaustion (OOMKill), and cascading
//    service degradation. A proactive traffic skew detector must trigger fallback to global round-robin
//    whenever a single pod handles >80% of local zone demand.
// 6. Zombie Orphan Resource Accumulation:
//    When K8s namespaces or Helm releases are destroyed without finalizer cleanup, external cloud
//    resources (unattached gp3/io2 EBS volumes, unassociated Elastic IPs, orphaned NLB/ALB target groups)
//    remain allocated, running up idle hourly charges ($0.08/GB-month for gp3, $0.005/hr for idle EIP,
//    $0.0225/hr for ALB base). The automated reaper identifies resources idle >= 48h and prompts for
//    safe backup-snapshot-verified deletion.
type FinOpsTelemetryTracker struct {
	_                       [8]uint64 // 64-byte cache-line padding boundary
	crossAZEgressBytesSaved atomic.Uint64
	_                       [8]uint64 // 64-byte cache-line padding boundary
	unhintedServicesScanned atomic.Uint64
	_                       [8]uint64 // 64-byte cache-line padding boundary
	topologyHintsInjected   atomic.Uint64
	_                       [8]uint64 // 64-byte cache-line padding boundary
	skewFallbacksTriggered  atomic.Uint64
	_                       [8]uint64 // 64-byte cache-line padding boundary
	orphansDetected         atomic.Uint64
	_                       [8]uint64 // 64-byte cache-line padding boundary
	orphansReaped           atomic.Uint64
	_                       [8]uint64 // 64-byte cache-line padding boundary
	snapshotsGenerated      atomic.Uint64
	_                       [8]uint64 // 64-byte cache-line padding boundary
}

// NewFinOpsTelemetryTracker constructs an isolated cache-padded telemetry tracker.
func NewFinOpsTelemetryTracker() *FinOpsTelemetryTracker {
	return &FinOpsTelemetryTracker{}
}

// AddCrossAZEgressBytesSaved increments the cumulative egress byte volume saved.
func (t *FinOpsTelemetryTracker) AddCrossAZEgressBytesSaved(bytes uint64) {
	t.crossAZEgressBytesSaved.Add(bytes)
}

// GetCrossAZEgressBytesSaved atomically reads saved egress bytes.
func (t *FinOpsTelemetryTracker) GetCrossAZEgressBytesSaved() uint64 {
	return t.crossAZEgressBytesSaved.Load()
}

// IncUnhintedServicesScanned increments detected unhinted multi-AZ services count.
func (t *FinOpsTelemetryTracker) IncUnhintedServicesScanned() {
	t.unhintedServicesScanned.Add(1)
}

// GetUnhintedServicesScanned atomically reads unhinted services count.
func (t *FinOpsTelemetryTracker) GetUnhintedServicesScanned() uint64 {
	return t.unhintedServicesScanned.Load()
}

// IncTopologyHintsInjected increments successfully injected topology hints count.
func (t *FinOpsTelemetryTracker) IncTopologyHintsInjected() {
	t.topologyHintsInjected.Add(1)
}

// GetTopologyHintsInjected atomically reads injected topology hints count.
func (t *FinOpsTelemetryTracker) GetTopologyHintsInjected() uint64 {
	return t.topologyHintsInjected.Load()
}

// IncSkewFallbacksTriggered increments count of topology hints reverted due to traffic skew.
func (t *FinOpsTelemetryTracker) IncSkewFallbacksTriggered() {
	t.skewFallbacksTriggered.Add(1)
}

// GetSkewFallbacksTriggered atomically reads skew fallback events count.
func (t *FinOpsTelemetryTracker) GetSkewFallbacksTriggered() uint64 {
	return t.skewFallbacksTriggered.Load()
}

// IncOrphansDetected increments detected orphan resources count.
func (t *FinOpsTelemetryTracker) IncOrphansDetected() {
	t.orphansDetected.Add(1)
}

// GetOrphansDetected atomically reads detected orphan resources count.
func (t *FinOpsTelemetryTracker) GetOrphansDetected() uint64 {
	return t.orphansDetected.Load()
}

// IncOrphansReaped increments successfully deleted orphan cloud assets count.
func (t *FinOpsTelemetryTracker) IncOrphansReaped() {
	t.orphansReaped.Add(1)
}

// GetOrphansReaped atomically reads reaped orphan resources count.
func (t *FinOpsTelemetryTracker) GetOrphansReaped() uint64 {
	return t.orphansReaped.Load()
}

// IncSnapshotsGenerated increments ephemeral safety snapshots created before reaping.
func (t *FinOpsTelemetryTracker) IncSnapshotsGenerated() {
	t.snapshotsGenerated.Add(1)
}

// GetSnapshotsGenerated atomically reads generated safety snapshots count.
func (t *FinOpsTelemetryTracker) GetSnapshotsGenerated() uint64 {
	return t.snapshotsGenerated.Load()
}

// ServiceTopologyInfo encapsulates availability zone distribution for a K8s service.
type ServiceTopologyInfo struct {
	Namespace          string            `json:"namespace"`
	Name               string            `json:"name"`
	Annotations        map[string]string `json:"annotations"`
	Zones              []string          `json:"zones"`
	EndpointsCount     int               `json:"endpoints_count"`
	HasTopologyHint    bool              `json:"has_topology_hint"`
	TrafficSkewPercent float64           `json:"traffic_skew_percent"`
}

// SpansMultipleAZs returns true if endpoints exist across at least two distinct AZs.
func (s ServiceTopologyInfo) SpansMultipleAZs() bool {
	return len(s.Zones) > 1
}

// QualifiesForTopologyHints returns true if service spans multiple AZs, has > 3 endpoints,
// and does not already possess topology hints.
func (s ServiceTopologyInfo) QualifiesForTopologyHints() bool {
	return s.SpansMultipleAZs() && s.EndpointsCount > 3 && !s.HasTopologyHint
}

// FinOpsAutomationEngine governs automated cost optimization and orphan cloud resource reaping.
type FinOpsAutomationEngine interface {
	AuditTopologyHints(ctx context.Context) (unhintedServicesCount int, err error)
	InjectTopologyAwareHints(ctx context.Context, namespace, serviceName string) error
	ScanOrphanCloudResources(ctx context.Context) ([]OrphanCloudResource, error)
	ReapApprovedResource(ctx context.Context, resourceID string) error
	DispatchTelegramCostReport(ctx context.Context, orphans []OrphanCloudResource) error
}

// K8sServiceTopologyPort defines cluster interactions for discovering and tuning service topology.
type K8sServiceTopologyPort interface {
	ListServices(ctx context.Context) ([]ServiceTopologyInfo, error)
	GetService(ctx context.Context, namespace, name string) (*ServiceTopologyInfo, error)
	AnnotateService(ctx context.Context, namespace, name string, annotations map[string]string) error
}

// CloudResourceScannerPort abstracts cloud provider IaaS APIs (AWS/GCP/Azure) for discovery and lifecycle.
type CloudResourceScannerPort interface {
	ListCloudResources(ctx context.Context) ([]OrphanCloudResource, error)
	CreateVolumeSnapshot(ctx context.Context, resourceID string) (snapshotID string, err error)
	DeleteCloudResource(ctx context.Context, resourceID string, resType CloudResourceType) error
}

// TelegramNotifierPort abstracts interactive communication to SRE operations channels.
type TelegramNotifierPort interface {
	SendMessage(ctx context.Context, message string) error
}
