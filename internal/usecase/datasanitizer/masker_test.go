package datasanitizer_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"github.com/datdt/k8sselfhost/internal/domain/datasanitizer"
	usecase "github.com/datdt/k8sselfhost/internal/usecase/datasanitizer"
)

func TestMasker_ValueMasking(t *testing.T) {
	masker := usecase.NewSQLMasker(datasanitizer.SanitizationPolicy{
		PolicyID: "test-policy",
		Database: "users_db",
		Rules: []datasanitizer.MaskingColumnRule{
			{
				TableName:  "users",
				ColumnName: "email",
				RuleType:   datasanitizer.RuleTypeSaltedHash,
				FormatHint: "email",
				Salt:       "salty-pepper",
			},
			{
				TableName:  "users",
				ColumnName: "phone",
				RuleType:   datasanitizer.RuleTypeFakeGenerator,
				FormatHint: "phone",
				Salt:       "phone-salt",
			},
			{
				TableName:  "users",
				ColumnName: "password_hash",
				RuleType:   datasanitizer.RuleTypeFixedPlaceholder,
				FormatHint: "password",
			},
			{
				TableName:  "users",
				ColumnName: "api_key",
				RuleType:   datasanitizer.RuleTypeFixedPlaceholder,
				FormatHint: "api_key",
			},
			{
				TableName:  "users",
				ColumnName: "credit_card",
				RuleType:   datasanitizer.RuleTypeRedact,
				FormatHint: "credit_card",
			},
		},
	})

	t.Run("email masking", func(t *testing.T) {
		rule, ok := masker.Policy().FindRule("users", "email")
		require.True(t, ok)

		masked := masker.MaskValue(*rule, "real_customer@gmail.com")
		assert.True(t, strings.HasSuffix(masked, "@redacted.invalid"), "expected suffix @redacted.invalid, got: %s", masked)
		assert.NotContains(t, masked, "real_customer")
		assert.NotContains(t, masked, "gmail.com")

		// Deterministic check
		masked2 := masker.MaskValue(*rule, "real_customer@gmail.com")
		assert.Equal(t, masked, masked2)

		// Different email gives different prefix
		otherMasked := masker.MaskValue(*rule, "other_customer@yahoo.com")
		assert.NotEqual(t, masked, otherMasked)
	})

	t.Run("phone masking", func(t *testing.T) {
		rule, ok := masker.Policy().FindRule("users", "phone")
		require.True(t, ok)

		masked := masker.MaskValue(*rule, "0912345678")
		assert.Len(t, masked, 10)
		assert.True(t, strings.HasPrefix(masked, "09"), "phone must start with 09, got %s", masked)

		// Deterministic check
		masked2 := masker.MaskValue(*rule, "0912345678")
		assert.Equal(t, masked, masked2)

		// Different phone
		other := masker.MaskValue(*rule, "0987654321")
		assert.NotEqual(t, masked, other)
		assert.True(t, strings.HasPrefix(other, "09"))
	})

	t.Run("password hash masking", func(t *testing.T) {
		rule, ok := masker.Policy().FindRule("users", "password_hash")
		require.True(t, ok)

		masked := masker.MaskValue(*rule, "$2a$10$supersecretproductionhash")
		assert.NotEmpty(t, masked)
		assert.NotEqual(t, "$2a$10$supersecretproductionhash", masked)

		// Must verify as bcrypt hash of "StagingDefaultPass123!"
		err := bcrypt.CompareHashAndPassword([]byte(masked), []byte("StagingDefaultPass123!"))
		assert.NoError(t, err, "masked password hash must match StagingDefaultPass123!")
	})

	t.Run("api_key and token masking", func(t *testing.T) {
		rule, ok := masker.Policy().FindRule("users", "api_key")
		require.True(t, ok)

		masked := masker.MaskValue(*rule, "live_sk_9876543210fedcba")
		assert.Equal(t, "sk_test_staging_placeholder", masked)
	})

	t.Run("credit card masking", func(t *testing.T) {
		rule, ok := masker.Policy().FindRule("users", "credit_card")
		require.True(t, ok)

		masked := masker.MaskValue(*rule, "4111-2222-3333-4444")
		assert.Equal(t, "4111-XXXX-XXXX-1111", masked)
	})
}

