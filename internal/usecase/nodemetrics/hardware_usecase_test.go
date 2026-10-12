package nodemetrics_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/datdt/k8sselfhost/internal/domain/nodemetrics"
	usecase "github.com/datdt/k8sselfhost/internal/usecase/nodemetrics"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockWatchdog struct {
	timeDrifts []nodemetrics.TimeDriftStatus
	driftErr   error
	disks      []nodemetrics.DiskHealthReport
	diskErr    error
	oomShield  bool
	oomErr     error
}

func (m *mockWatchdog) GetClusterTimeDrift(ctx context.Context, clusterID string) ([]nodemetrics.TimeDriftStatus, error) {
	return m.timeDrifts, m.driftErr
}

func (m *mockWatchdog) InspectNodeDisks(ctx context.Context, nodeName string) ([]nodemetrics.DiskHealthReport, error) {
	return m.disks, m.diskErr
}

func (m *mockWatchdog) VerifyOOMProtection(ctx context.Context, nodeName string) (bool, error) {
	return m.oomShield, m.oomErr
}

type mockAlertSink struct {
	mu     sync.Mutex
	alerts []nodemetrics.SREAlertEvent
}

func (s *mockAlertSink) EmitAlert(ctx context.Context, event nodemetrics.SREAlertEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.alerts = append(s.alerts, event)
	return nil
}

func (s *mockAlertSink) getAlerts() []nodemetrics.SREAlertEvent {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]nodemetrics.SREAlertEvent, len(s.alerts))
	copy(out, s.alerts)
	return out
}

type mockResyncRunner struct {
	mu          sync.Mutex
	resyncCalls []string
}

func (r *mockResyncRunner) ResyncNodeTime(ctx context.Context, nodeName string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.resyncCalls = append(r.resyncCalls, nodeName)
	return nil
}

// Task Test 1: TestTimeDrift_DetectsHighOffset
// Mock returns offset 650ms. Assert evaluation marks status CRITICAL and triggers alert and resync.
func TestTimeDrift_DetectsHighOffset(t *testing.T) {
	ctx := context.Background()
	alertSink := &mockAlertSink{}
	resyncRunner := &mockResyncRunner{}

	watchdog := &mockWatchdog{
		timeDrifts: []nodemetrics.TimeDriftStatus{
			{
				NodeName:       "worker-prod-1",
				OffsetDuration: 650 * time.Millisecond,
				IsSynchronized: true,
				NTPServer:      "ntp.cluster.local",
			},
		},
	}

	uc := usecase.NewHardwareUsecase(watchdog, alertSink, resyncRunner)

	drifts, err := uc.EvaluateTimeDrift(ctx, "cluster-prod")
	require.Error(t, err)
	assert.ErrorIs(t, err, nodemetrics.ErrTimeDriftExceeded)
	require.Len(t, drifts, 1)
	assert.Equal(t, nodemetrics.TimeDriftStatusCritical, drifts[0].Status)

	// Check alert emission
	alerts := alertSink.getAlerts()
	require.Len(t, alerts, 1)
	assert.Equal(t, "CRITICAL", alerts[0].Severity)
	assert.Equal(t, "ERR_TIME_DRIFT_EXCEEDED", alerts[0].ErrorCode)
	assert.Equal(t, "worker-prod-1", alerts[0].NodeName)

	// Check resync execution
	resyncRunner.mu.Lock()
	assert.Equal(t, []string{"worker-prod-1"}, resyncRunner.resyncCalls)
	resyncRunner.mu.Unlock()
}

func TestTimeDrift_DetectsWarningAndHealthy(t *testing.T) {
	ctx := context.Background()
	alertSink := &mockAlertSink{}
	resyncRunner := &mockResyncRunner{}

	watchdog := &mockWatchdog{
		timeDrifts: []nodemetrics.TimeDriftStatus{
			{
				NodeName:       "node-healthy",
				OffsetDuration: 40 * time.Millisecond,
				IsSynchronized: true,
			},
			{
				NodeName:       "node-warning",
				OffsetDuration: -250 * time.Millisecond,
				IsSynchronized: true,
			},
		},
	}

	uc := usecase.NewHardwareUsecase(watchdog, alertSink, resyncRunner)

	drifts, err := uc.EvaluateTimeDrift(ctx, "cluster-prod")
	require.NoError(t, err)
	require.Len(t, drifts, 2)
	assert.Equal(t, nodemetrics.TimeDriftStatusHealthy, drifts[0].Status)
	assert.Equal(t, nodemetrics.TimeDriftStatusWarning, drifts[1].Status)

	alerts := alertSink.getAlerts()
	require.Len(t, alerts, 1)
	assert.Equal(t, "WARNING", alerts[0].Severity)
	assert.Equal(t, "ERR_TIME_DRIFT_WARNING", alerts[0].ErrorCode)
	assert.Equal(t, "node-warning", alerts[0].NodeName)
}

func TestEvaluateDiskHealth_DetectsDegradedAndAlerts(t *testing.T) {
	ctx := context.Background()
	alertSink := &mockAlertSink{}

	watchdog := &mockWatchdog{
		disks: []nodemetrics.DiskHealthReport{
			{
				DevicePath:      "/dev/sda",
				Model:           "Crucial MX500",
				BadSectorsCount: 45,
				HealthStatus:    nodemetrics.DiskHealthDegraded,
			},
			{
				DevicePath:      "/dev/nvme0n1",
				Model:           "Samsung SSD 980",
				MediaErrors:     0,
				HealthStatus:    nodemetrics.DiskHealthPassed,
			},
		},
	}

	uc := usecase.NewHardwareUsecase(watchdog, alertSink, nil)

	reports, err := uc.EvaluateDiskHealth(ctx, "node-storage-1")
	require.Error(t, err)
	assert.ErrorIs(t, err, nodemetrics.ErrSmartDegradationDetected)
	require.Len(t, reports, 2)

	alerts := alertSink.getAlerts()
	require.Len(t, alerts, 1)
	assert.Equal(t, "CRITICAL", alerts[0].Severity)
	assert.Equal(t, "ERR_SMART_DEGRADATION_DETECTED", alerts[0].ErrorCode)
	assert.Equal(t, "node-storage-1", alerts[0].NodeName)
	assert.Contains(t, alerts[0].Message, "/dev/sda")
}

func TestVerifyOOMProtection_SuccessAndFailure(t *testing.T) {
	ctx := context.Background()
	alertSink := &mockAlertSink{}

	// Case 1: Protected
	watchdogOk := &mockWatchdog{oomShield: true}
	ucOk := usecase.NewHardwareUsecase(watchdogOk, alertSink, nil)
	ok, err := ucOk.VerifyOOMProtection(ctx, "node-1")
	require.NoError(t, err)
	assert.True(t, ok)

	// Case 2: Unprotected
	watchdogFail := &mockWatchdog{oomShield: false}
	ucFail := usecase.NewHardwareUsecase(watchdogFail, alertSink, nil)
	ok, err = ucFail.VerifyOOMProtection(ctx, "node-2")
	require.Error(t, err)
	assert.ErrorIs(t, err, nodemetrics.ErrOOMShieldFailed)
	assert.False(t, ok)

	alerts := alertSink.getAlerts()
	require.Len(t, alerts, 1)
	assert.Equal(t, "WARNING", alerts[0].Severity)
	assert.Equal(t, "ERR_OOM_SHIELD_UNPROTECTED", alerts[0].ErrorCode)
	assert.Equal(t, "node-2", alerts[0].NodeName)
}
