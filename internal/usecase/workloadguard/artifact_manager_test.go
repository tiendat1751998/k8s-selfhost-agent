package workloadguard_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"

	domain "github.com/datdt/k8sselfhost/internal/domain/workloadguard"
	usecase "github.com/datdt/k8sselfhost/internal/usecase/workloadguard"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestArtifactManager_FloatingTagRejection verifies that mutable floating tags are rejected.
func TestArtifactManager_FloatingTagRejection(t *testing.T) {
	mgr := usecase.NewArtifactManager()
	ctx := context.Background()

	testCases := []struct {
		name  string
		image string
	}{
		{"latest tag", "nginx:latest"},
		{"staging tag", "myrepo/backend:staging"},
		{"dev tag", "company.io/app:dev"},
		{"implicit latest", "redis"},
		{"uppercase latest", "postgres:LATEST"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := mgr.ResolveImageToDigest(ctx, tc.image)
			assert.Nil(t, res)
			require.Error(t, err)
			assert.True(t, errors.Is(err, domain.ErrMutableImageTagRejected), "Expected ErrMutableImageTagRejected, got: %v", err)
		})
	}
}

// TestArtifactManager_StaticTagResolution verifies static tag mutation to immutable sha256 digest.
func TestArtifactManager_StaticTagResolution(t *testing.T) {
	const expectedDigest = "sha256:452e9ced7b5f1a5b772bd533d114c5a3739815587e90abb6fb1465630c1a5601"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v2/myapp/manifests/v1.2.3", r.URL.Path)
		w.Header().Set("Docker-Content-Digest", expectedDigest)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	u, err := url.Parse(server.URL)
	require.NoError(t, err)

	mgr := usecase.NewArtifactManager(
		usecase.WithHTTPClient(server.Client()),
		usecase.WithRegistryHostOverride(u.Host),
	)

	ctx := context.Background()
	testImage := u.Host + "/myapp:v1.2.3"
	res, err := mgr.ResolveImageToDigest(ctx, testImage)
	require.NoError(t, err)
	require.NotNil(t, res)

	assert.Equal(t, testImage, res.OriginalImage)
	assert.Equal(t, u.Host+"/myapp@"+expectedDigest, res.ResolvedDigest)
	assert.True(t, res.IsImmutable)

	// Second call should hit high-frequency cache
	resCached, err := mgr.ResolveImageToDigest(ctx, testImage)
	require.NoError(t, err)
	assert.Equal(t, res.ResolvedDigest, resCached.ResolvedDigest)
	assert.GreaterOrEqual(t, mgr.DigestCounter().Load(), uint64(1))
}

// TestArtifactManager_AlreadyImmutable verifies images with @sha256 are recognized as immutable.
func TestArtifactManager_AlreadyImmutable(t *testing.T) {
	mgr := usecase.NewArtifactManager()
	ctx := context.Background()

	img := "quay.io/coreos/etcd@sha256:452e9ced7b5f1a5b772bd533d114c5a3739815587e90abb6fb1465630c1a5601"
	res, err := mgr.ResolveImageToDigest(ctx, img)
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Equal(t, img, res.OriginalImage)
	assert.Equal(t, img, res.ResolvedDigest)
	assert.True(t, res.IsImmutable)
}

// TestArtifactManager_CircuitBreakerFailurePolicy verifies Ignore vs Fail on unreachable registry.
func TestArtifactManager_CircuitBreakerFailurePolicy(t *testing.T) {
	ctx := context.Background()

	// 1. FailurePolicy: Ignore (default)
	mgrIgnore := usecase.NewArtifactManager(
		usecase.WithFailurePolicy(usecase.FailurePolicyIgnore),
		usecase.WithRegistryHostOverride("127.0.0.1:59999"), // non-existent registry
	)
	res, err := mgrIgnore.ResolveImageToDigest(ctx, "127.0.0.1:59999/app:v1.0.0")
	require.NoError(t, err, "FailurePolicyIgnore must not return error on unreachable registry")
	require.NotNil(t, res)
	assert.Equal(t, "127.0.0.1:59999/app:v1.0.0", res.ResolvedDigest)

	// 2. FailurePolicy: Fail
	mgrFail := usecase.NewArtifactManager(
		usecase.WithFailurePolicy(usecase.FailurePolicyFail),
		usecase.WithRegistryHostOverride("127.0.0.1:59999"),
	)
	_, errFail := mgrFail.ResolveImageToDigest(ctx, "127.0.0.1:59999/app:v1.0.0")
	require.Error(t, errFail, "FailurePolicyFail must report error when registry is unreachable")
}

