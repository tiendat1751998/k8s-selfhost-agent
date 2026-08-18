package drivers

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"time"

	"github.com/datdt/k8sselfhost/internal/domain/backup"
	"github.com/datdt/k8sselfhost/internal/pkg/errors"
)

type MySQLDriver struct{}

func NewMySQLDriver() *MySQLDriver {
	return &MySQLDriver{}
}

func (d *MySQLDriver) Type() string {
	return "mysql"
}

func (d *MySQLDriver) ValidateConnection(ctx context.Context, opts backup.DumpOptions) error {
	port := opts.Port
	if port == 0 {
		port = 3306
	}
	cmd := exec.CommandContext(ctx, "mysqladmin",
		"-h", opts.Host,
		"-P", fmt.Sprintf("%d", port),
		"-u", opts.Username,
		"ping",
	)
	if opts.Password != "" {
		cmd.Env = append(cmd.Environ(), fmt.Sprintf("MYSQL_PWD=%s", opts.Password))
	}
	return cmd.Run()
}

func (d *MySQLDriver) Dump(ctx context.Context, opts backup.DumpOptions, writer io.Writer) (*backup.DumpResult, error) {
	start := time.Now()
	port := opts.Port
	if port == 0 {
		port = 3306
	}

	args := []string{
		"-h", opts.Host,
		"-P", fmt.Sprintf("%d", port),
		"-u", opts.Username,
		"--single-transaction",
		"--quick",
		opts.Database,
	}

	if _, err := exec.LookPath("mysqldump"); err != nil {
		return nil, fmt.Errorf("%w: mysqldump", errors.ErrBinaryNotFound)
	}

	cmd := exec.CommandContext(ctx, "mysqldump", args...)
	if opts.Password != "" {
		cmd.Env = append(cmd.Environ(), fmt.Sprintf("MYSQL_PWD=%s", opts.Password))
	}

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, errors.Wrap(err, "creating stdout pipe for mysqldump")
	}

	if err := cmd.Start(); err != nil {
		return nil, errors.Wrap(err, "starting mysqldump")
	}

	n, err := io.Copy(writer, stdoutPipe)
	if err != nil {
		return nil, errors.Wrap(err, "streaming mysqldump output")
	}

	if err := cmd.Wait(); err != nil {
		return nil, errors.Wrap(err, "mysqldump execution failed")
	}

	return &backup.DumpResult{
		UncompressedBytes: n,
		Duration:          time.Since(start),
	}, nil
}

func (d *MySQLDriver) Restore(ctx context.Context, opts backup.RestoreOptions, reader io.Reader) error {
	port := opts.Port
	if port == 0 {
		port = 3306
	}

	args := []string{
		"-h", opts.Host,
		"-P", fmt.Sprintf("%d", port),
		"-u", opts.Username,
		opts.Database,
	}

	if _, err := exec.LookPath("mysql"); err != nil {
		return fmt.Errorf("%w: mysql", errors.ErrBinaryNotFound)
	}

	cmd := exec.CommandContext(ctx, "mysql", args...)
	if opts.Password != "" {
		cmd.Env = append(cmd.Environ(), fmt.Sprintf("MYSQL_PWD=%s", opts.Password))
	}

	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return errors.Wrap(err, "creating stdin pipe for mysql restore")
	}

	if err := cmd.Start(); err != nil {
		return errors.Wrap(err, "starting mysql restore")
	}

	if _, err := io.Copy(stdinPipe, reader); err != nil {
		return errors.Wrap(err, "streaming restore to mysql")
	}
	_ = stdinPipe.Close()

	if err := cmd.Wait(); err != nil {
		return errors.Wrap(err, "mysql restore execution failed")
	}

	return nil
}
