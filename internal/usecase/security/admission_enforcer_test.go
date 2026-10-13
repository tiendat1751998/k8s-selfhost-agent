package security

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	domainsec "github.com/datdt/k8sselfhost/internal/domain/security"
)

func makeAdmissionReview(namespace, name string, pod *corev1.Pod) *admissionv1.AdmissionReview {
	raw, _ := json.Marshal(pod)
	return &admissionv1.AdmissionReview{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "admission.k8s.io/v1",
			Kind:       "AdmissionReview",
		},
		Request: &admissionv1.AdmissionRequest{
			UID:       "test-uid-12345",
			Kind:      metav1.GroupVersionKind{Group: "", Version: "v1", Kind: "Pod"},
			Resource:  metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
			Name:      name,
			Namespace: namespace,
			Object:    runtime.RawExtension{Raw: raw},
		},
	}
}

func makeCompliantPod() *corev1.Pod {
	readOnly := true
	privileged := false
	runAsUser := int64(1000)
	runAsNonRoot := true

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "secure-workload",
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			SecurityContext: &corev1.PodSecurityContext{
				RunAsNonRoot: &runAsNonRoot,
				RunAsUser:    &runAsUser,
			},
			Containers: []corev1.Container{
				{
					Name:  "app",
					Image: "harbor.internal/apps/payment:v2.1.0@sha256:7176805178696860000000000000000000000000000000000000000000000000",
					SecurityContext: &corev1.SecurityContext{
						ReadOnlyRootFilesystem: &readOnly,
						Privileged:             &privileged,
					},
				},
			},
		},
	}
}

func TestAdmission_DockerSockMount_Rejected(t *testing.T) {
	enforcer := NewAdmissionEnforcer()
	ctx := context.Background()

	t.Run("hostPath docker.sock rejected", func(t *testing.T) {
		pod := makeCompliantPod()
		pod.Spec.Volumes = append(pod.Spec.Volumes, corev1.Volume{
			Name: "docker-socket",
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/var/run/docker.sock",
				},
			},
		})
		review := makeAdmissionReview("production", "bad-pod", pod)

		res, err := enforcer.ValidatePodSpec(ctx, review)
		require.NoError(t, err)
		assert.False(t, res.Allowed)
		assert.Equal(t, int32(403), res.StatusCode)

		hasDinDViolation := false
		for _, v := range res.Violations {
			if v.Rule == domainsec.CodeDinDNotAllowed {
				hasDinDViolation = true
				break
			}
		}
		assert.True(t, hasDinDViolation, "Expected ERR_DIND_NOT_ALLOWED violation")
	})

	t.Run("volumeMount containerd.sock rejected", func(t *testing.T) {
		pod := makeCompliantPod()
		pod.Spec.Containers[0].VolumeMounts = append(pod.Spec.Containers[0].VolumeMounts, corev1.VolumeMount{
			Name:      "containerd-sock",
			MountPath: "/var/run/containerd/containerd.sock",
		})
		review := makeAdmissionReview("production", "bad-pod-containerd", pod)

		res, err := enforcer.ValidatePodSpec(ctx, review)
		require.NoError(t, err)
		assert.False(t, res.Allowed)
		assert.Equal(t, int32(403), res.StatusCode)
	})
}

func TestAdmission_LatestTag_Rejected(t *testing.T) {
	enforcer := NewAdmissionEnforcer()
	ctx := context.Background()

	t.Run("explicit latest tag rejected", func(t *testing.T) {
		pod := makeCompliantPod()
		pod.Spec.Containers[0].Image = "nginx:latest"
		review := makeAdmissionReview("production", "pod-latest", pod)

		res, err := enforcer.ValidatePodSpec(ctx, review)
		require.NoError(t, err)
		assert.False(t, res.Allowed)
		assert.Equal(t, int32(422), res.StatusCode)

		hasTagViolation := false
		for _, v := range res.Violations {
			if v.Rule == "ERR_IMMUTABLE_TAGS_REQUIRED" {
				hasTagViolation = true
				break
			}
		}
		assert.True(t, hasTagViolation, "Expected ERR_IMMUTABLE_TAGS_REQUIRED violation")
	})

	t.Run("missing tag rejected", func(t *testing.T) {
		pod := makeCompliantPod()
		pod.Spec.Containers[0].Image = "nginx"
		review := makeAdmissionReview("production", "pod-no-tag", pod)

		res, err := enforcer.ValidatePodSpec(ctx, review)
		require.NoError(t, err)
		assert.False(t, res.Allowed)
		assert.Equal(t, int32(422), res.StatusCode)
	})
}

