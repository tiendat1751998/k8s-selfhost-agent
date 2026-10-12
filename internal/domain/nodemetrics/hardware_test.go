package nodemetrics_test

import (
	"testing"
	"time"

	"github.com/datdt/k8sselfhost/internal/domain/nodemetrics"
	"github.com/stretchr/testify/assert"
)

func TestHardwareDomain_ConstantsAndTypes(t *testing.T) {
	assert.Equal(t, "HEALTHY", nodemetrics.TimeDriftStatusHealthy)
	assert.Equal(t, "WARNING", nodemetrics.TimeDriftStatusWarning)
	assert.Equal(t, "CRITICAL", nodemetrics.TimeDriftStatusCritical)

	assert.Equal(t, "PASSED", nodemetrics.DiskHealthPassed)
	assert.Equal(t, "WARNING", nodemetrics.DiskHealthWarning)
	assert.Equal(t, "DEGRADED", nodemetrics.DiskHealthDegraded)
	assert.Equal(t, "FAILING", nodemetrics.DiskHealthFailing)

	now := time.Now()
	td := nodemetrics.TimeDriftStatus{
		NodeName:       "node-1",
		OffsetDuration: 120 * time.Millisecond,
		IsSynchronized: true,
		NTPServer:      "192.168.1.100",
		Status:         nodemetrics.TimeDriftStatusWarning,
		UpdatedAt:      now,
	}
	assert.Equal(t, "node-1", td.NodeName)
	assert.Equal(t, 120*time.Millisecond, td.OffsetDuration)
	assert.True(t, td.IsSynchronized)

	disk := nodemetrics.DiskHealthReport{
		DevicePath:      "/dev/nvme0n1",
		Model:           "Samsung SSD 980",
		Serial:          "S123456",
		TemperatureC:    42,
		BadSectorsCount: 0,
		MediaErrors:     2,
		PowerOnHours:    1200,
		HealthStatus:    nodemetrics.DiskHealthDegraded,
	}
	assert.Equal(t, "/dev/nvme0n1", disk.DevicePath)
	assert.Equal(t, int64(2), disk.MediaErrors)
	assert.Equal(t, nodemetrics.DiskHealthDegraded, disk.HealthStatus)
}
