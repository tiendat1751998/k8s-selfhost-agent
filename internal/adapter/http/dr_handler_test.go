package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.uber.org/zap/zaptest"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/go-chi/chi/v5"
	"github.com/datdt/k8sselfhost/internal/adapter/http/middleware"
	"github.com/datdt/k8sselfhost/internal/pkg/health"
	"github.com/datdt/k8sselfhost/internal/usecase/dr"
	"github.com/datdt/k8sselfhost/internal/usecase/sre"
)

func setupDRTestRouter(t *testing.T) (http.Handler, string, string, string, *fake.Clientset) {
	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "worker-test-1",
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

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "web-pod",
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			NodeName: "worker-test-1",
		},
	}

	client := fake.NewSimpleClientset(node, pod)
	logger := zaptest.NewLogger(t)

	remediationCtrl := sre.NewRemediationController(client, nil, logger)
	etcdDriver := dr.NewEtcdctlDriver(t.TempDir(), dr.WithIsolatedTestMode())
	drUc := dr.NewDRUsecase(client, nil, logger, dr.WithEtcdDriver(etcdDriver))
	drH := NewDRHandler(drUc, remediationCtrl, logger)

	platform := &PlatformHandlers{
		DR: drH,
	}

	healthHandler := health.NewHandler(5 * time.Second)
	router := NewRouterWithWS(healthHandler, nil, platform)

	adminToken, err := middleware.GenerateJWT("admin", "platform_admin", "default-tenant")
	if err != nil {
		t.Fatalf("failed to generate admin token: %v", err)
	}

	tenantAdminToken, err := middleware.GenerateJWT("t-admin", "tenant_admin", "default-tenant")
	if err != nil {
		t.Fatalf("failed to generate tenant admin token: %v", err)
	}

	viewerToken, err := middleware.GenerateJWT("viewer", "viewer", "default-tenant")
	if err != nil {
		t.Fatalf("failed to generate viewer token: %v", err)
	}

	return router, adminToken, tenantAdminToken, viewerToken, client
}

