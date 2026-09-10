package postgres

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datdt/k8sselfhost/internal/domain/audit"
	"github.com/datdt/k8sselfhost/internal/pkg/tenancy"
)

type auditMockDBTX struct {
	lastSQL  string
	lastArgs []any
}

func (m *auditMockDBTX) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	m.lastSQL = sql
	m.lastArgs = arguments
	return pgconn.NewCommandTag("INSERT 0 1"), nil
}

func (m *auditMockDBTX) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	m.lastSQL = sql
	m.lastArgs = args
	return nil, nil
}

func (m *auditMockDBTX) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	m.lastSQL = sql
	m.lastArgs = args
	return nil
}

func TestAuditRepo_RecordAction_UUIDBugfix(t *testing.T) {
	ctx := context.Background()

	t.Run("invalid UUID in targetID gets cast to nil target_id and stored in target_name", func(t *testing.T) {
		mock := &auditMockDBTX{}
		repo := NewAuditRepo(mock)

		err := repo.RecordAction(
			ctx,
			"devops@enterprise.io",
			"apply",
			"k8s_manifest",
			"istio-ingress-gateway.yaml", // invalid UUID
			"",                          // empty targetName
			"success",
			map[string]interface{}{"namespace": "istio-system"},
			"10.0.0.1",
			"kubectl/1.30",
		)

		require.NoError(t, err)
		require.Len(t, mock.lastArgs, 10)

		// $1: actor, $2: action, $3: target_type, $4: target_id, $5: target_name, $6: result
		assert.Equal(t, "devops@enterprise.io", mock.lastArgs[0])
		assert.Equal(t, "apply", mock.lastArgs[1])
		assert.Equal(t, "k8s_manifest", mock.lastArgs[2])
		// $4 must be nil pointer to prevent Postgres UUID syntax error
		var nilTargetUUID *string
		assert.Equal(t, nilTargetUUID, mock.lastArgs[3])
		// $5 must preserve the target identifier string
		assert.Equal(t, "istio-ingress-gateway.yaml", mock.lastArgs[4])
		assert.Equal(t, "success", mock.lastArgs[5])
		assert.Equal(t, "default-tenant", mock.lastArgs[9])
	})

	t.Run("invalid UUID with existing target_name preserves target_name and nils target_id", func(t *testing.T) {
		mock := &auditMockDBTX{}
		repo := NewAuditRepo(mock)

		err := repo.RecordAction(
			ctx,
			"sre@enterprise.io",
			"scale",
			"k8s_deployment",
			"payment-service", // invalid UUID
			"payment-service", // targetName provided
			"success",
			map[string]interface{}{"replicas": 6},
			"10.0.0.2",
			"web-console",
		)

		require.NoError(t, err)
		require.Len(t, mock.lastArgs, 10)

		var nilTargetUUID *string
		assert.Equal(t, nilTargetUUID, mock.lastArgs[3])
		assert.Equal(t, "payment-service", mock.lastArgs[4])
		assert.Equal(t, "default-tenant", mock.lastArgs[9])
	})

	t.Run("valid UUID is preserved in target_id", func(t *testing.T) {
		mock := &auditMockDBTX{}
		repo := NewAuditRepo(mock)

		validUUID := "550e8400-e29b-41d4-a716-446655440000"
		err := repo.RecordAction(
			ctx,
			"admin@enterprise.io",
			"delete",
			"kubernetes",
			validUUID,
			"cluster-prod-1",
			"success",
			map[string]interface{}{},
			"10.0.0.3",
			"cli",
		)

		require.NoError(t, err)
		require.Len(t, mock.lastArgs, 10)

		expectedUUID := &validUUID
		assert.Equal(t, expectedUUID, mock.lastArgs[3])
		assert.Equal(t, "cluster-prod-1", mock.lastArgs[4])
		assert.Equal(t, "default-tenant", mock.lastArgs[9])
	})
}
func TestAuditRepo_ListLogs_NonAdminEmptyTenantIsolation(t *testing.T) {
	t.Run("tenant_admin with empty tenant returns empty logs without DB query", func(t *testing.T) {
		mock := &auditMockDBTX{}
		repo := NewAuditRepo(mock)

		ctx := tenancy.WithUserRole(context.Background(), "tenant_admin")
		logs, total, err := repo.ListLogs(ctx, audit.AuditLogFilter{})
		require.NoError(t, err)
		assert.Empty(t, logs)
		assert.Equal(t, 0, total)
		assert.Empty(t, mock.lastSQL, "database should not be queried when tenant_admin has empty tenant")
	})

	t.Run("operator with empty tenant returns empty logs without DB query", func(t *testing.T) {
		mock := &auditMockDBTX{}
		repo := NewAuditRepo(mock)

		ctx := tenancy.WithUserRole(context.Background(), "operator")
		logs, total, err := repo.ListLogs(ctx, audit.AuditLogFilter{})
		require.NoError(t, err)
		assert.Empty(t, logs)
		assert.Equal(t, 0, total)
		assert.Empty(t, mock.lastSQL, "database should not be queried when operator has empty tenant")
	})

	t.Run("unauthenticated/empty context returns empty logs without DB query", func(t *testing.T) {
		mock := &auditMockDBTX{}
		repo := NewAuditRepo(mock)

		ctx := context.Background()
		logs, total, err := repo.ListLogs(ctx, audit.AuditLogFilter{})
		require.NoError(t, err)
		assert.Empty(t, logs)
		assert.Equal(t, 0, total)
		assert.Empty(t, mock.lastSQL, "database should not be queried when context has empty tenant")
	})
}