// TestArtifactManager_ValidateEphemeralStoragePolicy verifies storage limits and /tmp emptyDir check.
func TestArtifactManager_ValidateEphemeralStoragePolicy(t *testing.T) {
	mgr := usecase.NewArtifactManager()
	ctx := context.Background()

	// 1. Missing ephemeral storage limit
	podWithoutLimit := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "test-pod", Namespace: "default"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "app", Image: "app:v1.0.0"},
			},
		},
	}
	podJSON, err := json.Marshal(podWithoutLimit)
	require.NoError(t, err)

	check, err := mgr.ValidateEphemeralStoragePolicy(ctx, podJSON)
	require.NoError(t, err)
	assert.False(t, check.IsPassed)
	assert.False(t, check.HasLimit)
	assert.Contains(t, check.RejectReason, "ephemeral-storage")

	// 2. Has limit, but /tmp is not mounted as emptyDir
	podWithLimitNoTmp := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "test-pod", Namespace: "default"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "app",
					Image: "app:v1.0.0",
					Resources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceEphemeralStorage: resource.MustParse("2Gi"),
						},
					},
				},
			},
		},
	}
	podJSON2, err := json.Marshal(podWithLimitNoTmp)
	require.NoError(t, err)

	check2, err := mgr.ValidateEphemeralStoragePolicy(ctx, podJSON2)
	require.NoError(t, err)
	assert.False(t, check2.IsPassed)
	assert.True(t, check2.HasLimit)
	assert.False(t, check2.TmpMountedAsEmptyDir)
	assert.Contains(t, check2.RejectReason, "/tmp")

	// 3. Fully compliant pod (has limit and /tmp emptyDir)
	compliantPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "test-pod", Namespace: "default"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "app",
					Image: "app:v1.0.0",
					Resources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceEphemeralStorage: resource.MustParse("2Gi"),
						},
					},
					VolumeMounts: []corev1.VolumeMount{
						{Name: "tmp-dir", MountPath: "/tmp"},
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "tmp-dir",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{
							SizeLimit: resource.NewQuantity(1024*1024*1024, resource.BinarySI),
						},
					},
				},
			},
		},
	}
	podJSON3, err := json.Marshal(compliantPod)
	require.NoError(t, err)

	check3, err := mgr.ValidateEphemeralStoragePolicy(ctx, podJSON3)
	require.NoError(t, err)
	assert.True(t, check3.IsPassed)
	assert.True(t, check3.HasLimit)
	assert.True(t, check3.TmpMountedAsEmptyDir)
	assert.Equal(t, int64(2*1024*1024*1024), check3.LimitBytes)
}

// TestArtifactManager_InjectEphemeralStorageDefaults verifies 2Gi and /tmp injection.
func TestArtifactManager_InjectEphemeralStorageDefaults(t *testing.T) {
	mgr := usecase.NewArtifactManager()
	ctx := context.Background()

	rawPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "bare-pod", Namespace: "default"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "worker", Image: "worker:v1.0"},
			},
		},
	}
	rawBytes, err := json.Marshal(rawPod)
	require.NoError(t, err)

	mutatedBytes, err := mgr.InjectEphemeralStorageDefaults(ctx, rawBytes)
	require.NoError(t, err)

	// Validate mutated pod
	check, err := mgr.ValidateEphemeralStoragePolicy(ctx, mutatedBytes)
	require.NoError(t, err)
	assert.True(t, check.IsPassed)
	assert.True(t, check.HasLimit)
	assert.True(t, check.TmpMountedAsEmptyDir)
	assert.Equal(t, domain.DefaultEphemeralStorageLimitBytes, check.LimitBytes)
}

// TestArtifactManager_EnforcePodPIDLimit verifies 1024 PID injection and fork bomb rejection.
func TestArtifactManager_EnforcePodPIDLimit(t *testing.T) {
	mgr := usecase.NewArtifactManager()
	ctx := context.Background()

	rawPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "unconstrained-pod", Namespace: "default"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "api", Image: "api:v1.0"},
			},
		},
	}
	rawBytes, err := json.Marshal(rawPod)
	require.NoError(t, err)

	// 1. Enforce default PID limit (1024)
	enforcedBytes, err := mgr.EnforcePodPIDLimit(ctx, rawBytes, 1024)
	require.NoError(t, err)

	var resultPod corev1.Pod
	err = json.Unmarshal(enforcedBytes, &resultPod)
	require.NoError(t, err)

	assert.Equal(t, "1024", resultPod.Annotations["cgroup.kubernetes.io/pod-pids-limit"])
	assert.Equal(t, "1024", resultPod.Annotations["security.kubernetes.io/pod-pids-limit"])

	// 2. Reject fork bomb risk exceeding kernel limit (e.g. 65535 > 32768)
	_, errForkBomb := mgr.EnforcePodPIDLimit(ctx, rawBytes, 50000)
	require.Error(t, errForkBomb)
	assert.True(t, errors.Is(errForkBomb, domain.ErrPIDExhaustionRisk))
}

// TestArtifactManager_ConcurrentStress verifies thread-safety and padded counter updates.
func TestArtifactManager_ConcurrentStress(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Docker-Content-Digest", "sha256:abc1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	u, err := url.Parse(server.URL)
	require.NoError(t, err)

	mgr := usecase.NewArtifactManager(
		usecase.WithHTTPClient(server.Client()),
		usecase.WithRegistryHostOverride(u.Host),
	)

	ctx := context.Background()
	podBytes, _ := json.Marshal(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "stress-pod"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{Name: "c1", Image: "c1:v1.0"}},
		},
	})

	var wg sync.WaitGroup
	goroutines := 30
	iterations := 20

	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				_, _ = mgr.ResolveImageToDigest(ctx, u.Host+"/app:v1.0")
				_, _ = mgr.ValidateEphemeralStoragePolicy(ctx, podBytes)
				_, _ = mgr.EnforcePodPIDLimit(ctx, podBytes, 1024)
			}
		}()
	}

	wg.Wait()

	assert.Greater(t, mgr.DigestCounter().Load(), uint64(0))
	assert.Greater(t, mgr.StorageCounter().Load(), uint64(0))
	assert.Greater(t, mgr.PIDCounter().Load(), uint64(0))
}
