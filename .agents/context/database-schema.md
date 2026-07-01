# Database Schema

## Overview

PostgreSQL 16 with 24 timestamped migration files. All tables use UUID primary keys via `uuid-ossp` extension.

---

## Core Tables

### `incidents` (Migration 001)
The aggregate root for incident management.

| Column | Type | Constraints |
|--------|------|-------------|
| `id` | UUID | PK, DEFAULT uuid_generate_v4() |
| `cluster_name` | VARCHAR(255) | NOT NULL |
| `namespace` | VARCHAR(255) | NOT NULL |
| `pod_name` | VARCHAR(255) | NOT NULL |
| `type` | VARCHAR(50) | NOT NULL |
| `status` | VARCHAR(50) | NOT NULL DEFAULT 'detected' |
| `severity` | VARCHAR(20) | NOT NULL DEFAULT 'medium' |
| `message` | TEXT | NOT NULL DEFAULT '' |
| `raw_data` | JSONB | NOT NULL DEFAULT '{}' |
| `created_at` | TIMESTAMPTZ | NOT NULL DEFAULT NOW() |
| `updated_at` | TIMESTAMPTZ | NOT NULL DEFAULT NOW() |
| `resolved_at` | TIMESTAMPTZ | nullable |

**Indexes**: status, type, severity, namespace, cluster_name, (namespace+pod_name+type), created_at DESC

### `rca_reports` (Migration 001)

| Column | Type | Constraints |
|--------|------|-------------|
| `id` | UUID | PK |
| `incident_id` | UUID | FK -> incidents(id) CASCADE |
| `root_cause` | TEXT | NOT NULL |
| `evidence` | JSONB | NOT NULL DEFAULT '[]' |
| `confidence` | DECIMAL(4,3) | NOT NULL, CHECK 0-1 |
| `risk_level` | VARCHAR(20) | NOT NULL |
| `remediation` | TEXT | NOT NULL |
| `rollback_plan` | TEXT | NOT NULL DEFAULT '' |
| `llm_model` | VARCHAR(100) | NOT NULL DEFAULT '' |
| `prompt_tokens` | INTEGER | NOT NULL DEFAULT 0 |
| `response_tokens` | INTEGER | NOT NULL DEFAULT 0 |
| `created_at` | TIMESTAMPTZ | NOT NULL DEFAULT NOW() |

### `gitops_prs` (Migration 001)

| Column | Type | Constraints |
|--------|------|-------------|
| `id` | UUID | PK |
| `incident_id` | UUID | FK -> incidents(id) CASCADE |
| `provider` | VARCHAR(20) | NOT NULL |
| `repo_url` | VARCHAR(500) | NOT NULL |
| `branch` | VARCHAR(255) | NOT NULL |
| `title` | VARCHAR(500) | NOT NULL |
| `pr_url` | VARCHAR(500) | NOT NULL DEFAULT '' |
| `pr_number` | INTEGER | NOT NULL DEFAULT 0 |
| `status` | VARCHAR(20) | NOT NULL DEFAULT 'pending' |
| `files_changed` | JSONB | NOT NULL DEFAULT '[]' |

---

## Auth & RBAC Tables (Migration 020)

### `users`

| Column | Type | Constraints |
|--------|------|-------------|
| `id` | UUID | PK |
| `email` | VARCHAR(255) | UNIQUE NOT NULL |
| `password_hash` | VARCHAR(255) | NOT NULL (bcrypt) |
| `first_name` | VARCHAR(100) | nullable |
| `last_name` | VARCHAR(100) | nullable |
| `status` | VARCHAR(50) | DEFAULT 'active' |

### `roles`

| Column | Type | Constraints |
|--------|------|-------------|
| `id` | UUID | PK |
| `name` | VARCHAR(50) | UNIQUE NOT NULL |
| `description` | TEXT | nullable |
| `permissions` | JSONB | NOT NULL DEFAULT '[]' |

**Seeded Roles**: `platform_admin` ("[*]"), `tenant_admin` ("[tenant:*]"), `viewer` ("[tenant:read]")

### `user_roles`

| Column | Type | Constraints |
|--------|------|-------------|
| `user_id` | UUID | FK -> users, PK |
| `role_id` | UUID | FK -> roles, PK |

### `tenant_bindings`

| Column | Type | Constraints |
|--------|------|-------------|
| `id` | UUID | PK |
| `user_id` | UUID | FK -> users |
| `tenant_id` | VARCHAR(255) | NOT NULL |
| `role_id` | UUID | FK -> roles |

---

## Migration Index

| # | File | Purpose |
|---|------|---------|
| 001 | `001_initial.sql` | incidents, rca_reports, gitops_prs |
| 002 | `002_enterprise_config.sql` | Enterprise configuration tables |
| 003 | `003_platform_features.sql` | Platform feature tables |
| 004 | `004_capacity_planning.sql` | Capacity planning |
| 005 | `005_drift_detection.sql` | Drift detection |
| 006 | `006_event_correlation.sql` | Event correlation |
| 007 | `007_change_management.sql` | Change management |
| 008 | `008_deployment_promotion.sql` | Deployment promotion |
| 009 | `009_resource_explorer.sql` | Resource explorer |
| 010 | `010_tagging_system.sql` | Resource tagging |
| 011 | `011_reporting_center.sql` | Reporting center |
| 012 | `012_health_center.sql` | Health center |
| 013 | `013_fleet_view.sql` | Fleet cluster management |
| 014 | `014_platform_audit.sql` | Audit logging |
| 015 | `015_timeline.sql` | Event timeline |
| 016 | `016_runbooks.sql` | Runbook definitions |
| 017 | `017_automation.sql` | Automation rules |
| 018 | `018_notification.sql` | Notification channels |
| 019 | `019_observability_slo.sql` | SLO/SLI definitions |
| 020 | `020_auth_rbac.sql` | Users, roles, RBAC, tenant bindings |
| 021 | `021_tenant_resources.sql` | Tenant resource scoping |
| 022 | `022_cost_backup_seeds.sql` | Cost and backup seed data |
| 023 | `023_agent_framework.sql` | Agent framework tables |
| 024 | `024_fix_observability_slo_columns.sql` | Fix SLO column types |

---

## Key Conventions

- **Primary Keys**: UUID via `uuid_generate_v4()`
- **Timestamps**: All tables use `TIMESTAMPTZ` with `DEFAULT NOW()`
- **Soft Deletes**: Not used. Hard deletes with `ON DELETE CASCADE` on FKs.
- **JSON Storage**: JSONB for flexible data (evidence, permissions, raw_data)
- **Encryption**: `fleet_clusters.encrypted_token` stores AES-256-GCM encrypted credentials
- **SQL Injection Prevention**: All queries use parameter-bound placeholders (`$1, $2`)
