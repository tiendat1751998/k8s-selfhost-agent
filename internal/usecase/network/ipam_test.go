package network_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap/zaptest"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"

	domainnet "github.com/datdt/k8sselfhost/internal/domain/network"
	usecasenet "github.com/datdt/k8sselfhost/internal/usecase/network"
)

func TestIPAM_DetectOrphanedLeases(t *testing.T) {
	ctx := context.Background()
	logger := zaptest.NewLogger(t)

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "worker-node-1",
		},
		Spec: corev1.NodeSpec{
			PodCIDR: "10.244.1.0/24", // 254 allocatable
		},
	}

	clientObjects := []runtime.Object{node}
	var allLeases []domainnet.IPAMLease

	// 120 active running pods
	for i := 1; i <= 120; i++ {
		podIP := fmt.Sprintf("10.244.1.%d", i)
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("active-pod-%d", i),
				Namespace: "default",
			},
			Spec: corev1.PodSpec{
				NodeName: "worker-node-1",
			},
			Status: corev1.PodStatus{
				Phase: corev1.PodRunning,
				PodIP: podIP,
			},
		}
		clientObjects = append(clientObjects, pod)

		allLeases = append(allLeases, domainnet.IPAMLease{
			IP:        podIP,
			NodeName:  "worker-node-1",
			PodName:   pod.Name,
			Namespace: pod.Namespace,
			CreatedAt: time.Now().UTC(),
		})
	}

	// 80 leaked orphaned IP handles (e.g. from previously deleted jobs or Calico CRD leaks)
	for i := 121; i <= 200; i++ {
		orphanIP := fmt.Sprintf("10.244.1.%d", i)
		allLeases = append(allLeases, domainnet.IPAMLease{
			IP:        orphanIP,
			NodeName:  "worker-node-1",
			PodName:   fmt.Sprintf("deleted-batch-job-%d", i),
			Namespace: "default",
			CreatedAt: time.Now().UTC().Add(-2 * time.Hour),
		})
	}

	k8sClient := fake.NewSimpleClientset(clientObjects...)

	backend := usecasenet.NewInMemoryIPAMBackend()
	if err := backend.RegisterLeases(ctx, "cluster-1", allLeases); err != nil {
		t.Fatalf("Failed to register leases: %v", err)
	}

	usecase := usecasenet.NewNetworkUsecase(k8sClient, nil, backend, nil, logger)

	utils, err := usecase.GetIPAMUtilization(ctx, "cluster-1")
	if err != nil {
		t.Fatalf("GetIPAMUtilization returned error: %v", err)
	}

	if len(utils) != 1 {
		t.Fatalf("Expected 1 node utilization report, got %d", len(utils))
	}

	u := utils[0]
	if u.NodeName != "worker-node-1" {
		t.Errorf("NodeName = %q, want %q", u.NodeName, "worker-node-1")
	}
	if u.AllocatedIPs != 200 {
		t.Errorf("AllocatedIPs = %d, want 200", u.AllocatedIPs)
	}
	if u.ActivePodCount != 120 {
		t.Errorf("ActivePodCount = %d, want 120", u.ActivePodCount)
	}
	if u.LeakedOrphanIPs != 80 {
		t.Errorf("LeakedOrphanIPs = %d, want 80", u.LeakedOrphanIPs)
	}
	if u.Status != domainnet.StatusWarning {
		t.Errorf("Status = %q, want %q", u.Status, domainnet.StatusWarning)
	}
}

