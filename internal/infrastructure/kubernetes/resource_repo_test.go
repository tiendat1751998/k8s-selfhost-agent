package kubernetes

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	storagev1 "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestResourceRepo_Namespaces(t *testing.T) {
	ctx := context.Background()
	fakeClient := fake.NewSimpleClientset()
	repo := &ResourceRepo{client: fakeClient}

	// 1. Create Namespace
	ns, err := repo.CreateNamespace(ctx, "", "test-ns")
	if err != nil {
		t.Fatalf("CreateNamespace failed: %v", err)
	}
	if ns.Name != "test-ns" {
		t.Fatalf("expected namespace test-ns, got %s", ns.Name)
	}

	// 2. List Namespaces
	list, err := repo.ListNamespaces(ctx, "")
	if err != nil {
		t.Fatalf("ListNamespaces failed: %v", err)
	}
	found := false
	for _, item := range list {
		if item.Name == "test-ns" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected to find test-ns in list")
	}

	// 3. Delete Namespace
	if err := repo.DeleteNamespace(ctx, "", "test-ns"); err != nil {
		t.Fatalf("DeleteNamespace failed: %v", err)
	}
}

func TestResourceRepo_SecretMasking(t *testing.T) {
	ctx := context.Background()
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"password": []byte("supersecret"),
		},
	}
	fakeClient := fake.NewSimpleClientset(secret)
	repo := &ResourceRepo{client: fakeClient}

	// 1. Get Secret
	res, err := repo.GetResource(ctx, "", "secrets", "default", "test-secret")
	if err != nil {
		t.Fatalf("GetResource secret failed: %v", err)
	}
	data, ok := res["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected data field in secret response")
	}
	if data["password"] != "***" {
		t.Fatalf("expected secret data to be masked as ***, got %v", data["password"])
	}

	// 2. List Secrets
	list, err := repo.ListResources(ctx, "", "secrets", "default")
	if err != nil {
		t.Fatalf("ListResources secrets failed: %v", err)
	}
	if len(list) == 0 {
		t.Fatalf("expected at least 1 secret in list")
	}
	for _, item := range list {
		itemData, ok := item["data"].(map[string]interface{})
		if ok {
			for k, v := range itemData {
				if v != "***" {
					t.Fatalf("expected secret key %s to be masked, got %v", k, v)
				}
			}
		}
	}
}

