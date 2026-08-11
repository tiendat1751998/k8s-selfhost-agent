-- DOWN migration for 023_agent_framework.sql

DELETE FROM agent_project_state WHERE id = 'latest';

DROP TABLE IF EXISTS agent_project_state;
DROP TABLE IF EXISTS agent_executions;
DROP TABLE IF EXISTS agent_subtasks;
DROP TABLE IF EXISTS agent_tasks;
