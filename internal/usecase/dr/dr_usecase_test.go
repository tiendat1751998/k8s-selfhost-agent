package dr

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"

	domain "github.com/datdt/k8sselfhost/internal/domain/dr"
)

func TestDRUsecase_TriggerEtcdSnapshot(t *testing.T) {
	tempDir := t.TempDir()
	client := fake.NewSimpleClientset()
	logger := zaptest.NewLogger(t)
	uc := NewDRUsecase(client, nil, logger,
		WithBaseDir(tempDir),
		WithEtcdDriver(NewEtcdctlDriver(tempDir, WithIsolatedTestMode())),
	)

	res, err := uc.TriggerEtcdSnapshot(context.Background(), "local")
	require.NoError(t, err)

	assert.Equal(t, "local", res.ClusterID)
	assert.Equal(t, "Completed", res.Status)
	assert.Greater(t, res.Size, int64(0), "Size must be real positive size")
	assert.Equal(t, res.Size, res.SizeBytes)
	assert.True(t, strings.HasPrefix(res.SnapshotID, "etcd-snap-local-"))
	assert.True(t, strings.Contains(res.StoragePath, res.SnapshotID))
	assert.NotEmpty(t, res.SHA256Hash)
	assert.Len(t, res.SHA256Hash, 64, "SHA256 must be a 64-char hex string")

	// Verify file actually exists on disk
	fi, err := os.Stat(res.FilePath)
	require.NoError(t, err)
	assert.Equal(t, res.SizeBytes, fi.Size())

	// Verify real SHA256 matches actual file bytes
	f, err := os.Open(res.FilePath)
	require.NoError(t, err)
	defer f.Close()
	hasher := sha256.New()
	_, err = io.Copy(hasher, f)
	require.NoError(t, err)
	expectedHash := hex.EncodeToString(hasher.Sum(nil))
	assert.Equal(t, expectedHash, res.SHA256Hash)
}

func TestDRUsecase_RestoreEtcdSnapshot(t *testing.T) {
	tempDir := t.TempDir()
	client := fake.NewSimpleClientset()
	logger := zaptest.NewLogger(t)
	uc := NewDRUsecase(client, nil, logger,
		WithBaseDir(tempDir),
		WithEtcdDriver(NewEtcdctlDriver(tempDir, WithIsolatedTestMode())),
	)

	snap, err := uc.TriggerEtcdSnapshot(context.Background(), "local")
	require.NoError(t, err)

	// Successful restore
	err = uc.RestoreEtcdSnapshot(context.Background(), "local", snap.SnapshotID)
	assert.NoError(t, err)

	// Non-existent snapshot
	err = uc.RestoreEtcdSnapshot(context.Background(), "local", "non-existent-snap")
	assert.Error(t, err)

	// Cluster mismatch
	err = uc.RestoreEtcdSnapshot(context.Background(), "other-cluster", snap.SnapshotID)
	assert.Error(t, err)

	// Empty IDs
	assert.Error(t, uc.RestoreEtcdSnapshot(context.Background(), "", snap.SnapshotID))
	assert.Error(t, uc.RestoreEtcdSnapshot(context.Background(), "local", ""))
}

func TestDRUsecase_TriggerClusterBackupAndList(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
		},
	}
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "default",
		},
	}

	client := fake.NewSimpleClientset(pod, ns)
	logger := zaptest.NewLogger(t)
	uc := NewDRUsecase(client, nil, logger)

	// List on empty cluster
	list, err := uc.ListClusterBackups(context.Background(), "local")
	require.NoError(t, err)
	assert.Empty(t, list)

	// Trigger custom named backup
	b1, err := uc.TriggerClusterBackup(context.Background(), "local", "manual-backup-01")
	require.NoError(t, err)
	assert.Equal(t, "manual-backup-01", b1.BackupName)
	assert.Equal(t, "Completed", b1.Phase)
	assert.GreaterOrEqual(t, b1.TotalItems, 1)

	// Trigger auto-named backup
	b2, err := uc.TriggerClusterBackup(context.Background(), "local", "")
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(b2.BackupName, "velero-backup-local-"))

	// Verify both exist in list
	backups, err := uc.ListClusterBackups(context.Background(), "local")
	require.NoError(t, err)
	assert.Len(t, backups, 2)
}

