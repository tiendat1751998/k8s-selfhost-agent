package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datdt/k8sselfhost/internal/infrastructure/config"
)

func TestLoad_FailFastMissingDBPassword(t *testing.T) {
	// Clear any environment variables that might supply the DB password
	t.Setenv("POSTGRES_PASSWORD", "")
	t.Setenv("K8S_POSTGRES_PASSWORD", "")
	t.Setenv("DB_PASSWORD", "")

	// When postgres is configured with a host and no password, Load must fail fast
	cfg, err := config.Load()
	assert.Error(t, err)
	assert.Nil(t, cfg)
	assert.Contains(t, err.Error(), "postgres password is required")
}

func TestLoad_PostgresPasswordFromEnv(t *testing.T) {
	testPassword := "super-secure-dev-db-password"
	t.Setenv("POSTGRES_PASSWORD", testPassword)
	t.Setenv("K8S_POSTGRES_PASSWORD", "")
	t.Setenv("DB_PASSWORD", "")

	cfg, err := config.Load()
	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, testPassword, cfg.Postgres.Password)
}

func TestLoad_PostgresPasswordFromK8SEnv(t *testing.T) {
	testPassword := "k8s-prefixed-db-password"
	t.Setenv("POSTGRES_PASSWORD", "")
	t.Setenv("K8S_POSTGRES_PASSWORD", testPassword)
	t.Setenv("DB_PASSWORD", "")

	cfg, err := config.Load()
	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, testPassword, cfg.Postgres.Password)
}

func TestLoad_PostgresPasswordFromDBPasswordEnv(t *testing.T) {
	testPassword := "db-password-alias"
	t.Setenv("POSTGRES_PASSWORD", "")
	t.Setenv("K8S_POSTGRES_PASSWORD", "")
	t.Setenv("DB_PASSWORD", testPassword)

	cfg, err := config.Load()
	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, testPassword, cfg.Postgres.Password)
}

func TestLoad_DockerHostDefaultAndOverride(t *testing.T) {
	t.Setenv("POSTGRES_PASSWORD", "mock-pass")

	t.Run("default docker socket", func(t *testing.T) {
		t.Setenv("DOCKER_HOST", "")
		t.Setenv("K8S_DOCKER_HOST", "")

		cfg, err := config.Load()
		require.NoError(t, err)
		assert.Equal(t, "unix:///var/run/docker.sock", cfg.Docker.Host)
	})

	t.Run("override docker host", func(t *testing.T) {
		t.Setenv("DOCKER_HOST", "tcp://custom-docker:2375")

		cfg, err := config.Load()
		require.NoError(t, err)
		assert.Equal(t, "tcp://custom-docker:2375", cfg.Docker.Host)
	})
}

func TestLoad_DotEnvFileLoading(t *testing.T) {
	t.Setenv("POSTGRES_PASSWORD", "")
	t.Setenv("K8S_POSTGRES_PASSWORD", "")
	t.Setenv("DB_PASSWORD", "")

	// Create temporary directory with a .env file and config.yaml
	tmpDir := t.TempDir()
	origWd, err := os.Getwd()
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.Chdir(origWd) })

	envFile := filepath.Join(tmpDir, ".env")
	err = os.WriteFile(envFile, []byte("POSTGRES_PASSWORD=from-dotenv-secret\n"), 0600)
	require.NoError(t, err)

	err = os.Chdir(tmpDir)
	require.NoError(t, err)

	cfg, err := config.Load()
	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, "from-dotenv-secret", cfg.Postgres.Password)
}

func TestLoad_SecurityAndServerHardenedSettings(t *testing.T) {
	t.Setenv("POSTGRES_PASSWORD", "mock-pass")
	t.Setenv("MASTER_ENCRYPTION_KEY", "masterkey12345678901234567890123")
	t.Setenv("METRICS_TOKEN", "test-metrics-token")
	t.Setenv("CORS_ALLOWED_ORIGINS", "http://localhost:5173,http://localhost:3000")
	t.Setenv("COOKIE_SECURE", "false")

	cfg, err := config.Load()
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, "masterkey12345678901234567890123", cfg.Security.MasterEncryptionKey)
	assert.Equal(t, "test-metrics-token", cfg.Security.MetricsToken)
	assert.Equal(t, "http://localhost:5173,http://localhost:3000", cfg.Server.CORSAllowedOrigins)
	assert.False(t, cfg.Server.CookieSecure)
}

func TestLoad_K8sClientDefaults(t *testing.T) {
	t.Setenv("POSTGRES_PASSWORD", "mock-pass")
	t.Setenv("K8S_CLIENT_QPS", "")
	t.Setenv("K8S_CLIENT_BURST", "")
	t.Setenv("K8S_CLIENT_TIMEOUT", "")

	cfg, err := config.Load()
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, float32(50), cfg.K8sClient.QPS)
	assert.Equal(t, 100, cfg.K8sClient.Burst)
	assert.Equal(t, 30*time.Second, cfg.K8sClient.Timeout)

	active := config.GetK8sClientConfig()
	assert.Equal(t, float32(50), active.QPS)
	assert.Equal(t, 100, active.Burst)
	assert.Equal(t, 30*time.Second, active.Timeout)
}

func TestLoad_K8sClientEnvOverrides(t *testing.T) {
	t.Setenv("POSTGRES_PASSWORD", "mock-pass")
	t.Setenv("K8S_CLIENT_QPS", "120.5")
	t.Setenv("K8S_CLIENT_BURST", "250")
	t.Setenv("K8S_CLIENT_TIMEOUT", "45s")

	cfg, err := config.Load()
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, float32(120.5), cfg.K8sClient.QPS)
	assert.Equal(t, 250, cfg.K8sClient.Burst)
	assert.Equal(t, 45*time.Second, cfg.K8sClient.Timeout)

	active := config.GetK8sClientConfig()
	assert.Equal(t, float32(120.5), active.QPS)
	assert.Equal(t, 250, active.Burst)
	assert.Equal(t, 45*time.Second, active.Timeout)
}

func TestLoad_K8sClientInvalidValuesFailFast(t *testing.T) {
	t.Setenv("POSTGRES_PASSWORD", "mock-pass")

	cases := []struct {
		name    string
		envKey  string
		envVal  string
		wantErr string
	}{
		{"qps is zero", "K8S_CLIENT_QPS", "0", "qps"},
		{"qps is negative", "K8S_CLIENT_QPS", "-5", "qps"},
		{"qps is non-numeric", "K8S_CLIENT_QPS", "invalid", "qps"},
		{"burst is zero", "K8S_CLIENT_BURST", "0", "burst"},
		{"burst is negative", "K8S_CLIENT_BURST", "-10", "burst"},
		{"burst is non-numeric", "K8S_CLIENT_BURST", "notanint", "burst"},
		{"timeout is zero", "K8S_CLIENT_TIMEOUT", "0s", "timeout"},
		{"timeout is negative", "K8S_CLIENT_TIMEOUT", "-5s", "timeout"},
		{"timeout is invalid", "K8S_CLIENT_TIMEOUT", "invalid-duration", "timeout"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("K8S_CLIENT_QPS", "")
			t.Setenv("K8S_CLIENT_BURST", "")
			t.Setenv("K8S_CLIENT_TIMEOUT", "")
			t.Setenv(tc.envKey, tc.envVal)

			cfg, err := config.Load()
			assert.Error(t, err, "expected failure for %s=%s", tc.envKey, tc.envVal)
			assert.Nil(t, cfg)
			assert.Contains(t, strings.ToLower(err.Error()), tc.wantErr)
		})
	}
}

