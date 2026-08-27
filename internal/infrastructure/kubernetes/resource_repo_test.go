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

func TestResourceRepo_Nodes(t *testing.T) {
	ctx := context.Background()
	node1 := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "node-1",
			Labels: map[string]string{"env": "prod"},
		},
		Spec: corev1.NodeSpec{
			Unschedulable: false,
			Taints: []corev1.Taint{
				{Key: "dedicated", Value: "gpu", Effect: corev1.TaintEffectNoSchedule},
			},
		},
	}
	node2 := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "node-2",
		},
	}
	pod1 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "regular-pod-1",
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			NodeName: "node-1",
		},
	}
	dsPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ds-pod",
			Namespace: "kube-system",
			OwnerReferences: []metav1.OwnerReference{
				{Kind: "DaemonSet", Name: "ds-app"},
			},
		},
		Spec: corev1.PodSpec{
			NodeName: "node-1",
		},
	}
	mirrorPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mirror-pod",
			Namespace: "kube-system",
			Annotations: map[string]string{
				corev1.MirrorPodAnnotationKey: "mirror-hash",
			},
		},
		Spec: corev1.PodSpec{
			NodeName: "node-1",
		},
	}
	pod2 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "regular-pod-2",
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			NodeName: "node-2",
		},
	}

	fakeClient := fake.NewSimpleClientset(node1, node2, pod1, dsPod, mirrorPod, pod2)
	repo := &ResourceRepo{client: fakeClient}

	// 1. List Nodes
	nodesList, err := repo.ListResources(ctx, "", "nodes", "")
	if err != nil {
		t.Fatalf("ListResources nodes failed: %v", err)
	}
	if len(nodesList) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(nodesList))
	}

	// 2. Get Node
	n1, err := repo.GetResource(ctx, "", "nodes", "", "node-1")
	if err != nil {
		t.Fatalf("GetResource node-1 failed: %v", err)
	}
	meta := n1["metadata"].(map[string]interface{})
	if meta["name"] != "node-1" {
		t.Fatalf("expected node-1, got %v", meta["name"])
	}

	// 3. Cordon Node
	if err := repo.CordonNode(ctx, "", "node-1"); err != nil {
		t.Fatalf("CordonNode failed: %v", err)
	}
	updatedNode, err := fakeClient.CoreV1().Nodes().Get(ctx, "node-1", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("fetching node-1 after cordon: %v", err)
	}
	if !updatedNode.Spec.Unschedulable {
		t.Fatalf("expected node-1 to be unschedulable true")
	}

	// 4. Uncordon Node
	if err := repo.UncordonNode(ctx, "", "node-1"); err != nil {
		t.Fatalf("UncordonNode failed: %v", err)
	}
	updatedNode, err = fakeClient.CoreV1().Nodes().Get(ctx, "node-1", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("fetching node-1 after uncordon: %v", err)
	}
	if updatedNode.Spec.Unschedulable {
		t.Fatalf("expected node-1 to be unschedulable false")
	}

	// 5. Update Node Taints
	newTaints := []corev1.Taint{
		{Key: "env", Value: "stage", Effect: corev1.TaintEffectNoExecute},
	}
	if err := repo.UpdateNodeTaints(ctx, "", "node-1", newTaints); err != nil {
		t.Fatalf("UpdateNodeTaints failed: %v", err)
	}
	updatedNode, err = fakeClient.CoreV1().Nodes().Get(ctx, "node-1", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("fetching node-1 after taint update: %v", err)
	}
	if len(updatedNode.Spec.Taints) != 1 || updatedNode.Spec.Taints[0].Key != "env" {
		t.Fatalf("expected updated taints, got %v", updatedNode.Spec.Taints)
	}

	// 6. Update Node Labels
	newLabels := map[string]string{
		"tier": "frontend",
	}
	if err := repo.UpdateNodeLabels(ctx, "", "node-1", newLabels); err != nil {
		t.Fatalf("UpdateNodeLabels failed: %v", err)
	}
	updatedNode, err = fakeClient.CoreV1().Nodes().Get(ctx, "node-1", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("fetching node-1 after label update: %v", err)
	}
	if updatedNode.Labels["tier"] != "frontend" || updatedNode.Labels["env"] != "prod" {
		t.Fatalf("expected merged labels, got %v", updatedNode.Labels)
	}

	// 7. Drain Node
	if err := repo.DrainNode(ctx, "", "node-1", 30, true); err != nil {
		t.Fatalf("DrainNode failed: %v", err)
	}
	// Verify node is cordoned
	updatedNode, err = fakeClient.CoreV1().Nodes().Get(ctx, "node-1", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("fetching node-1 after drain: %v", err)
	}
	if !updatedNode.Spec.Unschedulable {
		t.Fatalf("expected node-1 to be unschedulable after drain")
	}
	// Verify regular-pod-1 is deleted
	_, err = fakeClient.CoreV1().Pods("default").Get(ctx, "regular-pod-1", metav1.GetOptions{})
	if err == nil {
		t.Fatalf("expected regular-pod-1 to be deleted during drain")
	}
	// Verify ds-pod still exists (ignoreDaemonSets=true)
	_, err = fakeClient.CoreV1().Pods("kube-system").Get(ctx, "ds-pod", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("expected ds-pod to be preserved during drain, got error: %v", err)
	}
	// Verify mirror-pod still exists
	_, err = fakeClient.CoreV1().Pods("kube-system").Get(ctx, "mirror-pod", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("expected mirror-pod to be preserved during drain, got error: %v", err)
	}
	// Verify regular-pod-2 on node-2 still exists
	_, err = fakeClient.CoreV1().Pods("default").Get(ctx, "regular-pod-2", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("expected regular-pod-2 on node-2 to be preserved during drain, got error: %v", err)
	}

	// 8. Delete Resource Node
	if err := repo.DeleteResource(ctx, "", "nodes", "", "node-2"); err != nil {
		t.Fatalf("DeleteResource node-2 failed: %v", err)
	}
	_, err = fakeClient.CoreV1().Nodes().Get(ctx, "node-2", metav1.GetOptions{})
	if err == nil {
		t.Fatalf("expected node-2 to be deleted")
	}
}

