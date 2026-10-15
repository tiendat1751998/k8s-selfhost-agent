package finopsguard

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/datdt/k8sselfhost/internal/domain/finopsguard"
)

const (
	// TopologyModeAnnotation is the standard Kubernetes annotation for topology-aware routing.
	TopologyModeAnnotation = "service.kubernetes.io/topology-mode"

	// TopologyModeAuto configures topology-aware endpoint routing (local AZ affinity).
	TopologyModeAuto = "Auto"

	// TopologyModeDisabled disables topology-aware routing, falling back to round-robin.
	TopologyModeDisabled = "Disabled"

	// IdleOrphanThresholdDuration defines the idle duration before an unattached resource qualifies as orphan.
	IdleOrphanThresholdDuration = 48 * time.Hour

	// TrafficSkewThresholdPercent defines the max traffic skew on a single pod before fallback triggers.
	TrafficSkewThresholdPercent = 80.0
)

// FinOpsManager implements the domain FinOpsAutomationEngine interface.
type FinOpsManager struct {
	k8sPort      finopsguard.K8sServiceTopologyPort
	scannerPort  finopsguard.CloudResourceScannerPort
	telegramPort finopsguard.TelegramNotifierPort
	tracker      *finopsguard.FinOpsTelemetryTracker

	mu          sync.RWMutex
	orphansByID map[string]*finopsguard.OrphanCloudResource
}

// NewFinOpsManager constructs a production-ready FinOpsManager instance.
func NewFinOpsManager(
	k8sPort finopsguard.K8sServiceTopologyPort,
	scannerPort finopsguard.CloudResourceScannerPort,
	telegramPort finopsguard.TelegramNotifierPort,
	tracker *finopsguard.FinOpsTelemetryTracker,
) *FinOpsManager {
	if tracker == nil {
		tracker = finopsguard.NewFinOpsTelemetryTracker()
	}
	return &FinOpsManager{
		k8sPort:      k8sPort,
		scannerPort:  scannerPort,
		telegramPort: telegramPort,
		tracker:      tracker,
		orphansByID:  make(map[string]*finopsguard.OrphanCloudResource),
	}
}

// GetTracker returns the cache-line padded FinOps telemetry tracker.
func (m *FinOpsManager) GetTracker() *finopsguard.FinOpsTelemetryTracker {
	return m.tracker
}

// AuditTopologyHints audits all cluster services for cross-AZ network egress waste.
// If multi-AZ services with >3 endpoints lack topology hints, it records telemetry
// and returns ErrFinOpsCrossAZWasteDetected.
func (m *FinOpsManager) AuditTopologyHints(ctx context.Context) (int, error) {
	if m.k8sPort == nil {
		return 0, errors.New("finops k8s port is uninitialized")
	}
	services, err := m.k8sPort.ListServices(ctx)
	if err != nil {
		return 0, fmt.Errorf("listing k8s services: %w", err)
	}

	unhintedCount := 0
	for _, svc := range services {
		if svc.QualifiesForTopologyHints() {
			unhintedCount++
			m.tracker.IncUnhintedServicesScanned()
		}
	}

	if unhintedCount > 0 {
		return unhintedCount, finopsguard.ErrFinOpsCrossAZWasteDetected
	}

	return 0, nil
}

// InjectTopologyAwareHints configures topology-aware hints on target service with safety guardrails.
// If traffic skew exceeds 80% on a single pod in an AZ, it falls back to global round-robin.
func (m *FinOpsManager) InjectTopologyAwareHints(ctx context.Context, namespace, serviceName string) error {
	if m.k8sPort == nil {
		return errors.New("finops k8s port is uninitialized")
	}
	svc, err := m.k8sPort.GetService(ctx, namespace, serviceName)
	if err != nil {
		return fmt.Errorf("retrieving service %s/%s: %w", namespace, serviceName, err)
	}

	// Safety Guardrail: Traffic Skew Detector.
	// If traffic skews > 80% on a single pod in an AZ, applying topology hints naively would
	// funnel 100% of zone traffic into that single pod, causing CPU/memory overload and cascading failure.
	if svc.TrafficSkewPercent > TrafficSkewThresholdPercent {
		annotations := map[string]string{
			TopologyModeAnnotation: TopologyModeDisabled,
		}
		if err := m.k8sPort.AnnotateService(ctx, namespace, serviceName, annotations); err != nil {
			return fmt.Errorf("annotating service skew fallback %s/%s: %w", namespace, serviceName, err)
		}
		m.tracker.IncSkewFallbacksTriggered()
		return nil
	}

	// If service spans multiple AZs and has >3 endpoints, inject Auto topology mode.
	if svc.SpansMultipleAZs() && svc.EndpointsCount > 3 {
		annotations := map[string]string{
			TopologyModeAnnotation: TopologyModeAuto,
		}
		if err := m.k8sPort.AnnotateService(ctx, namespace, serviceName, annotations); err != nil {
			return fmt.Errorf("injecting topology annotation %s/%s: %w", namespace, serviceName, err)
		}
		m.tracker.IncTopologyHintsInjected()
		// Confinement of 90%+ traffic to local AZ saves estimated 1GB egress per endpoint daily
		const estimatedEgressBytesPerEndpoint = uint64(1024 * 1024 * 1024)
		m.tracker.AddCrossAZEgressBytesSaved(uint64(svc.EndpointsCount) * estimatedEgressBytesPerEndpoint)
	}

	return nil
}

