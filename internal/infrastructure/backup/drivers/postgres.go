package drivers

import (
	"bytes"
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
	// 1. Dry run validation: inspect archive structure without touching target database
	if opts.DryRun {
		header := make([]byte, 5)
		n, err := io.ReadFull(reader, header)
		if err != nil && err != io.ErrUnexpectedEOF {
			return errors.NewValidation("archive_stream", fmt.Sprintf("failed to read archive header: %v", err))
		}
		if n < 2 {
			return errors.NewValidation("archive_stream", "invalid archive stream: header too short")
		}

		isCustomFormat := n >= 5 && bytes.Equal(header[:5], []byte("PGDMP"))
		isSQLText := bytes.HasPrefix(header[:n], []byte("--")) ||
			bytes.HasPrefix(header[:n], []byte("/*")) ||
			bytes.Contains(header[:n], []byte("SET")) ||
			bytes.Contains(header[:n], []byte("CREATE"))

		if !isCustomFormat && !isSQLText {
			return errors.NewValidation("archive_stream", fmt.Sprintf("invalid PostgreSQL archive header: got %q, expected 'PGDMP' or SQL preamble", string(header[:n])))
		}

		// When pg_restore is installed, run pg_restore --list to validate archive TOC without database writes
		if isCustomFormat {
			if _, pathErr := exec.LookPath("pg_restore"); pathErr == nil {
				cmd := exec.CommandContext(ctx, "pg_restore", "--list")
				stdinPipe, pipeErr := cmd.StdinPipe()
				if pipeErr != nil {
					return errors.Wrap(pipeErr, "creating stdin pipe for pg_restore list")
				}
				if startErr := cmd.Start(); startErr != nil {
					return errors.Wrap(startErr, "starting pg_restore list")
				}
				fullReader := io.MultiReader(bytes.NewReader(header[:n]), reader)
				if _, copyErr := io.Copy(stdinPipe, fullReader); copyErr != nil {
					return errors.Wrap(copyErr, "streaming archive to pg_restore list")
				}
				_ = stdinPipe.Close()
				if waitErr := cmd.Wait(); waitErr != nil {
					return errors.Wrap(waitErr, "pg_restore list verification failed")
				}
			}
		}
		return nil
	}

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


