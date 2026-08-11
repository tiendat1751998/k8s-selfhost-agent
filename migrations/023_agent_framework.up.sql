-- Migration: 023_agent_framework.sql
-- Description: Create agent tasks, subtasks, executions, and project state tables.

CREATE TABLE IF NOT EXISTS agent_tasks (
    id VARCHAR(255) PRIMARY KEY,
    phase VARCHAR(100) NOT NULL,
    module VARCHAR(255) NOT NULL,
    feature VARCHAR(255) NOT NULL,
    title VARCHAR(255) NOT NULL,
    description TEXT NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'pending', -- pending, inprogress, success, blocked, failed
    dependencies TEXT NOT NULL DEFAULT '[]', -- JSON array of parent task IDs
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMP WITH TIME ZONE
);

CREATE TABLE IF NOT EXISTS agent_subtasks (
    id VARCHAR(255) PRIMARY KEY,
    task_id VARCHAR(255) NOT NULL REFERENCES agent_tasks(id) ON DELETE CASCADE,
    title VARCHAR(255) NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'pending', -- pending, inprogress, success, failed
    complexity INTEGER NOT NULL DEFAULT 1,
    exec_order INTEGER NOT NULL,
    completed_at TIMESTAMP WITH TIME ZONE
);

CREATE TABLE IF NOT EXISTS agent_executions (
    id VARCHAR(255) PRIMARY KEY,
    task_id VARCHAR(255) NOT NULL REFERENCES agent_tasks(id) ON DELETE CASCADE,
    agent_type VARCHAR(100) NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'running', -- running, success, failed
    input TEXT,
    output TEXT,
    error_detail TEXT,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMP WITH TIME ZONE
);

CREATE TABLE IF NOT EXISTS agent_project_state (
    id VARCHAR(255) PRIMARY KEY DEFAULT 'latest',
    current_phase VARCHAR(100) NOT NULL,
    current_module VARCHAR(255) NOT NULL,
    current_feature VARCHAR(255) NOT NULL,
    current_task_id VARCHAR(255),
    current_subtask_id VARCHAR(255),
    repository_health DOUBLE PRECISION NOT NULL DEFAULT 100.0,
    technical_debt DOUBLE PRECISION NOT NULL DEFAULT 0.0,
    architecture_score DOUBLE PRECISION NOT NULL DEFAULT 100.0,
    quality_score DOUBLE PRECISION NOT NULL DEFAULT 100.0,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Insert initial project state if not exists
INSERT INTO agent_project_state (id, current_phase, current_module, current_feature, repository_health, technical_debt, architecture_score, quality_score)
VALUES ('latest', 'Phase 11', 'Multi-Agent Framework', 'Core Setup', 100.0, 0.0, 100.0, 100.0)
ON CONFLICT (id) DO NOTHING;