func TestResourceRepo_Events(t *testing.T) {
	ctx := context.Background()
	ev1 := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ev-1",
			Namespace: "default",
		},
		InvolvedObject: corev1.ObjectReference{
			Kind:      "Pod",
			Name:      "app-pod",
			Namespace: "default",
		},
		Reason:  "Started",
		Message: "Started container app",
		Type:    "Normal",
	}
	ev2 := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ev-2",
			Namespace: "prod",
		},
		InvolvedObject: corev1.ObjectReference{
			Kind:      "Deployment",
			Name:      "db",
			Namespace: "prod",
		},
		Reason:  "Failed",
		Message: "Back-off restarting failed container",
		Type:    "Warning",
	}

	fakeClient := fake.NewSimpleClientset(ev1, ev2)
	repo := &ResourceRepo{client: fakeClient}

	// 1. List Events in default namespace
	eventsDefault, err := repo.ListResources(ctx, "", "events", "default")
	if err != nil {
		t.Fatalf("ListResources events in default namespace failed: %v", err)
	}
	if len(eventsDefault) != 1 {
		t.Fatalf("expected 1 event in default namespace, got %d", len(eventsDefault))
	}

	// 2. List Events in all namespaces
	eventsAll, err := repo.ListResources(ctx, "", "events", "")
	if err != nil {
		t.Fatalf("ListResources events in all namespaces failed: %v", err)
	}
	if len(eventsAll) != 2 {
		t.Fatalf("expected 2 events in total, got %d", len(eventsAll))
	}

	// 3. Get Event
	item, err := repo.GetResource(ctx, "", "events", "default", "ev-1")
	if err != nil {
		t.Fatalf("GetResource event ev-1 failed: %v", err)
	}
	meta := item["metadata"].(map[string]interface{})
	if meta["name"] != "ev-1" {
		t.Fatalf("expected ev-1, got %v", meta["name"])
	}

	// 4. Delete Event
	if err := repo.DeleteResource(ctx, "", "events", "default", "ev-1"); err != nil {
		t.Fatalf("DeleteResource event ev-1 failed: %v", err)
	}
	_, err = fakeClient.CoreV1().Events("default").Get(ctx, "ev-1", metav1.GetOptions{})
	if err == nil {
		t.Fatalf("expected ev-1 to be deleted")
	}
}


