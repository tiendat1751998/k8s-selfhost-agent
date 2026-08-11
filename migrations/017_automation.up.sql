-- Migration: 017_automation.sql
-- Creates tables for automation rules and executions.

CREATE TABLE IF NOT EXISTS automation_rules (
    id             UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name           VARCHAR(255) NOT NULL,
    trigger_type   VARCHAR(100) NOT NULL,
    trigger_config JSONB DEFAULT '{}'::jsonb,
    action_type    VARCHAR(100) NOT NULL,
    action_config  JSONB DEFAULT '{}'::jsonb,
    enabled        BOOLEAN NOT NULL DEFAULT true,
    executions     INT NOT NULL DEFAULT 0,
    last_triggered TIMESTAMPTZ,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS automation_executions (
    id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    rule_id       UUID NOT NULL REFERENCES automation_rules(id) ON DELETE CASCADE,
    rule_name     VARCHAR(255) NOT NULL,
    trigger_event TEXT NOT NULL,
    action_taken  TEXT NOT NULL,
    result        VARCHAR(50) NOT NULL,
    error_detail  TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_automation_rules_enabled ON automation_rules (enabled);
CREATE INDEX idx_automation_executions_rule_id ON automation_executions (rule_id);
CREATE INDEX idx_automation_executions_created_at ON automation_executions (created_at DESC);
