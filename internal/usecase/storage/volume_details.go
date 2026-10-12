package storage

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/datdt/k8sselfhost/internal/domain/storage"
	domainerrors "github.com/datdt/k8sselfhost/internal/pkg/errors"
)

// GetVolumeDetails queries persistent volume claim details, including StorageClass capability and real usage metrics.
func (u *VolumeUsecase) GetVolumeDetails(ctx context.Context, clusterID, namespace, name string) (*storage.VolumeDetails, error) {
	if strings.TrimSpace(name) == "" {
		return nil, domainerrors.NewValidation("name", "volume name cannot be empty")
	}

	client, err := u.getK8sClient(ctx, clusterID)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(namespace) == "" {
		namespace = u.discoverPVCNamespace(ctx, client, name)
	}

	pvc, err := client.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return nil, domainerrors.NewNotFound("pvc", name)
		}
		return nil, fmt.Errorf("getting pvc %s/%s: %w", namespace, name, err)
	}

	capacityBytes := int64(0)
	if qty, ok := pvc.Status.Capacity[corev1.ResourceStorage]; ok {
		capacityBytes = qty.Value()
	} else if qty, ok := pvc.Spec.Resources.Requests[corev1.ResourceStorage]; ok {
		capacityBytes = qty.Value()
	}

	scName := ""
	if pvc.Spec.StorageClassName != nil {
		scName = *pvc.Spec.StorageClassName
	}

	allowExpansion := false
	if scName != "" {
		if sc, scErr := client.StorageV1().StorageClasses().Get(ctx, scName, metav1.GetOptions{}); scErr == nil && sc != nil {
			if sc.AllowVolumeExpansion != nil && *sc.AllowVolumeExpansion {
				allowExpansion = true
			}
		}
	} else {
		if scList, listErr := client.StorageV1().StorageClasses().List(ctx, metav1.ListOptions{}); listErr == nil && scList != nil {
			for i := range scList.Items {
				if scList.Items[i].Annotations["storageclass.kubernetes.io/is-default-class"] == "true" {
					if scList.Items[i].AllowVolumeExpansion != nil && *scList.Items[i].AllowVolumeExpansion {
						allowExpansion = true
					}
					break
				}
			}
		}
	}

	volumeMode := "Filesystem"
	if pvc.Spec.VolumeMode != nil {
		volumeMode = string(*pvc.Spec.VolumeMode)
	}

	accessModes := make([]string, 0, len(pvc.Spec.AccessModes))
	for _, mode := range pvc.Spec.AccessModes {
		accessModes = append(accessModes, string(mode))
	}

	attachedNode := pvc.Annotations["volume.kubernetes.io/selected-node"]
	attachmentStatus := ""
	pvName := pvc.Spec.VolumeName
	if pvName != "" {
		if vaList, vaErr := client.StorageV1().VolumeAttachments().List(ctx, metav1.ListOptions{}); vaErr == nil && vaList != nil {
			for _, va := range vaList.Items {
				if va.Spec.Source.PersistentVolumeName != nil && *va.Spec.Source.PersistentVolumeName == pvName {
					if attachedNode == "" {
						attachedNode = va.Spec.NodeName
					}
					if va.Status.Attached {
						attachmentStatus = "Attached"
					} else {
						attachmentStatus = "Attaching"
					}
					break
				}
			}
		}
	}

	var usedBytes *int64
	var usagePercentage *float64
	usageSource := storage.UsageSourceUnknown

	provider := u.metricsProvider
	if provider == nil {
		provider = u.defaultMetricsProvider
	}
	if provider != nil {
		if uBytes, mErr := provider.GetVolumeMetrics(ctx, client, namespace, pvc.Name); mErr == nil && uBytes != nil {
			uVal := *uBytes
			usedBytes = &uVal
			usageSource = storage.UsageSourceKubeletMetrics
			if capacityBytes > 0 {
				pct := (float64(uVal) / float64(capacityBytes)) * 100.0
				usagePercentage = &pct
			}
		}
	}

	volID := string(pvc.UID)
	if volID == "" {
		volID = pvc.Name
	}

	return &storage.VolumeDetails{
		VolumeID:         volID,
		PersistentVolume: pvName,
		ClaimName:        pvc.Name,
		Namespace:        pvc.Namespace,
		StorageClassName: scName,
		CapacityBytes:    capacityBytes,
		UsedBytes:        usedBytes,
		UsagePercentage:  usagePercentage,
		UsageSource:      usageSource,
		AllowExpansion:   allowExpansion,
		VolumeMode:       volumeMode,
		AccessModes:      accessModes,
		Phase:            string(pvc.Status.Phase),
		AttachedNode:     attachedNode,
		AttachmentStatus: attachmentStatus,
		UpdatedAt:        pvc.CreationTimestamp.Time,
	}, nil
}
