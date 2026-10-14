package kubernetes

import (
	"context"
	"strings"
	"testing"

	"k8s.io/client-go/kubernetes/fake"
)

func TestApplyYAML_SecurityPolicy_Rejections(t *testing.T) {
	tests := []struct {
		name        string
		yaml        string
		errContains string
	}{
		{
			name: "reject privileged container in Pod",
			yaml: `
apiVersion: v1
kind: Pod
metadata:
  name: privileged-pod
  namespace: default
spec:
  containers:
  - name: app
    image: nginx
    securityContext:
      privileged: true
`,
			errContains: "privileged",
		},
		{
			name: "reject hostNetwork in Pod",
			yaml: `
apiVersion: v1
kind: Pod
metadata:
  name: hostnet-pod
  namespace: default
spec:
  hostNetwork: true
  containers:
  - name: app
    image: nginx
`,
			errContains: "hostNetwork",
		},
		{
			name: "reject hostPID in Pod",
			yaml: `
apiVersion: v1
kind: Pod
metadata:
  name: hostpid-pod
  namespace: default
spec:
  hostPID: true
  containers:
  - name: app
    image: nginx
`,
			errContains: "hostPID",
		},
		{
			name: "reject hostIPC in Pod",
			yaml: `
apiVersion: v1
kind: Pod
metadata:
  name: hostipc-pod
  namespace: default
spec:
  hostIPC: true
  containers:
  - name: app
    image: nginx
`,
			errContains: "hostIPC",
		},
		{
			name: "reject hostPath /var/run/docker.sock",
			yaml: `
apiVersion: v1
kind: Pod
metadata:
  name: dockersock-pod
  namespace: default
spec:
  containers:
  - name: app
    image: nginx
    volumeMounts:
    - name: sock
      mountPath: /var/run/docker.sock
  volumes:
  - name: sock
    hostPath:
      path: /var/run/docker.sock
`,
			errContains: "hostPath",
		},
		{
			name: "reject hostPath /etc mount",
			yaml: `
apiVersion: v1
kind: Pod
metadata:
  name: etc-pod
  namespace: default
spec:
  containers:
  - name: app
    image: nginx
    volumeMounts:
    - name: host-etc
      mountPath: /mnt/etc
  volumes:
  - name: host-etc
    hostPath:
      path: /etc/shadow
`,
			errContains: "hostPath",
		},
		{
			name: "reject privileged container in Deployment",
			yaml: `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: privileged-deploy
  namespace: default
spec:
  replicas: 1
  template:
    spec:
      containers:
      - name: worker
        image: busybox
        securityContext:
          privileged: true
`,
			errContains: "privileged",
		},
		{
			name: "reject hostNetwork in DaemonSet",
			yaml: `
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: hostnet-ds
  namespace: default
spec:
  template:
    spec:
      hostNetwork: true
      containers:
      - name: worker
        image: busybox
`,
			errContains: "hostNetwork",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := NewResourceRepoWithInterface(fake.NewSimpleClientset(), nil)
			err := repo.ApplyYAML(context.Background(), "default", "default", []byte(tc.yaml))
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.errContains)
			}
			if !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(tc.errContains)) {
				t.Fatalf("expected error containing %q, got %v", tc.errContains, err)
			}
		})
	}
}

func TestApplyYAML_SecurityPolicy_AllowsSafePod(t *testing.T) {
	safeYAML := `
apiVersion: v1
kind: Pod
metadata:
  name: safe-pod
  namespace: default
spec:
  containers:
  - name: app
    image: nginx
`
	repo := NewResourceRepoWithInterface(fake.NewSimpleClientset(), nil)
	err := repo.ApplyYAML(context.Background(), "default", "default", []byte(safeYAML))
	if err != nil {
		t.Fatalf("expected safe pod to apply successfully, got: %v", err)
	}
}
