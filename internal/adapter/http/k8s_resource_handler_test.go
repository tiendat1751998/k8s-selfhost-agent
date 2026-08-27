package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/datdt/k8sselfhost/internal/adapter/http/middleware"
	"github.com/datdt/k8sselfhost/internal/domain/audit"
	infraK8s "github.com/datdt/k8sselfhost/internal/infrastructure/kubernetes"
)

type mockK8sAuditRepo struct {
	actions []string
}

func (m *mockK8sAuditRepo) ListFindings(ctx context.Context, status string) ([]audit.AuditFinding, error) {
	return nil, nil
}
func (m *mockK8sAuditRepo) GetFinding(ctx context.Context, id string) (*audit.AuditFinding, error) {
	return nil, nil
}
func (m *mockK8sAuditRepo) ResolveFinding(ctx context.Context, id string) error {
	return nil
}
func (m *mockK8sAuditRepo) RecordRun(ctx context.Context, run *audit.AuditRun) error {
	return nil
}
func (m *mockK8sAuditRepo) GetLastRun(ctx context.Context) (*audit.AuditRun, error) {
	return nil, nil
}
func (m *mockK8sAuditRepo) RecordAction(ctx context.Context, actor, action, targetType, targetID, targetName, result string, details map[string]interface{}, ipAddress, userAgent string) error {
	m.actions = append(m.actions, action+":"+targetType+":"+targetName)
	return nil
}

func TestK8sResourceHandler_OfflineGraceful(t *testing.T) {
	// Handler with nil repo should return 503
	handler := NewK8sResourceHandler(nil, nil)
	r := chi.NewRouter()
	r.Route("/k8s/{cluster}", handler.RegisterRoutes)

	endpoints := []struct {
		method string
		path   string
	}{
		{"GET", "/k8s/local/namespaces"},
		{"POST", "/k8s/local/namespaces"},
		{"DELETE", "/k8s/local/namespaces/foo"},
		{"GET", "/k8s/local/resources/configmaps"},
		{"GET", "/k8s/local/resources/configmaps/bar"},
		{"POST", "/k8s/local/resources/configmaps"},
		{"PUT", "/k8s/local/resources/configmaps/bar"},
		{"DELETE", "/k8s/local/resources/configmaps/bar"},
		{"POST", "/k8s/local/apply"},
		{"GET", "/k8s/local/events"},
		{"POST", "/k8s/local/nodes/node-1/cordon"},
		{"POST", "/k8s/local/nodes/node-1/uncordon"},
		{"POST", "/k8s/local/nodes/node-1/drain"},
		{"PUT", "/k8s/local/nodes/node-1/taints"},
		{"PUT", "/k8s/local/nodes/node-1/labels"},
		{"POST", "/k8s/local/resources/deployments/foo/scale"},
		{"POST", "/k8s/local/resources/deployments/foo/restart"},
		{"POST", "/k8s/local/resources/statefulsets/foo/scale"},
		{"POST", "/k8s/local/resources/daemonsets/foo/restart"},
		{"POST", "/k8s/local/resources/cronjobs/foo/trigger"},
		{"PUT", "/k8s/local/resources/cronjobs/foo/suspend"},
	}

	for _, ep := range endpoints {
		req := httptest.NewRequest(ep.method, ep.path, bytes.NewBufferString(`{}`))
		ctx := context.WithValue(req.Context(), middleware.UserRoleKey, "platform_admin")
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf("[%s %s] expected status 503, got %d", ep.method, ep.path, rec.Code)
		}

		var res map[string]string
		if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
			t.Errorf("[%s %s] failed to unmarshal response: %v", ep.method, ep.path, err)
		}
		if res["error"] != "Kubernetes cluster not connected or unconfigured" {
			t.Errorf("[%s %s] expected error 'Kubernetes cluster not connected or unconfigured', got '%s'", ep.method, ep.path, res["error"])
		}
		if res["code"] != "K8S_UNAVAILABLE" {
			t.Errorf("[%s %s] expected code 'K8S_UNAVAILABLE', got '%s'", ep.method, ep.path, res["code"])
		}
		if res["message"] != "Import a kubeconfig via Fleet to enable this feature" {
			t.Errorf("[%s %s] expected message 'Import a kubeconfig via Fleet to enable this feature', got '%s'", ep.method, ep.path, res["message"])
		}
	}
}

