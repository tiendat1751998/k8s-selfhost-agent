package finopsguard_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datdt/k8sselfhost/internal/domain/finopsguard"
	usecase "github.com/datdt/k8sselfhost/internal/usecase/finopsguard"
)

type mockK8sTopologyPort struct {
	services map[string]finopsguard.ServiceTopologyInfo
	err      error
}

func (m *mockK8sTopologyPort) ListServices(ctx context.Context) ([]finopsguard.ServiceTopologyInfo, error) {
	if m.err != nil {
		return nil, m.err
	}
	res := make([]finopsguard.ServiceTopologyInfo, 0, len(m.services))
	for _, s := range m.services {
		res = append(res, s)
	}
	return res, nil
}

func (m *mockK8sTopologyPort) GetService(ctx context.Context, namespace, name string) (*finopsguard.ServiceTopologyInfo, error) {
	if m.err != nil {
		return nil, m.err
	}
	key := namespace + "/" + name
	s, ok := m.services[key]
	if !ok {
		return nil, finopsguard.ErrResourceNotFound
	}
	return &s, nil
}

func (m *mockK8sTopologyPort) AnnotateService(ctx context.Context, namespace, name string, annotations map[string]string) error {
	if m.err != nil {
		return m.err
	}
	key := namespace + "/" + name
	s, ok := m.services[key]
	if !ok {
		return finopsguard.ErrResourceNotFound
	}
	if s.Annotations == nil {
		s.Annotations = make(map[string]string)
	}
	for k, v := range annotations {
		s.Annotations[k] = v
	}
	m.services[key] = s
	return nil
}

type mockCloudScannerPort struct {
	resources       []finopsguard.OrphanCloudResource
	snapshotCalled  bool
	deleteCalled    bool
	snapshotFail    bool
	deleteFail      bool
	deletedID       string
	createdSnapshot string
}

func (m *mockCloudScannerPort) ListCloudResources(ctx context.Context) ([]finopsguard.OrphanCloudResource, error) {
	return m.resources, nil
}

func (m *mockCloudScannerPort) CreateVolumeSnapshot(ctx context.Context, resourceID string) (string, error) {
	m.snapshotCalled = true
	if m.snapshotFail {
		return "", errors.New("AWS EC2 Snapshot I/O timeout")
	}
	m.createdSnapshot = "snap-safe-ephemeral-" + resourceID
	return m.createdSnapshot, nil
}

func (m *mockCloudScannerPort) DeleteCloudResource(ctx context.Context, resourceID string, resType finopsguard.CloudResourceType) error {
	m.deleteCalled = true
	if m.deleteFail {
		return errors.New("delete API call failed")
	}
	m.deletedID = resourceID
	return nil
}

type mockTelegramPort struct {
	messages []string
	err      error
}

func (m *mockTelegramPort) SendMessage(ctx context.Context, message string) error {
	if m.err != nil {
		return m.err
	}
	m.messages = append(m.messages, message)
	return nil
}

func TestFinOpsManager_AuditTopologyHints(t *testing.T) {
	t.Parallel()

	k8sMock := &mockK8sTopologyPort{
		services: map[string]finopsguard.ServiceTopologyInfo{
			"prod/payment": {
				Namespace:          "prod",
				Name:               "payment",
				Zones:              []string{"us-east-1a", "us-east-1b", "us-east-1c"},
				EndpointsCount:     6,
				HasTopologyHint:    false,
				TrafficSkewPercent: 20.0,
			},
			"prod/auth": {
				Namespace:          "prod",
				Name:               "auth",
				Zones:              []string{"us-east-1a", "us-east-1b"},
				EndpointsCount:     2, // <= 3 endpoints, does not qualify
				HasTopologyHint:    false,
				TrafficSkewPercent: 10.0,
			},
			"prod/catalog": {
				Namespace:          "prod",
				Name:               "catalog",
				Zones:              []string{"us-east-1a"}, // single zone
				EndpointsCount:     8,
				HasTopologyHint:    false,
				TrafficSkewPercent: 15.0,
			},
			"prod/checkout": {
				Namespace:          "prod",
				Name:               "checkout",
				Zones:              []string{"us-east-1a", "us-east-1b"},
				EndpointsCount:     4,
				HasTopologyHint:    true, // already hinted
				TrafficSkewPercent: 25.0,
			},
		},
	}

	tracker := finopsguard.NewFinOpsTelemetryTracker()
	mgr := usecase.NewFinOpsManager(k8sMock, &mockCloudScannerPort{}, &mockTelegramPort{}, tracker)

	unhinted, err := mgr.AuditTopologyHints(context.Background())
	assert.ErrorIs(t, err, finopsguard.ErrFinOpsCrossAZWasteDetected)
	assert.Equal(t, 1, unhinted)
	assert.Equal(t, uint64(1), tracker.GetUnhintedServicesScanned())
}

