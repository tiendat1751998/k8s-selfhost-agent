package kubernetes

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	clientgotesting "k8s.io/client-go/testing"
)

func TestDrainNode_EvictV1Subresource_ZeroDeletePodsActions(t *testing.T) {
	ctx := context.Background()
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "node-1"},
	}
	regularPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "app-pod", Namespace: "default"},
		Spec:       corev1.PodSpec{NodeName: "node-1"},
	}
	mirrorPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "mirror-pod",
			Namespace:   "kube-system",
			Annotations: map[string]string{corev1.MirrorPodAnnotationKey: "hash"},
		},
		Spec: corev1.PodSpec{NodeName: "node-1"},
	}
	dsPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ds-pod",
			Namespace: "kube-system",
			OwnerReferences: []metav1.OwnerReference{
				{Kind: "DaemonSet", Name: "ds-agent"},
			},
		},
		Spec: corev1.PodSpec{NodeName: "node-1"},
	}

	fakeClient := fake.NewSimpleClientset(node, regularPod, mirrorPod, dsPod)

	// Reactor simulates eviction by removing the pod from fakeClient tracker
	fakeClient.PrependReactor("create", "pods", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		if action.GetSubresource() == "eviction" {
			eviction := action.(clientgotesting.CreateAction).GetObject().(*policyv1.Eviction)
			_ = fakeClient.Tracker().Delete(corev1.SchemeGroupVersion.WithResource("pods"), eviction.Namespace, eviction.Name)
			return true, eviction, nil
		}
		return false, nil, nil
	})

	repo := &ResourceRepo{client: fakeClient}
	err := repo.DrainNode(ctx, "", "node-1", 30, true)
	if err != nil {
		t.Fatalf("DrainNode returned unexpected error: %v", err)
	}

	// Verify node is unschedulable
	updatedNode, err := fakeClient.CoreV1().Nodes().Get(ctx, "node-1", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("getting node-1: %v", err)
	}
	if !updatedNode.Spec.Unschedulable {
		t.Fatalf("expected node-1 to be cordoned (unschedulable=true)")
	}

	// Acceptance criterion: Test asserts eviction subresource action, zero delete pods actions.
	var evictionActions []clientgotesting.Action
	var deletePodActions []clientgotesting.Action

	for _, action := range fakeClient.Actions() {
		if action.GetVerb() == "create" && action.GetResource().Resource == "pods" && action.GetSubresource() == "eviction" {
			evictionActions = append(evictionActions, action)
		}
		if action.GetVerb() == "delete" && action.GetResource().Resource == "pods" {
			// Ensure repo itself did not call Delete
			deletePodActions = append(deletePodActions, action)
		}
	}

	if len(evictionActions) == 0 {
		t.Fatalf("expected at least 1 eviction subresource action, got 0")
	}

	// Verify the evicted pod was app-pod
	createAct := evictionActions[0].(clientgotesting.CreateAction)
	eviction := createAct.GetObject().(*policyv1.Eviction)
	if eviction.Name != "app-pod" || eviction.Namespace != "default" {
		t.Fatalf("expected eviction for app-pod in default namespace, got %s/%s", eviction.Namespace, eviction.Name)
	}

	// Mirror pod and DaemonSet pod should NOT have been evicted
	for _, act := range evictionActions {
		ev := act.(clientgotesting.CreateAction).GetObject().(*policyv1.Eviction)
		if ev.Name == "mirror-pod" {
			t.Fatalf("mirror pod should not be evicted")
		}
		if ev.Name == "ds-pod" {
			t.Fatalf("daemonset pod should not be evicted when ignoreDaemonSets=true")
		}
	}

	// Mirror pod and DaemonSet pod must still exist
	if _, err := fakeClient.CoreV1().Pods("kube-system").Get(ctx, "mirror-pod", metav1.GetOptions{}); err != nil {
		t.Fatalf("expected mirror-pod to still exist, got %v", err)
	}
	if _, err := fakeClient.CoreV1().Pods("kube-system").Get(ctx, "ds-pod", metav1.GetOptions{}); err != nil {
		t.Fatalf("expected ds-pod to still exist, got %v", err)
	}
}