func TestDRUsecase_VeleroCRD_Creation(t *testing.T) {
	scheme := runtime.NewScheme()
	dynClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		veleroBackupGVR: "BackupList",
	})
	k8sClient := fake.NewSimpleClientset()
	logger := zaptest.NewLogger(t)

	uc := NewDRUsecase(k8sClient, nil, logger,
		WithDynamicClient(dynClient),
		WithVeleroNamespace("velero"),
	)

	backup, err := uc.TriggerClusterBackup(context.Background(), "prod-cluster", "prod-weekly-01")
	require.NoError(t, err)
	assert.Equal(t, "prod-weekly-01", backup.BackupName)
	assert.Equal(t, "prod-cluster", backup.ClusterID)

	// Verify dynamic client received actual Velero CRD object
	gvr := schema.GroupVersionResource{Group: "velero.io", Version: "v1", Resource: "backups"}
	obj, err := dynClient.Resource(gvr).Namespace("velero").Get(context.Background(), "prod-weekly-01", metav1.GetOptions{})
	require.NoError(t, err)

	assert.Equal(t, "Backup", obj.GetKind())
	assert.Equal(t, "velero.io/v1", obj.GetAPIVersion())
	assert.Equal(t, "prod-cluster", obj.GetLabels()["cluster_id"])

	spec, ok := obj.Object["spec"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "default", spec["storageLocation"])
}

func TestEtcdctlDriver_CLI_Execution(t *testing.T) {
	tempDir := t.TempDir()
	var executedCmd string
	var executedArgs []string

	mockRunner := func(ctx context.Context, name string, args ...string) ([]byte, error) {
		executedCmd = name
		executedArgs = args
		for i, arg := range args {
			if arg == "save" && i+1 < len(args) {
				targetFile := args[i+1]
				return nil, os.WriteFile(targetFile, []byte("mock-etcd-db-bytes-content"), 0600)
			}
		}
		return []byte(`{"header":{"cluster_id":1}}`), nil
	}

	driver := NewEtcdctlDriver(tempDir, WithCommandRunner(mockRunner))
	res, err := driver.SaveSnapshot(context.Background(), "https://127.0.0.1:2379", "/ca.crt", "/cert.crt", "/key.key")
	require.NoError(t, err)
	assert.Equal(t, "etcdctl", executedCmd)
	assert.Contains(t, executedArgs, "--endpoints=https://127.0.0.1:2379")
	assert.Contains(t, executedArgs, "--cacert=/ca.crt")
	assert.Contains(t, executedArgs, "--cert=/cert.crt")
	assert.Contains(t, executedArgs, "--key=/key.key")
	assert.Contains(t, executedArgs, "snapshot")
	assert.Contains(t, executedArgs, "save")
	assert.NotEmpty(t, res.SHA256Hash)
	assert.Greater(t, res.SizeBytes, int64(0))

	// Verify snapshot verification with command runner
	valid, err := driver.VerifySnapshot(context.Background(), res.FilePath)
	assert.NoError(t, err)
	assert.True(t, valid)
}

func TestDRUsecase_ValidationErrors(t *testing.T) {
	client := fake.NewSimpleClientset()
	logger := zaptest.NewLogger(t)
	uc := NewDRUsecase(client, nil, logger)

	if _, err := uc.TriggerEtcdSnapshot(context.Background(), ""); err == nil {
		t.Error("expected error for empty clusterID in snapshot")
	}

	if _, err := uc.TriggerClusterBackup(context.Background(), "", "test"); err == nil {
		t.Error("expected error for empty clusterID in backup")
	}

	if _, err := uc.ListClusterBackups(context.Background(), ""); err == nil {
		t.Error("expected error for empty clusterID in list")
	}

	// Verify domain error values
	assert.NotNil(t, domain.ErrEtcdSnapshotFailed)
}

func TestEtcdctlDriver_BinaryNotFoundInPATH(t *testing.T) {
	tempDir := t.TempDir()
	// Driver without cmdRunner and without isolated test mode
	driver := NewEtcdctlDriver(tempDir, WithBinaryPath("nonexistent-etcdctl-binary-xyz"))
	_, err := driver.SaveSnapshot(context.Background(), "https://127.0.0.1:2379", "", "", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "etcdctl binary not found in PATH")
}