func TestDRHandler_RBAC_Enforcement(t *testing.T) {
	router, adminToken, tenantAdminToken, viewerToken, _ := setupDRTestRouter(t)

	t.Run("GET backups allows viewer and admin", func(t *testing.T) {
		// Viewer can read backups
		req := httptest.NewRequest(http.MethodGet, "/api/v1/k8s/local/dr/backups", nil)
		req.Header.Set("Authorization", "Bearer "+viewerToken)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200 OK for viewer, got %d: %s", rec.Code, rec.Body.String())
		}

		// Platform admin can read backups
		reqAdmin := httptest.NewRequest(http.MethodGet, "/api/v1/k8s/local/dr/backups", nil)
		reqAdmin.Header.Set("Authorization", "Bearer "+adminToken)
		recAdmin := httptest.NewRecorder()
		router.ServeHTTP(recAdmin, reqAdmin)

		if recAdmin.Code != http.StatusOK {
			t.Errorf("expected 200 OK for admin, got %d: %s", recAdmin.Code, recAdmin.Body.String())
		}
	})

	t.Run("Mutating endpoints reject viewer with 403", func(t *testing.T) {
		mutations := []struct {
			name   string
			method string
			path   string
			body   string
		}{
			{
				name:   "TriggerEtcdSnapshot",
				method: http.MethodPost,
				path:   "/api/v1/k8s/local/dr/etcd/snapshot",
				body:   "",
			},
			{
				name:   "RestoreEtcdSnapshot",
				method: http.MethodPost,
				path:   "/api/v1/k8s/local/dr/etcd/restore",
				body:   `{"snapshot_id":"snap-1"}`,
			},
			{
				name:   "CreateBackup",
				method: http.MethodPost,
				path:   "/api/v1/k8s/local/dr/backups",
				body:   `{"backup_name":"test"}`,
			},
			{
				name:   "TriggerRemediation",
				method: http.MethodPost,
				path:   "/api/v1/k8s/local/dr/remediation/worker-test-1",
				body:   "",
			},
		}

		for _, tc := range mutations {
			t.Run(tc.name, func(t *testing.T) {
				req := httptest.NewRequest(tc.method, tc.path, bytes.NewBufferString(tc.body))
				req.Header.Set("Authorization", "Bearer "+viewerToken)
				req.Header.Set("Content-Type", "application/json")
				rec := httptest.NewRecorder()
				router.ServeHTTP(rec, req)

				if rec.Code != http.StatusForbidden {
					t.Errorf("expected 403 Forbidden for %s, got %d: %s", tc.name, rec.Code, rec.Body.String())
				}
			})
		}
	})

	t.Run("Mutating endpoints succeed with platform_admin and tenant_admin", func(t *testing.T) {
		// 1. Etcd snapshot
		reqSnap := httptest.NewRequest(http.MethodPost, "/api/v1/k8s/local/dr/etcd/snapshot", nil)
		reqSnap.Header.Set("Authorization", "Bearer "+adminToken)
		recSnap := httptest.NewRecorder()
		router.ServeHTTP(recSnap, reqSnap)

		if recSnap.Code != http.StatusCreated {
			t.Fatalf("expected 201 Created for snapshot, got %d: %s", recSnap.Code, recSnap.Body.String())
		}

		var snapResult dr.EtcdSnapshotResult
		if err := json.NewDecoder(recSnap.Body).Decode(&snapResult); err != nil {
			t.Fatalf("failed to decode snapshot response: %v", err)
		}
		if snapResult.SnapshotID == "" || snapResult.Status != "Completed" {
			t.Errorf("unexpected snapshot result: %+v", snapResult)
		}

		// 2. Etcd restore with tenant_admin
		restoreBody := `{"snapshot_id":"` + snapResult.SnapshotID + `"}`
		reqRestore := httptest.NewRequest(http.MethodPost, "/api/v1/k8s/local/dr/etcd/restore", bytes.NewBufferString(restoreBody))
		reqRestore.Header.Set("Authorization", "Bearer "+tenantAdminToken)
		reqRestore.Header.Set("Content-Type", "application/json")
		recRestore := httptest.NewRecorder()
		router.ServeHTTP(recRestore, reqRestore)

		if recRestore.Code != http.StatusOK {
			t.Fatalf("expected 200 OK for restore, got %d: %s", recRestore.Code, recRestore.Body.String())
		}

		// 3. Create backup
		backupBody := `{"backup_name":"api-backup-01"}`
		reqBackup := httptest.NewRequest(http.MethodPost, "/api/v1/k8s/local/dr/backups", bytes.NewBufferString(backupBody))
		reqBackup.Header.Set("Authorization", "Bearer "+adminToken)
		reqBackup.Header.Set("Content-Type", "application/json")
		recBackup := httptest.NewRecorder()
		router.ServeHTTP(recBackup, reqBackup)

		if recBackup.Code != http.StatusCreated {
			t.Fatalf("expected 201 Created for backup, got %d: %s", recBackup.Code, recBackup.Body.String())
		}

		// 4. Trigger remediation
		reqRemed := httptest.NewRequest(http.MethodPost, "/api/v1/k8s/local/dr/remediation/worker-test-1", nil)
		reqRemed.Header.Set("Authorization", "Bearer "+tenantAdminToken)
		recRemed := httptest.NewRecorder()
		router.ServeHTTP(recRemed, reqRemed)

		if recRemed.Code != http.StatusOK {
			t.Fatalf("expected 200 OK for remediation, got %d: %s", recRemed.Code, recRemed.Body.String())
		}

		var remedResult sre.RemediationResult
		if err := json.NewDecoder(recRemed.Body).Decode(&remedResult); err != nil {
			t.Fatalf("failed to decode remediation response: %v", err)
		}
		if remedResult.NodeName != "worker-test-1" {
			t.Errorf("expected node worker-test-1, got %s", remedResult.NodeName)
		}
		if len(remedResult.EvictedPods) != 1 || remedResult.EvictedPods[0] != "web-pod" {
			t.Errorf("expected evicted pod web-pod, got %v", remedResult.EvictedPods)
		}
	})
}

func TestDRHandler_RestoreEtcdSnapshot_BadRequest(t *testing.T) {
	router, adminToken, _, _, _ := setupDRTestRouter(t)

	// Missing body
	req := httptest.NewRequest(http.MethodPost, "/api/v1/k8s/local/dr/etcd/restore", bytes.NewBufferString(`{}`))
	req.Header.Set("Authorization", "Bearer "+adminToken)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for empty snapshot_id, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestDRHandler_NilGracefulHandling(t *testing.T) {
	healthHandler := health.NewHandler(5 * time.Second)
	// Platform with DR == nil
	platform := &PlatformHandlers{
		DR: nil,
	}
	router := NewRouterWithWS(healthHandler, nil, platform)

	adminToken, err := middleware.GenerateJWT("admin", "platform_admin", "default-tenant")
	if err != nil {
		t.Fatalf("failed to generate admin token: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/k8s/local/dr/backups", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	rec := httptest.NewRecorder()

	// Should not panic, should return 503 or 404
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable && rec.Code != http.StatusNotFound {
		t.Errorf("expected 503 or 404 when DR is nil, got %d", rec.Code)
	}
}

func setupFencingTestRouter(t *testing.T) (http.Handler, string, string, string, *fake.Clientset) {
	master := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "master-1", Labels: map[string]string{"node-role.kubernetes.io/control-plane": ""}},
		Spec:       corev1.NodeSpec{Taints: []corev1.Taint{{Key: "node-role.kubernetes.io/control-plane", Effect: corev1.TaintEffectNoSchedule}}},
	}
	worker := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "worker-dead"},
		Status:     corev1.NodeStatus{Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionFalse}, {Type: corev1.NodeMemoryPressure, Status: corev1.ConditionTrue}}},
	}
	stateless := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "default"}, Spec: corev1.PodSpec{NodeName: "worker-dead"}}
	stateful := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "db", Namespace: "default"},
		Spec:       corev1.PodSpec{NodeName: "worker-dead", Volumes: []corev1.Volume{{Name: "data", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "data-pvc"}}}}},
	}
	client := fake.NewSimpleClientset(master, worker, stateless, stateful)
	logger := zaptest.NewLogger(t)

	remediationCtrl := sre.NewRemediationController(client, nil, logger)
	drUc := dr.NewDRUsecase(client, nil, logger)
	drH := NewDRHandler(drUc, remediationCtrl, logger)

	r := chi.NewRouter()
	r.Use(middleware.JWTAuthMiddleware)
	r.Route("/api/v1", func(api chi.Router) {
		drH.WireToRouter(api)
		api.Route("/k8s/{cluster}/dr", drH.RegisterRoutes)
	})

	adminToken, _ := middleware.GenerateJWT("admin", "platform_admin", "default-tenant")
	tenantAdminToken, _ := middleware.GenerateJWT("t-admin", "tenant_admin", "default-tenant")
	viewerToken, _ := middleware.GenerateJWT("viewer", "viewer", "default-tenant")

	return r, adminToken, tenantAdminToken, viewerToken, client
}

