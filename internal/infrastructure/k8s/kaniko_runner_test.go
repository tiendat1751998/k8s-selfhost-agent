package k8s

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	domainsec "github.com/datdt/k8sselfhost/internal/domain/security"
)

func TestKaniko_PodSpecGeneration(t *testing.T) {
	runner := NewKanikoRunner(fake.NewClientset(), "build-sandbox")

	spec := domainsec.RootlessBuildSpec{
		BuildID:      "build-xyz-789",
		ContextURL:   "git://github.com/my-org/backend-service.git#refs/heads/main",
		Dockerfile:   "Dockerfile.prod",
		Destination:  "harbor.internal/apps/backend:v1.2.3",
		RegistryAuth: "harbor-pull-secret",
		BuildArgs: map[string]string{
			"GO_VERSION": "1.26",
			"ENV":        "production",
		},
		MemoryLimit: "2Gi",
		CPULimit:    "2000m",
	}

	pod, err := runner.BuildPodManifest(spec)
	require.NoError(t, err)
	require.NotNil(t, pod)

	// Invariant 1: Name and Namespace
	assert.Equal(t, "kaniko-build-build-xyz-789", pod.Name)
	assert.Equal(t, "build-sandbox", pod.Namespace)
	assert.Equal(t, corev1.RestartPolicyNever, pod.Spec.RestartPolicy)

	// Invariant 2: Rootless user & group (UID 1000, GID 1000)
	require.NotNil(t, pod.Spec.SecurityContext)
	require.NotNil(t, pod.Spec.SecurityContext.RunAsNonRoot)
	assert.True(t, *pod.Spec.SecurityContext.RunAsNonRoot)
	require.NotNil(t, pod.Spec.SecurityContext.RunAsUser)
	assert.Equal(t, int64(1000), *pod.Spec.SecurityContext.RunAsUser)
	require.NotNil(t, pod.Spec.SecurityContext.FSGroup)
	assert.Equal(t, int64(1000), *pod.Spec.SecurityContext.FSGroup)

	// Invariant 3: Single container running Kaniko executor debug image
	require.Len(t, pod.Spec.Containers, 1)
	c := pod.Spec.Containers[0]
	assert.Equal(t, "kaniko", c.Name)
	assert.Equal(t, "gcr.io/kaniko-project/executor:v1.20.0-debug", c.Image)

	// Invariant 4: Zero privileged container flags
	if c.SecurityContext != nil && c.SecurityContext.Privileged != nil {
		assert.False(t, *c.SecurityContext.Privileged)
	}

	// Invariant 5: CLI args formatting
	args := strings.Join(c.Args, " ")
	assert.Contains(t, args, "--dockerfile=Dockerfile.prod")
	assert.Contains(t, args, "--context=git://github.com/my-org/backend-service.git#refs/heads/main")
	assert.Contains(t, args, "--destination=harbor.internal/apps/backend:v1.2.3")
	assert.Contains(t, args, "--cache=true")
	assert.Contains(t, args, "--build-arg=GO_VERSION=1.26")
	assert.Contains(t, args, "--build-arg=ENV=production")

	// Invariant 6: Registry auth secret mounted to /kaniko/.docker/config.json
	var authMount *corev1.VolumeMount
	for _, m := range c.VolumeMounts {
		if m.MountPath == "/kaniko/.docker/config.json" {
			authMount = &m
			break
		}
	}
	require.NotNil(t, authMount, "Expected volumeMount at /kaniko/.docker/config.json")
	assert.Equal(t, "registry-auth", authMount.Name)

	var authVol *corev1.Volume
	for _, v := range pod.Spec.Volumes {
		if v.Name == "registry-auth" {
			authVol = &v
			break
		}
	}
	require.NotNil(t, authVol, "Expected secret volume 'registry-auth'")
	require.NotNil(t, authVol.Secret)
	assert.Equal(t, "harbor-pull-secret", authVol.Secret.SecretName)

	// Invariant 7: ZERO HostPath mounts anywhere!
	for _, v := range pod.Spec.Volumes {
		assert.Nil(t, v.HostPath, "Pod must not contain any hostPath volume")
	}

	// Invariant 8: ZERO docker.sock mounts anywhere!
	for _, m := range c.VolumeMounts {
		assert.NotContains(t, m.MountPath, "docker.sock")
		assert.NotContains(t, m.MountPath, "containerd.sock")
	}

	// Invariant 9: Resource requests/limits properly populated
	assert.False(t, c.Resources.Limits.Memory().IsZero())
	assert.False(t, c.Resources.Limits.Cpu().IsZero())
}

func TestKaniko_LaunchKanikoBuild(t *testing.T) {
	client := fake.NewClientset()
	runner := NewKanikoRunner(client, "build-sandbox")
	ctx := context.Background()

	spec := domainsec.RootlessBuildSpec{
		BuildID:     "b-1001",
		ContextURL:  "https://github.com/org/repo.tar.gz",
		Dockerfile:  "Dockerfile",
		Destination: "registry.local/demo:1.0.0",
	}

	podName, err := runner.LaunchKanikoBuild(ctx, spec)
	require.NoError(t, err)
	assert.Equal(t, "kaniko-build-b-1001", podName)

	// Verify pod actually created in Kubernetes fake clientset
	created, err := client.CoreV1().Pods("build-sandbox").Get(ctx, podName, metav1.GetOptions{})
	require.NoError(t, err)
	assert.Equal(t, podName, created.Name)
	assert.Equal(t, "b-1001", created.Labels["build-id"])
}

func TestKaniko_LaunchKanikoBuild_ValidationFailures(t *testing.T) {
	client := fake.NewClientset()
	runner := NewKanikoRunner(client, "build-sandbox")
	ctx := context.Background()

	t.Run("empty buildID rejected", func(t *testing.T) {
		_, err := runner.LaunchKanikoBuild(ctx, domainsec.RootlessBuildSpec{
			ContextURL:  "git://...",
			Destination: "reg/app:1",
		})
		assert.Error(t, err)
	})

	t.Run("empty contextURL rejected", func(t *testing.T) {
		_, err := runner.LaunchKanikoBuild(ctx, domainsec.RootlessBuildSpec{
			BuildID:     "b-1",
			Destination: "reg/app:1",
		})
		assert.Error(t, err)
	})

	t.Run("empty destination rejected", func(t *testing.T) {
		_, err := runner.LaunchKanikoBuild(ctx, domainsec.RootlessBuildSpec{
			BuildID:    "b-1",
			ContextURL: "git://...",
		})
		assert.Error(t, err)
	})
}

func TestKaniko_StreamBuildLogs_NotFound(t *testing.T) {
	client := fake.NewClientset()
	runner := NewKanikoRunner(client, "build-sandbox")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := runner.StreamBuildLogs(ctx, "nonexistent-build")
	assert.Error(t, err)
}