func TestResourceRepo_CRUD_PersistentVolumes(t *testing.T) {
	ctx := context.Background()
	fakeClient := fake.NewSimpleClientset()
	repo := &ResourceRepo{client: fakeClient}

	pv := map[string]interface{}{
		"metadata": map[string]interface{}{
			"name": "pv-test",
		},
		"spec": map[string]interface{}{
			"capacity": map[string]interface{}{
				"storage": "10Gi",
			},
			"accessModes": []interface{}{"ReadWriteOnce"},
			"storageClassName": "standard",
		},
	}

	// 1. Create PV
	created, err := repo.CreateResource(ctx, "", "persistentvolumes", "", pv)
	if err != nil {
		t.Fatalf("CreateResource PV failed: %v", err)
	}
	meta := created["metadata"].(map[string]interface{})
	if meta["name"] != "pv-test" {
		t.Fatalf("expected pv-test, got %v", meta["name"])
	}

	// 2. Get PV
	got, err := repo.GetResource(ctx, "", "pv", "", "pv-test")
	if err != nil {
		t.Fatalf("GetResource PV failed: %v", err)
	}
	gotMeta := got["metadata"].(map[string]interface{})
	if gotMeta["name"] != "pv-test" {
		t.Fatalf("expected pv-test, got %v", gotMeta["name"])
	}

	// 3. List PVs
	list, err := repo.ListResources(ctx, "", "persistentvolume", "")
	if err != nil {
		t.Fatalf("ListResources PV failed: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 PV, got %d", len(list))
	}

	// 4. Update PV
	pv["metadata"].(map[string]interface{})["labels"] = map[string]interface{}{"env": "prod"}
	updated, err := repo.UpdateResource(ctx, "", "persistentvolumes", "", "pv-test", pv)
	if err != nil {
		t.Fatalf("UpdateResource PV failed: %v", err)
	}
	updatedMeta := updated["metadata"].(map[string]interface{})
	labels := updatedMeta["labels"].(map[string]interface{})
	if labels["env"] != "prod" {
		t.Fatalf("expected env: prod label, got %v", labels["env"])
	}

	// 5. Delete PV
	if err := repo.DeleteResource(ctx, "", "pv", "", "pv-test"); err != nil {
		t.Fatalf("DeleteResource PV failed: %v", err)
	}
	_, err = fakeClient.CoreV1().PersistentVolumes().Get(ctx, "pv-test", metav1.GetOptions{})
	if err == nil {
		t.Fatalf("expected PV to be deleted")
	}
}

