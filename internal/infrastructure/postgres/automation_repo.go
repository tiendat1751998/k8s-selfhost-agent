package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/datdt/k8sselfhost/internal/domain/automation"
)

type automationRepo struct {
	db DBTX
}

// NewAutomationRepo creates a new Postgres-backed Automation repository.
func NewAutomationRepo(db DBTX) automation.Repository {
	return &automationRepo{db: db}
}

func (r *automationRepo) getDB(ctx context.Context) DBTX {
	return ExtractTx(ctx, r.db)
}

func (r *automationRepo) ListRules(ctx context.Context) ([]automation.Rule, error) {
	query := `
		SELECT id, name, trigger_type, trigger_config, action_type, action_config, 
		       enabled, executions, last_triggered, created_at, updated_at 
		FROM automation_rules 
		ORDER BY created_at DESC
	`
	rows, err := r.getDB(ctx).Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("querying automation rules: %w", err)
	}
	defer rows.Close()

	var rules []automation.Rule
	for rows.Next() {
		var rule automation.Rule
		var triggerBytes, actionBytes []byte
		
		if err := rows.Scan(
			&rule.ID, &rule.Name, &rule.TriggerType, &triggerBytes,
			&rule.ActionType, &actionBytes, &rule.Enabled, &rule.Executions,
			&rule.LastTriggered, &rule.CreatedAt, &rule.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning automation rule: %w", err)
		}
		
		if len(triggerBytes) > 0 {
			if err := json.Unmarshal(triggerBytes, &rule.TriggerConfig); err != nil {
				return nil, fmt.Errorf("unmarshaling trigger config: %w", err)
			}
		} else {
			rule.TriggerConfig = make(map[string]string)
		}
		
		if len(actionBytes) > 0 {
			if err := json.Unmarshal(actionBytes, &rule.ActionConfig); err != nil {
				return nil, fmt.Errorf("unmarshaling action config: %w", err)
			}
		} else {
			rule.ActionConfig = make(map[string]string)
		}

		rules = append(rules, rule)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating automation rules: %w", err)
	}

	return rules, nil
}

func (r *automationRepo) GetRule(ctx context.Context, id string) (*automation.Rule, error) {
	query := `
		SELECT id, name, trigger_type, trigger_config, action_type, action_config, 
		       enabled, executions, last_triggered, created_at, updated_at 
		FROM automation_rules 
		WHERE id = $1
	`
	var rule automation.Rule
	var triggerBytes, actionBytes []byte

	err := r.getDB(ctx).QueryRow(ctx, query, id).Scan(
		&rule.ID, &rule.Name, &rule.TriggerType, &triggerBytes,
		&rule.ActionType, &actionBytes, &rule.Enabled, &rule.Executions,
		&rule.LastTriggered, &rule.CreatedAt, &rule.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("automation rule not found")
		}
		return nil, fmt.Errorf("querying automation rule: %w", err)
	}

	if len(triggerBytes) > 0 {
		if err := json.Unmarshal(triggerBytes, &rule.TriggerConfig); err != nil {
			return nil, fmt.Errorf("unmarshaling trigger config: %w", err)
		}
	} else {
		rule.TriggerConfig = make(map[string]string)
	}
	
	if len(actionBytes) > 0 {
		if err := json.Unmarshal(actionBytes, &rule.ActionConfig); err != nil {
			return nil, fmt.Errorf("unmarshaling action config: %w", err)
		}
	} else {
		rule.ActionConfig = make(map[string]string)
	}

	return &rule, nil
}

func (r *automationRepo) CreateRule(ctx context.Context, rule *automation.Rule) error {
	query := `
		INSERT INTO automation_rules (name, trigger_type, trigger_config, action_type, action_config, enabled, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id
	`
	
	triggerBytes, err := json.Marshal(rule.TriggerConfig)
	if err != nil {
		return fmt.Errorf("marshaling trigger config: %w", err)
	}
	
	actionBytes, err := json.Marshal(rule.ActionConfig)
	if err != nil {
		return fmt.Errorf("marshaling action config: %w", err)
	}

	err = r.getDB(ctx).QueryRow(ctx, query,
		rule.Name, string(rule.TriggerType), triggerBytes, string(rule.ActionType), actionBytes, rule.Enabled, rule.CreatedAt, rule.UpdatedAt,
	).Scan(&rule.ID)
	
	if err != nil {
		return fmt.Errorf("inserting automation rule: %w", err)
	}
	
	return nil
}

