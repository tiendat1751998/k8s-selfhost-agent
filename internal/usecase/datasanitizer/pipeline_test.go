package datasanitizer_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/datdt/k8sselfhost/internal/domain/datasanitizer"
	usecase "github.com/datdt/k8sselfhost/internal/usecase/datasanitizer"
)

func TestPipeline_SafetyGuardrail(t *testing.T) {
	policy := datasanitizer.SanitizationPolicy{
		PolicyID: "pol-guardrail",
		Database: "users_db",
		Rules: []datasanitizer.MaskingColumnRule{
			{TableName: "users", ColumnName: "email", RuleType: datasanitizer.RuleTypeSaltedHash, FormatHint: "email"},
		},
	}
	pipeline := usecase.NewSanitizationPipeline(policy)

	t.Run("rejects clone when TargetClusterID is prod", func(t *testing.T) {
		exec, err := datasanitizer.NewCloneExecution("exec-1", "cluster-staging", "test_db", "prod", "staging_db")
		require.NoError(t, err)

		src := strings.NewReader("INSERT INTO users (email) VALUES ('leak@corp.com');")
		var dst bytes.Buffer

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		res, err := pipeline.ExecuteClone(ctx, exec, src, &dst)
		assert.Error(t, err)
		assert.ErrorIs(t, err, datasanitizer.ErrProductionTargetViolation)
		assert.Equal(t, datasanitizer.StatusFailed, res.Status)
		assert.Contains(t, res.ErrorMessage, "production")
		assert.Empty(t, dst.String(), "no data should be written to destination when target is production")
	})

	t.Run("rejects clone when TargetDB is production", func(t *testing.T) {
		exec, err := datasanitizer.NewCloneExecution("exec-2", "cluster-staging", "test_db", "staging-cluster", "users_production")
		require.NoError(t, err)

		src := strings.NewReader("INSERT INTO users (email) VALUES ('leak@corp.com');")
		var dst bytes.Buffer

		res, err := pipeline.ExecuteClone(context.Background(), exec, src, &dst)
		assert.Error(t, err)
		assert.ErrorIs(t, err, datasanitizer.ErrProductionTargetViolation)
		assert.Equal(t, datasanitizer.StatusFailed, res.Status)
		assert.Empty(t, dst.String())
	})

	t.Run("allows clone when target is valid staging sandbox", func(t *testing.T) {
		exec, err := datasanitizer.NewCloneExecution("exec-3", "prod-cluster-source", "prod_db", "staging-cluster", "staging_sandbox_db")
		require.NoError(t, err)

		src := strings.NewReader("INSERT INTO users (email) VALUES ('alice@gmail.com');\n")
		var dst bytes.Buffer

		res, err := pipeline.ExecuteClone(context.Background(), exec, src, &dst)
		require.NoError(t, err)
		assert.Equal(t, datasanitizer.StatusCompleted, res.Status)
		assert.Equal(t, int64(1), res.RowsMasked)
		assert.NotNil(t, res.FinishedAt)
		assert.Contains(t, dst.String(), "@staging.internal")
		assert.NotContains(t, dst.String(), "alice@gmail.com")
	})
}

func TestPipeline_PostSanitizationAudit(t *testing.T) {
	policy := datasanitizer.SanitizationPolicy{
		PolicyID: "pol-audit",
		Database: "users_db",
		Rules: []datasanitizer.MaskingColumnRule{
			{TableName: "users", ColumnName: "email", RuleType: datasanitizer.RuleTypeSaltedHash, FormatHint: "email"},
		},
	}

	t.Run("audit detects unmasked PII leak in output stream", func(t *testing.T) {
		exec, err := datasanitizer.NewCloneExecution("exec-4", "src-cluster", "src_db", "staging-cluster", "staging_db")
		require.NoError(t, err)

		// Input SQL has an unmasked email in a table without rule or raw leak
		src := strings.NewReader("INSERT INTO rogue_table (unmasked_email) VALUES ('unprotected_ceo@enterprise.com');\n")
		var dst bytes.Buffer

		// Enable strict audit on corporate domain leak
		pipelineWithAudit := usecase.NewSanitizationPipelineWithAudit(policy, []string{"@enterprise.com"})
		res, err := pipelineWithAudit.ExecuteClone(context.Background(), exec, src, &dst)

		assert.Error(t, err)
		assert.ErrorIs(t, err, datasanitizer.ErrPIILeakDetected)
		assert.Equal(t, datasanitizer.StatusFailed, res.Status)
	})
}

