package sre

import (
	"context"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap/zaptest"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

func TestRemediationController_HandleNodeOffline_CrashingNode(t *testing.T) {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "worker-1",
		},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{
				{
					Type:   corev1.NodeReady,
					Status: corev1.ConditionFalse,
				},
			},
		},
	}

	pod1 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "app-1",
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			NodeName: "worker-1",
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
		},
	}

	pod2 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "app-2",
			Namespace: "production",
		},
		Spec: corev1.PodSpec{
			NodeName: "worker-1",
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
		},
	}

	podOther := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "app-other",
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			NodeName: "worker-2",
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
		},
	}

	client := fake.NewSimpleClientset(node, pod1, pod2, podOther)
	logger := zaptest.NewLogger(t)
	ctrl := NewRemediationController(client, nil, logger)

	result, err := ctrl.HandleNodeOffline(context.Background(), "local", "worker-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.NodeName != "worker-1" {
		t.Errorf("expected node worker-1, got %s", result.NodeName)
	}
	if len(result.EvictedPods) != 2 {
		t.Errorf("expected 2 evicted pods, got %d (%v)", len(result.EvictedPods), result.EvictedPods)
	}
	if result.Timestamp == "" {
		t.Error("expected non-empty timestamp")
	}

	// Verify node is cordoned
	updatedNode, err := client.CoreV1().Nodes().Get(context.Background(), "worker-1", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to fetch updated node: %v", err)
	}
	if !updatedNode.Spec.Unschedulable {
		t.Error("expected node spec.unschedulable to be true")
	}

	// Verify worker-2 pod was untouched
	_, err = client.CoreV1().Pods("default").Get(context.Background(), "app-other", metav1.GetOptions{})
	if err != nil {
		t.Errorf("expected app-other to still exist: %v", err)
	}
}

func TestRemediationController_HandleNodeOffline_TerminatingPod(t *testing.T) {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "worker-ready",
		},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{
				{
					Type:   corev1.NodeReady,
					Status: corev1.ConditionTrue,
				},
			},
		},
	}

	now := metav1.NewTime(time.Now())
	stuckPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "stuck-pod",
			Namespace:         "default",
			DeletionTimestamp: &now,
		},
		Spec: corev1.PodSpec{
			NodeName: "worker-ready",
		},
	}

	healthyPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "healthy-pod",
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			NodeName: "worker-ready",
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
		},
	}

	client := fake.NewSimpleClientset(node, stuckPod, healthyPod)
	logger := zaptest.NewLogger(t)
	ctrl := NewRemediationController(client, nil, logger)

	result, err := ctrl.HandleNodeOffline(context.Background(), "local", "worker-ready")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.EvictedPods) != 1 || result.EvictedPods[0] != "stuck-pod" {
		t.Errorf("expected only stuck-pod evicted, got %v", result.EvictedPods)
	}

	// Healthy pod should still exist
	_, err = client.CoreV1().Pods("default").Get(context.Background(), "healthy-pod", metav1.GetOptions{})
	if err != nil {
		t.Errorf("expected healthy-pod to still exist: %v", err)
	}
}

func TestRemediationController_HandleNodeOffline_MirrorAndCompletedPods(t *testing.T) {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "worker-broken",
		},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{
				{
					Type:   corev1.NodeReady,
					Status: corev1.ConditionFalse,
				},
			},
		},
	}

	mirrorPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kube-proxy",
			Namespace: "kube-system",
			Annotations: map[string]string{
				corev1.MirrorPodAnnotationKey: "mirror-hash",
			},
		},
		Spec: corev1.PodSpec{
			NodeName: "worker-broken",
		},
	}

	completedPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "job-done",
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			NodeName: "worker-broken",
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodSucceeded,
		},
	}

	client := fake.NewSimpleClientset(node, mirrorPod, completedPod)
	logger := zaptest.NewLogger(t)
	ctrl := NewRemediationController(client, nil, logger)

	result, err := ctrl.HandleNodeOffline(context.Background(), "local", "worker-broken")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.EvictedPods) != 0 {
		t.Errorf("expected 0 evicted pods (mirror & completed skipped), got %v", result.EvictedPods)
	}
}

func TestRemediationController_HandleNodeOffline_Errors(t *testing.T) {
	client := fake.NewSimpleClientset()
	logger := zaptest.NewLogger(t)
	ctrl := NewRemediationController(client, nil, logger)

	// Empty node name
	_, err := ctrl.HandleNodeOffline(context.Background(), "local", "")
	if err == nil {
		t.Error("expected error for empty node name")
	}

	// Non-existent node
	_, err = ctrl.HandleNodeOffline(context.Background(), "local", "ghost-node")
	if err == nil {
		t.Error("expected error for non-existent node")
	}
}
func TestRemediationController_NilClientHardening(t *testing.T) {
	logger := zaptest.NewLogger(t)

	t.Run("untyped nil client", func(t *testing.T) {
		ctrl := NewRemediationController(nil, nil, logger)
		_, err := ctrl.HandleNodeOffline(context.Background(), "local", "worker-1")
		if err == nil {
			t.Fatal("expected error with uninitialized client")
		}
		if !strings.Contains(err.Error(), "no active cluster connection") {
			t.Errorf("expected 'no active cluster connection' in error, got: %v", err)
		}
	})

	t.Run("typed nil clientset", func(t *testing.T) {
		var cs *kubernetes.Clientset = nil
		ctrl := NewRemediationController(cs, nil, logger)
		_, err := ctrl.HandleNodeOffline(context.Background(), "default", "worker-1")
		if err == nil {
			t.Fatal("expected error with typed nil clientset")
		}
		if !strings.Contains(err.Error(), "no active cluster connection") {
			t.Errorf("expected 'no active cluster connection' in error, got: %v", err)
		}
	})

	t.Run("empty clusterID with nil client", func(t *testing.T) {
		ctrl := NewRemediationController(nil, nil, logger)
		_, err := ctrl.HandleNodeOffline(context.Background(), "", "worker-1")
		if err == nil {
			t.Fatal("expected error with uninitialized client")
		}
		if !strings.Contains(err.Error(), "no active cluster connection") {
			t.Errorf("expected 'no active cluster connection' in error, got: %v", err)
		}
	})

	t.Run("custom clusterID without client", func(t *testing.T) {
		ctrl := NewRemediationController(nil, nil, logger)
		_, err := ctrl.HandleNodeOffline(context.Background(), "cluster-remote-42", "worker-1")
		if err == nil {
			t.Fatal("expected error with uninitialized client")
		}
		if !strings.Contains(err.Error(), "no active cluster connection") {
			t.Errorf("expected 'no active cluster connection' in error, got: %v", err)
		}
	})
}
