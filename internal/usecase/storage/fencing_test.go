package storage

import (
	"context"
	"errors"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	domainstorage "github.com/datdt/k8sselfhost/internal/domain/storage"
	domainerrors "github.com/datdt/k8sselfhost/internal/pkg/errors"
)

func TestNodeFencing_ApplyOutOfServiceTaint(t *testing.T) {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "worker-offline-1",
		},
		Spec: corev1.NodeSpec{},
	}

	fakeClient := fake.NewSimpleClientset(node)
	helper := NewCSIFencingHelper()

	err := helper.ApplyOutOfServiceTaint(context.Background(), fakeClient, "worker-offline-1")
	if err != nil {
		t.Fatalf("ApplyOutOfServiceTaint failed: %v", err)
	}

	updated, err := fakeClient.CoreV1().Nodes().Get(context.Background(), "worker-offline-1", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to get updated node: %v", err)
	}

	found := false
	for _, taint := range updated.Spec.Taints {
		if taint.Key == OutOfServiceTaintKey && taint.Value == OutOfServiceTaintValue && taint.Effect == corev1.TaintEffectNoExecute {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected taint %s=%s:%s on node", OutOfServiceTaintKey, OutOfServiceTaintValue, corev1.TaintEffectNoExecute)
	}

	// Calling again should be idempotent (no duplicate taints)
	err = helper.ApplyOutOfServiceTaint(context.Background(), fakeClient, "worker-offline-1")
	if err != nil {
		t.Fatalf("second call to ApplyOutOfServiceTaint failed: %v", err)
	}
	updatedAgain, _ := fakeClient.CoreV1().Nodes().Get(context.Background(), "worker-offline-1", metav1.GetOptions{})
	if len(updatedAgain.Spec.Taints) != 1 {
		t.Errorf("expected exactly 1 taint, got %d", len(updatedAgain.Spec.Taints))
	}
}

func TestNodeFencing_VolumeAttachmentWait(t *testing.T) {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "worker-fenced-1",
		},
	}

	pvName := "pv-mysql-data"
	va := &storagev1.VolumeAttachment{
		ObjectMeta: metav1.ObjectMeta{
			Name: "va-mysql-attachment",
		},
		Spec: storagev1.VolumeAttachmentSpec{
			NodeName: "worker-fenced-1",
			Source: storagev1.VolumeAttachmentSource{
				PersistentVolumeName: &pvName,
			},
		},
		Status: storagev1.VolumeAttachmentStatus{
			Attached: true,
		},
	}

	fakeClient := fake.NewSimpleClientset(node, va)
	helper := NewCSIFencingHelper()
	helper.SetPollInterval(10 * time.Millisecond)

	// Simulate background detachment after 40ms
	go func() {
		time.Sleep(40 * time.Millisecond)
		_ = fakeClient.StorageV1().VolumeAttachments().Delete(context.Background(), "va-mysql-attachment", metav1.DeleteOptions{})
	}()

	err := helper.WaitForVolumeAttachmentsDetached(context.Background(), fakeClient, "worker-fenced-1", 500*time.Millisecond)
	if err != nil {
		t.Fatalf("expected detachment wait to succeed, got: %v", err)
	}
}

func TestNodeFencing_VolumeAttachmentTimeout_Locked(t *testing.T) {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "worker-locked-1",
		},
	}

	pvName := "pv-redis-data"
	va := &storagev1.VolumeAttachment{
		ObjectMeta: metav1.ObjectMeta{
			Name: "va-redis-attachment",
		},
		Spec: storagev1.VolumeAttachmentSpec{
			NodeName: "worker-locked-1",
			Source: storagev1.VolumeAttachmentSource{
				PersistentVolumeName: &pvName,
			},
		},
		Status: storagev1.VolumeAttachmentStatus{
			Attached: true,
		},
	}

	fakeClient := fake.NewSimpleClientset(node, va)
	helper := NewCSIFencingHelper()
	helper.SetPollInterval(10 * time.Millisecond)

	// Short timeout with volume remaining attached -> must return ERR_VOLUME_ATTACHMENT_LOCKED
	err := helper.WaitForVolumeAttachmentsDetached(context.Background(), fakeClient, "worker-locked-1", 30*time.Millisecond)
	if err == nil {
		t.Fatal("expected timeout error for locked volume attachment")
	}

	var domErr *domainerrors.DomainError
	if !errors.As(err, &domErr) {
		t.Fatalf("expected domain error, got %v", err)
	}
	if domErr.Code != domainstorage.ErrCodeVolumeAttachmentLocked {
		t.Errorf("expected error code %s, got %s", domainstorage.ErrCodeVolumeAttachmentLocked, domErr.Code)
	}
}

func TestNodeFencing_FenceOfflineNode_EndToEnd(t *testing.T) {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "worker-dead",
		},
	}

	fakeClient := fake.NewSimpleClientset(node)
	helper := NewCSIFencingHelper()
	helper.SetPollInterval(10 * time.Millisecond)

	err := helper.FenceOfflineNode(context.Background(), fakeClient, "worker-dead", 100*time.Millisecond)
	if err != nil {
		t.Fatalf("FenceOfflineNode failed on clean node: %v", err)
	}

	updated, err := fakeClient.CoreV1().Nodes().Get(context.Background(), "worker-dead", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to fetch updated node: %v", err)
	}
	if len(updated.Spec.Taints) != 1 || updated.Spec.Taints[0].Key != OutOfServiceTaintKey {
		t.Errorf("expected out-of-service taint on node, got %+v", updated.Spec.Taints)
	}
}
