package hardware_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/datdt/k8sselfhost/internal/domain/nodemetrics"
	"github.com/datdt/k8sselfhost/internal/infrastructure/hardware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Sample smartctl JSON payloads
const sataHealthyJSON = `{
  "json_format_version": [1, 0],
  "device": { "name": "/dev/sda", "type": "sat", "protocol": "ATA" },
  "model_name": "Samsung SSD 870 EVO 1TB",
  "serial_number": "S6PENF0R100001",
  "smart_status": { "passed": true },
  "temperature": { "current": 35 },
  "power_on_time": { "hours": 4200 },
  "ata_smart_attributes": {
    "table": [
      { "id": 5, "name": "Reallocated_Sector_Ct", "raw": { "value": 0, "string": "0" } },
      { "id": 197, "name": "Current_Pending_Sector", "raw": { "value": 0, "string": "0" } }
    ]
  }
}`

const sataDegradedJSON = `{
  "json_format_version": [1, 0],
  "device": { "name": "/dev/sda", "type": "sat", "protocol": "ATA" },
  "model_name": "WDC WD20EFRX",
  "serial_number": "WD-WCC4M123456",
  "smart_status": { "passed": true },
  "temperature": { "current": 41 },
  "power_on_time": { "hours": 18500 },
  "ata_smart_attributes": {
    "table": [
      { "id": 5, "name": "Reallocated_Sector_Ct", "raw": { "value": 45, "string": "45" } },
      { "id": 197, "name": "Current_Pending_Sector", "raw": { "value": 0, "string": "0" } }
    ]
  }
}`

const sataWarningJSON = `{
  "json_format_version": [1, 0],
  "device": { "name": "/dev/sdb", "type": "sat", "protocol": "ATA" },
  "model_name": "Crucial MX500",
  "serial_number": "CT1000MX500SSD1",
  "smart_status": { "passed": true },
  "temperature": { "current": 36 },
  "power_on_time": { "hours": 5000 },
  "ata_smart_attributes": {
    "table": [
      { "id": 5, "name": "Reallocated_Sector_Ct", "raw": { "value": 4, "string": "4" } },
      { "id": 197, "name": "Current_Pending_Sector", "raw": { "value": 0, "string": "0" } }
    ]
  }
}`

const nvmeHealthyJSON = `{
  "json_format_version": [1, 0],
  "device": { "name": "/dev/nvme0n1", "type": "nvme", "protocol": "NVMe" },
  "model_name": "Samsung SSD 980 PRO 2TB",
  "serial_number": "S5GXNF0R987654",
  "smart_status": { "passed": true },
  "temperature": { "current": 42 },
  "power_on_time": { "hours": 3100 },
  "nvme_smart_health_information_log": {
    "critical_warning": 0,
    "temperature": 42,
    "media_errors": 0,
    "power_on_hours": 3100
  }
}`

const nvmeDegradedJSON = `{
  "json_format_version": [1, 0],
  "device": { "name": "/dev/nvme0n1", "type": "nvme", "protocol": "NVMe" },
  "model_name": "Samsung SSD 980 PRO 2TB",
  "serial_number": "S5GXNF0R987654",
  "smart_status": { "passed": true },
  "temperature": { "current": 54 },
  "power_on_time": { "hours": 8900 },
  "nvme_smart_health_information_log": {
    "critical_warning": 0,
    "temperature": 54,
    "media_errors": 12,
    "power_on_hours": 8900
  }
}`

const failingDiskJSON = `{
  "json_format_version": [1, 0],
  "device": { "name": "/dev/sdc", "type": "sat", "protocol": "ATA" },
  "model_name": "ST3000DM001",
  "serial_number": "W1F12345",
  "smart_status": { "passed": false },
  "temperature": { "current": 48 },
  "power_on_time": { "hours": 32000 }
}`

func TestSMART_DetectsHealthySATA(t *testing.T) {
	report, err := hardware.ParseSmartCTLJSON([]byte(sataHealthyJSON))
	require.NoError(t, err)
	assert.Equal(t, "/dev/sda", report.DevicePath)
	assert.Equal(t, "Samsung SSD 870 EVO 1TB", report.Model)
	assert.Equal(t, "S6PENF0R100001", report.Serial)
	assert.Equal(t, 35, report.TemperatureC)
	assert.Equal(t, int64(0), report.BadSectorsCount)
	assert.Equal(t, int64(0), report.MediaErrors)
	assert.Equal(t, int64(4200), report.PowerOnHours)
	assert.Equal(t, nodemetrics.DiskHealthPassed, report.HealthStatus)
}

func TestSMART_DetectsFailingDisk(t *testing.T) {
	// Task Test 2 requirement: Feed JSON output with 45 reallocated sectors -> DEGRADED
	report, err := hardware.ParseSmartCTLJSON([]byte(sataDegradedJSON))
	require.NoError(t, err)
	assert.Equal(t, "/dev/sda", report.DevicePath)
	assert.Equal(t, int64(45), report.BadSectorsCount)
	assert.Equal(t, nodemetrics.DiskHealthDegraded, report.HealthStatus)
}

func TestSMART_DetectsWarningSATA(t *testing.T) {
	report, err := hardware.ParseSmartCTLJSON([]byte(sataWarningJSON))
	require.NoError(t, err)
	assert.Equal(t, int64(4), report.BadSectorsCount)
	assert.Equal(t, nodemetrics.DiskHealthWarning, report.HealthStatus)
}

