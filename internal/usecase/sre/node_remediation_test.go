package sre

import (
	"context"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap/zaptest"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
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

func TestRemediationController_HandleNodeOffline_PendingPodsSpared(t *testing.T) {
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

	scheduledPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "scheduled-pod",
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			NodeName: "worker-1",
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
		},
	}

	pendingPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pending-pod",
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			NodeName: "", // awaiting scheduling
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodPending,
		},
	}

	client := fake.NewSimpleClientset(node, scheduledPod, pendingPod)
	logger := zaptest.NewLogger(t)
	ctrl := NewRemediationController(client, nil, logger)

	result, err := ctrl.HandleNodeOffline(context.Background(), "local", "worker-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify scheduled pod was evicted
	if len(result.EvictedPods) != 1 || result.EvictedPods[0] != "scheduled-pod" {
		t.Errorf("expected only scheduled-pod to be evicted, got %v", result.EvictedPods)
	}

	// Verify pending pod still exists and was NOT deleted
	_, err = client.CoreV1().Pods("default").Get(context.Background(), "pending-pod", metav1.GetOptions{})
	if err != nil {
		t.Errorf("expected pending-pod to be spared, but got error: %v", err)
	}
}

func TestRemediationController_PreflightScan_TopologyAndRCA(t *testing.T) {
	master := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "master-node", Labels: map[string]string{"node-role.kubernetes.io/control-plane": ""}},
		Spec:       corev1.NodeSpec{Taints: []corev1.Taint{{Key: "node-role.kubernetes.io/control-plane", Effect: corev1.TaintEffectNoSchedule}}},
	}
	worker := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "worker-dead"},
		Status:     corev1.NodeStatus{Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionFalse}, {Type: corev1.NodeMemoryPressure, Status: corev1.ConditionTrue}}},
	}
	statefulPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "db-pod", Namespace: "default"},
		Spec:       corev1.PodSpec{NodeName: "worker-dead", Volumes: []corev1.Volume{{Name: "data", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "data-pvc"}}}}},
	}
	statelessPod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "api-pod", Namespace: "default"}, Spec: corev1.PodSpec{NodeName: "worker-dead"}}
	pvName := "pv-vol-1"
	pv := &corev1.PersistentVolume{ObjectMeta: metav1.ObjectMeta{Name: pvName}, Spec: corev1.PersistentVolumeSpec{StorageClassName: "fast-ssd", ClaimRef: &corev1.ObjectReference{Name: "data-pvc"}}}
	va := &storagev1.VolumeAttachment{ObjectMeta: metav1.ObjectMeta{Name: "va-1"}, Spec: storagev1.VolumeAttachmentSpec{NodeName: "worker-dead", Source: storagev1.VolumeAttachmentSource{PersistentVolumeName: &pvName}}, Status: storagev1.VolumeAttachmentStatus{Attached: true}}

	ctrl := NewRemediationController(fake.NewSimpleClientset(master, worker, statefulPod, statelessPod, pv, va), nil, zaptest.NewLogger(t))
	preflight, err := ctrl.PreflightScan(context.Background(), "local", "worker-dead")
	if err != nil {
		t.Fatalf("unexpected PreflightScan error: %v", err)
	}

	if !preflight.IsSingleWorkerCluster || preflight.WorkerNodes != 1 || preflight.MasterNodes != 1 {
		t.Errorf("expected 1M+1W single worker topology, got master=%d worker=%d isSingle=%v", preflight.MasterNodes, preflight.WorkerNodes, preflight.IsSingleWorkerCluster)
	}
	if preflight.MasterNodeName != "master-node" || !preflight.MasterHasNoScheduleTaint {
		t.Errorf("expected master-node with NoSchedule taint, got name=%s taint=%v", preflight.MasterNodeName, preflight.MasterHasNoScheduleTaint)
	}
	if preflight.RCA.ProbableCause != "OOM_KILLER_CASCADE" || preflight.RCA.Confidence != 0.94 {
		t.Errorf("expected OOM_KILLER_CASCADE (0.94), got %+v", preflight.RCA)
	}
	if len(preflight.StatefulPods) != 1 || len(preflight.StatelessPods) != 1 || len(preflight.VolumeInventory) != 1 {
		t.Errorf("expected 1 stateful, 1 stateless, 1 volume, got %+v", preflight)
	}
}