func TestResourceRepo_CRUD_NetworkPolicies(t *testing.T) {
	ctx := context.Background()
	fakeClient := fake.NewSimpleClientset()
	repo := &ResourceRepo{client: fakeClient}

	np := map[string]interface{}{
		"metadata": map[string]interface{}{
			"name":      "np-test",
			"namespace": "default",
		},
		"spec": map[string]interface{}{
			"podSelector": map[string]interface{}{
				"matchLabels": map[string]interface{}{
					"role": "db",
				},
			},
			"policyTypes": []interface{}{"Ingress"},
		},
	}

	// 1. Create NP
	created, err := repo.CreateResource(ctx, "", "networkpolicies", "default", np)
	if err != nil {
		t.Fatalf("CreateResource NP failed: %v", err)
	}
	meta := created["metadata"].(map[string]interface{})
	if meta["name"] != "np-test" {
		t.Fatalf("expected np-test, got %v", meta["name"])
	}

	// 2. Get NP
	got, err := repo.GetResource(ctx, "", "netpol", "default", "np-test")
	if err != nil {
		t.Fatalf("GetResource NP failed: %v", err)
	}
	gotMeta := got["metadata"].(map[string]interface{})
	if gotMeta["name"] != "np-test" {
		t.Fatalf("expected np-test, got %v", gotMeta["name"])
	}

	// 3. List NPs
	list, err := repo.ListResources(ctx, "", "networkpolicy", "default")
	if err != nil {
		t.Fatalf("ListResources NP failed: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 NP, got %d", len(list))
	}

	// 4. Update NP
	np["metadata"].(map[string]interface{})["labels"] = map[string]interface{}{"tier": "backend"}
	updated, err := repo.UpdateResource(ctx, "", "networkpolicies", "default", "np-test", np)
	if err != nil {
		t.Fatalf("UpdateResource NP failed: %v", err)
	}
	updatedMeta := updated["metadata"].(map[string]interface{})
	labels := updatedMeta["labels"].(map[string]interface{})
	if labels["tier"] != "backend" {
		t.Fatalf("expected tier: backend label, got %v", labels["tier"])
	}

	// 5. Delete NP
	if err := repo.DeleteResource(ctx, "", "netpol", "default", "np-test"); err != nil {
		t.Fatalf("DeleteResource NP failed: %v", err)
	}
	_, err = fakeClient.NetworkingV1().NetworkPolicies("default").Get(ctx, "np-test", metav1.GetOptions{})
	if err == nil {
		t.Fatalf("expected NP to be deleted")
	}
}

func TestResourceRepo_CRUD_ServiceAccounts(t *testing.T) {
	ctx := context.Background()
	fakeClient := fake.NewSimpleClientset()
	repo := &ResourceRepo{client: fakeClient}

	sa := map[string]interface{}{
		"metadata": map[string]interface{}{
			"name":      "sa-test",
			"namespace": "default",
		},
	}

	// 1. Create SA
	created, err := repo.CreateResource(ctx, "", "serviceaccounts", "default", sa)
	if err != nil {
		t.Fatalf("CreateResource SA failed: %v", err)
	}
	meta := created["metadata"].(map[string]interface{})
	if meta["name"] != "sa-test" {
		t.Fatalf("expected sa-test, got %v", meta["name"])
	}

	// 2. Get SA
	got, err := repo.GetResource(ctx, "", "sa", "default", "sa-test")
	if err != nil {
		t.Fatalf("GetResource SA failed: %v", err)
	}
	gotMeta := got["metadata"].(map[string]interface{})
	if gotMeta["name"] != "sa-test" {
		t.Fatalf("expected sa-test, got %v", gotMeta["name"])
	}

	// 3. List SAs
	list, err := repo.ListResources(ctx, "", "serviceaccount", "default")
	if err != nil {
		t.Fatalf("ListResources SA failed: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 SA, got %d", len(list))
	}

	// 4. Update SA
	sa["metadata"].(map[string]interface{})["labels"] = map[string]interface{}{"team": "infra"}
	updated, err := repo.UpdateResource(ctx, "", "serviceaccounts", "default", "sa-test", sa)
	if err != nil {
		t.Fatalf("UpdateResource SA failed: %v", err)
	}
	updatedMeta := updated["metadata"].(map[string]interface{})
	labels := updatedMeta["labels"].(map[string]interface{})
	if labels["team"] != "infra" {
		t.Fatalf("expected team: infra label, got %v", labels["team"])
	}

	// 5. Delete SA
	if err := repo.DeleteResource(ctx, "", "sa", "default", "sa-test"); err != nil {
		t.Fatalf("DeleteResource SA failed: %v", err)
	}
	_, err = fakeClient.CoreV1().ServiceAccounts("default").Get(ctx, "sa-test", metav1.GetOptions{})
	if err == nil {
		t.Fatalf("expected SA to be deleted")
	}
}

