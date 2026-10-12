package datasanitizer_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datdt/k8sselfhost/internal/domain/datasanitizer"
)

func TestNewSanitizationPolicy(t *testing.T) {
	t.Run("valid policy with rules", func(t *testing.T) {
		rules := []datasanitizer.MaskingColumnRule{
			{
				TableName:  "users",
				ColumnName: "email",
				RuleType:   datasanitizer.RuleTypeSaltedHash,
				FormatHint: "email",
				Salt:       "test-salt",
			},
			{
				TableName:  "users",
				ColumnName: "phone",
				RuleType:   datasanitizer.RuleTypeFakeGenerator,
				FormatHint: "phone",
			},
		}

		policy, err := datasanitizer.NewSanitizationPolicy("pol-1", "app_db", rules)
		require.NoError(t, err)
		assert.Equal(t, "pol-1", policy.PolicyID)
		assert.Equal(t, "app_db", policy.Database)
		assert.Len(t, policy.Rules, 2)
		assert.WithinDuration(t, time.Now().UTC(), policy.CreatedAt, 2*time.Second)

		rule, found := policy.FindRule("users", "email")
		assert.True(t, found)
		assert.Equal(t, datasanitizer.RuleTypeSaltedHash, rule.RuleType)

		_, notFound := policy.FindRule("users", "nonexistent")
		assert.False(t, notFound)
	})

	t.Run("empty policy id is rejected", func(t *testing.T) {
		_, err := datasanitizer.NewSanitizationPolicy("", "app_db", nil)
		assert.ErrorIs(t, err, datasanitizer.ErrInvalidPolicy)
	})

	t.Run("empty database is rejected", func(t *testing.T) {
		_, err := datasanitizer.NewSanitizationPolicy("pol-1", "", nil)
		assert.ErrorIs(t, err, datasanitizer.ErrInvalidPolicy)
	})

	t.Run("invalid rule type is rejected", func(t *testing.T) {
		rules := []datasanitizer.MaskingColumnRule{
			{
				TableName:  "users",
				ColumnName: "email",
				RuleType:   datasanitizer.RuleType("INVALID_RULE"),
			},
		}
		_, err := datasanitizer.NewSanitizationPolicy("pol-1", "app_db", rules)
		assert.ErrorIs(t, err, datasanitizer.ErrInvalidRule)
	})

	t.Run("missing column name in rule is rejected", func(t *testing.T) {
		rules := []datasanitizer.MaskingColumnRule{
			{
				TableName: "users",
				RuleType:  datasanitizer.RuleTypeRedact,
			},
		}
		_, err := datasanitizer.NewSanitizationPolicy("pol-1", "app_db", rules)
		assert.ErrorIs(t, err, datasanitizer.ErrInvalidRule)
	})
}

func TestCloneExecution_Lifecycle(t *testing.T) {
	exec, err := datasanitizer.NewCloneExecution("exec-101", "cluster-prod", "main_db", "cluster-staging", "staging_db")
	require.NoError(t, err)

	assert.Equal(t, datasanitizer.StatusDumping, exec.Status)
	assert.Equal(t, int64(0), exec.RowsMasked)
	assert.WithinDuration(t, time.Now().UTC(), exec.StartedAt, 2*time.Second)

	exec.StartMasking()
	assert.Equal(t, datasanitizer.StatusMasking, exec.Status)

	exec.StartRestoring()
	assert.Equal(t, datasanitizer.StatusRestoring, exec.Status)

	exec.Complete(15420)
	assert.Equal(t, datasanitizer.StatusCompleted, exec.Status)
	assert.Equal(t, int64(15420), exec.RowsMasked)
	assert.NotNil(t, exec.FinishedAt)

	// Test Fail transition
	exec2, err := datasanitizer.NewCloneExecution("exec-102", "cluster-prod", "main_db", "cluster-staging", "staging_db")
	require.NoError(t, err)
	exec2.Fail("disk write error")
	assert.Equal(t, datasanitizer.StatusFailed, exec2.Status)
	assert.Equal(t, "disk write error", exec2.ErrorMessage)
	assert.NotNil(t, exec2.FinishedAt)
}

func TestCloneExecution_Validation(t *testing.T) {
	_, err := datasanitizer.NewCloneExecution("", "src", "db1", "tgt", "db2")
	assert.ErrorIs(t, err, datasanitizer.ErrValidation)

	_, err = datasanitizer.NewCloneExecution("id", "", "db1", "tgt", "db2")
	assert.ErrorIs(t, err, datasanitizer.ErrValidation)

	_, err = datasanitizer.NewCloneExecution("id", "src", "", "tgt", "db2")
	assert.ErrorIs(t, err, datasanitizer.ErrValidation)

	_, err = datasanitizer.NewCloneExecution("id", "src", "db1", "", "db2")
	assert.ErrorIs(t, err, datasanitizer.ErrValidation)

	_, err = datasanitizer.NewCloneExecution("id", "src", "db1", "tgt", "")
	assert.ErrorIs(t, err, datasanitizer.ErrValidation)
}

func TestIsProductionTarget(t *testing.T) {
	testCases := []struct {
		name      string
		clusterID string
		dbName    string
		isProd    bool
	}{
		{"exact prod cluster", "prod", "staging_db", true},
		{"exact production cluster", "production", "sandbox", true},
		{"k8s prod cluster", "k8s-prod-us-east", "testdb", true},
		{"cluster prod suffix", "cluster-production", "staging", true},
		{"prd short form", "aws-prd-01", "dev_db", true},
		{"target db is prod", "staging-cluster", "prod", true},
		{"target db is production", "staging-cluster", "users_production", true},
		{"target db has prd suffix", "staging-cluster", "analytics_prd", true},
		{"valid staging cluster and db", "staging-cluster", "users_staging", false},
		{"valid dev cluster and sandbox db", "dev-k8s", "sandbox_db", false},
		{"valid qa cluster and test db", "qa-01", "qa_test", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := datasanitizer.IsProductionTarget(tc.clusterID, tc.dbName)
			assert.Equal(t, tc.isProd, result, "expected IsProductionTarget(%q, %q) to be %v", tc.clusterID, tc.dbName, tc.isProd)
		})
	}
}
