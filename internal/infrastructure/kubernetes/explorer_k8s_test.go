package kubernetes

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestExplorerRepo_Deployment_NilReplicas_NoPanic(t *testing.T) {
	ctx := context.Background()

	// Deployment with Spec.Replicas = nil
	nilReplicaDeploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "nil-replicas-app",
			Namespace: "default",
			UID:       "uid-nil-rep",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: nil, // Nil replicas explicitly
		},
		Status: appsv1.DeploymentStatus{
			ReadyReplicas: 1,
		},
	}

	// Deployment with Spec.Replicas = nil and Paused = true
	pausedDeploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "paused-nil-app",
			Namespace: "default",
			UID:       "uid-paused-nil",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: nil,
			Paused:   true,
		},
		Status: appsv1.DeploymentStatus{
			ReadyReplicas: 0,
		},
	}

	fakeClient := fake.NewSimpleClientset(nilReplicaDeploy, pausedDeploy)
	repo := NewExplorerRepoWithInterface(fakeClient, nil, nil)

	// 1. Test Search Deployments - should not panic when Spec.Replicas is nil
	res, total, err := repo.Search(ctx, "deployment", "", "default", "", 10, 0)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if total != 2 || len(res) != 2 {
		t.Fatalf("expected 2 deployments, got total=%d, len=%d", total, len(res))
	}

	for _, item := range res {
		if item.Status != "Ready" {
			t.Errorf("expected deployment %s to be Ready, got %s", item.Name, item.Status)
		}
	}

	// 2. Test GetByID - should not panic when Spec.Replicas is nil
	got, err := repo.GetByID(ctx, "deployment/default/nil-replicas-app")
	if err != nil {
		t.Fatalf("GetByID failed: %v", err)
	}
	if got.Status != "Ready" {
		t.Errorf("expected Ready status, got %s", got.Status)
	}

	gotPaused, err := repo.GetByID(ctx, "deployment/default/paused-nil-app")
	if err != nil {
		t.Fatalf("GetByID paused failed: %v", err)
	}
	if gotPaused.Status != "Ready" {
		t.Errorf("expected Ready status for paused with 0 ready, got %s", gotPaused.Status)
	}
}

func TestExplorerRepo_ComputeDeploymentStatus_EdgeCases(t *testing.T) {
	// 1. nil deployment pointer
	if status := computeDeploymentStatus(nil); status != "NotReady" {
		t.Errorf("expected NotReady for nil deployment, got %s", status)
	}

	// 2. nil replicas, not paused, 0 ready replicas -> NotReady (since desired defaults to 1)
	unreadyNil := &appsv1.Deployment{
		Spec: appsv1.DeploymentSpec{
			Replicas: nil,
			Paused:   false,
		},
		Status: appsv1.DeploymentStatus{
			ReadyReplicas: 0,
		},
	}
	if status := computeDeploymentStatus(unreadyNil); status != "NotReady" {
		t.Errorf("expected NotReady, got %s", status)
	}

	// 3. nil replicas, not paused, 1 ready replicas -> Ready (desired defaults to 1)
	readyNil := &appsv1.Deployment{
		Spec: appsv1.DeploymentSpec{
			Replicas: nil,
			Paused:   false,
		},
		Status: appsv1.DeploymentStatus{
			ReadyReplicas: 1,
		},
	}
	if status := computeDeploymentStatus(readyNil); status != "Ready" {
		t.Errorf("expected Ready, got %s", status)
	}

	// 4. explicit replicas = 3, ready = 2 -> NotReady
	var rep3 int32 = 3
	depNotReady := &appsv1.Deployment{
		Spec: appsv1.DeploymentSpec{
			Replicas: &rep3,
		},
		Status: appsv1.DeploymentStatus{
			ReadyReplicas: 2,
		},
	}
	if status := computeDeploymentStatus(depNotReady); status != "NotReady" {
		t.Errorf("expected NotReady, got %s", status)
	}

	// 5. explicit replicas = 0, ready = 0 -> Ready
	var rep0 int32 = 0
	depScaledZero := &appsv1.Deployment{
		Spec: appsv1.DeploymentSpec{
			Replicas: &rep0,
		},
		Status: appsv1.DeploymentStatus{
			ReadyReplicas: 0,
		},
	}
	if status := computeDeploymentStatus(depScaledZero); status != "Ready" {
		t.Errorf("expected Ready, got %s", status)
	}
}