func TestRemediationController_RCA_Heuristics(t *testing.T) {
	cases := []struct {
		name, wantCause string
		node            *corev1.Node
		leaseOk         bool
	}{
		{"DiskPressure", "DISK_IO_HANG", &corev1.Node{Status: corev1.NodeStatus{Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionFalse}, {Type: corev1.NodeDiskPressure, Status: corev1.ConditionTrue}}}}, true},
		{"PLEG deadlocked", "PLEG_CONTAINER_RUNTIME_DEADLOCK", &corev1.Node{Status: corev1.NodeStatus{Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionFalse, Reason: "PLEGNotHealthy"}}}}, true},
		{"Heartbeat lost", "HARDWARE_POWER_OR_KERNEL_PANIC", &corev1.Node{Status: corev1.NodeStatus{Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionUnknown, Reason: "NodeStatusUnknown"}}}}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if res := AnalyzeNodeRCA(tc.node, tc.leaseOk, nil); res.ProbableCause != tc.wantCause {
				t.Errorf("expected %s, got %s", tc.wantCause, res.ProbableCause)
			}
		})
	}
}

func TestRemediationController_ExecuteFencing_AntiSplitBrainBarrier(t *testing.T) {
	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "worker-1"}}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "stateful-app", Namespace: "default"},
		Spec:       corev1.PodSpec{NodeName: "worker-1", Volumes: []corev1.Volume{{Name: "vol", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "pvc-1"}}}}},
	}
	ctrl := NewRemediationController(fake.NewSimpleClientset(node, pod), nil, zaptest.NewLogger(t))
	if _, err := ctrl.ExecuteFencing(context.Background(), "local", "worker-1", FencingExecutionOptions{ConfirmPowerIsolated: false}); err != ErrPowerIsolationRequired {
		t.Fatalf("expected ErrPowerIsolationRequired, got %v", err)
	}
}

func TestRemediationController_ExecuteFencing_FullPipelineAndUntaint(t *testing.T) {
	master := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "master-1", Labels: map[string]string{"node-role.kubernetes.io/control-plane": ""}},
		Spec:       corev1.NodeSpec{Taints: []corev1.Taint{{Key: "node-role.kubernetes.io/control-plane", Effect: corev1.TaintEffectNoSchedule}}},
	}
	worker := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "worker-1"}}
	stateless := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "default"}, Spec: corev1.PodSpec{NodeName: "worker-1"}}
	stateful := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "db", Namespace: "default"},
		Spec:       corev1.PodSpec{NodeName: "worker-1", Volumes: []corev1.Volume{{Name: "data", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "data-pvc"}}}}},
	}
	client := fake.NewSimpleClientset(master, worker, stateless, stateful)
	ctrl := NewRemediationController(client, nil, zaptest.NewLogger(t))

	report, err := ctrl.ExecuteFencing(context.Background(), "local", "worker-1", FencingExecutionOptions{ConfirmPowerIsolated: true, EmergencyUntaintMaster: true})
	if err != nil {
		t.Fatalf("unexpected ExecuteFencing error: %v", err)
	}
	if !report.Success || !report.MasterUntainted || len(report.EvictedPods) != 2 {
		t.Errorf("unexpected report: %+v", report)
	}

	updMaster, _ := client.CoreV1().Nodes().Get(context.Background(), "master-1", metav1.GetOptions{})
	for _, taint := range updMaster.Spec.Taints {
		if taint.Key == "node-role.kubernetes.io/control-plane" && taint.Effect == corev1.TaintEffectNoSchedule {
			t.Errorf("control-plane taint still on master: %+v", taint)
		}
	}
	updWorker, _ := client.CoreV1().Nodes().Get(context.Background(), "worker-1", metav1.GetOptions{})
	if !updWorker.Spec.Unschedulable {
		t.Error("expected worker unschedulable=true")
	}
	hasTaint := false
	for _, taint := range updWorker.Spec.Taints {
		if taint.Key == "node.kubernetes.io/out-of-service" {
			hasTaint = true
		}
	}
	if !hasTaint {
		t.Error("expected out-of-service taint on worker")
	}
}