func TestResourceRepo_CRUD_HorizontalPodAutoscalers(t *testing.T) {
	ctx := context.Background()
	fakeClient := fake.NewSimpleClientset()
	repo := &ResourceRepo{client: fakeClient}

	minReplicas := int32(2)
	hpa := map[string]interface{}{
		"metadata": map[string]interface{}{
			"name":      "hpa-test",
			"namespace": "default",
		},
		"spec": map[string]interface{}{
			"scaleTargetRef": map[string]interface{}{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"name":       "my-app",
			},
			"minReplicas": float64(minReplicas),
			"maxReplicas": float64(10),
		},
	}

	// 1. Create HPA
	created, err := repo.CreateResource(ctx, "", "horizontalpodautoscalers", "default", hpa)
	if err != nil {
		t.Fatalf("CreateResource HPA failed: %v", err)
	}
	meta := created["metadata"].(map[string]interface{})
	if meta["name"] != "hpa-test" {
		t.Fatalf("expected hpa-test, got %v", meta["name"])
	}

	// 2. Get HPA
	got, err := repo.GetResource(ctx, "", "hpa", "default", "hpa-test")
	if err != nil {
		t.Fatalf("GetResource HPA failed: %v", err)
	}
	gotMeta := got["metadata"].(map[string]interface{})
	if gotMeta["name"] != "hpa-test" {
		t.Fatalf("expected hpa-test, got %v", gotMeta["name"])
	}

	// 3. List HPAs
	list, err := repo.ListResources(ctx, "", "horizontalpodautoscaler", "default")
	if err != nil {
		t.Fatalf("ListResources HPA failed: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 HPA, got %d", len(list))
	}

	// 4. Update HPA
	hpa["metadata"].(map[string]interface{})["labels"] = map[string]interface{}{"autoscale": "true"}
	updated, err := repo.UpdateResource(ctx, "", "hpa", "default", "hpa-test", hpa)
	if err != nil {
		t.Fatalf("UpdateResource HPA failed: %v", err)
	}
	updatedMeta := updated["metadata"].(map[string]interface{})
	labels := updatedMeta["labels"].(map[string]interface{})
	if labels["autoscale"] != "true" {
		t.Fatalf("expected autoscale: true label, got %v", labels["autoscale"])
	}

	// 5. Delete HPA
	if err := repo.DeleteResource(ctx, "", "hpa", "default", "hpa-test"); err != nil {
		t.Fatalf("DeleteResource HPA failed: %v", err)
	}
	_, err = fakeClient.AutoscalingV2().HorizontalPodAutoscalers("default").Get(ctx, "hpa-test", metav1.GetOptions{})
	if err == nil {
		t.Fatalf("expected HPA to be deleted")
	}
}

