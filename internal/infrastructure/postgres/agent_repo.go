package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/datdt/k8sselfhost/internal/domain/agent"
)

type agentRepo struct {
	db DBTX
}

// NewAgentRepo creates a new Postgres-backed Agent repository.
func NewAgentRepo(db DBTX) agent.Repository {
	return &agentRepo{db: db}
}

func (r *agentRepo) getDB(ctx context.Context) DBTX {
	return ExtractTx(ctx, r.db)
}

// Tasks
func (r *agentRepo) CreateTask(ctx context.Context, task *agent.Task) error {
	depsJSON, err := json.Marshal(task.Dependencies)
	if err != nil {
		depsJSON = []byte("[]")
	}

	query := `
		INSERT INTO agent_tasks (id, phase, module, feature, title, description, status, dependencies, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	_, err = r.getDB(ctx).Exec(ctx, query,
		task.ID, task.Phase, task.Module, task.Feature, task.Title, task.Description,
		string(task.Status), string(depsJSON), task.CreatedAt, task.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("creating agent task: %w", err)
	}

	// Insert subtasks if any
	for _, sub := range task.Subtasks {
		if err := r.CreateSubtask(ctx, &sub); err != nil {
			return err
		}
	}

	return nil
}

func (r *agentRepo) GetTask(ctx context.Context, id string) (*agent.Task, error) {
	query := `
		SELECT id, phase, module, feature, title, description, status, dependencies, created_at, updated_at, completed_at
		FROM agent_tasks
		WHERE id = $1
	`
	var t agent.Task
	var depsStr string
	var statusStr string
	err := r.getDB(ctx).QueryRow(ctx, query, id).Scan(
		&t.ID, &t.Phase, &t.Module, &t.Feature, &t.Title, &t.Description,
		&statusStr, &depsStr, &t.CreatedAt, &t.UpdatedAt, &t.CompletedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("agent task not found")
		}
		return nil, fmt.Errorf("getting agent task: %w", err)
	}
	t.Status = agent.TaskStatus(statusStr)

	if err := json.Unmarshal([]byte(depsStr), &t.Dependencies); err != nil {
		t.Dependencies = []string{}
	}

	// Fetch subtasks
	subtasks, err := r.listSubtasks(ctx, t.ID)
	if err != nil {
		return nil, err
	}
	t.Subtasks = subtasks

	return &t, nil
}

func (r *agentRepo) ListTasks(ctx context.Context) ([]agent.Task, error) {
	query := `
		SELECT id, phase, module, feature, title, description, status, dependencies, created_at, updated_at, completed_at
		FROM agent_tasks
		ORDER BY created_at ASC
	`
	rows, err := r.getDB(ctx).Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("listing agent tasks: %w", err)
	}
	defer rows.Close()

	var tasks []agent.Task
	for rows.Next() {
		var t agent.Task
		var depsStr string
		var statusStr string
		err := rows.Scan(
			&t.ID, &t.Phase, &t.Module, &t.Feature, &t.Title, &t.Description,
			&statusStr, &depsStr, &t.CreatedAt, &t.UpdatedAt, &t.CompletedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning agent task: %w", err)
		}
		t.Status = agent.TaskStatus(statusStr)
		if err := json.Unmarshal([]byte(depsStr), &t.Dependencies); err != nil {
			t.Dependencies = []string{}
		}

		subtasks, err := r.listSubtasks(ctx, t.ID)
		if err != nil {
			return nil, err
		}
		t.Subtasks = subtasks

		tasks = append(tasks, t)
	}

	return tasks, nil
}

func (r *agentRepo) UpdateTask(ctx context.Context, task *agent.Task) error {
	depsJSON, err := json.Marshal(task.Dependencies)
	if err != nil {
		depsJSON = []byte("[]")
	}

	query := `
		UPDATE agent_tasks
		SET phase = $1, module = $2, feature = $3, title = $4, description = $5,
		    status = $6, dependencies = $7, updated_at = $8, completed_at = $9
		WHERE id = $10
	`
	cmd, err := r.getDB(ctx).Exec(ctx, query,
		task.Phase, task.Module, task.Feature, task.Title, task.Description,
		string(task.Status), string(depsJSON), task.UpdatedAt, task.CompletedAt, task.ID,
	)
	if err != nil {
		return fmt.Errorf("updating agent task: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return fmt.Errorf("agent task to update not found")
	}
	return nil
}

// Subtasks
func (r *agentRepo) CreateSubtask(ctx context.Context, sub *agent.Subtask) error {
	query := `
		INSERT INTO agent_subtasks (id, task_id, title, status, complexity, exec_order, completed_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err := r.getDB(ctx).Exec(ctx, query,
		sub.ID, sub.TaskID, sub.Title, string(sub.Status), sub.Complexity, sub.ExecOrder, sub.CompletedAt,
	)
	if err != nil {
		return fmt.Errorf("creating agent subtask: %w", err)
	}
	return nil
}

func (r *agentRepo) UpdateSubtask(ctx context.Context, sub *agent.Subtask) error {
	query := `
		UPDATE agent_subtasks
		SET status = $1, completed_at = $2
		WHERE id = $3
	`
	cmd, err := r.getDB(ctx).Exec(ctx, query, string(sub.Status), sub.CompletedAt, sub.ID)
	if err != nil {
		return fmt.Errorf("updating agent subtask: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return fmt.Errorf("agent subtask to update not found")
	}
	return nil
}

func (r *agentRepo) listSubtasks(ctx context.Context, taskID string) ([]agent.Subtask, error) {
	query := `
		SELECT id, task_id, title, status, complexity, exec_order, completed_at
		FROM agent_subtasks
		WHERE task_id = $1
		ORDER BY exec_order ASC
	`
	rows, err := r.getDB(ctx).Query(ctx, query, taskID)
	if err != nil {
		return nil, fmt.Errorf("listing agent subtasks: %w", err)
	}
	defer rows.Close()

	var subs []agent.Subtask
	for rows.Next() {
		var s agent.Subtask
		var statusStr string
		err := rows.Scan(&s.ID, &s.TaskID, &s.Title, &statusStr, &s.Complexity, &s.ExecOrder, &s.CompletedAt)
		if err != nil {
			return nil, fmt.Errorf("scanning agent subtask: %w", err)
		}
		s.Status = agent.SubtaskStatus(statusStr)
		subs = append(subs, s)
	}
	return subs, nil
}

// Executions
func (r *agentRepo) RecordExecution(ctx context.Context, exec *agent.Execution) error {
	query := `
		INSERT INTO agent_executions (id, task_id, agent_type, status, input, output, error_detail, created_at, completed_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	_, err := r.getDB(ctx).Exec(ctx, query,
		exec.ID, exec.TaskID, string(exec.AgentType), exec.Status,
		exec.Input, exec.Output, exec.ErrorDetail, exec.CreatedAt, exec.CompletedAt,
	)
	if err != nil {
		return fmt.Errorf("recording agent execution: %w", err)
	}
	return nil
}

func (r *agentRepo) GetExecution(ctx context.Context, id string) (*agent.Execution, error) {
	query := `
		SELECT id, task_id, agent_type, status, input, output, error_detail, created_at, completed_at
		FROM agent_executions
		WHERE id = $1
	`
	var e agent.Execution
	var agentTypeStr string
	err := r.getDB(ctx).QueryRow(ctx, query, id).Scan(
		&e.ID, &e.TaskID, &agentTypeStr, &e.Status, &e.Input, &e.Output, &e.ErrorDetail, &e.CreatedAt, &e.CompletedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("agent execution not found")
		}
		return nil, fmt.Errorf("getting agent execution: %w", err)
	}
	e.AgentType = agent.AgentType(agentTypeStr)
	return &e, nil
}

func (r *agentRepo) ListExecutions(ctx context.Context, taskID string) ([]agent.Execution, error) {
	var rows pgx.Rows
	var err error
	if taskID != "" {
		query := `
			SELECT id, task_id, agent_type, status, input, output, error_detail, created_at, completed_at
			FROM agent_executions
			WHERE task_id = $1
			ORDER BY created_at DESC
		`
		rows, err = r.getDB(ctx).Query(ctx, query, taskID)
	} else {
		query := `
			SELECT id, task_id, agent_type, status, input, output, error_detail, created_at, completed_at
			FROM agent_executions
			ORDER BY created_at DESC
		`
		rows, err = r.getDB(ctx).Query(ctx, query)
	}
	if err != nil {
		return nil, fmt.Errorf("listing agent executions: %w", err)
	}
	defer rows.Close()

	var execs []agent.Execution
	for rows.Next() {
		var e agent.Execution
		var agentTypeStr string
		err := rows.Scan(
			&e.ID, &e.TaskID, &agentTypeStr, &e.Status, &e.Input, &e.Output, &e.ErrorDetail, &e.CreatedAt, &e.CompletedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scanning agent execution: %w", err)
		}
		e.AgentType = agent.AgentType(agentTypeStr)
		execs = append(execs, e)
	}
	return execs, nil
}

func (r *agentRepo) UpdateExecution(ctx context.Context, exec *agent.Execution) error {
	query := `
		UPDATE agent_executions
		SET status = $1, output = $2, error_detail = $3, completed_at = $4
		WHERE id = $5
	`
	cmd, err := r.getDB(ctx).Exec(ctx, query, exec.Status, exec.Output, exec.ErrorDetail, exec.CompletedAt, exec.ID)
	if err != nil {
		return fmt.Errorf("updating agent execution: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return fmt.Errorf("agent execution to update not found")
	}
	return nil
}

// Project State
func (r *agentRepo) GetProjectState(ctx context.Context) (*agent.ProjectState, error) {
	query := `
		SELECT id, current_phase, current_module, current_feature, current_task_id, current_subtask_id,
		       repository_health, technical_debt, architecture_score, quality_score, updated_at
		FROM agent_project_state
		WHERE id = 'latest'
	`
	var s agent.ProjectState
	err := r.getDB(ctx).QueryRow(ctx, query).Scan(
		&s.ID, &s.CurrentPhase, &s.CurrentModule, &s.CurrentFeature, &s.CurrentTaskID, &s.CurrentSubtaskID,
		&s.RepositoryHealth, &s.TechnicalDebt, &s.ArchitectureScore, &s.QualityScore, &s.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			// Seed if not exists
			now := time.Now().UTC()
			s = agent.ProjectState{
				ID:                "latest",
				CurrentPhase:      "Phase 11",
				CurrentModule:     "Multi-Agent Framework",
				CurrentFeature:    "Core Setup",
				RepositoryHealth:  100.0,
				TechnicalDebt:     0.0,
				ArchitectureScore: 100.0,
				QualityScore:      100.0,
				UpdatedAt:         now,
			}
			err = r.UpdateProjectState(ctx, &s)
			if err != nil {
				return nil, err
			}
			return &s, nil
		}
		return nil, fmt.Errorf("getting project state: %w", err)
	}
	return &s, nil
}

func (r *agentRepo) UpdateProjectState(ctx context.Context, state *agent.ProjectState) error {
	query := `
		INSERT INTO agent_project_state (id, current_phase, current_module, current_feature, current_task_id, current_subtask_id,
		                               repository_health, technical_debt, architecture_score, quality_score, updated_at)
		VALUES ('latest', $1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (id) DO UPDATE
		SET current_phase = EXCLUDED.current_phase,
		    current_module = EXCLUDED.current_module,
		    current_feature = EXCLUDED.current_feature,
		    current_task_id = EXCLUDED.current_task_id,
		    current_subtask_id = EXCLUDED.current_subtask_id,
		    repository_health = EXCLUDED.repository_health,
		    technical_debt = EXCLUDED.technical_debt,
		    architecture_score = EXCLUDED.architecture_score,
		    quality_score = EXCLUDED.quality_score,
		    updated_at = EXCLUDED.updated_at
	`
	_, err := r.getDB(ctx).Exec(ctx, query,
		state.CurrentPhase, state.CurrentModule, state.CurrentFeature, state.CurrentTaskID, state.CurrentSubtaskID,
		state.RepositoryHealth, state.TechnicalDebt, state.ArchitectureScore, state.QualityScore, state.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("updating project state: %w", err)
	}
	return nil
}
