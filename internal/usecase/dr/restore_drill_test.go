package dr_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	domain "github.com/datdt/k8sselfhost/internal/domain/dr"
	"github.com/datdt/k8sselfhost/internal/usecase/dr"
)

func TestDRUsecase_ExecuteRestoreDrill_Success(t *testing.T) {
	tempDir := t.TempDir()
	client := fake.NewSimpleClientset()
	logger := zaptest.NewLogger(t)

	uc := dr.NewDRUsecase(client, nil, logger,
		dr.WithBaseDir(tempDir),
		dr.WithEtcdDriver(dr.NewEtcdctlDriver(tempDir)),
	)

	// Step 1: Create a real snapshot on disk
	snap, err := uc.TriggerEtcdSnapshot(context.Background(), "local")
	require.NoError(t, err)
	require.NotEmpty(t, snap.SnapshotID)

	// Step 2: Execute automated restore drill in sandbox
	time.Sleep(5 * time.Millisecond) // Ensure measurable RTO and RPO lag
	report, err := uc.ExecuteRestoreDrill(context.Background(), "local", snap.SnapshotID)
	require.NoError(t, err)
	require.NotNil(t, report)

	// Step 3: Assert RTO and RPO metrics
	assert.Equal(t, "PASSED", report.Status)
	assert.Equal(t, "local", report.TargetClusterID)
	assert.Equal(t, snap.SnapshotID, report.SnapshotID)
	assert.Greater(t, report.RealRTODuration, time.Duration(0), "RealRTODuration must be > 0")
	assert.GreaterOrEqual(t, report.RealRPOLag, time.Duration(0), "RealRPOLag must be >= 0")
	assert.Greater(t, report.TablesRestored, 0)
	assert.Greater(t, report.RowsRestored, int64(0))
	assert.NotEmpty(t, report.LogOutput)

	// Ephemeral sandbox namespace should be cleaned up automatically
	namespaces, err := client.CoreV1().Namespaces().List(context.Background(), metav1.ListOptions{})
	require.NoError(t, err)
	for _, ns := range namespaces.Items {
		assert.NotContains(t, ns.Name, "restore-test-ephemeral-", "ephemeral namespace must be cleaned up")
	}
}

func TestDRUsecase_ExecuteRestoreDrill_IntegrityMismatch(t *testing.T) {
	tempDir := t.TempDir()
	client := fake.NewSimpleClientset()
	logger := zaptest.NewLogger(t)

	uc := dr.NewDRUsecase(client, nil, logger,
		dr.WithBaseDir(tempDir),
		dr.WithEtcdDriver(dr.NewEtcdctlDriver(tempDir)),
	)

	snap, err := uc.TriggerEtcdSnapshot(context.Background(), "local")
	require.NoError(t, err)

	// Corrupt snapshot file on disk by truncating it to 0 bytes
	err = os.Truncate(snap.FilePath, 0)
	require.NoError(t, err)

	report, err := uc.ExecuteRestoreDrill(context.Background(), "local", snap.SnapshotID)
	assert.ErrorIs(t, err, domain.ErrRestoreDrillIntegrityMismatch)
	require.NotNil(t, report)
	assert.Equal(t, "FAILED", report.Status)
	assert.Greater(t, report.RealRTODuration, time.Duration(0))
}

func TestDRUsecase_ExecuteRestoreDrill_Validation(t *testing.T) {
	tempDir := t.TempDir()
	client := fake.NewSimpleClientset()
	logger := zaptest.NewLogger(t)
	uc := dr.NewDRUsecase(client, nil, logger, dr.WithBaseDir(tempDir))

	// Missing clusterID
	_, err := uc.ExecuteRestoreDrill(context.Background(), "", "snap-1")
	assert.Error(t, err)

	// Missing snapshotID
	_, err = uc.ExecuteRestoreDrill(context.Background(), "local", "")
	assert.Error(t, err)

	// Non-existent snapshot
	_, err = uc.ExecuteRestoreDrill(context.Background(), "local", "non-existent-snap")
	assert.Error(t, err)
}

func TestDRUsecase_ExecuteRestoreDrill_WithK8sResources(t *testing.T) {
	tempDir := t.TempDir()
	existingNS := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}}
	client := fake.NewSimpleClientset(existingNS)
	logger := zaptest.NewLogger(t)

	uc := dr.NewDRUsecase(client, nil, logger,
		dr.WithBaseDir(tempDir),
		dr.WithEtcdDriver(dr.NewEtcdctlDriver(tempDir)),
	)

	snap, err := uc.TriggerEtcdSnapshot(context.Background(), "local")
	require.NoError(t, err)

	report, err := uc.ExecuteRestoreDrill(context.Background(), "local", snap.SnapshotID)
	require.NoError(t, err)
	assert.Equal(t, "PASSED", report.Status)
}
