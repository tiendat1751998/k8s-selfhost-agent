package storage

import (
	"context"
	"errors"
	"testing"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	domainstorage "github.com/datdt/k8sselfhost/internal/domain/storage"
	domainerrors "github.com/datdt/k8sselfhost/internal/pkg/errors"
)

func TestExpandVolume_StorageClassNotAllowed_Rejects(t *testing.T) {
	ctx := context.Background()

	scName := "fixed-iops"
	allowExpansion := false
	sc := &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: scName,
		},
		AllowVolumeExpansion: &allowExpansion,
	}

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pvc-static-storage",
			Namespace: "default",
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			StorageClassName: &scName,
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("100Gi"),
				},
			},
		},
		Status: corev1.PersistentVolumeClaimStatus{
			Phase: corev1.ClaimBound,
			Capacity: corev1.ResourceList{
				corev1.ResourceStorage: resource.MustParse("100Gi"),
			},
		},
	}

	fakeClient := fake.NewSimpleClientset(pvc, sc)
	uc := NewVolumeUsecase(fakeClient, nil, nil)

	newSizeBytes := int64(150 * 1024 * 1024 * 1024) // 150Gi > 100Gi
	err := uc.ExpandVolume(ctx, "local", "default", "pvc-static-storage", newSizeBytes)
	if err == nil {
		t.Fatal("expected expansion rejection for StorageClass with allowVolumeExpansion=false")
	}

	var domErr *domainerrors.DomainError
	if !errors.As(err, &domErr) {
		t.Fatalf("expected domain error, got: %v", err)
	}
	if domErr.Code != domainstorage.ErrCodeStorageClassNoExpansion {
		t.Errorf("expected error code %s, got %s", domainstorage.ErrCodeStorageClassNoExpansion, domErr.Code)
	}
}

func TestExpandVolume_ShrinkAttempt_Rejects(t *testing.T) {
	ctx := context.Background()

	scName := "standard"
	allowExpansion := true
	sc := &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: scName,
		},
		AllowVolumeExpansion: &allowExpansion,
	}

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pvc-large-db",
			Namespace: "prod",
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			StorageClassName: &scName,
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("100Gi"),
				},
			},
		},
		Status: corev1.PersistentVolumeClaimStatus{
			Phase: corev1.ClaimBound,
			Capacity: corev1.ResourceList{
				corev1.ResourceStorage: resource.MustParse("100Gi"),
			},
		},
	}

	fakeClient := fake.NewSimpleClientset(pvc, sc)
	uc := NewVolumeUsecase(fakeClient, nil, nil)

	// Attempt to shrink from 100Gi to 50Gi
	shrinkSizeBytes := int64(50 * 1024 * 1024 * 1024)
	err := uc.ExpandVolume(ctx, "local", "prod", "pvc-large-db", shrinkSizeBytes)
	if err == nil {
		t.Fatal("expected volume shrink rejection")
	}

	var domErr *domainerrors.DomainError
	if !errors.As(err, &domErr) {
		t.Fatalf("expected domain error, got: %v", err)
	}
	if domErr.Code != domainstorage.ErrCodeStorageShrinkUnsupported {
		t.Errorf("expected error code %s, got %s", domainstorage.ErrCodeStorageShrinkUnsupported, domErr.Code)
	}
}

func TestExpandVolume_EqualSize_Rejects(t *testing.T) {
	ctx := context.Background()

	scName := "standard"
	allowExpansion := true
	sc := &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: scName,
		},
		AllowVolumeExpansion: &allowExpansion,
	}

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pvc-equal-check",
			Namespace: "prod",
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			StorageClassName: &scName,
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("100Gi"),
				},
			},
		},
		Status: corev1.PersistentVolumeClaimStatus{
			Phase: corev1.ClaimBound,
			Capacity: corev1.ResourceList{
				corev1.ResourceStorage: resource.MustParse("100Gi"),
			},
		},
	}

	fakeClient := fake.NewSimpleClientset(pvc, sc)
	uc := NewVolumeUsecase(fakeClient, nil, nil)

	// Request exactly current capacity (100Gi)
	equalBytes := int64(100 * 1024 * 1024 * 1024)
	err := uc.ExpandVolume(ctx, "local", "prod", "pvc-equal-check", equalBytes)
	if err == nil {
		t.Fatal("expected rejection for expansion request equal to current size")
	}

	var domErr *domainerrors.DomainError
	if !errors.As(err, &domErr) || domErr.Code != domainstorage.ErrCodeStorageShrinkUnsupported {
		t.Errorf("expected ERR_STORAGE_SHRINK_UNSUPPORTED, got: %v", err)
	}
}

func TestExpandVolume_ValidExpansion_Patched(t *testing.T) {
	ctx := context.Background()

	scName := "expandable-sc"
	allowExpansion := true
	sc := &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: scName,
		},
		AllowVolumeExpansion: &allowExpansion,
	}

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pvc-valid-expand",
			Namespace: "default",
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			StorageClassName: &scName,
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

	fakeClient := fake.NewSimpleClientset(pvc, sc)
	uc := NewVolumeUsecase(fakeClient, nil, nil)

	newSizeBytes := int64(25 * 1024 * 1024 * 1024) // 25Gi > 10Gi
	err := uc.ExpandVolume(ctx, "local", "default", "pvc-valid-expand", newSizeBytes)
	if err != nil {
		t.Fatalf("ExpandVolume failed: %v", err)
	}

	updated, err := fakeClient.CoreV1().PersistentVolumeClaims("default").Get(ctx, "pvc-valid-expand", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to get updated PVC: %v", err)
	}

	qty := updated.Spec.Resources.Requests[corev1.ResourceStorage]
	if qty.Value() != newSizeBytes {
		t.Errorf("expected size %d bytes, got %d", newSizeBytes, qty.Value())
	}
}