func TestK8sResourceHandler_UnconfiguredClient(t *testing.T) {
	// Handler with a repo that has no kubernetes client configured
	repo := infraK8s.NewResourceRepoWithInterface(nil, nil)
	handler := NewK8sResourceHandler(repo, nil)
	r := chi.NewRouter()
	r.Route("/k8s/{cluster}", handler.RegisterRoutes)

	endpoints := []struct {
		method string
		path   string
	}{
		{"GET", "/k8s/unconfigured-cluster/namespaces"},
		{"POST", "/k8s/unconfigured-cluster/namespaces"},
		{"DELETE", "/k8s/unconfigured-cluster/namespaces/foo"},
		{"GET", "/k8s/unconfigured-cluster/resources/configmaps"},
		{"GET", "/k8s/unconfigured-cluster/resources/configmaps/bar"},
		{"POST", "/k8s/unconfigured-cluster/resources/configmaps"},
		{"PUT", "/k8s/unconfigured-cluster/resources/configmaps/bar"},
		{"DELETE", "/k8s/unconfigured-cluster/resources/configmaps/bar"},
		{"POST", "/k8s/unconfigured-cluster/apply"},
		{"GET", "/k8s/unconfigured-cluster/events"},
		{"POST", "/k8s/unconfigured-cluster/nodes/node-1/cordon"},
		{"POST", "/k8s/unconfigured-cluster/nodes/node-1/uncordon"},
		{"POST", "/k8s/unconfigured-cluster/nodes/node-1/drain"},
		{"PUT", "/k8s/unconfigured-cluster/nodes/node-1/taints"},
		{"PUT", "/k8s/unconfigured-cluster/nodes/node-1/labels"},
	}

	for _, ep := range endpoints {
		req := httptest.NewRequest(ep.method, ep.path, bytes.NewBufferString(`{"name":"test","apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"test"}}`))
		ctx := context.WithValue(req.Context(), middleware.UserRoleKey, "platform_admin")
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf("[%s %s] expected status 503, got %d (body: %s)", ep.method, ep.path, rec.Code, rec.Body.String())
		}

		var res map[string]string
		if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
			t.Errorf("[%s %s] failed to unmarshal response: %v", ep.method, ep.path, err)
		}
		if res["error"] != "Kubernetes cluster not connected or unconfigured" {
			t.Errorf("[%s %s] expected error 'Kubernetes cluster not connected or unconfigured', got '%s'", ep.method, ep.path, res["error"])
		}
		if res["code"] != "K8S_UNAVAILABLE" {
			t.Errorf("[%s %s] expected code 'K8S_UNAVAILABLE', got '%s'", ep.method, ep.path, res["code"])
		}
	}
}