func TestResourceRepo_CRUD_AllKinds(t *testing.T) {
	ctx := context.Background()
	fakeClient := fake.NewSimpleClientset()
	repo := &ResourceRepo{client: fakeClient}

	// 1. ConfigMap
	cmManifest := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]interface{}{
			"name":      "my-cm",
			"namespace": "default",
		},
		"data": map[string]interface{}{
			"key": "value",
		},
	}
	createdCM, err := repo.CreateResource(ctx, "", "configmaps", "default", cmManifest)
	if err != nil {
		t.Fatalf("CreateResource configmap failed: %v", err)
	}
	if createdCM["metadata"].(map[string]interface{})["name"] != "my-cm" {
		t.Fatalf("unexpected CM name: %v", createdCM)
	}

	gotCM, err := repo.GetResource(ctx, "", "configmap", "default", "my-cm")
	if err != nil {
		t.Fatalf("GetResource configmap failed: %v", err)
	}
	if gotCM["metadata"].(map[string]interface{})["name"] != "my-cm" {
		t.Fatalf("unexpected got CM: %v", gotCM)
	}

	cmList, err := repo.ListResources(ctx, "", "cm", "default")
	if err != nil {
		t.Fatalf("ListResources configmap failed: %v", err)
	}
	if len(cmList) != 1 {
		t.Fatalf("expected 1 CM, got %d", len(cmList))
	}

	// Update CM
	cmManifest["data"] = map[string]interface{}{"key": "new-value"}
	updatedCM, err := repo.UpdateResource(ctx, "", "configmaps", "default", "my-cm", cmManifest)
	if err != nil {
		t.Fatalf("UpdateResource configmap failed: %v", err)
	}
	if updatedCM["data"].(map[string]interface{})["key"] != "new-value" {
		t.Fatalf("expected updated value, got %v", updatedCM)
	}

	// Delete CM
	if err := repo.DeleteResource(ctx, "", "configmaps", "default", "my-cm"); err != nil {
		t.Fatalf("DeleteResource configmap failed: %v", err)
	}

	// 2. Deployment
	deployManifest := map[string]interface{}{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata": map[string]interface{}{
			"name":      "my-deploy",
			"namespace": "default",
		},
		"spec": map[string]interface{}{
			"replicas": 2,
		},
	}
	if _, err := repo.CreateResource(ctx, "", "deployments", "default", deployManifest); err != nil {
		t.Fatalf("CreateResource deployment failed: %v", err)
	}
	if _, err := repo.GetResource(ctx, "", "deployment", "default", "my-deploy"); err != nil {
		t.Fatalf("GetResource deployment failed: %v", err)
	}
	if err := repo.DeleteResource(ctx, "", "deploy", "default", "my-deploy"); err != nil {
		t.Fatalf("DeleteResource deployment failed: %v", err)
	}

	// 3. Service
	svcManifest := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Service",
		"metadata": map[string]interface{}{
			"name":      "my-svc",
			"namespace": "default",
		},
	}
	if _, err := repo.CreateResource(ctx, "", "services", "default", svcManifest); err != nil {
		t.Fatalf("CreateResource service failed: %v", err)
	}
	if err := repo.DeleteResource(ctx, "", "svc", "default", "my-svc"); err != nil {
		t.Fatalf("DeleteResource service failed: %v", err)
	}

	// 4. Ingress
	ingManifest := map[string]interface{}{
		"apiVersion": "networking.k8s.io/v1",
		"kind":       "Ingress",
		"metadata": map[string]interface{}{
			"name":      "my-ing",
			"namespace": "default",
		},
	}
	if _, err := repo.CreateResource(ctx, "", "ingresses", "default", ingManifest); err != nil {
		t.Fatalf("CreateResource ingress failed: %v", err)
	}
	if err := repo.DeleteResource(ctx, "", "ing", "default", "my-ing"); err != nil {
		t.Fatalf("DeleteResource ingress failed: %v", err)
	}

	// 5. StatefulSet
	stsManifest := map[string]interface{}{
		"apiVersion": "apps/v1",
		"kind":       "StatefulSet",
		"metadata": map[string]interface{}{
			"name":      "my-sts",
			"namespace": "default",
		},
	}
	if _, err := repo.CreateResource(ctx, "", "statefulsets", "default", stsManifest); err != nil {
		t.Fatalf("CreateResource sts failed: %v", err)
	}
	if err := repo.DeleteResource(ctx, "", "sts", "default", "my-sts"); err != nil {
		t.Fatalf("DeleteResource sts failed: %v", err)
	}

	// 6. DaemonSet
	dsManifest := map[string]interface{}{
		"apiVersion": "apps/v1",
		"kind":       "DaemonSet",
		"metadata": map[string]interface{}{
			"name":      "my-ds",
			"namespace": "default",
		},
	}
	if _, err := repo.CreateResource(ctx, "", "daemonsets", "default", dsManifest); err != nil {
		t.Fatalf("CreateResource ds failed: %v", err)
	}
	if err := repo.DeleteResource(ctx, "", "ds", "default", "my-ds"); err != nil {
		t.Fatalf("DeleteResource ds failed: %v", err)
	}

	// 7. Job
	jobManifest := map[string]interface{}{
		"apiVersion": "batch/v1",
		"kind":       "Job",
		"metadata": map[string]interface{}{
			"name":      "my-job",
			"namespace": "default",
		},
	}
	if _, err := repo.CreateResource(ctx, "", "jobs", "default", jobManifest); err != nil {
		t.Fatalf("CreateResource job failed: %v", err)
	}
	if err := repo.DeleteResource(ctx, "", "job", "default", "my-job"); err != nil {
		t.Fatalf("DeleteResource job failed: %v", err)
	}

	// 8. CronJob
	cjManifest := map[string]interface{}{
		"apiVersion": "batch/v1",
		"kind":       "CronJob",
		"metadata": map[string]interface{}{
			"name":      "my-cj",
			"namespace": "default",
		},
	}
	if _, err := repo.CreateResource(ctx, "", "cronjobs", "default", cjManifest); err != nil {
		t.Fatalf("CreateResource cronjob failed: %v", err)
	}
	if err := repo.DeleteResource(ctx, "", "cj", "default", "my-cj"); err != nil {
		t.Fatalf("DeleteResource cronjob failed: %v", err)
	}

	// 9. PVC
	pvcManifest := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "PersistentVolumeClaim",
		"metadata": map[string]interface{}{
			"name":      "my-pvc",
			"namespace": "default",
		},
	}
	if _, err := repo.CreateResource(ctx, "", "persistentvolumeclaims", "default", pvcManifest); err != nil {
		t.Fatalf("CreateResource pvc failed: %v", err)
	}
	if err := repo.DeleteResource(ctx, "", "pvc", "default", "my-pvc"); err != nil {
		t.Fatalf("DeleteResource pvc failed: %v", err)
	}

	// 10. StorageClass
	scManifest := map[string]interface{}{
		"apiVersion": "storage.k8s.io/v1",
		"kind":       "StorageClass",
		"metadata": map[string]interface{}{
			"name": "my-sc",
		},
		"provisioner": "kubernetes.io/no-provisioner",
	}
	if _, err := repo.CreateResource(ctx, "", "storageclasses", "", scManifest); err != nil {
		t.Fatalf("CreateResource storageclass failed: %v", err)
	}
	if err := repo.DeleteResource(ctx, "", "sc", "", "my-sc"); err != nil {
		t.Fatalf("DeleteResource storageclass failed: %v", err)
	}
}