func TestFinOpsManager_InjectTopologyAwareHints_NormalAndSkewFallback(t *testing.T) {
	t.Parallel()

	t.Run("NormalInjection_Success", func(t *testing.T) {
		t.Parallel()
		k8sMock := &mockK8sTopologyPort{
			services: map[string]finopsguard.ServiceTopologyInfo{
				"prod/payment": {
					Namespace:          "prod",
					Name:               "payment",
					Zones:              []string{"us-east-1a", "us-east-1b"},
					EndpointsCount:     6,
					HasTopologyHint:    false,
					TrafficSkewPercent: 30.0, // safe skew <= 80%
				},
			},
		}
		tracker := finopsguard.NewFinOpsTelemetryTracker()
		mgr := usecase.NewFinOpsManager(k8sMock, &mockCloudScannerPort{}, &mockTelegramPort{}, tracker)

		err := mgr.InjectTopologyAwareHints(context.Background(), "prod", "payment")
		require.NoError(t, err)

		svc := k8sMock.services["prod/payment"]
		assert.Equal(t, "Auto", svc.Annotations[usecase.TopologyModeAnnotation])
		assert.Equal(t, uint64(1), tracker.GetTopologyHintsInjected())
		assert.Greater(t, tracker.GetCrossAZEgressBytesSaved(), uint64(0))
	})

	t.Run("TrafficSkewExceeds80Percent_FallsBackToRoundRobin", func(t *testing.T) {
		t.Parallel()
		k8sMock := &mockK8sTopologyPort{
			services: map[string]finopsguard.ServiceTopologyInfo{
				"prod/order": {
					Namespace:          "prod",
					Name:               "order",
					Zones:              []string{"us-east-1a", "us-east-1b"},
					EndpointsCount:     8,
					HasTopologyHint:    false,
					TrafficSkewPercent: 88.5, // > 80% skew triggers guardrail!
				},
			},
		}
		tracker := finopsguard.NewFinOpsTelemetryTracker()
		mgr := usecase.NewFinOpsManager(k8sMock, &mockCloudScannerPort{}, &mockTelegramPort{}, tracker)

		err := mgr.InjectTopologyAwareHints(context.Background(), "prod", "order")
		require.NoError(t, err)

		svc := k8sMock.services["prod/order"]
		assert.Equal(t, "Disabled", svc.Annotations[usecase.TopologyModeAnnotation])
		assert.Equal(t, uint64(1), tracker.GetSkewFallbacksTriggered())
		assert.Equal(t, uint64(0), tracker.GetTopologyHintsInjected())
	})
}

func TestFinOpsManager_OrphanScanningAndReaping(t *testing.T) {
	t.Parallel()

	now := time.Now()
	orphans := []finopsguard.OrphanCloudResource{
		{
			ResourceID:              "vol-unattached-1",
			Type:                    finopsguard.ResourceEBSVolume,
			CloudProvider:           "aws",
			ClusterTag:              "prod-k8s",
			OriginalNamespace:       "analytics",
			IdleSince:               now.Add(-60 * time.Hour), // > 48h
			EstimatedMonthlyCostUSD: 120.0,
			ApprovalStatus:          finopsguard.ApprovalStatusPending,
		},
		{
			ResourceID:              "vol-active-recent",
			Type:                    finopsguard.ResourceEBSVolume,
			CloudProvider:           "aws",
			ClusterTag:              "prod-k8s",
			OriginalNamespace:       "billing",
			IdleSince:               now.Add(-10 * time.Hour), // < 48h, not orphan yet
			EstimatedMonthlyCostUSD: 80.0,
			ApprovalStatus:          finopsguard.ApprovalStatusPending,
		},
		{
			ResourceID:              "eip-orphan-1",
			Type:                    finopsguard.ResourceElasticIP,
			CloudProvider:           "aws",
			ClusterTag:              "prod-k8s",
			OriginalNamespace:       "ingress",
			IdleSince:               now.Add(-72 * time.Hour), // > 48h
			EstimatedMonthlyCostUSD: 3.65,
			ApprovalStatus:          finopsguard.ApprovalStatusPending,
		},
	}

	scannerMock := &mockCloudScannerPort{resources: orphans}
	telegramMock := &mockTelegramPort{}
	tracker := finopsguard.NewFinOpsTelemetryTracker()
	mgr := usecase.NewFinOpsManager(&mockK8sTopologyPort{}, scannerMock, telegramMock, tracker)

	// 1. Scan orphan cloud resources
	scanned, err := mgr.ScanOrphanCloudResources(context.Background())
	require.NoError(t, err)
	assert.Len(t, scanned, 2) // only the 2 with idle >= 48h
	assert.Equal(t, uint64(2), tracker.GetOrphansDetected())

	// 2. Dispatch Telegram Report
	err = mgr.DispatchTelegramCostReport(context.Background(), scanned)
	require.NoError(t, err)
	assert.Len(t, telegramMock.messages, 1)
	assert.Contains(t, telegramMock.messages[0], "vol-unattached-1")
	assert.Contains(t, telegramMock.messages[0], "eip-orphan-1")

	// 3. Attempt reaping before approval -> MUST fail with ErrResourceNotApproved
	err = mgr.ReapApprovedResource(context.Background(), "vol-unattached-1")
	assert.ErrorIs(t, err, finopsguard.ErrResourceNotApproved)
	assert.False(t, scannerMock.deleteCalled)

	// 4. Approve resource
	err = mgr.ApproveResource("vol-unattached-1")
	require.NoError(t, err)

	// 5. Reap approved EBS volume -> Snapshot generated first, then reaped
	err = mgr.ReapApprovedResource(context.Background(), "vol-unattached-1")
	require.NoError(t, err)
	assert.True(t, scannerMock.snapshotCalled)
	assert.True(t, scannerMock.deleteCalled)
	assert.Equal(t, "vol-unattached-1", scannerMock.deletedID)
	assert.Equal(t, uint64(1), tracker.GetSnapshotsGenerated())
	assert.Equal(t, uint64(1), tracker.GetOrphansReaped())

	// 6. Test snapshot failure guardrail
	scannerMock.snapshotFail = true
	scannerMock.deleteCalled = false
	err = mgr.ApproveResource("vol-unattached-1")
	require.NoError(t, err)
	// Reset snapshot status for test
	mgr.ResetResourceSnapshotStatus("vol-unattached-1")

	err = mgr.ReapApprovedResource(context.Background(), "vol-unattached-1")
	assert.ErrorIs(t, err, finopsguard.ErrSnapshotGenerationFailed)
	assert.False(t, scannerMock.deleteCalled, "Volume must NOT be deleted if snapshot generation fails!")
}
