package kubernetes_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datdt/k8sselfhost/internal/infrastructure/config"
	infraK8s "github.com/datdt/k8sselfhost/internal/infrastructure/kubernetes"
)

func writeTestKubeconfig(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	kcPath := filepath.Join(dir, "kubeconfig")
	content := `apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://127.0.0.1:6443
  name: test-cluster
contexts:
- context:
    cluster: test-cluster
    user: test-user
  name: test-context
current-context: test-context
users:
- name: test-user
  user:
    token: test-token
`
	err := os.WriteFile(kcPath, []byte(content), 0600)
	require.NoError(t, err)
	return kcPath
}

func TestNewClient_DefaultConfig(t *testing.T) {
	orig := config.GetK8sClientConfig()
	defer config.SetK8sClientConfig(orig)

	config.SetK8sClientConfig(config.K8sClientConfig{
		QPS:     50,
		Burst:   100,
		Timeout: 30 * time.Second,
	})

	kcPath := writeTestKubeconfig(t)
	client, err := infraK8s.NewClient(kcPath)
	require.NoError(t, err)
	require.NotNil(t, client)

	lastCfg := infraK8s.GetLastConfig()
	require.NotNil(t, lastCfg)
	assert.Equal(t, float32(50), lastCfg.QPS)
	assert.Equal(t, 100, lastCfg.Burst)
	assert.Equal(t, 30*time.Second, lastCfg.Timeout)
}

func TestNewClient_CustomConfig(t *testing.T) {
	orig := config.GetK8sClientConfig()
	defer config.SetK8sClientConfig(orig)

	config.SetK8sClientConfig(config.K8sClientConfig{
		QPS:     88.5,
		Burst:   188,
		Timeout: 48 * time.Second,
	})

	kcPath := writeTestKubeconfig(t)
	client, err := infraK8s.NewClient(kcPath)
	require.NoError(t, err)
	require.NotNil(t, client)

	lastCfg := infraK8s.GetLastConfig()
	require.NotNil(t, lastCfg)
	assert.Equal(t, float32(88.5), lastCfg.QPS)
	assert.Equal(t, 188, lastCfg.Burst)
	assert.Equal(t, 48*time.Second, lastCfg.Timeout)
}
