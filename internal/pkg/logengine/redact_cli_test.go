package logengine

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRedactor_CLISecretFlags_Masked(t *testing.T) {
	redactor := NewInStreamRedactor()
	require.NotNil(t, redactor)

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "CLI flag --password=Secret123",
			input:    "starting worker --password=Secret123 --port=8080",
			expected: "starting worker --password=[REDACTED_SECRET] --port=8080",
		},
		{
			name:     "CLI flag --password Secret123",
			input:    "deploy run --password Secret123 --env=prod",
			expected: "deploy run --password [REDACTED_SECRET] --env=prod",
		},
		{
			name:     "CLI flag --passwd=Secret123",
			input:    "useradd admin --passwd=Secret123",
			expected: "useradd admin --passwd=[REDACTED_SECRET]",
		},
		{
			name:     "CLI flag --token=tok_abc",
			input:    "kubeadm join 10.0.0.1:6443 --token=tok_abc --discovery-token-ca-cert-hash sha256:123",
			expected: "kubeadm join 10.0.0.1:6443 --token=[REDACTED_SECRET] --discovery-token-ca-cert-hash sha256:123",
		},
		{
			name:     "CLI flag --token tok_abc",
			input:    "helm upgrade app --token tok_abc",
			expected: "helm upgrade app --token [REDACTED_SECRET]",
		},
		{
			name:     "CLI flag --secret=s3cr3t",
			input:    "vault write secret/app --secret=s3cr3t",
			expected: "vault write secret/app --secret=[REDACTED_SECRET]",
		},
		{
			name:     "CLI flag --api-key=mykey",
			input:    "client connect --api-key=mykey",
			expected: "client connect --api-key=[REDACTED_SECRET]",
		},
		{
			name:     "CLI flag --client-secret=cs456",
			input:    "oauth2 client --client-id=app --client-secret=cs456",
			expected: "oauth2 client --client-id=app --client-secret=[REDACTED_SECRET]",
		},
		{
			name:     "CLI short flag -p Secret123",
			input:    "-p Secret123",
			expected: "-p [REDACTED_SECRET]",
		},
		{
			name:     "CLI short flag command with -p Secret123",
			input:    "mysql -u root -p Secret123 -h localhost",
			expected: "mysql -u root -p [REDACTED_SECRET] -h localhost",
		},
		{
			name:     "CLI curl with -u flag",
			input:    "curl -X POST -u admin:Secret123 https://example.com/api",
			expected: "curl -X POST -u [REDACTED_SECRET] https://example.com/api",
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

func TestWriter_MasksRawAndMessage(t *testing.T) {
	InitEngineRuntime()

	tmpDir, err := os.MkdirTemp("", "writer-raw-redact-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	dict := NewLabelDictionary()
	writer, err := NewWriter(tmpDir, dict, WithMaxRowsPerBlock(10))
	require.NoError(t, err)

	now := time.Now().UTC()
	rawInput := "exec: mysql -u root -p Secret123 --host=127.0.0.1"
	msgInput := "failed query with --password=Secret123"

	entry := Entry{
		Timestamp: now,
		Service:   "db-sync",
		Level:     "error",
		Message:   msgInput,
		Raw:       rawInput,
	}

	err = writer.Write(entry)
	require.NoError(t, err)

	err = writer.Close()
	require.NoError(t, err)

	reader, err := NewReader(tmpDir, dict)
	require.NoError(t, err)
	defer reader.Close()

	results, err := reader.Search(context.Background(), QueryParams{
		Service: "db-sync",
		Limit:   10,
	})
	require.NoError(t, err)
	require.Len(t, results, 1)

	assert.NotContains(t, results[0].Message, "Secret123")
	assert.Contains(t, results[0].Message, "--password=[REDACTED_SECRET]")
}