func TestAdmission_Privileged_Rejected(t *testing.T) {
	enforcer := NewAdmissionEnforcer()
	ctx := context.Background()

	pod := makeCompliantPod()
	isPrivileged := true
	pod.Spec.Containers[0].SecurityContext.Privileged = &isPrivileged
	review := makeAdmissionReview("production", "pod-privileged", pod)

	res, err := enforcer.ValidatePodSpec(ctx, review)
	require.NoError(t, err)
	assert.False(t, res.Allowed)
	assert.Equal(t, int32(422), res.StatusCode)

	hasPrivViolation := false
	for _, v := range res.Violations {
		if v.Rule == "ERR_PRIVILEGED_FORBIDDEN" {
			hasPrivViolation = true
			break
		}
	}
	assert.True(t, hasPrivViolation, "Expected ERR_PRIVILEGED_FORBIDDEN violation")
}

func TestAdmission_ReadOnlyRootFS_Enforced(t *testing.T) {
	enforcer := NewAdmissionEnforcer()
	ctx := context.Background()

	pod := makeCompliantPod()
	readOnlyFalse := false
	pod.Spec.Containers[0].SecurityContext.ReadOnlyRootFilesystem = &readOnlyFalse
	review := makeAdmissionReview("production", "pod-writable-root", pod)

	res, err := enforcer.ValidatePodSpec(ctx, review)
	require.NoError(t, err)
	assert.False(t, res.Allowed)
	assert.Equal(t, int32(422), res.StatusCode)

	hasRootViolation := false
	for _, v := range res.Violations {
		if v.Rule == "ERR_READONLY_ROOTFS_REQUIRED" {
			hasRootViolation = true
			break
		}
	}
	assert.True(t, hasRootViolation, "Expected ERR_READONLY_ROOTFS_REQUIRED violation")
}

func TestAdmission_CompliantPod_Allowed(t *testing.T) {
	enforcer := NewAdmissionEnforcer()
	ctx := context.Background()

	pod := makeCompliantPod()
	review := makeAdmissionReview("default", "compliant-pod", pod)

	res, err := enforcer.ValidatePodSpec(ctx, review)
	require.NoError(t, err)
	assert.True(t, res.Allowed)
	assert.Equal(t, int32(200), res.StatusCode)
	assert.Empty(t, res.Violations)
}

func TestAdmission_KubeSystem_Exempt(t *testing.T) {
	enforcer := NewAdmissionEnforcer()
	ctx := context.Background()

	// Pod has violations (mounts docker.sock, privileged, latest tag)
	pod := makeCompliantPod()
	pod.Spec.Volumes = append(pod.Spec.Volumes, corev1.Volume{
		Name: "docker-sock",
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Path: "/var/run/docker.sock",
			},
		},
	})
	isPrivileged := true
	pod.Spec.Containers[0].SecurityContext.Privileged = &isPrivileged
	pod.Spec.Containers[0].Image = "coredns:latest"

	t.Run("kube-system exempt", func(t *testing.T) {
		review := makeAdmissionReview("kube-system", "kube-system-pod", pod)
		res, err := enforcer.ValidatePodSpec(ctx, review)
		require.NoError(t, err)
		assert.True(t, res.Allowed)
		assert.Equal(t, int32(200), res.StatusCode)
	})

	t.Run("kube-public exempt", func(t *testing.T) {
		review := makeAdmissionReview("kube-public", "kube-public-pod", pod)
		res, err := enforcer.ValidatePodSpec(ctx, review)
		require.NoError(t, err)
		assert.True(t, res.Allowed)
		assert.Equal(t, int32(200), res.StatusCode)
	})
}

func BenchmarkAdmissionEnforcer(b *testing.B) {
	enforcer := NewAdmissionEnforcer()
	ctx := context.Background()
	pod := makeCompliantPod()
	review := makeAdmissionReview("default", "bench-pod", pod)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		res, err := enforcer.ValidatePodSpec(ctx, review)
		if err != nil || !res.Allowed {
			b.Fatalf("unexpected validation outcome: res=%v err=%v", res, err)
		}
	}
}
