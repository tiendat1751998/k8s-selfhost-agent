package logengine

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRedactor_PostgresConnectionString_Masked(t *testing.T) {
	redactor := NewInStreamRedactor()
	require.NotNil(t, redactor)

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Postgres connection string",
			input:    "FATAL: connection to postgres://root:123456@db:5432/app failed",
			expected: "FATAL: connection to postgres://[REDACTED_USER]:[REDACTED_SECRET]@db:5432/app failed",
		},
		{
			name:     "MySQL connection string",
			input:    "mysql://admin:SecretPassw0rd!@mysql.internal:3306/production",
			expected: "mysql://[REDACTED_USER]:[REDACTED_SECRET]@mysql.internal:3306/production",
		},
		{
			name:     "MongoDB connection string",
			input:    "mongodb://app_user:complex%20pass@mongo.cluster:27017/analytics",
			expected: "mongodb://[REDACTED_USER]:[REDACTED_SECRET]@mongo.cluster:27017/analytics",
		},
		{
			name:     "Redis connection string",
			input:    "redis://default:my_redis_auth@redis.cache:6379",
			expected: "redis://[REDACTED_USER]:[REDACTED_SECRET]@redis.cache:6379",
		},
		{
			name:     "AMQP connection string",
			input:    "amqp://guest:secretguest@rabbitmq.broker:5672/",
			expected: "amqp://[REDACTED_USER]:[REDACTED_SECRET]@rabbitmq.broker:5672/",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			maskedStr := redactor.MaskString(tc.input)
			assert.Equal(t, tc.expected, maskedStr)

			maskedBytes := redactor.MaskBytes([]byte(tc.input))
			assert.Equal(t, tc.expected, string(maskedBytes))
		})
	}
}

func TestRedactor_BearerToken_Masked(t *testing.T) {
	redactor := NewInStreamRedactor()
	require.NotNil(t, redactor)

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Authorization header Bearer token",
			input:    "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.doNotLeakThisPayload",
			expected: "Authorization: Bearer [REDACTED_BEARER_TOKEN]",
		},
		{
			name:     "Lowercase bearer token with multiple spaces",
			input:    "HTTP req header: bearer   abcdef1234567890abcdef1234567890",
			expected: "HTTP req header: bearer   [REDACTED_BEARER_TOKEN]",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			maskedStr := redactor.MaskString(tc.input)
			assert.Equal(t, tc.expected, maskedStr)

			maskedBytes := redactor.MaskBytes([]byte(tc.input))
			assert.Equal(t, tc.expected, string(maskedBytes))
		})
	}
}

func TestRedactor_APIKey_Masked(t *testing.T) {
	redactor := NewInStreamRedactor()
	require.NotNil(t, redactor)

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Stripe live API key",
			input:    "Stripe charge failed with apiKey: sk_live_9876543210abcdef123456",
			expected: "Stripe charge failed with apiKey: sk_live_[REDACTED_API_KEY]",
		},
		{
			name:     "GitHub personal access token",
			input:    "git clone https://ghp_0123456789abcdef0123456789@github.com/repo.git",
			expected: "git clone https://ghp_[REDACTED_API_KEY]@github.com/repo.git",
		},
		{
			name:     "GitLab personal access token",
			input:    "auth header glpat-ABCDEF0123456789xyz1234",
			expected: "auth header glpat-[REDACTED_API_KEY]",
		},
		{
			name:     "Slack bot token",
			input:    "slack message post token xoxb-123456789012-1234567890123-abcdef",
			expected: "slack message post token xoxb-[REDACTED_API_KEY]",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			maskedStr := redactor.MaskString(tc.input)
			assert.Equal(t, tc.expected, maskedStr)

			maskedBytes := redactor.MaskBytes([]byte(tc.input))
			assert.Equal(t, tc.expected, string(maskedBytes))
		})
	}
}

