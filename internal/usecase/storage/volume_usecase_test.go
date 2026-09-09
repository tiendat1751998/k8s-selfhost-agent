package storage

import (
	"context"
	"errors"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/datdt/k8sselfhost/internal/domain/storage"
	infraK8s "github.com/datdt/k8sselfhost/internal/infrastructure/kubernetes"
	domainerrors "github.com/datdt/k8sselfhost/internal/pkg/errors"
)

func TestStorageUsecase_ListVolumes_Longhorn(t *testing.T) {
	ctx := context.Background()

	node1 := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "worker-node-1", UID: "uid-worker-1"},
	}
	node2 := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "worker-node-2", UID: "uid-worker-2"},
	}
	node3 := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "worker-node-3", UID: "uid-worker-3"},
	}

	scLonghorn := "longhorn"
	longhornPVC := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pvc-longhorn-data",
			Namespace: "prod",
			Annotations: map[string]string{
				"volume.kubernetes.io/selected-node": "worker-node-1",
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			StorageClassName: &scLonghorn,
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("10Gi"),
				},
			},
		},
		Status: corev1.PersistentVolumeClaimStatus{
			Phase: corev1.ClaimBound,
			Capacity: corev1.ResourceList{
				corev1.ResourceStorage: resource.MustParse("10Gi"),
			},
		},
	}

	scStandard := "standard"
	standardPVC := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pvc-standard-cache",
			Namespace: "dev",
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			StorageClassName: &scStandard,
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("5Gi"),
				},
			},
		},
		Status: corev1.PersistentVolumeClaimStatus{
			Phase: corev1.ClaimPending,
		},
	}

	fakeClient := fake.NewSimpleClientset(node1, node2, node3, longhornPVC, standardPVC)
	uc := NewVolumeUsecase(fakeClient, nil, nil)

	volumes, err := uc.ListVolumes(ctx, "local")
	if err != nil {
		t.Fatalf("ListVolumes failed: %v", err)
	}

	if len(volumes) != 2 {
		t.Fatalf("expected 2 volumes, got %d", len(volumes))
	}

	// Find the Longhorn volume
	var lhVol *storage.DistributedVolume
	var stdVol *storage.DistributedVolume
	for i := range volumes {
		if volumes[i].Name == "pvc-longhorn-data" {
			lhVol = &volumes[i]
		}
		if volumes[i].Name == "pvc-standard-cache" {
			stdVol = &volumes[i]
		}
	}

	if lhVol == nil {
		t.Fatal("longhorn volume not found in results")
	}
	if !lhVol.IsHealthy() {
		t.Errorf("expected longhorn volume to be healthy, got %s", lhVol.HealthStatus)
	}
	if lhVol.ReplicationCount != 3 {
		t.Errorf("expected replication count 3 for longhorn, got %d", lhVol.ReplicationCount)
	}
	if len(lhVol.Replicas) != 3 {
		t.Fatalf("expected 3 replicas for longhorn, got %d", len(lhVol.Replicas))
	}
	if lhVol.Replicas[0].NodeName != "worker-node-1" || lhVol.Replicas[0].NodeID != "uid-worker-1" {
		t.Errorf("replica 0 mismatch: %+v", lhVol.Replicas[0])
	}
	if lhVol.Replicas[1].NodeName != "worker-node-2" {
		t.Errorf("replica 1 nodeName mismatch: %s", lhVol.Replicas[1].NodeName)
	}
	if lhVol.CapacityBytes != 10*1024*1024*1024 {
		t.Errorf("expected capacity 10Gi, got %d bytes", lhVol.CapacityBytes)
	}
	if lhVol.UsedBytes <= 0 {
		t.Errorf("expected positive used bytes estimate, got %d", lhVol.UsedBytes)
	}

	// Verify standard PVC
	if stdVol == nil {
		t.Fatal("standard volume not found in results")
	}
	if stdVol.IsHealthy() {
		t.Errorf("expected standard volume to not be healthy when Pending")
	}
	if stdVol.HealthStatus != storage.HealthStatusDegraded {
		t.Errorf("expected Degraded health status for pending PVC, got %s", stdVol.HealthStatus)
	}
	if stdVol.ReplicationCount != 1 {
		t.Errorf("expected replication count 1 for standard PVC, got %d", stdVol.ReplicationCount)
	}
}