func TestPipeline_StreamingMemoryLeakCheck(t *testing.T) {
	policy := datasanitizer.SanitizationPolicy{
		PolicyID: "pol-stress",
		Database: "large_db",
		Rules: []datasanitizer.MaskingColumnRule{
			{TableName: "users", ColumnName: "email", RuleType: datasanitizer.RuleTypeSaltedHash, FormatHint: "email"},
			{TableName: "users", ColumnName: "phone", RuleType: datasanitizer.RuleTypeFakeGenerator, FormatHint: "phone"},
		},
	}
	pipeline := usecase.NewSanitizationPipeline(policy)

	exec, err := datasanitizer.NewCloneExecution("exec-stress", "src", "db", "staging-k8s", "staging_db")
	require.NoError(t, err)

	// Stream 20,000 statements through an io.Pipe with zero intermediary file disk allocation
	totalRows := 20000
	pr, pw := io.Pipe()

	go func() {
		defer pw.Close()
		for i := 0; i < totalRows; i++ {
			line := fmt.Sprintf("INSERT INTO users (id, email, phone) VALUES (%d, 'user_%d@gmail.com', '091234%04d');\n", i, i, i%10000)
			if _, writeErr := pw.Write([]byte(line)); writeErr != nil {
				return
			}
		}
	}()

	// Discard output sink to test purely stream pipeline memory
	discardSink := io.Discard

	// Force GC before measurement
	runtime.GC()
	var memBefore runtime.MemStats
	runtime.ReadMemStats(&memBefore)

	res, err := pipeline.ExecuteClone(context.Background(), exec, pr, discardSink)
	require.NoError(t, err)
	assert.Equal(t, int64(totalRows), res.RowsMasked)

	// Force GC after streaming
	runtime.GC()
	var memAfter runtime.MemStats
	runtime.ReadMemStats(&memAfter)

	// HeapAlloc delta should be bounded and negligible (< 25MB for 20k streamed rows)
	heapDelta := int64(memAfter.HeapAlloc) - int64(memBefore.HeapAlloc)
	t.Logf("HeapAlloc before: %d KB, after: %d KB, delta: %d KB", memBefore.HeapAlloc/1024, memAfter.HeapAlloc/1024, heapDelta/1024)
	assert.Less(t, heapDelta, int64(25*1024*1024), "streaming transform must operate in constant bounded memory")
}

func TestPipeline_ContextCancellation(t *testing.T) {
	policy := datasanitizer.SanitizationPolicy{
		PolicyID: "pol-cancel",
		Database: "users_db",
		Rules: []datasanitizer.MaskingColumnRule{
			{TableName: "users", ColumnName: "email", RuleType: datasanitizer.RuleTypeSaltedHash, FormatHint: "email"},
		},
	}
	pipeline := usecase.NewSanitizationPipeline(policy)

	exec, err := datasanitizer.NewCloneExecution("exec-cancel", "src", "db", "staging-cluster", "staging_db")
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	src := strings.NewReader("INSERT INTO users (email) VALUES ('test@corp.com');\n")
	var dst bytes.Buffer

	res, err := pipeline.ExecuteClone(ctx, exec, src, &dst)
	assert.Error(t, err)
	assert.Equal(t, datasanitizer.StatusFailed, res.Status)
}

func TestPipeline_NilExecution(t *testing.T) {
	policy := datasanitizer.SanitizationPolicy{
		PolicyID: "pol-nil",
		Database: "users_db",
	}
	pipeline := usecase.NewSanitizationPipeline(policy)

	_, err := pipeline.ExecuteClone(context.Background(), nil, strings.NewReader(""), io.Discard)
	assert.ErrorIs(t, err, datasanitizer.ErrValidation)
}