func TestRedactor_JSONPayload_PasswordMasked(t *testing.T) {
	redactor := NewInStreamRedactor()
	require.NotNil(t, redactor)

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Password in JSON body",
			input:    `{"username":"admin","password":"MySecretPassword123!"}`,
			expected: `{"username":"admin","password":"[REDACTED_SECRET]"}`,
		},
		{
			name:     "Secret key in JSON body with spacing",
			input:    `{"client_id": "app", "secret" : "super-secret-key-456", "active": true}`,
			expected: `{"client_id": "app", "secret" : "[REDACTED_SECRET]", "active": true}`,
		},
		{
			name:     "Token in JSON body",
			input:    `{"token": "eyJhbGciOi...", "type": "refresh"}`,
			expected: `{"token": "[REDACTED_SECRET]", "type": "refresh"}`,
		},
		{
			name:     "API key in JSON body",
			input:    `{"api_key": "custom_api_key_value_999"}`,
			expected: `{"api_key": "[REDACTED_SECRET]"}`,
		},
		{
			name:     "Access key in JSON body",
			input:    `{"access_key": "AKIAIOSFODNN7EXAMPLE"}`,
			expected: `{"access_key": "[REDACTED_SECRET]"}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			maskedStr := redactor.MaskString(tc.input)
			assert.Equal(t, tc.expected, maskedStr)

			maskedBytes := redactor.MaskBytes([]byte(tc.input))
			assert.Equal(t, tc.expected, string(maskedBytes))
		})
	}
}

func TestRedactor_PrivateKey_Masked(t *testing.T) {
	redactor := NewInStreamRedactor()
	require.NotNil(t, redactor)

	pemKey := "-----BEGIN RSA PRIVATE KEY-----\nMIIEowIBAAKCAQEA0m4w...\nMIICXAIBAAKCAQEA...\n-----END RSA PRIVATE KEY-----"
	input := "Application failed with loaded key:\n" + pemKey + "\nError: invalid format"
	expected := "Application failed with loaded key:\n[REDACTED_PRIVATE_KEY]\nError: invalid format"

	maskedStr := redactor.MaskString(input)
	assert.Equal(t, expected, maskedStr)

	maskedBytes := redactor.MaskBytes([]byte(input))
	assert.Equal(t, expected, string(maskedBytes))

	ecKey := "-----BEGIN EC PRIVATE KEY-----\nMHcCAQEEI...\n-----END EC PRIVATE KEY-----"
	inputEC := "Loaded EC key: " + ecKey
	expectedEC := "Loaded EC key: [REDACTED_PRIVATE_KEY]"
	assert.Equal(t, expectedEC, redactor.MaskString(inputEC))

	multiKeyInput := "Primary: " + pemKey + "\nBackup: " + ecKey
	expectedMulti := "Primary: [REDACTED_PRIVATE_KEY]\nBackup: [REDACTED_PRIVATE_KEY]"
	assert.Equal(t, expectedMulti, redactor.MaskString(multiKeyInput))
	assert.Equal(t, expectedMulti, string(redactor.MaskBytes([]byte(multiKeyInput))))
}

func TestRedactor_CleanLogs_FastPath(t *testing.T) {
	redactor := NewInStreamRedactor()
	require.NotNil(t, redactor)

	cleanLogs := []string{
		"2026-10-12T10:35:00.123Z [INFO] Server started on port 8080 successfully",
		"HTTP 200 GET /healthz 2.4ms status=OK",
		"Job worker=4 processed 500 items in batch",
		"Database connection pool initialized with max_connections=20",
		"Cache miss for key user_profile_12345",
	}

	for _, logLine := range cleanLogs {
		maskedStr := redactor.MaskString(logLine)
		assert.Equal(t, logLine, maskedStr, "clean log must remain untouched")

		// Verify fast-path doesn't allocate or mutate
		rawBytes := []byte(logLine)
		maskedBytes := redactor.MaskBytes(rawBytes)
		assert.Equal(t, logLine, string(maskedBytes))
	}
}

func TestRedactor_MixedLogs(t *testing.T) {
	redactor := NewInStreamRedactor()
	require.NotNil(t, redactor)

	mixedLog := `Error syncing data: postgres://usr:pass123@db:5432/sync with token Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.xyz and payload {"password":"inner_secret_val"}`
	expected := `Error syncing data: postgres://[REDACTED_USER]:[REDACTED_SECRET]@db:5432/sync with token Authorization: Bearer [REDACTED_BEARER_TOKEN] and payload {"password":"[REDACTED_SECRET]"}`

	maskedStr := redactor.MaskString(mixedLog)
	assert.Equal(t, expected, maskedStr)
}

