package sre_test

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/datdt/k8sselfhost/internal/usecase/sre"
)

func TestEvaluateMasterAdmission_TierPriorityAndShedding(t *testing.T) {
	// Master node with 4000m CPU and 8Gi RAM allocatable
	masterNode := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "k8s-master-01",
			Labels: map[string]string{
				"node-role.kubernetes.io/control-plane": "",
			},
		},
		Status: corev1.NodeStatus{
			Allocatable: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("4000m"),
				corev1.ResourceMemory: resource.MustParse("8Gi"),
			},
		},
	}

	// Existing pod on master using 1000m CPU and 2Gi RAM
	existingMasterPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "etcd-master", Namespace: "kube-system"},
		Spec: corev1.PodSpec{
			NodeName: "k8s-master-01",
			Containers: []corev1.Container{
				{
					Name: "etcd",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("1000m"),
							corev1.ResourceMemory: resource.MustParse("2Gi"),
						},
					},
				},
			},
		},
	}

	// Available headroom on master: 3000m CPU, 6Gi RAM.
	// Worker pods to migrate:
	// 1. Tier 0: stateful pod with PVC (Postgres: 1000m, 2Gi)
	pvcPod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "postgres-0", Namespace: "prod"},
		Spec: corev1.PodSpec{
			NodeName: "k8s-worker-01",
			Volumes: []corev1.Volume{
				{
					Name: "data",
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "pgdata"},
					},
				},
			},
			Containers: []corev1.Container{
				{
					Name: "postgres",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("1000m"),
							corev1.ResourceMemory: resource.MustParse("2Gi"),
						},
					},
				},
			},
		},
	}

	// 2. Tier 0: ingress controller (nginx-ingress: 500m, 1Gi)
	ingressPod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "ingress-nginx-controller", Namespace: "ingress"},
		Spec: corev1.PodSpec{
			NodeName: "k8s-worker-01",
			Containers: []corev1.Container{
				{
					Name: "controller",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("500m"),
							corev1.ResourceMemory: resource.MustParse("1Gi"),
						},
					},
				},
			},
		},
	}

	// 3. Tier 1: core api service (api-backend: 1000m, 2Gi)
	apiPod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "api-backend-78b94", Namespace: "prod"},
		Spec: corev1.PodSpec{
			NodeName: "k8s-worker-01",
			Containers: []corev1.Container{
				{
					Name: "api",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("1000m"),
							corev1.ResourceMemory: resource.MustParse("2Gi"),
						},
					},
				},
			},
		},
	}

	// 4. Tier 2: background worker (worker-async: 1000m, 2Gi) -> Exceeds remaining 500m CPU / 1Gi RAM!
	workerPod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "worker-async-job", Namespace: "prod"},
		Spec: corev1.PodSpec{
			NodeName: "k8s-worker-01",
			Containers: []corev1.Container{
				{
					Name: "worker",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("1000m"),
							corev1.ResourceMemory: resource.MustParse("2Gi"),
						},
					},
				},
			},
		},
	}

	// 5. Tier 3: observability collector (promtail: 200m, 500Mi) -> Even lower priority
	promtailPod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "promtail-collector", Namespace: "monitoring"},
		Spec: corev1.PodSpec{
			NodeName: "k8s-worker-01",
			Containers: []corev1.Container{
				{
					Name: "promtail",
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("200m"),
							corev1.ResourceMemory: resource.MustParse("500Mi"),
						},
					},
				},
			},
		},
	}

	client := fake.NewSimpleClientset(masterNode, existingMasterPod)

	workerPods := []corev1.Pod{promtailPod, workerPod, apiPod, ingressPod, pvcPod} // unsorted order

	admissionEngine := sre.NewCapacityAdmissionEngine()
	plan, err := admissionEngine.EvaluateMasterAdmission(context.Background(), client, "k8s-master-01", workerPods)
	if err != nil {
		t.Fatalf("unexpected error evaluating admission: %v", err)
	}

	if plan == nil {
		t.Fatal("expected non-nil admission plan")
	}

	// Tier 0 (postgres-0, ingress) and Tier 1 (api-backend) should be admitted:
	// Headroom was 3000m CPU and 6Gi RAM.
	// postgres: 1000m, 2Gi (headroom left: 2000m, 4Gi)
	// ingress: 500m, 1Gi (headroom left: 1500m, 3Gi)
	// api-backend: 1000m, 2Gi (headroom left: 500m, 1Gi)
	// Total admitted: 3 pods
	if plan.TotalAdmitted != 3 {
		t.Errorf("expected 3 admitted pods, got %d", plan.TotalAdmitted)
	}

	// worker-async (1000m > 500m) and promtail (fit cpu maybe, but worker-async had priority or promtail shed)
	// Check shedded pods
	if plan.TotalShedded != 2 {
		t.Errorf("expected 2 shedded pods, got %d", plan.TotalShedded)
	}

	for _, shed := range plan.SheddedPods {
		if shed.Reason != sre.ReasonMasterOOMPrevention {
			t.Errorf("expected shed reason %s, got %s", sre.ReasonMasterOOMPrevention, shed.Reason)
		}
	}
}
