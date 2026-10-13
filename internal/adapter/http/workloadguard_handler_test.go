package http_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	adapterhttp "github.com/datdt/k8sselfhost/internal/adapter/http"
	domain "github.com/datdt/k8sselfhost/internal/domain/workloadguard"
	usecase "github.com/datdt/k8sselfhost/internal/usecase/workloadguard"
)

// setupWorkloadGuardRouter wires the WorkloadGuardHandler to a Chi router.
func setupWorkloadGuardRouter(guard domain.WorkloadArtifactGuard) *chi.Mux {
	r := chi.NewRouter()
	handler := adapterhttp.NewWorkloadGuardHandler(guard)
	handler.RegisterRoutes(r)
	return r
}

// TestWorkloadGuardHandler_ResolveDigest_FloatingTagRejected asserts HTTP 422 on mutable tags.
func TestWorkloadGuardHandler_ResolveDigest_FloatingTagRejected(t *testing.T) {
	mgr := usecase.NewArtifactManager()
	router := setupWorkloadGuardRouter(mgr)

	body, _ := json.Marshal(map[string]string{
		"image": "nginx:latest",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workload/images/resolve-digest", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	var problem adapterhttp.ProblemDetails
	err := json.Unmarshal(w.Body.Bytes(), &problem)
	require.NoError(t, err)
	assert.Equal(t, domain.CodeMutableImageTagRejected, problem.Code)
}

// TestWorkloadGuardHandler_ResolveDigest_StaticTagSuccess asserts HTTP 200 on immutable digest.
func TestWorkloadGuardHandler_ResolveDigest_StaticTagSuccess(t *testing.T) {
	const testDigest = "sha256:452e9ced7b5f1a5b772bd533d114c5a3739815587e90abb6fb1465630c1a5601"
	regServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Docker-Content-Digest", testDigest)
		w.WriteHeader(http.StatusOK)
	}))
	defer regServer.Close()

	mgr := usecase.NewArtifactManager(
		usecase.WithHTTPClient(regServer.Client()),
		usecase.WithRegistryHostOverride(regServer.Listener.Addr().String()),
	)
	router := setupWorkloadGuardRouter(mgr)

	body, _ := json.Marshal(map[string]string{
		"image": regServer.Listener.Addr().String() + "/app:v1.0.0",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/workload/images/resolve-digest", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp domain.ImageDigestResolution
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.True(t, resp.IsImmutable)
	assert.Contains(t, resp.ResolvedDigest, testDigest)
}

// TestWorkloadGuardHandler_ValidateEphemeral_MissingLimitRejected asserts HTTP 400 when limits missing.
func TestWorkloadGuardHandler_ValidateEphemeral_MissingLimitRejected(t *testing.T) {
	mgr := usecase.NewArtifactManager()
	router := setupWorkloadGuardRouter(mgr)

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "bad-pod"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "c1", Image: "c1:v1.0"}},
		},
	}
	podBytes, _ := json.Marshal(pod)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/workload/storage/validate-ephemeral", bytes.NewReader(podBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var problem adapterhttp.ProblemDetails
	err := json.Unmarshal(w.Body.Bytes(), &problem)
	require.NoError(t, err)
	assert.Equal(t, domain.CodeEphemeralStorageLimitMissing, problem.Code)
}

// TestWorkloadGuardHandler_ValidateEphemeral_CompliantPassed asserts HTTP 200 when compliant.
func TestWorkloadGuardHandler_ValidateEphemeral_CompliantPassed(t *testing.T) {
	mgr := usecase.NewArtifactManager()
	router := setupWorkloadGuardRouter(mgr)

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "good-pod"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "c1",
					Image: "c1:v1.0",
					Resources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceEphemeralStorage: resource.MustParse("2Gi"),
						},
					},
					VolumeMounts: []corev1.VolumeMount{
						{Name: "tmp", MountPath: "/tmp"},
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "tmp",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{
							SizeLimit: resource.NewQuantity(1024*1024*1024, resource.BinarySI),
						},
					},
				},
			},
		},
	}
	podBytes, _ := json.Marshal(pod)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/workload/storage/validate-ephemeral", bytes.NewReader(podBytes))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var check domain.EphemeralStorageCheck
	err := json.Unmarshal(w.Body.Bytes(), &check)
	require.NoError(t, err)
	assert.True(t, check.IsPassed)
	assert.True(t, check.HasLimit)
	assert.True(t, check.TmpMountedAsEmptyDir)
}

// TestWorkloadGuardHandler_EnforcePIDLimits_Success asserts HTTP 200 with injected PID limits.
func TestWorkloadGuardHandler_EnforcePIDLimits_Success(t *testing.T) {
	mgr := usecase.NewArtifactManager()
	router := setupWorkloadGuardRouter(mgr)

	payload, _ := json.Marshal(map[string]interface{}{
		"pod_spec": map[string]interface{}{
			"metadata": map[string]interface{}{"name": "target-pod"},
			"spec": map[string]interface{}{
				"containers": []interface{}{
					map[string]interface{}{"name": "app", "image": "app:v1.0"},
				},
			},
		},
		"default_max_pids": 1024,
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/workload/pids/enforce-limits", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "enforced", resp["status"])
	assert.Equal(t, float64(1024), resp["pod_pids_limit"])
}

// TestWorkloadGuardHandler_EnforcePIDLimits_ForkBombRisk asserts HTTP 429 when PID limit exceeds kernel max.
func TestWorkloadGuardHandler_EnforcePIDLimits_ForkBombRisk(t *testing.T) {
	mgr := usecase.NewArtifactManager()
	router := setupWorkloadGuardRouter(mgr)

	payload, _ := json.Marshal(map[string]interface{}{
		"pod_spec": map[string]interface{}{
			"metadata": map[string]interface{}{"name": "fork-bomb-pod"},
		},
		"default_max_pids": 65535, // > 32768
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/workload/pids/enforce-limits", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusTooManyRequests, w.Code)
	var problem adapterhttp.ProblemDetails
	err := json.Unmarshal(w.Body.Bytes(), &problem)
	require.NoError(t, err)
	assert.Equal(t, domain.CodePIDExhaustionRisk, problem.Code)
}