func TestK8sResourceHandler_LiveOperations(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "db-secret",
			Namespace: "default",
		},
		Data: map[string][]byte{
			"password": []byte("super-secret-password"),
		},
	}
	fakeClient := fake.NewSimpleClientset(secret)
	repo := infraK8s.NewResourceRepoWithInterface(fakeClient, nil)
	auditMock := &mockK8sAuditRepo{}
	handler := NewK8sResourceHandler(repo, auditMock)

	r := chi.NewRouter()
	r.Route("/k8s/{cluster}", handler.RegisterRoutes)

	// 1. Create Namespace
	nsBody := `{"name": "test-zone"}`
	req := httptest.NewRequest("POST", "/k8s/local/namespaces", bytes.NewBufferString(nsBody))
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", rec.Code, rec.Body.String())
	}

	// 2. List Namespaces
	req = httptest.NewRequest("GET", "/k8s/local/namespaces", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	// 3. Get Secret (verifying masked data)
	req = httptest.NewRequest("GET", "/k8s/local/resources/secrets/db-secret?ns=default", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}
	var secretResp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &secretResp); err != nil {
		t.Fatalf("unmarshaling secret resp: %v", err)
	}
	dataMap := secretResp["data"].(map[string]interface{})
	if dataMap["password"] != "***" {
		t.Fatalf("expected masked secret value, got %v", dataMap["password"])
	}

	// 4. Create ConfigMap
	cmBody := `{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"app-config","namespace":"default"},"data":{"PORT":"8080"}}`
	req = httptest.NewRequest("POST", "/k8s/local/resources/configmaps?ns=default", bytes.NewBufferString(cmBody))
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", rec.Code, rec.Body.String())
	}

	// 5. Update ConfigMap
	cmUpdateBody := `{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"app-config","namespace":"default"},"data":{"PORT":"9090"}}`
	req = httptest.NewRequest("PUT", "/k8s/local/resources/configmaps/app-config?ns=default", bytes.NewBufferString(cmUpdateBody))
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	// 6. Delete ConfigMap
	req = httptest.NewRequest("DELETE", "/k8s/local/resources/configmaps/app-config?ns=default", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	// 7. Apply YAML
	applyBody := `{"yaml": "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: applied-cm\n  namespace: default\ndata:\n  test: val"}`
	req = httptest.NewRequest("POST", "/k8s/local/apply?ns=default", bytes.NewBufferString(applyBody))
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	// 8. Delete Namespace
	req = httptest.NewRequest("DELETE", "/k8s/local/namespaces/test-zone", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify Audit Actions were recorded
	if len(auditMock.actions) == 0 {
		t.Fatalf("expected audit actions to be recorded, got none")
	}
}

func TestNodeOperations(t *testing.T) {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "worker-1",
			Labels: map[string]string{"env": "staging"},
		},
		Spec: corev1.NodeSpec{
			Unschedulable: false,
		},
	}
	pod1 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "web-pod",
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			NodeName: "worker-1",
		},
	}
	ev1 := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ev-pod-start",
			Namespace: "default",
		},
		InvolvedObject: corev1.ObjectReference{
			Kind:      "Pod",
			Name:      "web-pod",
			Namespace: "default",
		},
		Reason:  "Started",
		Message: "Container started",
		Type:    "Normal",
	}
	ev2 := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ev-node-fail",
			Namespace: "default",
		},
		InvolvedObject: corev1.ObjectReference{
			Kind: "Node",
			Name: "worker-1",
		},
		Reason:  "DiskPressure",
		Message: "Node has disk pressure",
		Type:    "Warning",
	}

	fakeClient := fake.NewSimpleClientset(node, pod1, ev1, ev2)
	repo := infraK8s.NewResourceRepoWithInterface(fakeClient, nil)
	auditMock := &mockK8sAuditRepo{}
	handler := NewK8sResourceHandler(repo, auditMock)

	r := chi.NewRouter()
	r.Route("/k8s/{cluster}", handler.RegisterRoutes)

	adminCtx := context.WithValue(context.Background(), middleware.UserRoleKey, "platform_admin")
	tenantAdminCtx := context.WithValue(context.Background(), middleware.UserRoleKey, "tenant_admin")
	viewerCtx := context.WithValue(context.Background(), middleware.UserRoleKey, "viewer")

	// 1. List Events without filter
	req := httptest.NewRequest("GET", "/k8s/local/events?ns=default", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for ListEvents, got %d: %s", rec.Code, rec.Body.String())
	}
	var evResp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &evResp); err != nil {
		t.Fatalf("unmarshaling events response: %v", err)
	}
	if int(evResp["total"].(float64)) != 2 {
		t.Fatalf("expected 2 events, got %v", evResp["total"])
	}

	// 2. List Events with kind filter
	req = httptest.NewRequest("GET", "/k8s/local/events?ns=default&kind=Pod", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for ListEvents with kind filter, got %d", rec.Code)
	}
	var evKindResp map[string]interface{}
	_ = json.Unmarshal(rec.Body.Bytes(), &evKindResp)
	if int(evKindResp["total"].(float64)) != 1 {
		t.Fatalf("expected 1 pod event, got %v", evKindResp["total"])
	}

	// 3. List Events with name filter
	req = httptest.NewRequest("GET", "/k8s/local/events?ns=default&name=web-pod", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for ListEvents with name filter, got %d", rec.Code)
	}
	var evNameResp map[string]interface{}
	_ = json.Unmarshal(rec.Body.Bytes(), &evNameResp)
	if int(evNameResp["total"].(float64)) != 1 {
		t.Fatalf("expected 1 event with name web-pod, got %v", evNameResp["total"])
	}

	// 4. List Events with type filter
	req = httptest.NewRequest("GET", "/k8s/local/events?ns=default&type=Warning", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for ListEvents with type filter, got %d", rec.Code)
	}
	var evTypeResp map[string]interface{}
	_ = json.Unmarshal(rec.Body.Bytes(), &evTypeResp)
	if int(evTypeResp["total"].(float64)) != 1 {
		t.Fatalf("expected 1 Warning event, got %v", evTypeResp["total"])
	}

	// 5. List Events with limit
	req = httptest.NewRequest("GET", "/k8s/local/events?ns=default&limit=1", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for ListEvents with limit, got %d", rec.Code)
	}
	var evLimitResp map[string]interface{}
	_ = json.Unmarshal(rec.Body.Bytes(), &evLimitResp)
	if int(evLimitResp["total"].(float64)) != 1 {
		t.Fatalf("expected 1 event with limit 1, got %v", evLimitResp["total"])
	}

	// 6. RBAC rejection on node mutation without role or with viewer role
	req = httptest.NewRequest("POST", "/k8s/local/nodes/worker-1/cordon", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized for unauthenticated node cordon, got %d", rec.Code)
	}

	req = httptest.NewRequest("POST", "/k8s/local/nodes/worker-1/cordon", nil).WithContext(viewerCtx)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for viewer on node cordon, got %d", rec.Code)
	}

	// 7. Cordon Node with platform_admin
	req = httptest.NewRequest("POST", "/k8s/local/nodes/worker-1/cordon", nil).WithContext(adminCtx)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for CordonNode, got %d: %s", rec.Code, rec.Body.String())
	}
	var cordonResp map[string]string
	_ = json.Unmarshal(rec.Body.Bytes(), &cordonResp)
	if cordonResp["status"] != "cordoned" {
		t.Fatalf("expected status cordoned, got %s", cordonResp["status"])
	}

	// 8. Uncordon Node with tenant_admin
	req = httptest.NewRequest("POST", "/k8s/local/nodes/worker-1/uncordon", nil).WithContext(tenantAdminCtx)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for UncordonNode, got %d: %s", rec.Code, rec.Body.String())
	}
	var uncordonResp map[string]string
	_ = json.Unmarshal(rec.Body.Bytes(), &uncordonResp)
	if uncordonResp["status"] != "uncordoned" {
		t.Fatalf("expected status uncordoned, got %s", uncordonResp["status"])
	}

	// 9. Update Node Taints
	taintsBody := `{"taints":[{"key":"dedicated","value":"infra","effect":"NoSchedule"}]}`
	req = httptest.NewRequest("PUT", "/k8s/local/nodes/worker-1/taints", bytes.NewBufferString(taintsBody)).WithContext(adminCtx)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for UpdateNodeTaints, got %d: %s", rec.Code, rec.Body.String())
	}

	// 10. Update Node Labels
	labelsBody := `{"labels":{"role":"worker","tier":"backend"}}`
	req = httptest.NewRequest("PUT", "/k8s/local/nodes/worker-1/labels", bytes.NewBufferString(labelsBody)).WithContext(adminCtx)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for UpdateNodeLabels, got %d: %s", rec.Code, rec.Body.String())
	}

	// 11. Drain Node
	drainBody := `{"gracePeriodSeconds": 20, "ignoreDaemonSets": true}`
	req = httptest.NewRequest("POST", "/k8s/local/nodes/worker-1/drain", bytes.NewBufferString(drainBody)).WithContext(adminCtx)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for DrainNode, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify Audit Actions were recorded for node operations
	expectedActions := []string{
		"cordon:k8s_node:worker-1",
		"uncordon:k8s_node:worker-1",
		"update_taints:k8s_node:worker-1",
		"update_labels:k8s_node:worker-1",
		"drain:k8s_node:worker-1",
	}
	for _, exp := range expectedActions {
		found := false
		for _, act := range auditMock.actions {
			if act == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected audit action %s to be recorded in %v", exp, auditMock.actions)
		}
	}
}