func TestResourceRepo_ApplyYAML(t *testing.T) {
	ctx := context.Background()
	fakeClient := fake.NewSimpleClientset()
	repo := &ResourceRepo{client: fakeClient}

	yamlData := `
apiVersion: v1
kind: ConfigMap
metadata:
  name: applied-cm
  namespace: default
data:
  message: hello
---
apiVersion: v1
kind: Service
metadata:
  name: applied-svc
  namespace: default
spec:
  ports:
  - port: 80
`
	if err := repo.ApplyYAML(ctx, "", "default", []byte(yamlData)); err != nil {
		t.Fatalf("ApplyYAML failed: %v", err)
	}

	// Verify CM was created
	cm, err := repo.GetResource(ctx, "", "configmaps", "default", "applied-cm")
	if err != nil {
		t.Fatalf("expected applied-cm to exist: %v", err)
	}
	if cm["metadata"].(map[string]interface{})["name"] != "applied-cm" {
		t.Fatalf("unexpected CM name: %v", cm)
	}

	// Apply updated YAML
	updatedYaml := `
apiVersion: v1
kind: ConfigMap
metadata:
  name: applied-cm
  namespace: default
data:
  message: updated-hello
`
	if err := repo.ApplyYAML(ctx, "", "default", []byte(updatedYaml)); err != nil {
		t.Fatalf("ApplyYAML update failed: %v", err)
	}

	updatedCM, err := repo.GetResource(ctx, "", "configmaps", "default", "applied-cm")
	if err != nil {
		t.Fatalf("expected updated applied-cm to exist: %v", err)
	}
	if updatedCM["data"].(map[string]interface{})["message"] != "updated-hello" {
		t.Fatalf("expected updated-hello message, got %v", updatedCM["data"])
	}
}

func TestResourceRepo_Pods(t *testing.T) {
	ctx := context.Background()
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
			Labels: map[string]string{
				"app": "test",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "test-container",
					Image: "nginx:alpine",
				},
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
		},
	}
	fakeClient := fake.NewSimpleClientset(pod)
	repo := &ResourceRepo{client: fakeClient}

	// 1. List Pods
	list, err := repo.ListResources(ctx, "", "pods", "default")
	if err != nil {
		t.Fatalf("ListResources pods failed: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 pod, got %d", len(list))
	}
	if list[0]["metadata"].(map[string]interface{})["name"] != "test-pod" {
		t.Fatalf("unexpected pod name: %v", list[0])
	}

	// 2. Get Pod
	item, err := repo.GetResource(ctx, "", "pod", "default", "test-pod")
	if err != nil {
		t.Fatalf("GetResource pod failed: %v", err)
	}
	if item["metadata"].(map[string]interface{})["name"] != "test-pod" {
		t.Fatalf("unexpected pod name: %v", item)
	}

	// 3. Delete Pod
	if err := repo.DeleteResource(ctx, "", "pods", "default", "test-pod"); err != nil {
		t.Fatalf("DeleteResource pod failed: %v", err)
	}

	// 4. Verify Deleted
	listAfter, err := repo.ListResources(ctx, "", "pods", "default")
	if err != nil {
		t.Fatalf("ListResources after delete failed: %v", err)
	}
	if len(listAfter) != 0 {
		t.Fatalf("expected 0 pods after delete, got %d", len(listAfter))
	}
}

// Suppress unused imports check if any
var _ = appsv1.Deployment{}
var _ = batchv1.Job{}
var _ = networkingv1.Ingress{}
var _ = storagev1.StorageClass{}