func (r *automationRepo) UpdateRule(ctx context.Context, rule *automation.Rule) error {
	rule.UpdatedAt = time.Now()
	query := `
		UPDATE automation_rules 
		SET name = $1, trigger_type = $2, trigger_config = $3, action_type = $4, action_config = $5, updated_at = $6
		WHERE id = $7
	`
	
	triggerBytes, err := json.Marshal(rule.TriggerConfig)
	if err != nil {
		return fmt.Errorf("marshaling trigger config: %w", err)
	}
	
	actionBytes, err := json.Marshal(rule.ActionConfig)
	if err != nil {
		return fmt.Errorf("marshaling action config: %w", err)
	}

	cmd, err := r.getDB(ctx).Exec(ctx, query,
		rule.Name, string(rule.TriggerType), triggerBytes, string(rule.ActionType), actionBytes, rule.UpdatedAt, rule.ID,
	)
	if err != nil {
		return fmt.Errorf("updating automation rule: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return fmt.Errorf("automation rule not found")
	}
	return nil
}

func (r *automationRepo) DeleteRule(ctx context.Context, id string) error {
	query := `DELETE FROM automation_rules WHERE id = $1`
	cmd, err := r.getDB(ctx).Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("deleting automation rule: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return fmt.Errorf("automation rule not found")
	}
	return nil
}

func (r *automationRepo) ToggleRule(ctx context.Context, id string, enabled bool) error {
	query := `UPDATE automation_rules SET enabled = $1, updated_at = NOW() WHERE id = $2`
	cmd, err := r.getDB(ctx).Exec(ctx, query, enabled, id)
	if err != nil {
		return fmt.Errorf("toggling automation rule: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return fmt.Errorf("automation rule not found")
	}
	return nil
}

func (r *automationRepo) ListExecutions(ctx context.Context, limit, offset int) ([]automation.Execution, int, error) {
	query := `
		SELECT id, rule_id, rule_name, trigger_event, action_taken, result, error_detail, created_at 
		FROM automation_executions 
		ORDER BY created_at DESC 
		LIMIT $1 OFFSET $2
	`
	countQuery := `SELECT COUNT(*) FROM automation_executions`

	var total int
	err := r.getDB(ctx).QueryRow(ctx, countQuery).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("counting automation executions: %w", err)
	}

	rows, err := r.getDB(ctx).Query(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("querying automation executions: %w", err)
	}
	defer rows.Close()

	var execs []automation.Execution
	for rows.Next() {
		var e automation.Execution
		var errDetail *string

		if err := rows.Scan(
			&e.ID, &e.RuleID, &e.RuleName, &e.TriggerEvent, &e.ActionTaken, &e.Result, &errDetail, &e.CreatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scanning automation execution: %w", err)
		}
		
		if errDetail != nil {
			e.ErrorDetail = *errDetail
		}

		execs = append(execs, e)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterating automation executions: %w", err)
	}

	return execs, total, nil
}

func (r *automationRepo) CreateExecution(ctx context.Context, e *automation.Execution) error {
	query := `
		INSERT INTO automation_executions (rule_id, rule_name, trigger_event, action_taken, result, error_detail, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id
	`
	
	var errDetail *string
	if e.ErrorDetail != "" {
		errDetail = &e.ErrorDetail
	}

	err := r.getDB(ctx).QueryRow(ctx, query,
		e.RuleID, e.RuleName, e.TriggerEvent, e.ActionTaken, e.Result, errDetail, e.CreatedAt,
	).Scan(&e.ID)
	
	if err != nil {
		return fmt.Errorf("inserting automation execution: %w", err)
	}
	
	// Update the rule stats
	updateQuery := `
		UPDATE automation_rules 
		SET executions = executions + 1, last_triggered = NOW() 
		WHERE id = $1
	`
	_, _ = r.getDB(ctx).Exec(ctx, updateQuery, e.RuleID)
	
	return nil
}