func TestMasker_TransformSQLStatements(t *testing.T) {
	policy := datasanitizer.SanitizationPolicy{
		PolicyID: "pol-test",
		Database: "app_db",
		Rules: []datasanitizer.MaskingColumnRule{
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
				Salt:       "salt",
			},
			{
				TableName:  "users",
				ColumnName: "password_hash",
				RuleType:   datasanitizer.RuleTypeFixedPlaceholder,
				FormatHint: "password",
			},
		},
	}
	masker := usecase.NewSQLMasker(policy)

	t.Run("INSERT statement transformation", func(t *testing.T) {
		inputSQL := "INSERT INTO users (id, name, email, phone, password_hash) VALUES (1, 'Alice Smith', 'alice@company.com', '0912345678', '$2a$10$realprodhash123456789012');\n"
		var output bytes.Buffer

		rowsMasked, err := masker.TransformStream(strings.NewReader(inputSQL), &output)
		require.NoError(t, err)
		assert.Equal(t, int64(1), rowsMasked)

		outStr := output.String()
		assert.NotContains(t, outStr, "alice@company.com")
		assert.NotContains(t, outStr, "0912345678")
		assert.NotContains(t, outStr, "$2a$10$realprodhash123456789012")
		assert.Contains(t, outStr, "@redacted.invalid")
		assert.Contains(t, outStr, "Alice Smith")
	})

	t.Run("UPDATE statement transformation", func(t *testing.T) {
		inputSQL := "UPDATE users SET email = 'bob@corp.com', phone = '0988888888' WHERE id = 42;\n"
		var output bytes.Buffer

		rowsMasked, err := masker.TransformStream(strings.NewReader(inputSQL), &output)
		require.NoError(t, err)
		assert.Equal(t, int64(1), rowsMasked)

		outStr := output.String()
		assert.NotContains(t, outStr, "bob@corp.com")
		assert.NotContains(t, outStr, "0988888888")
		assert.Contains(t, outStr, "@redacted.invalid")
		assert.Contains(t, outStr, "WHERE id = 42")
	})

	t.Run("Multi-row INSERT statement transformation", func(t *testing.T) {
		inputSQL := "INSERT INTO users (id, email) VALUES (1, 'carl@test.com'), (2, 'diana@test.com');\n"
		var output bytes.Buffer

		rowsMasked, err := masker.TransformStream(strings.NewReader(inputSQL), &output)
		require.NoError(t, err)
		assert.Equal(t, int64(2), rowsMasked)

		outStr := output.String()
		assert.NotContains(t, outStr, "carl@test.com")
		assert.NotContains(t, outStr, "diana@test.com")
		assert.Contains(t, outStr, "@redacted.invalid")
	})

	t.Run("Non-PII DDL and Comments pass through untouched", func(t *testing.T) {
		inputSQL := "-- Dumping table structure\nCREATE TABLE users (id int, email text);\nALTER TABLE users ADD PRIMARY KEY (id);\n"
		var output bytes.Buffer

		rowsMasked, err := masker.TransformStream(strings.NewReader(inputSQL), &output)
		require.NoError(t, err)
		assert.Equal(t, int64(0), rowsMasked)
		assert.Equal(t, inputSQL, output.String())
	})

	t.Run("Schema-qualified and quoted identifiers with escaped strings", func(t *testing.T) {
		inputSQL := "INSERT INTO \"public\".\"users\" (\"id\", \"name\", \"email\") VALUES (1, 'O''Reilly', 'oreilly@corp.com');\n"
		var output bytes.Buffer

		rowsMasked, err := masker.TransformStream(strings.NewReader(inputSQL), &output)
		require.NoError(t, err)
		assert.Equal(t, int64(1), rowsMasked)

		outStr := output.String()
		assert.Contains(t, outStr, "O''Reilly")
		assert.NotContains(t, outStr, "oreilly@corp.com")
		assert.Contains(t, outStr, "@redacted.invalid")
	})

	t.Run("Case-insensitive keywords and no space before VALUES paren", func(t *testing.T) {
		inputSQL := "insert into users (email) values('someone@live.com');\n"
		var output bytes.Buffer

		rowsMasked, err := masker.TransformStream(strings.NewReader(inputSQL), &output)
		require.NoError(t, err)
		assert.Equal(t, int64(1), rowsMasked)

		outStr := output.String()
		assert.NotContains(t, outStr, "someone@live.com")
		assert.Contains(t, outStr, "@redacted.invalid")
	})
}

