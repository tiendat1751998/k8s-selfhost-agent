# Anti-Mock & Code Quality Inventory

> Generated: 2026-09-09T15:48:00+07:00 | Branch: fix/comprehensive-audit

## Critical (Production Blockers)

| # | Description | File:Line | Evidence |
|---|-------------|-----------|----------|
| C1 | Faked Terraform Execution — returns mock success when binary missing | internal/infrastructure/iac/terraform.go:106-114 | `// Mock baseline output in mock/test environment without Terraform binary` |
| C2 | Faked Ansible Execution — returns mock success when binary missing | internal/infrastructure/iac/ansible.go:72-80 | `// Mock baseline output in test environment without Ansible binary` |
| C3 | Fake API Key generation via Math.random() on frontend | frontend-vue/src/components/settings/SettingsApiKeysTab.vue:47-51 | `Math.random().toString(36).substring(2, 6)` → `'k8s_live_' + rand` |
| C4 | Ignored errgroup.Wait() error | internal/adapter/event/collector.go:124 | `_ = eg.Wait()` |

## High (Mock/Stub/Fake in Production)

| # | Description | File:Line | Evidence |
|---|-------------|-----------|----------|
| H1 | Test-only branch in production code (fake clientset sync) | internal/usecase/storage/volume_usecase.go:241-247 | `// Sync the memory tracker in fake clientset for consistent test assertions` |
| H2 | Fake UI delay via time.Sleep | internal/usecase/agent/orchestrator.go:195 | `time.Sleep(1 * time.Second) // Small delay for UI smoothness` |

## Medium (TODO/FIXME/Missing Error Handling)

| # | Description | File:Line | Evidence |
|---|-------------|-----------|----------|
| M1 | Ignored RuleEngine init result | cmd/server/bootstrap_services.go:242 | `_ = usecaseAlert.NewRuleEngine(...)` |
| M2 | Ignored io.ReadAll error (S3 backup) | internal/infrastructure/backup/storage/s3.go:156 | `body, _ := io.ReadAll(resp.Body)` |
| M3 | 4x ignored json.Unmarshal errors | internal/usecase/ecosystem/detector_k8s.go:44,183,234,262 | `_ = json.Unmarshal(body, &payload)` |
| M4 | Ignored recovery codes update error | internal/usecase/auth/totp.go:317 | `_ = u.repo.SetRecoveryCodes(ctx, userID, "")` |
| M5 | Ignored alert notification error | internal/usecase/alert/rule_engine.go:70 | `_ = notifier.Send(...)` |
| M6 | Ignored metrics collect error | internal/usecase/metrics/tps_collector.go:233 | `_, _ = c.Collect(ctx)` |

## Low

| # | Description | Notes |
|---|-------------|-------|
| L1 | go_diagnostics clean | No issues |
| L2 | go_vulncheck: 33 known vulns in dependencies | stdlib + docker/chi/pgx — no first-party vuln code |

## Summary
- **Total: 4 critical, 2 high, 6 medium, 2 low**
- **Zero TODO/FIXME/HACK comments found** (good — previous session cleaned them)
- **33 dependency vulnerabilities** detected by go_vulncheck