func TestAuditRepo_RecordAction_SensitiveRedaction(t *testing.T) {
	ctx := context.Background()
	mock := &auditMockDBTX{}
	repo := NewAuditRepo(mock)

	details := map[string]interface{}{
		"token":         "super-secret-token",
		"trace_token":   "trace-token-abc",
		"authorization": "Bearer secret-jwt",
		"password":      "plaintext-password",
		"secret":        "top-secret",
		"cookie":        "session=secret-cookie",
		"access_token":  "access-123",
		"refresh_token": "refresh-456",
		"apiKey":        "apikey-789",
		"api_key":       "apikey-012",
		"private_key":   "-----BEGIN RSA PRIVATE KEY-----",
		"safe_key":      "safe_value",
		"nested": map[string]interface{}{
			"password": "nested-password",
			"secret":   "nested-secret",
			"info":     "nested-info",
		},
		"slice": []interface{}{
			map[string]interface{}{
				"token": "slice-token",
				"name":  "slice-item",
			},
		},
	}

	err := repo.RecordAction(
		ctx,
		"admin@example.com",
		"login",
		"user",
		"u1",
		"u1",
		"success",
		details,
		"127.0.0.1",
		"test-agent",
	)
	require.NoError(t, err)
	require.Len(t, mock.lastArgs, 10)

	detailsJSON, ok := mock.lastArgs[6].([]byte)
	require.True(t, ok)

	var recorded map[string]interface{}
	err = json.Unmarshal(detailsJSON, &recorded)
	require.NoError(t, err)

	// Check top-level sensitive fields are redacted
	assert.Equal(t, "[REDACTED]", recorded["token"])
	assert.Equal(t, "[REDACTED]", recorded["trace_token"])
	assert.Equal(t, "[REDACTED]", recorded["authorization"])
	assert.Equal(t, "[REDACTED]", recorded["password"])
	assert.Equal(t, "[REDACTED]", recorded["secret"])
	assert.Equal(t, "[REDACTED]", recorded["cookie"])
	assert.Equal(t, "[REDACTED]", recorded["access_token"])
	assert.Equal(t, "[REDACTED]", recorded["refresh_token"])
	assert.Equal(t, "[REDACTED]", recorded["apiKey"])
	assert.Equal(t, "[REDACTED]", recorded["api_key"])
	assert.Equal(t, "[REDACTED]", recorded["private_key"])

	// Check safe field is preserved
	assert.Equal(t, "safe_value", recorded["safe_key"])

	// Check nested map redaction
	nested, ok := recorded["nested"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "[REDACTED]", nested["password"])
	assert.Equal(t, "[REDACTED]", nested["secret"])
	assert.Equal(t, "nested-info", nested["info"])

	// Check slice redaction
	slice, ok := recorded["slice"].([]interface{})
	require.True(t, ok)
	require.Len(t, slice, 1)
	sliceItem, ok := slice[0].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "[REDACTED]", sliceItem["token"])
	assert.Equal(t, "slice-item", sliceItem["name"])
}