func TestSMART_DetectsNVMe(t *testing.T) {
	reportHealthy, err := hardware.ParseSmartCTLJSON([]byte(nvmeHealthyJSON))
	require.NoError(t, err)
	assert.Equal(t, int64(0), reportHealthy.MediaErrors)
	assert.Equal(t, nodemetrics.DiskHealthPassed, reportHealthy.HealthStatus)

	reportDegraded, err := hardware.ParseSmartCTLJSON([]byte(nvmeDegradedJSON))
	require.NoError(t, err)
	assert.Equal(t, int64(12), reportDegraded.MediaErrors)
	assert.Equal(t, nodemetrics.DiskHealthDegraded, reportDegraded.HealthStatus)
}

func TestSMART_DetectsFailedStatus(t *testing.T) {
	report, err := hardware.ParseSmartCTLJSON([]byte(failingDiskJSON))
	require.NoError(t, err)
	assert.Equal(t, nodemetrics.DiskHealthFailing, report.HealthStatus)
}

func TestSMART_ParseInvalidJSON(t *testing.T) {
	_, err := hardware.ParseSmartCTLJSON([]byte(`not json`))
	assert.Error(t, err)
}

// Test OOM Score Shield
func TestOOM_VerifyAndShieldScore(t *testing.T) {
	// Task Test 3 requirement: Test OOM score adjuster
	tempProc := t.TempDir()

	// Simulate kubelet process at PID 1001
	kubeletDir := filepath.Join(tempProc, "1001")
	require.NoError(t, os.MkdirAll(kubeletDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(kubeletDir, "comm"), []byte("kubelet\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(kubeletDir, "oom_score_adj"), []byte("0\n"), 0644))

	// Simulate containerd process at PID 1002
	containerdDir := filepath.Join(tempProc, "1002")
	require.NoError(t, os.MkdirAll(containerdDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(containerdDir, "comm"), []byte("containerd\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(containerdDir, "oom_score_adj"), []byte("-999\n"), 0644))

	shield := hardware.NewOOMShieldWithRoot(tempProc)

	// Check kubelet - currently 0, so not shielded
	score, isShielded, err := shield.VerifyDaemonOOMScore("kubelet")
	require.NoError(t, err)
	assert.Equal(t, 0, score)
	assert.False(t, isShielded)

	// Shield kubelet to -999
	err = shield.ShieldDaemon("kubelet")
	require.NoError(t, err)

	// Verify it now has -999
	score, isShielded, err = shield.VerifyDaemonOOMScore("kubelet")
	require.NoError(t, err)
	assert.Equal(t, -999, score)
	assert.True(t, isShielded)

	// Check containerd - already -999
	score, isShielded, err = shield.VerifyDaemonOOMScore("containerd")
	require.NoError(t, err)
	assert.Equal(t, -999, score)
	assert.True(t, isShielded)

	// Shield all protected daemons
	results, err := shield.ShieldProtectedDaemons()
	require.NoError(t, err)
	assert.True(t, results["kubelet"])
	assert.True(t, results["containerd"])
}

func TestClockReader_Default(t *testing.T) {
	reader := hardware.NewClockReader()
	offset, isSync, err := reader.ReadTimeOffset()
	require.NoError(t, err)
	_ = offset
	_ = isSync
}

type mockClockReader struct {
	offset time.Duration
	sync   bool
	err    error
}

func (m *mockClockReader) ReadTimeOffset() (time.Duration, bool, error) {
	return m.offset, m.sync, m.err
}

type mockSmartRunner struct {
	outputs map[string][]byte
}

func (m *mockSmartRunner) RunSmartCTL(ctx context.Context, devicePath string) ([]byte, error) {
	out, ok := m.outputs[devicePath]
	if !ok {
		return nil, fmt.Errorf("device %s not found", devicePath)
	}
	return out, nil
}

func TestHardwareWatchdogImpl_Operations(t *testing.T) {
	tempProc := t.TempDir()
	kubeletDir := filepath.Join(tempProc, "2001")
	require.NoError(t, os.MkdirAll(kubeletDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(kubeletDir, "comm"), []byte("kubelet\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(kubeletDir, "oom_score_adj"), []byte("-999\n"), 0644))

	clock := &mockClockReader{
		offset: 35 * time.Millisecond,
		sync:   true,
	}
	smartRunner := &mockSmartRunner{
		outputs: map[string][]byte{
			"/dev/sda": []byte(sataHealthyJSON),
		},
	}
	oomShield := hardware.NewOOMShieldWithRoot(tempProc)

	watchdog := hardware.NewHardwareWatchdog(hardware.WatchdogConfig{
		NodeName:    "worker-node-1",
		NTPServer:   "ntp.internal.local",
		DiskDevices: []string{"/dev/sda"},
		Clock:       clock,
		SmartRunner: smartRunner,
		OOMShield:   oomShield,
	})

	ctx := context.Background()

	// 1. Time drift inspection
	drifts, err := watchdog.GetClusterTimeDrift(ctx, "cluster-1")
	require.NoError(t, err)
	require.Len(t, drifts, 1)
	assert.Equal(t, "worker-node-1", drifts[0].NodeName)
	assert.Equal(t, 35*time.Millisecond, drifts[0].OffsetDuration)
	assert.True(t, drifts[0].IsSynchronized)

	// 2. Disk inspection
	disks, err := watchdog.InspectNodeDisks(ctx, "worker-node-1")
	require.NoError(t, err)
	require.Len(t, disks, 1)
	assert.Equal(t, "/dev/sda", disks[0].DevicePath)
	assert.Equal(t, nodemetrics.DiskHealthPassed, disks[0].HealthStatus)

	// 3. OOM protection verification
	allShielded, err := watchdog.VerifyOOMProtection(ctx, "worker-node-1")
	require.NoError(t, err)
	assert.True(t, allShielded)
}
