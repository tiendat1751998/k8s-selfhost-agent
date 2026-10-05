package kubernetes_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datdt/k8sselfhost/internal/infrastructure/kubernetes"
)

func makeKubeconfig(serverURL string) []byte {
	return []byte(fmt.Sprintf(`apiVersion: v1
clusters:
- cluster:
    server: %s
    insecure-skip-tls-verify: true
  name: test-cluster
contexts:
- context:
    cluster: test-cluster
    user: test-user
  name: test-context
current-context: test-context
kind: Config
preferences: {}
users:
- name: test-user
  user:
    token: test-token
`, serverURL))
}

func TestDiscoveryAdapter_RequiresHTTPS(t *testing.T) {
	adapter := kubernetes.NewDiscoveryAdapter()
	kc := makeKubeconfig("http://insecure-k8s.example.com:8080")

	err := adapter.ValidateConnectivity(context.Background(), kc)
	require.Error(t, err, "expected error when server url uses http")
	assert.Contains(t, strings.ToLower(err.Error()), "https")
}

func TestDiscoveryAdapter_BlocksMetadataAndPrivateByDefault(t *testing.T) {
	adapter := kubernetes.NewDiscoveryAdapter()

	t.Run("blocks cloud metadata address", func(t *testing.T) {
		kc := makeKubeconfig("https://169.254.169.254:6443")
		err := adapter.ValidateConnectivity(context.Background(), kc)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "blocked")
	})

	t.Run("blocks private RFC1918 address without allowlist", func(t *testing.T) {
		kc := makeKubeconfig("https://10.250.0.1:6443")
		err := adapter.ValidateConnectivity(context.Background(), kc)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "blocked")
	})
}
