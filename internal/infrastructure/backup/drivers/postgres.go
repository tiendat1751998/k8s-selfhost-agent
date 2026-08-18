package drivers

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/datdt/k8sselfhost/internal/domain/backup"
	"github.com/datdt/k8sselfhost/internal/pkg/errors"
)

// PostgresDriver handles PostgreSQL backup and restore via pg_dump and psql/pg_restore
type PostgresDriver struct{}

func NewPostgresDriver() *PostgresDriver {
	return &PostgresDriver{}
}

func (d *PostgresDriver) Type() string {
	return "postgres"
}

func (d *PostgresDriver) buildConnString(opts backup.DumpOptions) string {
	port := opts.Port
	if port == 0 {
		port = 5432
	}
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		opts.Username, opts.Password, opts.Host, port, opts.Database)
}

func (d *PostgresDriver) ValidateConnection(ctx context.Context, opts backup.DumpOptions) error {
	connStr := d.buildConnString(opts)
	conn, err := pgx.Connect(ctx, connStr)
	if err != nil {
		return errors.Wrap(err, "connecting to postgres")
	}
	defer conn.Close(ctx)

	return conn.Ping(ctx)
}

func (d *PostgresDriver) Dump(ctx context.Context, opts backup.DumpOptions, writer io.Writer) (*backup.DumpResult, error) {
	start := time.Now()
	port := opts.Port
	if port == 0 {
		port = 5432
	}

	args := []string{
		"-h", opts.Host,
		"-p", fmt.Sprintf("%d", port),
		"-U", opts.Username,
		"-d", opts.Database,
		"--format=c", // Custom archive format for pg_restore
		"--no-owner",
		"--no-privileges",
	}

	if opts.BackupType == "schema_only" {
		args = append(args, "--schema-only")
	} else if opts.BackupType == "data_only" {
		args = append(args, "--data-only")
	}

	for _, t := range opts.Tables {
		args = append(args, "-t", t)
	}
	for _, t := range opts.ExcludeTab {
		args = append(args, "-T", t)
	}

	cmd := exec.CommandContext(ctx, "pg_dump", args...)
	if opts.Password != "" {
		cmd.Env = append(cmd.Environ(), fmt.Sprintf("PGPASSWORD=%s", opts.Password))
	}

	if _, err := exec.LookPath("pg_dump"); err != nil {
		return nil, fmt.Errorf("%w: pg_dump", errors.ErrBinaryNotFound)
	}

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, errors.Wrap(err, "creating stdout pipe for pg_dump")
	}

	if err := cmd.Start(); err != nil {
		return nil, errors.Wrap(err, "starting pg_dump")
	}

	copiedBytes, err := io.Copy(writer, stdoutPipe)
	if err != nil {
		return nil, errors.Wrap(err, "streaming pg_dump output")
	}

	if err := cmd.Wait(); err != nil {
		return nil, errors.Wrap(err, "pg_dump execution failed")
	}

	return &backup.DumpResult{
		UncompressedBytes: copiedBytes,
		Duration:          time.Since(start),
	}, nil
}



func (d *PostgresDriver) Restore(ctx context.Context, opts backup.RestoreOptions, reader io.Reader) error {
	port := opts.Port
	if port == 0 {
		port = 5432
	}

	args := []string{
		"-h", opts.Host,
		"-p", fmt.Sprintf("%d", port),
		"-U", opts.Username,
		"-d", opts.Database,
		"--no-owner",
		"--no-privileges",
	}

	if opts.CleanTarget {
		args = append(args, "--clean")
	}

	cmd := exec.CommandContext(ctx, "pg_restore", args...)
	if opts.Password != "" {
		cmd.Env = append(cmd.Environ(), fmt.Sprintf("PGPASSWORD=%s", opts.Password))
	}

	if _, err := exec.LookPath("pg_restore"); err != nil {
		return fmt.Errorf("%w: pg_restore", errors.ErrBinaryNotFound)
	}

	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return errors.Wrap(err, "creating stdin pipe for pg_restore")
	}

	if err := cmd.Start(); err != nil {
		return errors.Wrap(err, "starting pg_restore")
	}

	if _, err := io.Copy(stdinPipe, reader); err != nil {
		return errors.Wrap(err, "streaming restore data to pg_restore")
	}
	_ = stdinPipe.Close()

	if err := cmd.Wait(); err != nil {
		return errors.Wrap(err, "pg_restore execution failed")
	}

	return nil
}


