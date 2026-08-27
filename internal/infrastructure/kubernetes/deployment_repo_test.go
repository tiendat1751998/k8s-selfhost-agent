package kubernetes

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func int32Ptr(i int32) *int32 {
	return &i
}

func TestDeploymentRepo_List_AllNamespaces(t *testing.T) {
	ctx := context.Background()

	deployments := []appsv1.Deployment{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "coredns",
				Namespace: "kube-system",
				Labels: map[string]string{
					"k8s-app": "kube-dns",
					"team":    "Infra",
					"env":     "production",
				},
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: int32Ptr(2),
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "coredns",
								Image: "coredns/coredns:v1.10.1",
								Ports: []corev1.ContainerPort{
									{ContainerPort: 53},
								},
								Resources: corev1.ResourceRequirements{
									Limits: corev1.ResourceList{
										corev1.ResourceMemory: resource.MustParse("170Mi"),
										corev1.ResourceCPU:    resource.MustParse("200m"),
									},
									Requests: corev1.ResourceList{
										corev1.ResourceMemory: resource.MustParse("70Mi"),
										corev1.ResourceCPU:    resource.MustParse("100m"),
									},
								},
							},
						},
					},
				},
			},
			Status: appsv1.DeploymentStatus{
				Replicas:      2,
				ReadyReplicas: 2,
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cilium-operator",
				Namespace: "kube-system",
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: int32Ptr(1),
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "cilium-operator",
								Image: "quay.io/cilium/cilium-operator:v1.14.0",
							},
						},
					},
				},
			},
			Status: appsv1.DeploymentStatus{
				Replicas:      1,
				ReadyReplicas: 1,
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "frontend-app",
				Namespace: "default",
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: int32Ptr(3),
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "web",
								Image: "nginx:alpine",
								Ports: []corev1.ContainerPort{
									{ContainerPort: 8080},
								},
							},
						},
					},
				},
			},
			Status: appsv1.DeploymentStatus{
				Replicas:      3,
				ReadyReplicas: 1, // Degraded
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "payment-service",
				Namespace: "finance",
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: int32Ptr(2),
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "payment",
								Image: "payment-api:v2.1",
								Ports: []corev1.ContainerPort{
									{ContainerPort: 9000},
								},
							},
						},
						Volumes: []corev1.Volume{
							{
								Name: "data-vol",
							},
						},
					},
				},
			},
			Status: appsv1.DeploymentStatus{
				Replicas:      2,
				ReadyReplicas: 0, // Down
			},
		},
	}

	fakeClient := fake.NewSimpleClientset(
		&deployments[0],
		&deployments[1],
		&deployments[2],
		&deployments[3],
	)

	repo := NewDeploymentRepoWithInterface(fakeClient, nil, nil, nil)
	apps, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("List() unexpected error: %v", err)
	}

	if len(apps) != 4 {
		t.Fatalf("expected 4 deployments across all namespaces, got %d", len(apps))
	}

	appMap := make(map[string]map[string]interface{})
	for _, app := range apps {
		key := app.Namespace + "/" + app.Name
		appMap[key] = map[string]interface{}{
			"namespace":   app.Namespace,
			"target":      app.Target,
			"type":        app.Type,
			"status":      app.Status,
			"replicas":    app.Replicas,
			"image":       app.Image,
			"port":        app.Port,
			"cpuLimit":    app.CPULimit,
			"memoryLimit": app.MemoryLimit,
			"volume":      app.Volume,
		}
	}

	// Verify coredns in kube-system
	coredns, ok := appMap["kube-system/coredns"]
	if !ok {
		t.Fatalf("expected kube-system/coredns to be included in apps")
	}
	if coredns["namespace"] != "kube-system" {
		t.Errorf("expected namespace kube-system, got %v", coredns["namespace"])
	}
	if coredns["type"] != "kubernetes" {
		t.Errorf("expected type kubernetes, got %v", coredns["type"])
	}
	if coredns["status"] != "healthy" {
		t.Errorf("expected status healthy, got %v", coredns["status"])
	}
	if coredns["replicas"] != 2 {
		t.Errorf("expected replicas 2, got %v", coredns["replicas"])
	}
	if coredns["image"] != "coredns/coredns:v1.10.1" {
		t.Errorf("expected image coredns/coredns:v1.10.1, got %v", coredns["image"])
	}
	if coredns["port"] != 53 {
		t.Errorf("expected port 53, got %v", coredns["port"])
	}
	if coredns["cpuLimit"] != "200m" {
		t.Errorf("expected cpuLimit 200m, got %v", coredns["cpuLimit"])
	}
	if coredns["memoryLimit"] != "170Mi" {
		t.Errorf("expected memoryLimit 170Mi, got %v", coredns["memoryLimit"])
	}

	// Verify cilium-operator in kube-system
	cilium, ok := appMap["kube-system/cilium-operator"]
	if !ok {
		t.Fatalf("expected kube-system/cilium-operator to be included in apps")
	}
	if cilium["status"] != "healthy" {
		t.Errorf("expected status healthy, got %v", cilium["status"])
	}

	// Verify frontend-app in default (degraded)
	fe, ok := appMap["default/frontend-app"]
	if !ok {
		t.Fatalf("expected default/frontend-app to be included in apps")
	}
	if fe["status"] != "degraded" {
		t.Errorf("expected status degraded, got %v", fe["status"])
	}
	if fe["port"] != 8080 {
		t.Errorf("expected port 8080, got %v", fe["port"])
	}

	// Verify payment-service in finance (down, volume mounted)
	pay, ok := appMap["finance/payment-service"]
	if !ok {
		t.Fatalf("expected finance/payment-service to be included in apps")
	}
	if pay["status"] != "down" {
		t.Errorf("expected status down, got %v", pay["status"])
	}
	if pay["volume"] != "mounted" {
		t.Errorf("expected volume mounted, got %v", pay["volume"])
	}
}

func TestDeploymentRepo_MapK8sDeployment_StatusEvaluation(t *testing.T) {
	repo := &deploymentRepo{}

	tests := []struct {
		name          string
		desired       int32
		ready         int32
		expectedState string
	}{
		{
			name:          "fully ready",
			desired:       3,
			ready:         3,
			expectedState: "healthy",
		},
		{
			name:          "over provisioned / rolling update",
			desired:       3,
			ready:         4,
			expectedState: "healthy",
		},
		{
			name:          "partially ready",
			desired:       3,
			ready:         2,
			expectedState: "degraded",
		},
		{
			name:          "no ready replicas",
			desired:       3,
			ready:         0,
			expectedState: "down",
		},
		{
			name:          "zero desired zero ready",
			desired:       0,
			ready:         0,
			expectedState: "down",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deploy",
					Namespace: "kube-system",
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: &tt.desired,
				},
				Status: appsv1.DeploymentStatus{
					ReadyReplicas: tt.ready,
				},
			}
			app := repo.mapK8sDeployment(d, "k8snode")
			if app.Status != tt.expectedState {
				t.Errorf("mapK8sDeployment status = %s, want %s", app.Status, tt.expectedState)
			}
			if app.Target != "k8snode" {
				t.Errorf("mapK8sDeployment target = %s, want k8snode", app.Target)
			}
			if app.Namespace != "kube-system" {
				t.Errorf("mapK8sDeployment namespace = %s, want kube-system", app.Namespace)
			}
			if app.Type != "kubernetes" {
				t.Errorf("mapK8sDeployment type = %s, want kubernetes", app.Type)
			}
		})
	}
}
