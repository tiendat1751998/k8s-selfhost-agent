package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

type recordingExecutor struct {
	executed []string
	failOn   string
}

func (r *recordingExecutor) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	r.executed = append(r.executed, sql)
	if r.failOn != "" && strings.Contains(sql, r.failOn) {
		return pgconn.CommandTag{}, errors.New("simulated migration failure")
	}
	return pgconn.NewCommandTag("OK"), nil
}

func TestMigration_SQLConstants(t *testing.T) {
	if !strings.Contains(AdvisoryLockQuery, "pg_advisory_lock(hashtext('k8s_selfhost_migrations'))") {
		t.Errorf("AdvisoryLockQuery missing required lock expression: %s", AdvisoryLockQuery)
	}
	if !strings.Contains(AdvisoryUnlockQuery, "pg_advisory_unlock(hashtext('k8s_selfhost_migrations'))") {
		t.Errorf("AdvisoryUnlockQuery missing required unlock expression: %s", AdvisoryUnlockQuery)
	}
	if !strings.Contains(SetLockTimeoutQuery, "SET lock_timeout = '2s'") {
		t.Errorf("SetLockTimeoutQuery missing lock_timeout = '2s': %s", SetLockTimeoutQuery)
	}
}

func TestMigration_AdvisoryLock_AcquiredAndReleasedInDefer(t *testing.T) {
	// Setup temporary migration files
	tempDir, err := os.MkdirTemp("", "migration-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	file1 := filepath.Join(tempDir, "001_test.up.sql")
	file2 := filepath.Join(tempDir, "002_test.up.sql")
	_ = os.WriteFile(file1, []byte("CREATE TABLE t1 (id int);"), 0644)
	_ = os.WriteFile(file2, []byte("CREATE TABLE t2 (id int);"), 0644)

	rec := &recordingExecutor{}
	err = RunMigrations(context.Background(), rec, []string{file1, file2}, "UP")
	if err != nil {
		t.Fatalf("unexpected error running migrations: %v", err)
	}

	if len(rec.executed) < 4 {
		t.Fatalf("expected at least 4 queries executed, got %d", len(rec.executed))
	}

	// 1. First query must be pg_advisory_lock
	if rec.executed[0] != AdvisoryLockQuery {
		t.Errorf("first query should be advisory lock, got: %s", rec.executed[0])
	}

	// 2. Second query must be lock_timeout
	if rec.executed[1] != SetLockTimeoutQuery {
		t.Errorf("second query should be lock timeout, got: %s", rec.executed[1])
	}

	// 3. Middle queries are DDLs
	if rec.executed[2] != "CREATE TABLE t1 (id int);" {
		t.Errorf("expected DDL 1, got: %s", rec.executed[2])
	}
	if rec.executed[3] != "CREATE TABLE t2 (id int);" {
		t.Errorf("expected DDL 2, got: %s", rec.executed[3])
	}

	// 4. Final query must be pg_advisory_unlock (from defer)
	lastIdx := len(rec.executed) - 1
	if rec.executed[lastIdx] != AdvisoryUnlockQuery {
		t.Errorf("last query should be advisory unlock from defer, got: %s", rec.executed[lastIdx])
	}
}

func TestMigration_AdvisoryLock_ReleasedOnFailure(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "migration-fail-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	file1 := filepath.Join(tempDir, "001_fail.up.sql")
	_ = os.WriteFile(file1, []byte("BAD DDL STATEMENT"), 0644)

	rec := &recordingExecutor{failOn: "BAD DDL"}
	err = RunMigrations(context.Background(), rec, []string{file1}, "UP")
	if err == nil {
		t.Fatalf("expected error from bad DDL, got nil")
	}

	// Verify advisory unlock was called despite failure
	unlocked := false
	for _, q := range rec.executed {
		if q == AdvisoryUnlockQuery {
			unlocked = true
			break
		}
	}
	if !unlocked {
		t.Errorf("expected AdvisoryUnlockQuery to be executed in defer on failure, but was not found in: %v", rec.executed)
	}
}

func TestGetEnv(t *testing.T) {
	t.Setenv("TEST_PRIMARY", "val1")
	t.Setenv("TEST_SECONDARY", "val2")
	if v := getEnv("TEST_PRIMARY", "TEST_SECONDARY", "fallback"); v != "val1" {
		t.Errorf("expected val1, got %s", v)
	}

	t.Setenv("TEST_PRIMARY", "")
	if v := getEnv("TEST_PRIMARY", "TEST_SECONDARY", "fallback"); v != "val2" {
		t.Errorf("expected val2, got %s", v)
	}

	t.Setenv("TEST_SECONDARY", "")
	if v := getEnv("TEST_PRIMARY", "TEST_SECONDARY", "fallback"); v != "fallback" {
		t.Errorf("expected fallback, got %s", v)
	}
}
