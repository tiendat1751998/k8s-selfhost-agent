package storage

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	domainstorage "github.com/datdt/k8sselfhost/internal/domain/storage"
)

const (
	// OutOfServiceTaintKey is the standard Kubernetes 1.28+ out-of-service taint key.
	OutOfServiceTaintKey   = "node.kubernetes.io/out-of-service"
	OutOfServiceTaintValue = "nodeshutdown"
)

// CSIFencingHelper encapsulates volume and node storage fencing routines for offline worker nodes.
type CSIFencingHelper struct {
	pollInterval time.Duration
}

// NewCSIFencingHelper creates a new CSIFencingHelper instance.
func NewCSIFencingHelper() *CSIFencingHelper {
	return &CSIFencingHelper{
		pollInterval: 100 * time.Millisecond,
	}
}

// SetPollInterval configures the polling interval for VolumeAttachment detach checks.
func (f *CSIFencingHelper) SetPollInterval(interval time.Duration) {
	if interval > 0 {
		f.pollInterval = interval
	}
}

// ApplyOutOfServiceTaint applies the node.kubernetes.io/out-of-service=nodeshutdown:NoExecute taint
// to an unresponsive or failing node to trigger CSI detached reconciliation and allow safe volume movement.
func (f *CSIFencingHelper) ApplyOutOfServiceTaint(ctx context.Context, client kubernetes.Interface, nodeName string) error {
	if client == nil {
		return fmt.Errorf("kubernetes client is nil")
	}
	if nodeName == "" {
		return fmt.Errorf("nodeName cannot be empty")
	}

	node, err := client.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("getting node %s: %w", nodeName, err)
	}

	for _, t := range node.Spec.Taints {
		if t.Key == OutOfServiceTaintKey && t.Value == OutOfServiceTaintValue && t.Effect == corev1.TaintEffectNoExecute {
			// Taint already present
			return nil
		}
	}

	node.Spec.Taints = append(node.Spec.Taints, corev1.Taint{
		Key:    OutOfServiceTaintKey,
		Value:  OutOfServiceTaintValue,
		Effect: corev1.TaintEffectNoExecute,
	})

	if _, err := client.CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{}); err != nil {
		return fmt.Errorf("applying out-of-service taint on node %s: %w", nodeName, err)
	}

	return nil
}

// WaitForVolumeAttachmentsDetached polls VolumeAttachments attached to nodeName until all of them
// have been detached or deleted. If timeout is exceeded and attachments remain attached,
// returns ERR_VOLUME_ATTACHMENT_LOCKED.
func (f *CSIFencingHelper) WaitForVolumeAttachmentsDetached(ctx context.Context, client kubernetes.Interface, nodeName string, timeout time.Duration) error {
	if client == nil {
		return fmt.Errorf("kubernetes client is nil")
	}
	if nodeName == "" {
		return fmt.Errorf("nodeName cannot be empty")
	}

	deadline := time.Now().Add(timeout)
	interval := f.pollInterval
	if interval <= 0 {
		interval = 50 * time.Millisecond
	}

	intervalTimer := time.NewTimer(interval)
	defer intervalTimer.Stop()

	for {
		vaList, err := client.StorageV1().VolumeAttachments().List(ctx, metav1.ListOptions{})
		if err != nil {
			return fmt.Errorf("listing volume attachments: %w", err)
		}

		attachedCount := 0
		var lastLockedVolume string

		for _, va := range vaList.Items {
			if va.Spec.NodeName == nodeName {
				// Attachment still present and active/attached
				if va.Status.Attached || va.DeletionTimestamp == nil {
					attachedCount++
					if va.Spec.Source.PersistentVolumeName != nil {
						lastLockedVolume = *va.Spec.Source.PersistentVolumeName
					} else {
						lastLockedVolume = va.Name
					}
				}
			}
		}

		if attachedCount == 0 {
			return nil
		}

		if time.Now().After(deadline) {
			return domainstorage.NewErrVolumeAttachmentLocked(lastLockedVolume, nodeName)
		}

		if !intervalTimer.Stop() {
			select {
			case <-intervalTimer.C:
			default:
			}
		}
		intervalTimer.Reset(interval)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-intervalTimer.C:
		}
	}
}

// FenceOfflineNode combines applying the out-of-service taint and waiting for VolumeAttachments to detach.
func (f *CSIFencingHelper) FenceOfflineNode(ctx context.Context, client kubernetes.Interface, nodeName string, timeout time.Duration) error {
	if err := f.ApplyOutOfServiceTaint(ctx, client, nodeName); err != nil {
		return err
	}
	return f.WaitForVolumeAttachmentsDetached(ctx, client, nodeName, timeout)
}