func TestRedactor_TriggerWithoutMatch_ZeroAllocs(t *testing.T) {
	redactor := NewInStreamRedactor()
	require.NotNil(t, redactor)

	// Line contains triggers '://' and 'token' but does not match credentials regex patterns
	nonSecretLine := []byte("2026-10-12T10:35:00.123Z [INFO] request to https://k8s.local/api with token verification status=valid")
	allocs := testing.AllocsPerRun(100, func() {
		res := redactor.MaskBytes(nonSecretLine)
		_ = res
	})
	assert.Equal(t, float64(0), allocs, "MaskBytes must have 0 allocations when triggers do not match regex")
}

func TestRedactor_Benchmark_Throughput(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping throughput benchmark test in short mode")
	}
	redactor := NewInStreamRedactor()
	require.NotNil(t, redactor)

	cleanLine := []byte("2026-10-12T10:35:00.123Z [INFO] processed request id=12345 status=200 client=10.0.0.1 duration=15ms details=success_log_entry_payload")
	const totalLines = 100000

	start := time.Now()
	for i := 0; i < totalLines; i++ {
		_ = redactor.MaskBytes(cleanLine)
	}
	elapsed := time.Since(start)

	linesPerSec := float64(totalLines) / elapsed.Seconds()
	t.Logf("Processed %d lines in %v (%.2f lines/sec)", totalLines, elapsed, linesPerSec)

	assert.Greater(t, linesPerSec, 200000.0, "Throughput baseline sanity check must exceed 200,000 lines/second")
}

func TestWriter_WithRedactor_MasksSecretsInStoredBlocks(t *testing.T) {
	InitEngineRuntime()

	tmpDir, err := os.MkdirTemp("", "redactor-writer-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	dict := NewLabelDictionary()
	writer, err := NewWriter(tmpDir, dict, WithMaxRowsPerBlock(10))
	require.NoError(t, err)

	now := time.Now().UTC()
	err = writer.Write(Entry{
		Timestamp: now,
		Service:   "payment-svc",
		Level:     "error",
		Message:   "db failure on postgres://root:supersecret123@db.internal:5432/orders with token sk_live_1234567890abcdef1234",
	})
	require.NoError(t, err)

	err = writer.Close()
	require.NoError(t, err)

	reader, err := NewReader(tmpDir, dict)
	require.NoError(t, err)
	defer reader.Close()

	results, err := reader.Search(context.Background(), QueryParams{
		Service: "payment-svc",
		Limit:   10,
	})
	require.NoError(t, err)
	require.Len(t, results, 1)

	assert.NotContains(t, results[0].Message, "supersecret123")
	assert.NotContains(t, results[0].Message, "1234567890abcdef1234")
	assert.Contains(t, results[0].Message, "postgres://[REDACTED_USER]:[REDACTED_SECRET]@db.internal:5432/orders")
	assert.Contains(t, results[0].Message, "sk_live_[REDACTED_API_KEY]")
}

func BenchmarkRedactor_CleanLogs_Throughput(b *testing.B) {
	redactor := NewInStreamRedactor()
	line := []byte("2026-10-12T10:35:00.123Z [INFO] processed request id=12345 status=200 client=10.0.0.1 duration=15ms details=success_log_entry_payload")

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = redactor.MaskBytes(line)
	}
}

func BenchmarkRedactor_SecretLogs_Throughput(b *testing.B) {
	redactor := NewInStreamRedactor()
	line := []byte(`2026-10-12T10:35:00.123Z [ERROR] connection to postgres://admin:SuperSecretP@ss123@db.internal:5432/production failed with payload {"password":"SecretPassword123!"}`)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = redactor.MaskBytes(line)
	}
}

func BenchmarkRedactor_Throughput(b *testing.B) {
	redactor := NewInStreamRedactor()
	line := []byte("2026-10-12T10:35:00.123Z [INFO] processed request id=12345 status=200 client=10.0.0.1 duration=15ms details=success_log_entry_payload")

	b.SetBytes(int64(len(line)))
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = redactor.MaskBytes(line)
	}
}

