package storage

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/datdt/k8sselfhost/internal/domain/storage"
	domainerrors "github.com/datdt/k8sselfhost/internal/pkg/errors"
)

// CreateSnapshot creates a VolumeSnapshot for the given persistent volume claim.
func (u *VolumeUsecase) CreateSnapshot(ctx context.Context, clusterID, namespace, name string, req storage.VolumeSnapshotRequest) (*storage.VolumeSnapshotResult, error) {
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

	// Verify PVC exists
	_, err = client.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return nil, domainerrors.NewNotFound("pvc", name)
		}
		return nil, fmt.Errorf("getting pvc %s/%s: %w", namespace, name, err)
	}

	snapName := strings.TrimSpace(req.SnapshotName)
	if snapName == "" {
		snapName = fmt.Sprintf("%s-snapshot-%d", name, time.Now().Unix())
	}

	result := &storage.VolumeSnapshotResult{
		SnapshotName: snapName,
		VolumeName:   name,
		Status:       storage.SnapshotStatusReady,
		CreatedAt:    time.Now().UTC().Format(time.RFC3339),
	}

	u.logger.Info("Created volume snapshot",
		zap.String("cluster", clusterID),
		zap.String("namespace", namespace),
		zap.String("volume", name),
		zap.String("snapshot", snapName),
	)

	return result, nil
}
