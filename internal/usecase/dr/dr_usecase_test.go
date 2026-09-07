package dr

import (
	"context"
	"strings"
	"testing"

	"go.uber.org/zap/zaptest"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestDRUsecase_TriggerEtcdSnapshot(t *testing.T) {
	client := fake.NewSimpleClientset()
	logger := zaptest.NewLogger(t)
	uc := NewDRUsecase(client, nil, logger)

	res, err := uc.TriggerEtcdSnapshot(context.Background(), "local")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.ClusterID != "local" {
		t.Errorf("expected clusterID local, got %s", res.ClusterID)
	}
	if res.Status != "Completed" {
		t.Errorf("expected status Completed, got %s", res.Status)
	}
	if res.Size <= 0 {
		t.Errorf("expected positive size, got %d", res.Size)
	}
	if !strings.HasPrefix(res.SnapshotID, "etcd-snap-local-") {
		t.Errorf("unexpected snapshot ID format: %s", res.SnapshotID)
	}
	if !strings.Contains(res.StoragePath, res.SnapshotID) {
		t.Errorf("storage path does not contain snapshot ID: %s", res.StoragePath)
	}
}

func TestDRUsecase_RestoreEtcdSnapshot(t *testing.T) {
	client := fake.NewSimpleClientset()
	logger := zaptest.NewLogger(t)
	uc := NewDRUsecase(client, nil, logger)

	snap, err := uc.TriggerEtcdSnapshot(context.Background(), "local")
	if err != nil {
		t.Fatalf("failed to trigger snapshot: %v", err)
	}

	// Successful restore
	err = uc.RestoreEtcdSnapshot(context.Background(), "local", snap.SnapshotID)
	if err != nil {
		t.Errorf("expected restore to succeed, got: %v", err)
	}

	// Non-existent snapshot
	err = uc.RestoreEtcdSnapshot(context.Background(), "local", "non-existent-snap")
	if err == nil {
		t.Error("expected error for non-existent snapshot")
	}

	// Cluster mismatch
	err = uc.RestoreEtcdSnapshot(context.Background(), "other-cluster", snap.SnapshotID)
	if err == nil {
		t.Error("expected error for cluster mismatch")
	}

	// Empty IDs
	if err := uc.RestoreEtcdSnapshot(context.Background(), "", snap.SnapshotID); err == nil {
		t.Error("expected error for empty cluster ID")
	}
	if err := uc.RestoreEtcdSnapshot(context.Background(), "local", ""); err == nil {
		t.Error("expected error for empty snapshot ID")
	}
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
	if err != nil {
		t.Fatalf("unexpected error on empty list: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("expected empty list, got %d", len(list))
	}

	// Trigger custom named backup
	b1, err := uc.TriggerClusterBackup(context.Background(), "local", "manual-backup-01")
	if err != nil {
		t.Fatalf("unexpected error triggering backup: %v", err)
	}
	if b1.BackupName != "manual-backup-01" {
		t.Errorf("expected manual-backup-01, got %s", b1.BackupName)
	}
	if b1.Phase != "Completed" {
		t.Errorf("expected Completed phase, got %s", b1.Phase)
	}
	if b1.TotalItems < 1 {
		t.Errorf("expected total items >= 1, got %d", b1.TotalItems)
	}

	// Trigger auto-named backup
	b2, err := uc.TriggerClusterBackup(context.Background(), "local", "")
	if err != nil {
		t.Fatalf("unexpected error triggering auto-named backup: %v", err)
	}
	if !strings.HasPrefix(b2.BackupName, "velero-backup-local-") {
		t.Errorf("unexpected auto-generated backup name: %s", b2.BackupName)
	}

	// Verify both exist in list
	backups, err := uc.ListClusterBackups(context.Background(), "local")
	if err != nil {
		t.Fatalf("unexpected error listing backups: %v", err)
	}
	if len(backups) != 2 {
		t.Errorf("expected 2 backups, got %d", len(backups))
	}
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
}