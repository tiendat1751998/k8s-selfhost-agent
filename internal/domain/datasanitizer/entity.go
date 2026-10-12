package datasanitizer

import (
	"fmt"
	"strings"
	"time"
)

// RuleType defines the strategy used to mask or anonymize a database column.
type RuleType string

const (
	RuleTypeSaltedHash       RuleType = "SALTED_HASH"
	RuleTypeFixedPlaceholder RuleType = "FIXED_PLACEHOLDER"
	RuleTypeFakeGenerator    RuleType = "FAKE_GENERATOR"
	RuleTypeRedact           RuleType = "REDACT"
)

// Valid checks if the RuleType is supported.
func (r RuleType) Valid() bool {
	switch r {
	case RuleTypeSaltedHash, RuleTypeFixedPlaceholder, RuleTypeFakeGenerator, RuleTypeRedact:
		return true
	default:
		return false
	}
}

// MaskingColumnRule defines how a specific column should be anonymized.
type MaskingColumnRule struct {
	TableName    string   `json:"table_name"`
	ColumnName   string   `json:"column_name"`
	RuleType     RuleType `json:"rule_type"`
	FormatHint   string   `json:"format_hint"` // email, phone, cccd, password, api_key, token, credit_card
	Salt         string   `json:"salt,omitempty"`
	DefaultValue string   `json:"default_value,omitempty"`
}

// Validate ensures rule invariants are satisfied.
func (r *MaskingColumnRule) Validate() error {
	if strings.TrimSpace(r.ColumnName) == "" {
		return fmt.Errorf("%w: column_name is required", ErrInvalidRule)
	}
	if !r.RuleType.Valid() {
		return fmt.Errorf("%w: unsupported rule type '%s' for column '%s'", ErrInvalidRule, r.RuleType, r.ColumnName)
	}
	return nil
}

// SanitizationPolicy holds the set of masking rules for a target database schema.
type SanitizationPolicy struct {
	PolicyID  string              `json:"policy_id"`
	Database  string              `json:"database"`
	Rules     []MaskingColumnRule `json:"rules"`
	CreatedAt time.Time           `json:"created_at"`
}

// NewSanitizationPolicy constructs a validated SanitizationPolicy.
func NewSanitizationPolicy(policyID string, database string, rules []MaskingColumnRule) (*SanitizationPolicy, error) {
	if strings.TrimSpace(policyID) == "" {
		return nil, fmt.Errorf("%w: policy_id must not be empty", ErrInvalidPolicy)
	}
	if strings.TrimSpace(database) == "" {
		return nil, fmt.Errorf("%w: database must not be empty", ErrInvalidPolicy)
	}

	for i := range rules {
		if err := rules[i].Validate(); err != nil {
			return nil, err
		}
	}

	return &SanitizationPolicy{
		PolicyID:  strings.TrimSpace(policyID),
		Database:  strings.TrimSpace(database),
		Rules:     rules,
		CreatedAt: time.Now().UTC(),
	}, nil
}

// FindRule retrieves a masking rule for the given table and column name.
// Matches case-insensitively and allows table wildcard if TableName is empty or "*".
func (p SanitizationPolicy) FindRule(tableName, columnName string) (*MaskingColumnRule, bool) {
	cleanTable := strings.ToLower(strings.Trim(tableName, "\"`'"))
	cleanCol := strings.ToLower(strings.Trim(columnName, "\"`'"))

	for i := range p.Rules {
		r := &p.Rules[i]
		rCol := strings.ToLower(strings.Trim(r.ColumnName, "\"`'"))
		if rCol != cleanCol {
			continue
		}
		rTable := strings.ToLower(strings.Trim(r.TableName, "\"`'"))
		if rTable == "" || rTable == "*" || rTable == cleanTable {
			return r, true
		}
	}
	return nil, false
}

// CloneExecutionStatus describes the lifecycle stage of a clone execution.
type CloneExecutionStatus string

const (
	StatusDumping   CloneExecutionStatus = "DUMPING"
	StatusMasking   CloneExecutionStatus = "MASKING"
	StatusRestoring CloneExecutionStatus = "RESTORING"
	StatusCompleted CloneExecutionStatus = "COMPLETED"
	StatusFailed    CloneExecutionStatus = "FAILED"
)

// CloneExecution models an in-flight or completed sanitized database clone job.
type CloneExecution struct {
	ExecutionID     string               `json:"execution_id"`
	SourceClusterID string               `json:"source_cluster_id"`
	SourceDB        string               `json:"source_db"`
	TargetClusterID string               `json:"target_cluster_id"`
	TargetDB        string               `json:"target_db"`
	Status          CloneExecutionStatus `json:"status"`
	RowsMasked      int64                `json:"rows_masked"`
	StartedAt       time.Time            `json:"started_at"`
	FinishedAt      *time.Time           `json:"finished_at,omitempty"`
	ErrorMessage    string               `json:"error_message,omitempty"`
}

// NewCloneExecution instantiates and validates a new CloneExecution aggregate.
func NewCloneExecution(id, sourceClusterID, sourceDB, targetClusterID, targetDB string) (*CloneExecution, error) {
	if strings.TrimSpace(id) == "" {
		return nil, fmt.Errorf("%w: execution_id cannot be empty", ErrValidation)
	}
	if strings.TrimSpace(sourceClusterID) == "" {
		return nil, fmt.Errorf("%w: source_cluster_id cannot be empty", ErrValidation)
	}
	if strings.TrimSpace(sourceDB) == "" {
		return nil, fmt.Errorf("%w: source_db cannot be empty", ErrValidation)
	}
	if strings.TrimSpace(targetClusterID) == "" {
		return nil, fmt.Errorf("%w: target_cluster_id cannot be empty", ErrValidation)
	}
	if strings.TrimSpace(targetDB) == "" {
		return nil, fmt.Errorf("%w: target_db cannot be empty", ErrValidation)
	}

	return &CloneExecution{
		ExecutionID:     strings.TrimSpace(id),
		SourceClusterID: strings.TrimSpace(sourceClusterID),
		SourceDB:        strings.TrimSpace(sourceDB),
		TargetClusterID: strings.TrimSpace(targetClusterID),
		TargetDB:        strings.TrimSpace(targetDB),
		Status:          StatusDumping,
		RowsMasked:      0,
		StartedAt:       time.Now().UTC(),
	}, nil
}

// StartMasking transitions status to MASKING.
func (c *CloneExecution) StartMasking() {
	c.Status = StatusMasking
}

// StartRestoring transitions status to RESTORING.
func (c *CloneExecution) StartRestoring() {
	c.Status = StatusRestoring
}

// Complete transitions status to COMPLETED and sets finish time.
func (c *CloneExecution) Complete(rowsMasked int64) {
	c.Status = StatusCompleted
	c.RowsMasked = rowsMasked
	now := time.Now().UTC()
	c.FinishedAt = &now
}

// Fail transitions status to FAILED and records the error message.
func (c *CloneExecution) Fail(errMsg string) {
	c.Status = StatusFailed
	c.ErrorMessage = errMsg
	now := time.Now().UTC()
	c.FinishedAt = &now
}
