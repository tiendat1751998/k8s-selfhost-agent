package dr_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/datdt/k8sselfhost/internal/domain/dr"
)

func TestDomainDREntities(t *testing.T) {
	t.Run("EtcdSnapshotResult creation", func(t *testing.T) {
		now := time.Now().UTC()
		snap := &dr.EtcdSnapshotResult{
			SnapshotID:  "etcd-snap-01",
			ClusterID:   "c1",
			FilePath:    "/data/snap-01.db",
			SizeBytes:   4096,
			Size:        4096,
			SHA256Hash:  "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
			TotalKeys:   10,
			RaftTerm:    1,
			RaftIndex:   20,
			Status:      "Completed",
			CreatedAt:   now,
		}

		assert.Equal(t, "etcd-snap-01", snap.SnapshotID)
		assert.Equal(t, int64(4096), snap.SizeBytes)
		assert.Equal(t, "Completed", snap.Status)
	})

	t.Run("RestoreDrillReport creation", func(t *testing.T) {
		now := time.Now().UTC()
		report := &dr.RestoreDrillReport{
			DrillID:         "drill-01",
			SnapshotID:      "etcd-snap-01",
			TargetClusterID: "c1",
			Status:          "PASSED",
			RealRTODuration: 250 * time.Millisecond,
			RealRPOLag:      1 * time.Second,
			TablesRestored:  5,
			RowsRestored:    100,
			ExecutedAt:      now,
			LogOutput:       "Restore drill passed successfully",
		}

		assert.Equal(t, "drill-01", report.DrillID)
		assert.Equal(t, "PASSED", report.Status)
		assert.Greater(t, report.RealRTODuration, time.Duration(0))
	})

	t.Run("DR Error constants", func(t *testing.T) {
		assert.ErrorIs(t, dr.ErrEtcdSnapshotFailed, dr.ErrEtcdSnapshotFailed)
		assert.ErrorIs(t, dr.ErrRestoreDrillIntegrityMismatch, dr.ErrRestoreDrillIntegrityMismatch)
	})
}
