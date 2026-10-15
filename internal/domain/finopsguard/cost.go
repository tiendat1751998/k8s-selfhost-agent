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
type FinOpsTelemetryTracker struct {
	crossAZEgressBytesSaved atomic.Uint64
	unhintedServicesScanned atomic.Uint64
	topologyHintsInjected   atomic.Uint64
	skewFallbacksTriggered  atomic.Uint64
	orphansDetected         atomic.Uint64
	orphansReaped           atomic.Uint64
	snapshotsGenerated      atomic.Uint64
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