func TestIPAM_GarbageCollection_ReclaimsIPs(t *testing.T) {
	ctx := context.Background()
	logger := zaptest.NewLogger(t)

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "worker-node-1",
		},
		Spec: corev1.NodeSpec{
			PodCIDR: "10.244.1.0/24",
		},
	}

	clientObjects := []runtime.Object{node}
	var allLeases []domainnet.IPAMLease

	// 120 Running pods
	for i := 1; i <= 120; i++ {
		podIP := fmt.Sprintf("10.244.1.%d", i)
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("active-pod-%d", i),
				Namespace: "production",
			},
			Spec: corev1.PodSpec{
				NodeName: "worker-node-1",
			},
			Status: corev1.PodStatus{
				Phase: corev1.PodRunning,
				PodIP: podIP,
			},
		}
		clientObjects = append(clientObjects, pod)
		allLeases = append(allLeases, domainnet.IPAMLease{
			IP:        podIP,
			NodeName:  "worker-node-1",
			PodName:   pod.Name,
			Namespace: pod.Namespace,
			CreatedAt: time.Now().UTC(),
		})
	}

	// 1 Pod in ContainerCreating state (Guardrail: Must NOT be garbage collected)
	creatingPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "creating-pod-121",
			Namespace: "production",
		},
		Spec: corev1.PodSpec{
			NodeName: "worker-node-1",
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodPending,
			PodIP: "10.244.1.121",
			ContainerStatuses: []corev1.ContainerStatus{
				{
					State: corev1.ContainerState{
						Waiting: &corev1.ContainerStateWaiting{
							Reason: "ContainerCreating",
						},
					},
				},
			},
		},
	}
	clientObjects = append(clientObjects, creatingPod)
	allLeases = append(allLeases, domainnet.IPAMLease{
		IP:        "10.244.1.121",
		NodeName:  "worker-node-1",
		PodName:   creatingPod.Name,
		Namespace: creatingPod.Namespace,
		CreatedAt: time.Now().UTC(),
	})

	// 80 Orphaned IP leases from dead/unreferenced pods
	for i := 122; i <= 201; i++ {
		orphanIP := fmt.Sprintf("10.244.1.%d", i)
		allLeases = append(allLeases, domainnet.IPAMLease{
			IP:        orphanIP,
			NodeName:  "worker-node-1",
			PodName:   fmt.Sprintf("orphaned-pod-%d", i),
			Namespace: "production",
			CreatedAt: time.Now().UTC().Add(-3 * time.Hour),
		})
	}

	k8sClient := fake.NewSimpleClientset(clientObjects...)
	backend := usecasenet.NewInMemoryIPAMBackend()
	if err := backend.RegisterLeases(ctx, "cluster-1", allLeases); err != nil {
		t.Fatalf("Failed to register leases: %v", err)
	}

	usecase := usecasenet.NewNetworkUsecase(k8sClient, nil, backend, nil, logger)

	// Step 1: Pre-GC Check - should have 80 orphans (total 201 leases, 121 active pods)
	utilsBefore, err := usecase.GetIPAMUtilization(ctx, "cluster-1")
	if err != nil {
		t.Fatalf("GetIPAMUtilization failed: %v", err)
	}
	if utilsBefore[0].LeakedOrphanIPs != 80 {
		t.Fatalf("LeakedOrphanIPs before GC = %d, want 80", utilsBefore[0].LeakedOrphanIPs)
	}

	// Step 2: Trigger Garbage Collection
	reclaimed, err := usecase.TriggerIPAMGarbageCollection(ctx, "cluster-1", "worker-node-1")
	if err != nil {
		t.Fatalf("TriggerIPAMGarbageCollection failed: %v", err)
	}
	if reclaimed != 80 {
		t.Errorf("Reclaimed IPs = %d, want 80", reclaimed)
	}

	// Step 3: Post-GC Check - LeakedOrphanIPs should drop to 0 and status should become HEALTHY
	utilsAfter, err := usecase.GetIPAMUtilization(ctx, "cluster-1")
	if err != nil {
		t.Fatalf("GetIPAMUtilization after GC failed: %v", err)
	}

	uAfter := utilsAfter[0]
	if uAfter.LeakedOrphanIPs != 0 {
		t.Errorf("LeakedOrphanIPs after GC = %d, want 0", uAfter.LeakedOrphanIPs)
	}
	if uAfter.AllocatedIPs != 121 {
		t.Errorf("AllocatedIPs after GC = %d, want 121 (120 running + 1 creating)", uAfter.AllocatedIPs)
	}
	if uAfter.Status != domainnet.StatusHealthy {
		t.Errorf("Status after GC = %q, want %q", uAfter.Status, domainnet.StatusHealthy)
	}

	// Verify the creating pod's lease was NOT deleted
	remainingLeases, _ := backend.GetAllocatedLeases(ctx, "cluster-1", "worker-node-1")
	foundCreatingPodLease := false
	for _, l := range remainingLeases {
		if l.IP == "10.244.1.121" {
			foundCreatingPodLease = true
			break
		}
	}
	if !foundCreatingPodLease {
		t.Errorf("Guardrail violation: ContainerCreating pod IP lease was reclaimed!")
	}
}

func TestIPAM_ConcurrencySafety(t *testing.T) {
	ctx := context.Background()
	logger := zaptest.NewLogger(t)

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "worker-node-concurrent"},
		Spec:       corev1.NodeSpec{PodCIDR: "10.244.2.0/24"},
	}

	k8sClient := fake.NewSimpleClientset(node)
	backend := usecasenet.NewInMemoryIPAMBackend()
	usecase := usecasenet.NewNetworkUsecase(k8sClient, nil, backend, nil, logger)

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			lease := domainnet.IPAMLease{
				IP:        fmt.Sprintf("10.244.2.%d", workerID+10),
				NodeName:  "worker-node-concurrent",
				PodName:   fmt.Sprintf("pod-%d", workerID),
				Namespace: "default",
			}
			_ = backend.RegisterLease(ctx, "cluster-1", lease)
			_, _ = usecase.GetIPAMUtilization(ctx, "cluster-1")
			_, _ = usecase.TriggerIPAMGarbageCollection(ctx, "cluster-1", "worker-node-concurrent")
		}(i)
	}
	wg.Wait()
}
