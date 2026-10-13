// Package k8s provides Kubernetes infrastructure implementations for rootless
// container builds, admission webhooks, and workload execution.
package k8s

import (
	"bufio"
	"context"
	"fmt"
	"sort"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	domainsec "github.com/datdt/k8sselfhost/internal/domain/security"
	"github.com/datdt/k8sselfhost/internal/pkg/errors"
)

const (
	// KanikoExecutorImage is the debug image containing busybox shell and rootless kaniko binary
	KanikoExecutorImage = "gcr.io/kaniko-project/executor:v1.20.0-debug"

	// DefaultKanikoNamespace is the default sandbox namespace for isolated builds
	DefaultKanikoNamespace = "build-sandbox"
)

// KanikoRunner implements domainsec.RootlessBuildRunner using Kubernetes client-go.
type KanikoRunner struct {
	client           kubernetes.Interface
	defaultNamespace string
}

// NewKanikoRunner creates a new KanikoRunner instance.
func NewKanikoRunner(client kubernetes.Interface, defaultNamespace string) *KanikoRunner {
	if defaultNamespace == "" {
		defaultNamespace = DefaultKanikoNamespace
	}
	return &KanikoRunner{
		client:           client,
		defaultNamespace: defaultNamespace,
	}
}

// BuildPodManifest generates a strictly compliant corev1.Pod manifest for Kaniko.
// Invariants enforced:
// - runAsNonRoot: true, runAsUser: 1000, fsGroup: 1000
// - Zero hostPath volumes, zero privileged flags, zero docker.sock mounts
// - Registry auth mounted to /kaniko/.docker/config.json
func (r *KanikoRunner) BuildPodManifest(spec domainsec.RootlessBuildSpec) (*corev1.Pod, error) {
	if spec.BuildID == "" {
		return nil, errors.NewValidation("build_id", "build_id cannot be empty")
	}
	if spec.ContextURL == "" {
		return nil, errors.NewValidation("context_url", "context_url cannot be empty")
	}
	if spec.Destination == "" {
		return nil, errors.NewValidation("destination", "destination registry image cannot be empty")
	}

	ns := spec.Namespace
	if ns == "" {
		ns = r.defaultNamespace
	}

	podName := fmt.Sprintf("kaniko-build-%s", spec.BuildID)
	dockerfile := spec.Dockerfile
	if dockerfile == "" {
		dockerfile = "Dockerfile"
	}

	// Kaniko CLI arguments
	args := []string{
		fmt.Sprintf("--dockerfile=%s", dockerfile),
		fmt.Sprintf("--context=%s", spec.ContextURL),
		fmt.Sprintf("--destination=%s", spec.Destination),
		"--cache=true",
	}

	// Sort build args deterministically
	if len(spec.BuildArgs) > 0 {
		keys := make([]string, 0, len(spec.BuildArgs))
		for k := range spec.BuildArgs {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			args = append(args, fmt.Sprintf("--build-arg=%s=%s", k, spec.BuildArgs[k]))
		}
	}

	nonRoot := true
	runAsUser := int64(1000)
	fsGroup := int64(1000)
	privileged := false

	volumes := make([]corev1.Volume, 0, 1)
	volumeMounts := make([]corev1.VolumeMount, 0, 1)

	// Mount registry auth secret if provided
	if spec.RegistryAuth != "" {
		volumes = append(volumes, corev1.Volume{
			Name: "registry-auth",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: spec.RegistryAuth,
					Items: []corev1.KeyToPath{
						{
							Key:  ".dockerconfigjson",
							Path: "config.json",
						},
					},
				},
			},
		})

		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      "registry-auth",
			MountPath: "/kaniko/.docker/config.json",
			SubPath:   "config.json",
			ReadOnly:  true,
		})
	}

	// Resources allocation
	resources := corev1.ResourceRequirements{
		Limits:   corev1.ResourceList{},
		Requests: corev1.ResourceList{},
	}
	if spec.MemoryLimit != "" {
		if q, err := resource.ParseQuantity(spec.MemoryLimit); err == nil {
			resources.Limits[corev1.ResourceMemory] = q
			resources.Requests[corev1.ResourceMemory] = q
		}
	}
	if spec.CPULimit != "" {
		if q, err := resource.ParseQuantity(spec.CPULimit); err == nil {
			resources.Limits[corev1.ResourceCPU] = q
			resources.Requests[corev1.ResourceCPU] = q
		}
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: ns,
			Labels: map[string]string{
				"app":       "kaniko-build",
				"build-id":  spec.BuildID,
				"managed-by": "k8sselfhost",
			},
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			SecurityContext: &corev1.PodSecurityContext{
				RunAsNonRoot: &nonRoot,
				RunAsUser:    &runAsUser,
				FSGroup:      &fsGroup,
			},
			Containers: []corev1.Container{
				{
					Name:            "kaniko",
					Image:           KanikoExecutorImage,
					ImagePullPolicy: corev1.PullIfNotPresent,
					Args:            args,
					Resources:       resources,
					VolumeMounts:    volumeMounts,
					SecurityContext: &corev1.SecurityContext{
						Privileged:               &privileged,
						AllowPrivilegeEscalation: &privileged,
					},
				},
			},
			Volumes: volumes,
		},
	}

	return pod, nil
}

// LaunchKanikoBuild launches an isolated Kaniko pod and returns the pod name.
func (r *KanikoRunner) LaunchKanikoBuild(ctx context.Context, spec domainsec.RootlessBuildSpec) (string, error) {
	pod, err := r.BuildPodManifest(spec)
	if err != nil {
		return "", err
	}

	if r.client == nil {
		return "", errors.NewK8sUnavailable("kubernetes client is unconfigured", nil)
	}

	created, err := r.client.CoreV1().Pods(pod.Namespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		return "", errors.Wrap(err, fmt.Sprintf("launching kaniko build pod '%s' in namespace '%s'", pod.Name, pod.Namespace))
	}

	return created.Name, nil
}

// StreamBuildLogs streams container logs from the running Kaniko pod line by line.
func (r *KanikoRunner) StreamBuildLogs(ctx context.Context, buildID string) (<-chan string, error) {
	if buildID == "" {
		return nil, errors.NewValidation("build_id", "build_id cannot be empty")
	}
	if r.client == nil {
		return nil, errors.NewK8sUnavailable("kubernetes client is unconfigured", nil)
	}

	podName := fmt.Sprintf("kaniko-build-%s", buildID)
	ns := r.defaultNamespace

	// Verify pod exists
	_, err := r.client.CoreV1().Pods(ns).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("locating kaniko build pod '%s'", podName))
	}

	req := r.client.CoreV1().Pods(ns).GetLogs(podName, &corev1.PodLogOptions{
		Container: "kaniko",
		Follow:    true,
	})

	stream, err := req.Stream(ctx)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("opening log stream for pod '%s'", podName))
	}

	outChan := make(chan string, 100)
	go func() {
		defer close(outChan)
		defer stream.Close()

		scanner := bufio.NewScanner(stream)
		for scanner.Scan() {
			select {
			case <-ctx.Done():
				return
			case outChan <- scanner.Text():
			}
		}
	}()

	return outChan, nil
}
