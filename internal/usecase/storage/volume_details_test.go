package storage

import (
	"context"
	"errors"
	"testing"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/datdt/k8sselfhost/internal/domain/storage"
	domainerrors "github.com/datdt/k8sselfhost/internal/pkg/errors"
)

type mockMetricsProvider struct {
	usedBytes *int64
	err       error
}

func (m *mockMetricsProvider) GetVolumeMetrics(ctx context.Context, client kubernetes.Interface, namespace, pvcName string) (*int64, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.usedBytes, nil
}

func TestVolumeDetails_RealMetrics_Success(t *testing.T) {
	ctx := context.Background()
	scName := "gp3"
	allowExpansion := true
	sc := &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{Name: scName},
		AllowVolumeExpansion: &allowExpansion,
	}

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pvc-analytics-db",
			Namespace: "production",
			UID:       "uid-analytics-100",
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

	// Inject mock metrics provider returning 40GB out of 100GB
	used40Gi := int64(40 * 1024 * 1024 * 1024)
	uc.SetMetricsProvider(&mockMetricsProvider{
		usedBytes: &used40Gi,
	})

	details, err := uc.GetVolumeDetails(ctx, "local", "production", "pvc-analytics-db")
	if err != nil {
		t.Fatalf("GetVolumeDetails failed: %v", err)
	}

	if details.ClaimName != "pvc-analytics-db" {
		t.Errorf("expected claim name pvc-analytics-db, got %s", details.ClaimName)
	}
	if details.CapacityBytes != 100*1024*1024*1024 {
		t.Errorf("expected capacity 100Gi, got %d", details.CapacityBytes)
	}
	if details.UsedBytes == nil || *details.UsedBytes != used40Gi {
		t.Fatalf("expected usedBytes %d, got %v", used40Gi, details.UsedBytes)
	}
	if details.UsagePercentage == nil || *details.UsagePercentage != 40.0 {
		t.Errorf("expected usage percentage 40.0, got %v", details.UsagePercentage)
	}
	if details.UsageSource != storage.UsageSourceKubeletMetrics {
		t.Errorf("expected usage source %s, got %s", storage.UsageSourceKubeletMetrics, details.UsageSource)
	}
	if !details.AllowExpansion {
		t.Error("expected allowExpansion to be true")
	}
}

func TestVolumeDetails_MetricsUnavailable_NoFake(t *testing.T) {
	ctx := context.Background()
	scName := "standard"

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pvc-legacy-data",
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

	fakeClient := fake.NewSimpleClientset(pvc)
	uc := NewVolumeUsecase(fakeClient, nil, nil)

	// Inject failing metrics provider to simulate metrics server outage
	uc.SetMetricsProvider(&mockMetricsProvider{
		err: errors.New("metrics agent unreachable: connection refused"),
	})

	details, err := uc.GetVolumeDetails(ctx, "local", "default", "pvc-legacy-data")
	if err != nil {
		t.Fatalf("GetVolumeDetails failed: %v", err)
	}

	// Invariant: Zero fake telemetry. When metrics are down, UsedBytes and UsagePercentage MUST be nil.
	if details.UsedBytes != nil {
		t.Errorf("expected nil UsedBytes when metrics unavailable (found fake value %d)", *details.UsedBytes)
	}
	if details.UsagePercentage != nil {
		t.Errorf("expected nil UsagePercentage when metrics unavailable (found fake value %f)", *details.UsagePercentage)
	}
	if details.UsageSource != storage.UsageSourceUnknown {
		t.Errorf("expected UsageSource %s, got %s", storage.UsageSourceUnknown, details.UsageSource)
	}
}

func TestVolumeDetails_NotFound(t *testing.T) {
	ctx := context.Background()
	fakeClient := fake.NewSimpleClientset()
	uc := NewVolumeUsecase(fakeClient, nil, nil)

	_, err := uc.GetVolumeDetails(ctx, "local", "default", "missing-volume")
	if err == nil {
		t.Fatal("expected error for non-existent PVC")
	}
	var domErr *domainerrors.DomainError
	if !errors.As(err, &domErr) || domErr.Code != domainerrors.CodeNotFound {
		t.Errorf("expected NotFound error, got: %v", err)
	}
}