func TestStorageUsecase_ListVolumes_SyntheticNodesFallback(t *testing.T) {
	ctx := context.Background()

	scLonghorn := "longhorn-fast"
	longhornPVC := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pvc-app",
			Namespace: "default",
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			StorageClassName: &scLonghorn,
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("20Gi"),
				},
			},
		},
		Status: corev1.PersistentVolumeClaimStatus{
			Phase: corev1.ClaimBound,
		},
	}

	// No nodes registered in fake client
	fakeClient := fake.NewSimpleClientset(longhornPVC)
	uc := NewVolumeUsecase(fakeClient, nil, nil)

	volumes, err := uc.ListVolumes(ctx, "local")
	if err != nil {
		t.Fatalf("ListVolumes failed: %v", err)
	}

	if len(volumes) != 1 {
		t.Fatalf("expected 1 volume, got %d", len(volumes))
	}

	vol := volumes[0]
	if len(vol.Replicas) != 3 {
		t.Fatalf("expected 3 synthetic replicas, got %d", len(vol.Replicas))
	}
	if vol.Replicas[0].NodeName != "node-1" || vol.Replicas[1].NodeName != "node-2" || vol.Replicas[2].NodeName != "node-3" {
		t.Errorf("synthetic node names mismatch: %+v", vol.Replicas)
	}
}

func TestStorageUsecase_ExpandVolume_Success(t *testing.T) {
	ctx := context.Background()

	scLonghorn := "longhorn"
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pvc-expand-target",
			Namespace: "default",
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			StorageClassName: &scLonghorn,
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("10Gi"),
				},
			},
		},
		Status: corev1.PersistentVolumeClaimStatus{
			Phase: corev1.ClaimBound,
		},
	}

	fakeClient := fake.NewSimpleClientset(pvc)
	uc := NewVolumeUsecase(fakeClient, nil, nil)

	newSizeBytes := int64(20 * 1024 * 1024 * 1024) // 20Gi
	err := uc.ExpandVolume(ctx, "local", "default", "pvc-expand-target", newSizeBytes)
	if err != nil {
		t.Fatalf("ExpandVolume failed: %v", err)
	}

	// Fetch updated PVC
	updatedPVC, err := fakeClient.CoreV1().PersistentVolumeClaims("default").Get(ctx, "pvc-expand-target", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to get updated PVC: %v", err)
	}

	qty := updatedPVC.Spec.Resources.Requests[corev1.ResourceStorage]
	if qty.Value() != newSizeBytes {
		t.Errorf("expected expanded capacity %d bytes, got %d bytes (%s)", newSizeBytes, qty.Value(), qty.String())
	}
}

func TestStorageUsecase_ExpandVolume_AutoDiscoverNamespace(t *testing.T) {
	ctx := context.Background()

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pvc-auto-discover",
			Namespace: "custom-ns",
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("10Gi"),
				},
			},
		},
	}

	fakeClient := fake.NewSimpleClientset(pvc)
	uc := NewVolumeUsecase(fakeClient, nil, nil)

	// Pass empty namespace -> discover
	newSizeBytes := int64(30 * 1024 * 1024 * 1024)
	err := uc.ExpandVolume(ctx, "local", "", "pvc-auto-discover", newSizeBytes)
	if err != nil {
		t.Fatalf("ExpandVolume with empty namespace failed: %v", err)
	}

	updatedPVC, err := fakeClient.CoreV1().PersistentVolumeClaims("custom-ns").Get(ctx, "pvc-auto-discover", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to get updated PVC from custom-ns: %v", err)
	}
	qty := updatedPVC.Spec.Resources.Requests[corev1.ResourceStorage]
	if qty.Value() != newSizeBytes {
		t.Errorf("expanded size mismatch: got %d", qty.Value())
	}
}

func TestStorageUsecase_ExpandVolume_Errors(t *testing.T) {
	ctx := context.Background()
	fakeClient := fake.NewSimpleClientset()
	uc := NewVolumeUsecase(fakeClient, nil, nil)

	// Test invalid size
	err := uc.ExpandVolume(ctx, "local", "default", "pvc-test", 0)
	if err == nil {
		t.Fatal("expected error for zero size")
	}
	var domErr *domainerrors.DomainError
	if !errors.As(err, &domErr) || domErr.Code != domainerrors.CodeValidation {
		t.Errorf("expected validation error, got: %v", err)
	}

	// Test empty name
	err = uc.ExpandVolume(ctx, "local", "default", "", 1024)
	if err == nil {
		t.Fatal("expected error for empty name")
	}

	// Test PVC not found
	err = uc.ExpandVolume(ctx, "local", "default", "non-existent", 1024)
	if err == nil {
		t.Fatal("expected error for non-existent PVC")
	}
	if !errors.As(err, &domErr) || domErr.Code != domainerrors.CodeNotFound {
		t.Errorf("expected not found error, got: %v", err)
	}
}

