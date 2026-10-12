package storage

import (
	"context"
	"fmt"
	"strings"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/datdt/k8sselfhost/internal/domain/storage"
	domainerrors "github.com/datdt/k8sselfhost/internal/pkg/errors"
)

// ExpandVolume performs an online volume expansion by verifying StorageClass allowVolumeExpansion and patching PVC requests.storage.
func (u *VolumeUsecase) ExpandVolume(ctx context.Context, clusterID, namespace, name string, newSizeBytes int64) error {
	if newSizeBytes <= 0 {
		return domainerrors.NewValidation("new_size_bytes", "volume size must be greater than zero")
	}
	if strings.TrimSpace(name) == "" {
		return domainerrors.NewValidation("name", "volume name cannot be empty")
	}

	client, err := u.getK8sClient(ctx, clusterID)
	if err != nil {
		return err
	}

	if strings.TrimSpace(namespace) == "" {
		namespace = u.discoverPVCNamespace(ctx, client, name)
	}

	pvc, err := client.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return domainerrors.NewNotFound("pvc", name)
		}
		return fmt.Errorf("getting pvc %s/%s: %w", namespace, name, err)
	}

	// 1. Guard against volume shrink attempts (Kubernetes PVC resizing cannot decrease size)
	var currentCapacityBytes int64
	if qty, ok := pvc.Status.Capacity[corev1.ResourceStorage]; ok {
		currentCapacityBytes = qty.Value()
	} else if qty, ok := pvc.Spec.Resources.Requests[corev1.ResourceStorage]; ok {
		currentCapacityBytes = qty.Value()
	}

	if currentCapacityBytes > 0 && newSizeBytes <= currentCapacityBytes {
		return storage.NewErrStorageShrinkUnsupported()
	}

	// 2. Guard against non-expandable StorageClass
	scName := ""
	if pvc.Spec.StorageClassName != nil {
		scName = *pvc.Spec.StorageClassName
	}

	if scName != "" {
		sc, scErr := client.StorageV1().StorageClasses().Get(ctx, scName, metav1.GetOptions{})
		if scErr != nil {
			if k8serrors.IsNotFound(scErr) {
				return storage.NewErrStorageClassNoExpansion(scName)
			}
			return fmt.Errorf("getting storageclass %s: %w", scName, scErr)
		}
		if sc.AllowVolumeExpansion == nil || !*sc.AllowVolumeExpansion {
			return storage.NewErrStorageClassNoExpansion(scName)
		}
	} else {
		// Look up default cluster StorageClass
		scList, listErr := client.StorageV1().StorageClasses().List(ctx, metav1.ListOptions{})
		if listErr == nil && scList != nil {
			var defaultSC *storagev1.StorageClass
			for i := range scList.Items {
				if scList.Items[i].Annotations["storageclass.kubernetes.io/is-default-class"] == "true" {
					defaultSC = &scList.Items[i]
					break
				}
			}
			if defaultSC != nil {
				if defaultSC.AllowVolumeExpansion == nil || !*defaultSC.AllowVolumeExpansion {
					return storage.NewErrStorageClassNoExpansion(defaultSC.Name)
				}
			}
		}
	}

	newQty := resource.NewQuantity(newSizeBytes, resource.BinarySI)

	// Execute patch against Kubernetes API
	patchBytes := []byte(fmt.Sprintf(`{"spec":{"resources":{"requests":{"storage":"%s"}}}}`, newQty.String()))
	_, patchErr := client.CoreV1().PersistentVolumeClaims(namespace).Patch(ctx, name, types.MergePatchType, patchBytes, metav1.PatchOptions{})
	if patchErr != nil {
		// Fallback to direct update if patch is not supported by the client implementation
		if pvc.Spec.Resources.Requests == nil {
			pvc.Spec.Resources.Requests = corev1.ResourceList{}
		}
		pvc.Spec.Resources.Requests[corev1.ResourceStorage] = *newQty
		if _, updateErr := client.CoreV1().PersistentVolumeClaims(namespace).Update(ctx, pvc, metav1.UpdateOptions{}); updateErr != nil {
			return fmt.Errorf("expanding pvc %s/%s: %w", namespace, name, updateErr)
		}
	}

	u.logger.Info("Expanded distributed volume",
		zap.String("cluster", clusterID),
		zap.String("namespace", namespace),
		zap.String("name", name),
		zap.Int64("new_size_bytes", newSizeBytes),
	)

	return nil
}