func TestK8sResourceHandler_WorkloadLifecycleEndpoints(t *testing.T) {
	ctx := context.Background()

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "nginx-dep",
			Namespace: "default",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: func() *int32 { i := int32(1); return &i }(),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "nginx"}},
				Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "nginx", Image: "nginx:latest"}}},
			},
		},
	}

	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "redis-sts",
			Namespace: "default",
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: func() *int32 { i := int32(1); return &i }(),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "redis"}},
				Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "redis", Image: "redis:alpine"}}},
			},
		},
	}

	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "fluentd-ds",
			Namespace: "default",
		},
		Spec: appsv1.DaemonSetSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "fluentd"}},
				Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "fluentd", Image: "fluentd:latest"}}},
			},
		},
	}

	cj := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "nightly-cron",
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

	fakeClient := fake.NewSimpleClientset(dep, sts, ds, cj)
	k8sRepo := infraK8s.NewResourceRepoWithInterface(fakeClient, nil)
	auditMock := &mockK8sAuditRepo{}
	handler := NewK8sResourceHandler(k8sRepo, auditMock)

	r := chi.NewRouter()
	r.Route("/k8s/{cluster}", handler.RegisterRoutes)

	adminCtx := context.WithValue(ctx, middleware.UserRoleKey, "platform_admin")
	adminCtx = context.WithValue(adminCtx, middleware.UserIDKey, "admin-1")
	viewerCtx := context.WithValue(ctx, middleware.UserRoleKey, "viewer")

	// 1. Scale Deployment - Success
	req := httptest.NewRequest("POST", "/k8s/local/resources/deployments/nginx-dep/scale?namespace=default", bytes.NewBufferString(`{"replicas": 4}`)).WithContext(adminCtx)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for ScaleDeployment, got %d: %s", rec.Code, rec.Body.String())
	}
	var scaleResp map[string]interface{}
	_ = json.Unmarshal(rec.Body.Bytes(), &scaleResp)
	if scaleResp["message"] != "deployment scaled successfully" || int(scaleResp["replicas"].(float64)) != 4 {
		t.Fatalf("unexpected response body: %v", scaleResp)
	}

	// 2. Scale Deployment - Bad Request (negative replicas)
	req = httptest.NewRequest("POST", "/k8s/local/resources/deployments/nginx-dep/scale?namespace=default", bytes.NewBufferString(`{"replicas": -5}`)).WithContext(adminCtx)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for negative replicas, got %d", rec.Code)
	}

	// 3. Restart Deployment - Success
	req = httptest.NewRequest("POST", "/k8s/local/resources/deployments/nginx-dep/restart?namespace=default", nil).WithContext(adminCtx)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for RestartDeployment, got %d: %s", rec.Code, rec.Body.String())
	}

	// 4. Scale StatefulSet - Success
	req = httptest.NewRequest("POST", "/k8s/local/resources/statefulsets/redis-sts/scale?namespace=default", bytes.NewBufferString(`{"replicas": 3}`)).WithContext(adminCtx)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for ScaleStatefulSet, got %d: %s", rec.Code, rec.Body.String())
	}

	// 5. Restart DaemonSet - Success
	req = httptest.NewRequest("POST", "/k8s/local/resources/daemonsets/fluentd-ds/restart?namespace=default", nil).WithContext(adminCtx)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for RestartDaemonSet, got %d: %s", rec.Code, rec.Body.String())
	}

	// 6. Trigger CronJob - Success
	req = httptest.NewRequest("POST", "/k8s/local/resources/cronjobs/nightly-cron/trigger?namespace=default", nil).WithContext(adminCtx)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for TriggerCronJob, got %d: %s", rec.Code, rec.Body.String())
	}
	var triggerResp map[string]interface{}
	_ = json.Unmarshal(rec.Body.Bytes(), &triggerResp)
	if triggerResp["message"] != "cronjob triggered successfully" || triggerResp["job"] == "" {
		t.Fatalf("unexpected trigger response body: %v", triggerResp)
	}

	// 7. Suspend CronJob - Success
	req = httptest.NewRequest("PUT", "/k8s/local/resources/cronjobs/nightly-cron/suspend?namespace=default", bytes.NewBufferString(`{"suspend": true}`)).WithContext(adminCtx)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for SuspendCronJob, got %d: %s", rec.Code, rec.Body.String())
	}

	// 8. Resume CronJob - Success
	req = httptest.NewRequest("PUT", "/k8s/local/resources/cronjobs/nightly-cron/suspend?namespace=default", bytes.NewBufferString(`{"suspend": false}`)).WithContext(adminCtx)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for ResumeCronJob, got %d: %s", rec.Code, rec.Body.String())
	}

	// 9. RBAC - Viewer role denied on mutation
	req = httptest.NewRequest("POST", "/k8s/local/resources/deployments/nginx-dep/scale?namespace=default", bytes.NewBufferString(`{"replicas": 2}`)).WithContext(viewerCtx)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for viewer on scale endpoint, got %d", rec.Code)
	}

	// 10. Audit log verification
	expectedAuditActions := []string{
		"scale:k8s_deployment:nginx-dep",
		"restart:k8s_deployment:nginx-dep",
		"scale:k8s_statefulset:redis-sts",
		"restart:k8s_daemonset:fluentd-ds",
		"trigger:k8s_cronjob:nightly-cron",
		"suspend:k8s_cronjob:nightly-cron",
		"resume:k8s_cronjob:nightly-cron",
	}
	for _, exp := range expectedAuditActions {
		found := false
		for _, act := range auditMock.actions {
			if act == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected audit action %s in recorded actions: %v", exp, auditMock.actions)
		}
	}
}