func TestStorageUsecase_CreateSnapshot_Success(t *testing.T) {
	ctx := context.Background()

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pvc-db",
			Namespace: "production",
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("50Gi"),
				},
			},
		},
		Status: corev1.PersistentVolumeClaimStatus{
			Phase: corev1.ClaimBound,
		},
	}

	fakeClient := fake.NewSimpleClientset(pvc)
	uc := NewVolumeUsecase(fakeClient, nil, nil)

	req := storage.VolumeSnapshotRequest{
		SnapshotName: "pvc-db-snap-1",
		Labels:       map[string]string{"env": "production"},
	}
	res, err := uc.CreateSnapshot(ctx, "local", "production", "pvc-db", req)
	if err != nil {
		t.Fatalf("CreateSnapshot failed: %v", err)
	}

	if res.SnapshotName != "pvc-db-snap-1" {
		t.Errorf("expected snapshot name 'pvc-db-snap-1', got '%s'", res.SnapshotName)
	}
	if res.VolumeName != "pvc-db" {
		t.Errorf("expected volume name 'pvc-db', got '%s'", res.VolumeName)
	}
	if res.Status != storage.SnapshotStatusReady {
		t.Errorf("expected status Ready, got '%s'", res.Status)
	}
	if res.CreatedAt == "" {
		t.Error("expected non-empty CreatedAt")
	}
}

func TestStorageUsecase_CreateSnapshot_GeneratedName(t *testing.T) {
	ctx := context.Background()

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pvc-redis",
			Namespace: "default",
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("10Gi"),
				},
			},
		},
	}

	fakeClient := fake.NewSimpleClientset(pvc)
	uc := NewVolumeUsecase(fakeClient, nil, nil)

	res, err := uc.CreateSnapshot(ctx, "local", "default", "pvc-redis", storage.VolumeSnapshotRequest{})
	if err != nil {
		t.Fatalf("CreateSnapshot failed: %v", err)
	}

	if !strings.HasPrefix(res.SnapshotName, "pvc-redis-snapshot-") {
		t.Errorf("expected generated snapshot name starting with 'pvc-redis-snapshot-', got '%s'", res.SnapshotName)
	}
}

func TestStorageUsecase_CreateSnapshot_NotFound(t *testing.T) {
	ctx := context.Background()
	fakeClient := fake.NewSimpleClientset()
	uc := NewVolumeUsecase(fakeClient, nil, nil)

	_, err := uc.CreateSnapshot(ctx, "local", "default", "non-existent-pvc", storage.VolumeSnapshotRequest{})
	if err == nil {
		t.Fatal("expected error for non-existent PVC snapshot")
	}
	var domErr *domainerrors.DomainError
	if !errors.As(err, &domErr) || domErr.Code != domainerrors.CodeNotFound {
		t.Errorf("expected not found error, got: %v", err)
	}
}

func TestStorageUsecase_K8sUnavailable(t *testing.T) {
	ctx := context.Background()
	uc := NewVolumeUsecase(nil, nil, nil)

	_, err := uc.ListVolumes(ctx, "local")
	if !errors.Is(err, infraK8s.ErrK8sUnavailable) {
		t.Errorf("expected ErrK8sUnavailable for ListVolumes, got: %v", err)
	}

	err = uc.ExpandVolume(ctx, "local", "default", "pvc-1", 1024)
	if !errors.Is(err, infraK8s.ErrK8sUnavailable) {
		t.Errorf("expected ErrK8sUnavailable for ExpandVolume, got: %v", err)
	}

	_, err = uc.CreateSnapshot(ctx, "local", "default", "pvc-1", storage.VolumeSnapshotRequest{})
	if !errors.Is(err, infraK8s.ErrK8sUnavailable) {
		t.Errorf("expected ErrK8sUnavailable for CreateSnapshot, got: %v", err)
	}
}
