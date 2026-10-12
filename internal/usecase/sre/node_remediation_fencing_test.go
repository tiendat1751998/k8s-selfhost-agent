package sre

import (
	"context"
	"errors"
	"testing"

	"go.uber.org/zap/zaptest"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	clientgotesting "k8s.io/client-go/testing"
)

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

func TestRemediationController_ExecuteFencing_EmergencyUntaintMaster_ErrorLogged(t *testing.T) {
	master := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "master-1", Labels: map[string]string{"node-role.kubernetes.io/control-plane": ""}},
		Spec:       corev1.NodeSpec{Taints: []corev1.Taint{{Key: "node-role.kubernetes.io/control-plane", Effect: corev1.TaintEffectNoSchedule}}},
	}
	worker := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "worker-1"}}
	client := fake.NewSimpleClientset(master, worker)

	// Simulate failure when updating master node
	client.PrependReactor("update", "nodes", func(action clientgotesting.Action) (bool, runtime.Object, error) {
		updateAction := action.(clientgotesting.UpdateAction)
		n := updateAction.GetObject().(*corev1.Node)
		if n.Name == "master-1" {
			return true, nil, errors.New("simulated update conflict on master")
		}
		return false, nil, nil
	})

	ctrl := NewRemediationController(client, nil, zaptest.NewLogger(t))
	report, err := ctrl.ExecuteFencing(context.Background(), "local", "worker-1", FencingExecutionOptions{
		ConfirmPowerIsolated:   true,
		EmergencyUntaintMaster: true,
	})
	if err != nil {
		t.Fatalf("unexpected ExecuteFencing error: %v", err)
	}

	if report.MasterUntainted {
		t.Errorf("expected report.MasterUntainted to be false when node update fails, got true")
	}
}

func TestRemediationController_ExecuteFencing_ForceVolumeDetachRespected(t *testing.T) {
	pvName := "pv-data-1"
	va := &storagev1.VolumeAttachment{
		ObjectMeta: metav1.ObjectMeta{Name: "va-locked-1"},
		Spec: storagev1.VolumeAttachmentSpec{
			NodeName: "worker-1",
			Source:   storagev1.VolumeAttachmentSource{PersistentVolumeName: &pvName},
		},
		Status: storagev1.VolumeAttachmentStatus{Attached: true},
	}

	t.Run("ForceVolumeDetach false does not delete locked VolumeAttachment", func(t *testing.T) {
		worker := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "worker-1"}}
		client := fake.NewSimpleClientset(worker, va.DeepCopy())
		ctrl := NewRemediationController(client, nil, zaptest.NewLogger(t))

		_, err := ctrl.ExecuteFencing(context.Background(), "local", "worker-1", FencingExecutionOptions{
			ConfirmPowerIsolated:   true,
			ForceVolumeDetach:      false,
			VolumeDetachTimeoutSec: 1,
		})
		if err != nil {
			t.Fatalf("unexpected ExecuteFencing error: %v", err)
		}

		// Volume attachment should still exist
		stillVA, getErr := client.StorageV1().VolumeAttachments().Get(context.Background(), "va-locked-1", metav1.GetOptions{})
		if getErr != nil || stillVA == nil {
			t.Errorf("expected VolumeAttachment va-locked-1 to NOT be deleted when ForceVolumeDetach=false, got err=%v", getErr)
		}
	})

	t.Run("ForceVolumeDetach true force deletes locked VolumeAttachment", func(t *testing.T) {
		worker := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "worker-1"}}
		client := fake.NewSimpleClientset(worker, va.DeepCopy())
		ctrl := NewRemediationController(client, nil, zaptest.NewLogger(t))

		_, err := ctrl.ExecuteFencing(context.Background(), "local", "worker-1", FencingExecutionOptions{
			ConfirmPowerIsolated:   true,
			ForceVolumeDetach:      true,
			VolumeDetachTimeoutSec: 1,
		})
		if err != nil {
			t.Fatalf("unexpected ExecuteFencing error: %v", err)
		}

		// Volume attachment should be force deleted
		_, getErr := client.StorageV1().VolumeAttachments().Get(context.Background(), "va-locked-1", metav1.GetOptions{})
		if getErr == nil {
			t.Errorf("expected VolumeAttachment va-locked-1 to be deleted when ForceVolumeDetach=true, but it still exists")
		}
	})
}