func TestDRHandler_PreflightFencing(t *testing.T) {
	router, _, _, viewerToken, _ := setupFencingTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/clusters/local/nodes/worker-dead/fencing/preflight", nil)
	req.Header.Set("Authorization", "Bearer "+viewerToken)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for preflight, got %d: %s", rec.Code, rec.Body.String())
	}

	var preflight sre.NodeFencingPreflight
	if err := json.NewDecoder(rec.Body).Decode(&preflight); err != nil {
		t.Fatalf("failed to decode preflight response: %v", err)
	}
	if !preflight.IsSingleWorkerCluster || preflight.RCA.ProbableCause != "OOM_KILLER_CASCADE" {
		t.Errorf("unexpected preflight data: %+v", preflight)
	}

	// Relative route under /k8s/{cluster}/dr
	reqRel := httptest.NewRequest(http.MethodGet, "/api/v1/k8s/local/dr/nodes/worker-dead/fencing/preflight", nil)
	reqRel.Header.Set("Authorization", "Bearer "+viewerToken)
	recRel := httptest.NewRecorder()
	router.ServeHTTP(recRel, reqRel)
	if recRel.Code != http.StatusOK {
		t.Errorf("expected 200 OK for relative preflight, got %d", recRel.Code)
	}
}

func TestDRHandler_ExecuteFencing_AntiSplitBrainAndRBAC(t *testing.T) {
	router, adminToken, _, viewerToken, _ := setupFencingTestRouter(t)

	// 1. Viewer is forbidden
	reqViewer := httptest.NewRequest(http.MethodPost, "/api/v1/clusters/local/nodes/worker-dead/fencing/execute", bytes.NewBufferString(`{"confirm_power_isolated":true}`))
	reqViewer.Header.Set("Authorization", "Bearer "+viewerToken)
	reqViewer.Header.Set("Content-Type", "application/json")
	recViewer := httptest.NewRecorder()
	router.ServeHTTP(recViewer, reqViewer)
	if recViewer.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden for viewer execute, got %d", recViewer.Code)
	}

	// 2. Admin with confirm_power_isolated=false rejected with 400 Bad Request
	reqUnsafe := httptest.NewRequest(http.MethodPost, "/api/v1/clusters/local/nodes/worker-dead/fencing/execute", bytes.NewBufferString(`{"confirm_power_isolated":false}`))
	reqUnsafe.Header.Set("Authorization", "Bearer "+adminToken)
	reqUnsafe.Header.Set("Content-Type", "application/json")
	recUnsafe := httptest.NewRecorder()
	router.ServeHTTP(recUnsafe, reqUnsafe)
	if recUnsafe.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for split-brain rejection, got %d: %s", recUnsafe.Code, recUnsafe.Body.String())
	}
}

func TestDRHandler_ExecuteFencing_SuccessPipeline(t *testing.T) {
	router, _, tenantAdminToken, _, client := setupFencingTestRouter(t)

	body := `{"confirm_power_isolated":true,"emergency_untaint_master":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/clusters/local/nodes/worker-dead/fencing/execute", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer "+tenantAdminToken)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for execute, got %d: %s", rec.Code, rec.Body.String())
	}

	var report sre.FencingExecutionReport
	if err := json.NewDecoder(rec.Body).Decode(&report); err != nil {
		t.Fatalf("failed to decode report: %v", err)
	}
	if !report.Success || !report.MasterUntainted {
		t.Errorf("unexpected report output: %+v", report)
	}

	// Verify master untainted in fake client
	master, _ := client.CoreV1().Nodes().Get(context.Background(), "master-1", metav1.GetOptions{})
	for _, taint := range master.Spec.Taints {
		if taint.Key == "node-role.kubernetes.io/control-plane" && taint.Effect == corev1.TaintEffectNoSchedule {
			t.Errorf("expected control-plane taint removed, still present: %+v", taint)
		}
	}
}