// ScanOrphanCloudResources scans cloud inventory for idle resources >= 48 hours.
func (m *FinOpsManager) ScanOrphanCloudResources(ctx context.Context) ([]finopsguard.OrphanCloudResource, error) {
	if m.scannerPort == nil {
		return nil, errors.New("finops scanner port is uninitialized")
	}
	rawResources, err := m.scannerPort.ListCloudResources(ctx)
	if err != nil {
		return nil, fmt.Errorf("scanning cloud resources: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	var qualifiedOrphans []finopsguard.OrphanCloudResource
	for _, res := range rawResources {
		if res.IsIdleBeyond(IdleOrphanThresholdDuration) {
			rCopy := res
			m.orphansByID[res.ResourceID] = &rCopy
			qualifiedOrphans = append(qualifiedOrphans, rCopy)
			m.tracker.IncOrphansDetected()
		}
	}

	return qualifiedOrphans, nil
}

// DispatchTelegramCostReport formats and sends an interactive Telegram approval prompt for orphan assets.
func (m *FinOpsManager) DispatchTelegramCostReport(ctx context.Context, orphans []finopsguard.OrphanCloudResource) error {
	if len(orphans) == 0 {
		return nil
	}

	var sb strings.Builder
	var totalMonthlyUSD float64

	sb.WriteString("<b>🚨 FinOps Orphan Cloud Resource Reaper Report</b>\n")
	sb.WriteString("The following cloud resources have been idle >= 48 hours without active PVC or Service bindings:\n\n")

	for _, o := range orphans {
		totalMonthlyUSD += o.EstimatedMonthlyCostUSD
		sb.WriteString(fmt.Sprintf("• <b>%s</b> (%s) in <code>%s</code>\n", o.ResourceID, o.Type, o.OriginalNamespace))
		sb.WriteString(fmt.Sprintf("  Idle Since: %s | Est: $%.2f/mo\n", o.IdleSince.UTC().Format(time.RFC3339), o.EstimatedMonthlyCostUSD))
		sb.WriteString(fmt.Sprintf("  Approval: <code>%s</code>\n\n", o.ApprovalStatus))
	}

	sb.WriteString(fmt.Sprintf("<b>Total Estimated Waste: $%.2f/mo</b>\n", totalMonthlyUSD))
	sb.WriteString("To approve destruction, trigger <code>POST /api/v1/finops/resources/{id}/reap</code>")

	if m.telegramPort != nil {
		if err := m.telegramPort.SendMessage(ctx, sb.String()); err != nil {
			return fmt.Errorf("sending telegram report: %w", err)
		}
	}

	return nil
}

// ApproveResource transitions an orphan resource to Approved status for safe reaping.
func (m *FinOpsManager) ApproveResource(resourceID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	res, ok := m.orphansByID[resourceID]
	if !ok {
		return finopsguard.ErrResourceNotFound
	}
	res.ApprovalStatus = finopsguard.ApprovalStatusApproved
	return nil
}

// ResetResourceSnapshotStatus clears snapshot status for testing error scenarios.
func (m *FinOpsManager) ResetResourceSnapshotStatus(resourceID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if res, ok := m.orphansByID[resourceID]; ok {
		res.HasSnapshot = false
		res.SnapshotID = ""
	}
}

// ReapApprovedResource reaps an approved orphan cloud asset.
// CRITICAL SAFETY GUARDRAIL: Never reap an orphan volume without first generating an ephemeral backup snapshot!
func (m *FinOpsManager) ReapApprovedResource(ctx context.Context, resourceID string) error {
	m.mu.Lock()
	res, ok := m.orphansByID[resourceID]
	if !ok {
		m.mu.Unlock()
		return finopsguard.ErrResourceNotFound
	}

	if !res.IsApproved() {
		m.mu.Unlock()
		return finopsguard.ErrResourceNotApproved
	}

	// Safety Guardrail: EBS Volume Ephemeral Snapshot.
	// If resource is an EBS volume, an ephemeral safety snapshot MUST be verified or created
	// prior to issuing the destructive delete command.
	if res.Type == finopsguard.ResourceEBSVolume && !res.HasSnapshot {
		m.mu.Unlock()
		snapID, err := m.scannerPort.CreateVolumeSnapshot(ctx, resourceID)
		if err != nil {
			// Abort deletion immediately if snapshot creation fails.
			return finopsguard.ErrSnapshotGenerationFailed
		}

		m.mu.Lock()
		res.HasSnapshot = true
		res.SnapshotID = snapID
		m.tracker.IncSnapshotsGenerated()
	}

	// Issue destructive cloud provider API deletion call.
	if err := m.scannerPort.DeleteCloudResource(ctx, resourceID, res.Type); err != nil {
		m.mu.Unlock()
		return fmt.Errorf("deleting cloud resource %s: %w", resourceID, err)
	}

	res.ApprovalStatus = finopsguard.ApprovalStatusReaped
	m.tracker.IncOrphansReaped()
	m.mu.Unlock()

	return nil
}
