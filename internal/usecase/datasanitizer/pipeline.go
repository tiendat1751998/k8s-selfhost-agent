package datasanitizer

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/datdt/k8sselfhost/internal/domain/datasanitizer"
)

// SanitizationPipeline coordinates the streaming transformation and safety audits.
type SanitizationPipeline struct {
	masker             *SQLMasker
	prohibitedPatterns []string
}

// NewSanitizationPipeline builds a new SanitizationPipeline with the specified policy.
func NewSanitizationPipeline(policy datasanitizer.SanitizationPolicy) *SanitizationPipeline {
	return &SanitizationPipeline{
		masker: NewSQLMasker(policy),
	}
}

// NewSanitizationPipelineWithAudit builds a pipeline with strict post-masking PII leak checks.
func NewSanitizationPipelineWithAudit(policy datasanitizer.SanitizationPolicy, prohibitedPatterns []string) *SanitizationPipeline {
	return &SanitizationPipeline{
		masker:             NewSQLMasker(policy),
		prohibitedPatterns: prohibitedPatterns,
	}
}

// ExecuteClone runs the streaming sanitization pipeline from source to destination.
// Strictly enforces safety guardrails to prohibit writing to production targets.
func (p *SanitizationPipeline) ExecuteClone(
	ctx context.Context,
	execution *datasanitizer.CloneExecution,
	src io.Reader,
	dst io.Writer,
) (*datasanitizer.CloneExecution, error) {
	if execution == nil {
		return nil, fmt.Errorf("%w: execution cannot be nil", datasanitizer.ErrValidation)
	}

	// 1. Safety Guardrail: Reject if target is production
	if datasanitizer.IsProductionTarget(execution.TargetClusterID, execution.TargetDB) {
		errMsg := fmt.Sprintf("safety guardrail triggered: target cluster '%s' or database '%s' is production",
			execution.TargetClusterID, execution.TargetDB)
		execution.Fail(errMsg)
		return execution, datasanitizer.ErrProductionTargetViolation
	}

	if err := ctx.Err(); err != nil {
		execution.Fail(err.Error())
		return execution, err
	}

	// 2. Transition state to Masking
	execution.StartMasking()

	// 3. Setup streaming sink with optional audit detector
	auditSink := newAuditWriter(dst, p.prohibitedPatterns)

	// 4. Transform in-stream
	rowsMasked, err := p.masker.TransformStreamWithContext(ctx, src, auditSink)
	if err != nil {
		execution.Fail(fmt.Sprintf("streaming transform failed: %v", err))
		return execution, err
	}

	// 5. Post-sanitization audit check
	if auditSink.leakDetected {
		errMsg := fmt.Sprintf("unmasked PII pattern '%s' detected in output stream", auditSink.detectedPattern)
		execution.Fail(errMsg)
		return execution, datasanitizer.ErrPIILeakDetected
	}

	// 6. Complete
	execution.Complete(rowsMasked)
	return execution, nil
}

type auditWriter struct {
	dst             io.Writer
	patterns        []string
	leakDetected    bool
	detectedPattern string
	tailBuffer      string
}

func newAuditWriter(dst io.Writer, patterns []string) *auditWriter {
	return &auditWriter{
		dst:      dst,
		patterns: patterns,
	}
}

func (a *auditWriter) Write(p []byte) (int, error) {
	if len(a.patterns) > 0 && !a.leakDetected {
		// Sliding window check over chunk + tail of previous chunk
		combined := a.tailBuffer + string(p)
		for _, pat := range a.patterns {
			if strings.Contains(combined, pat) {
				a.leakDetected = true
				a.detectedPattern = pat
				break
			}
		}
		if len(combined) > 256 {
			a.tailBuffer = combined[len(combined)-256:]
		} else {
			a.tailBuffer = combined
		}
	}

	return a.dst.Write(p)
}