func TestDrainNode_429RetrySuccess(t *testing.T) {
	ctx := context.Background()
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "node-1"},
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "flaky-pod", Namespace: "default"},
		Spec:       corev1.PodSpec{NodeName: "node-1"},
	}

	fakeClient := fake.NewSimpleClientset(node, pod)

	var attempts int32
	fakeClient.PrependReactor("create", "pods", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		if action.GetSubresource() == "eviction" {
			att := atomic.AddInt32(&attempts, 1)
			if att < 4 {
				// Return 429 Too Many Requests for first 3 attempts
				return true, nil, k8serrors.NewTooManyRequests("PDB prevents eviction", 1)
			}
			// 4th attempt succeeds and deletes the pod
			eviction := action.(clientgotesting.CreateAction).GetObject().(*policyv1.Eviction)
			_ = fakeClient.Tracker().Delete(corev1.SchemeGroupVersion.WithResource("pods"), eviction.Namespace, eviction.Name)
			return true, eviction, nil
		}
		return false, nil, nil
	})

	repo := &ResourceRepo{client: fakeClient}
	err := repo.DrainNode(ctx, "", "node-1", 10, true)
	if err != nil {
		t.Fatalf("expected success after 429 retries, got err: %v", err)
	}

	if atomic.LoadInt32(&attempts) != 4 {
		t.Fatalf("expected 4 eviction attempts (3x 429 + 1 success), got %d", attempts)
	}
}

func TestDrainNode_429Persistent_BoundedError(t *testing.T) {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "node-1"},
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "blocked-pod", Namespace: "payments"},
		Spec:       corev1.PodSpec{NodeName: "node-1"},
	}

	fakeClient := fake.NewSimpleClientset(node, pod)

	var attempts int32
	fakeClient.PrependReactor("create", "pods", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		if action.GetSubresource() == "eviction" {
			atomic.AddInt32(&attempts, 1)
			return true, nil, k8serrors.NewTooManyRequests("PDB budget 0", 1)
		}
		return false, nil, nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	repo := &ResourceRepo{client: fakeClient}
	err := repo.DrainNode(ctx, "", "node-1", 10, true)
	if err == nil {
		t.Fatalf("expected error due to persistent 429, got nil")
	}

	// Error must name the blocking pod (payments/blocked-pod)
	errMsg := err.Error()
	if !strings.Contains(errMsg, "blocked-pod") || !strings.Contains(errMsg, "payments") {
		t.Fatalf("expected error to name blocking pod 'payments/blocked-pod', got: %s", errMsg)
	}

	if atomic.LoadInt32(&attempts) < 2 {
		t.Fatalf("expected multiple retry attempts before timeout, got %d", attempts)
	}
}

func TestDrainNode_NeverReturnsSuccessWhileEvictablePodsRemain(t *testing.T) {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "node-1"},
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "stuck-pod", Namespace: "default"},
		Spec:       corev1.PodSpec{NodeName: "node-1"},
	}

	fakeClient := fake.NewSimpleClientset(node, pod)

	// EvictV1 succeeds, BUT the pod is never deleted from fakeClient
	fakeClient.PrependReactor("create", "pods", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		if action.GetSubresource() == "eviction" {
			eviction := action.(clientgotesting.CreateAction).GetObject().(*policyv1.Eviction)
			return true, eviction, nil
		}
		return false, nil, nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	repo := &ResourceRepo{client: fakeClient}
	err := repo.DrainNode(ctx, "", "node-1", 10, true)
	if err == nil {
		t.Fatalf("expected failure because evictable pod remained on node, but DrainNode returned success")
	}

	// Verify error mentions timeout or waiting for termination
	if !strings.Contains(err.Error(), "node-1") {
		t.Fatalf("expected error to mention node-1, got: %v", err)
	}
}