func TestResourceRepo_WorkloadLifecycleOperations(t *testing.T) {
	ctx := context.Background()

	// Initial resources
	initDeploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "web-deploy",
			Namespace: "default",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: func() *int32 { i := int32(1); return &i }(),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "web"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "web", Image: "nginx:latest"}},
				},
			},
		},
	}

	initSts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "redis-sts",
			Namespace: "default",
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: func() *int32 { i := int32(1); return &i }(),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "redis"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "redis", Image: "redis:alpine"}},
				},
			},
		},
	}

	initDs := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "fluentd-ds",
			Namespace: "default",
		},
		Spec: appsv1.DaemonSetSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "fluentd"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "fluentd", Image: "fluentd:v1"}},
				},
			},
		},
	}

	initCron := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "nightly-cj",
			Namespace: "default",
		},
		Spec: batchv1.CronJobSpec{
			Schedule: "0 0 * * *",
			JobTemplate: batchv1.JobTemplateSpec{
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers:    []corev1.Container{{Name: "job", Image: "busybox"}},
							RestartPolicy: corev1.RestartPolicyNever,
						},
					},
				},
			},
		},
	}

	fakeClient := fake.NewSimpleClientset(initDeploy, initSts, initDs, initCron)
	repo := &ResourceRepo{client: fakeClient}

	// 1. Scale Deployment
	if err := repo.ScaleDeployment(ctx, "", "default", "web-deploy", 5); err != nil {
		t.Fatalf("ScaleDeployment failed: %v", err)
	}
	dep, err := fakeClient.AppsV1().Deployments("default").Get(ctx, "web-deploy", metav1.GetOptions{})
	if err != nil || *dep.Spec.Replicas != 5 {
		t.Fatalf("expected replicas 5, got %v (err: %v)", dep.Spec.Replicas, err)
	}

	// 2. Restart Deployment
	if err := repo.RestartDeployment(ctx, "", "default", "web-deploy"); err != nil {
		t.Fatalf("RestartDeployment failed: %v", err)
	}
	depAfterRestart, _ := fakeClient.AppsV1().Deployments("default").Get(ctx, "web-deploy", metav1.GetOptions{})
	if depAfterRestart.Spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"] == "" {
		t.Fatalf("expected restartedAt annotation on deployment")
	}

	// 3. Scale StatefulSet
	if err := repo.ScaleStatefulSet(ctx, "", "default", "redis-sts", 3); err != nil {
		t.Fatalf("ScaleStatefulSet failed: %v", err)
	}
	sts, err := fakeClient.AppsV1().StatefulSets("default").Get(ctx, "redis-sts", metav1.GetOptions{})
	if err != nil || *sts.Spec.Replicas != 3 {
		t.Fatalf("expected replicas 3, got %v (err: %v)", sts.Spec.Replicas, err)
	}

	// 4. Restart DaemonSet
	if err := repo.RestartDaemonSet(ctx, "", "default", "fluentd-ds"); err != nil {
		t.Fatalf("RestartDaemonSet failed: %v", err)
	}
	dsAfterRestart, _ := fakeClient.AppsV1().DaemonSets("default").Get(ctx, "fluentd-ds", metav1.GetOptions{})
	if dsAfterRestart.Spec.Template.Annotations["kubectl.kubernetes.io/restartedAt"] == "" {
		t.Fatalf("expected restartedAt annotation on daemonset")
	}

	// 5. Trigger CronJob
	job, err := repo.TriggerCronJob(ctx, "", "default", "nightly-cj")
	if err != nil {
		t.Fatalf("TriggerCronJob failed: %v", err)
	}
	if job == nil || job.Name == "" {
		t.Fatalf("expected created Job object")
	}
	createdJob, err := fakeClient.BatchV1().Jobs("default").Get(ctx, job.Name, metav1.GetOptions{})
	if err != nil || createdJob.Annotations["cronjob.kubernetes.io/instantiate"] != "manual" {
		t.Fatalf("expected manual trigger annotation on Job: %v", err)
	}

	// 6. Suspend CronJob
	if err := repo.SuspendCronJob(ctx, "", "default", "nightly-cj", true); err != nil {
		t.Fatalf("SuspendCronJob(true) failed: %v", err)
	}
	cjSuspended, _ := fakeClient.BatchV1().CronJobs("default").Get(ctx, "nightly-cj", metav1.GetOptions{})
	if cjSuspended.Spec.Suspend == nil || !*cjSuspended.Spec.Suspend {
		t.Fatalf("expected CronJob to be suspended")
	}

	// 7. Resume CronJob
	if err := repo.SuspendCronJob(ctx, "", "default", "nightly-cj", false); err != nil {
		t.Fatalf("SuspendCronJob(false) failed: %v", err)
	}
	cjResumed, _ := fakeClient.BatchV1().CronJobs("default").Get(ctx, "nightly-cj", metav1.GetOptions{})
	if cjResumed.Spec.Suspend == nil || *cjResumed.Spec.Suspend {
		t.Fatalf("expected CronJob to be resumed (not suspended)")
	}

	// 8. Test Not Found errors
	if err := repo.ScaleDeployment(ctx, "", "default", "non-existent", 2); err == nil {
		t.Fatalf("expected error scaling non-existent deployment")
	}
	if err := repo.RestartDaemonSet(ctx, "", "default", "non-existent"); err == nil {
		t.Fatalf("expected error restarting non-existent daemonset")
	}
	if _, err := repo.TriggerCronJob(ctx, "", "default", "non-existent"); err == nil {
		t.Fatalf("expected error triggering non-existent cronjob")
	}
}
