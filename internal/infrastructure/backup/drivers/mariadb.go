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

type MariaDBDriver struct{}

func NewMariaDBDriver() *MariaDBDriver {
	return &MariaDBDriver{}
}

func (d *MariaDBDriver) Type() string {
	return "mariadb"
}

func (d *MariaDBDriver) ValidateConnection(ctx context.Context, opts backup.DumpOptions) error {
	port := opts.Port
	if port == 0 {
		port = 3306
	}
	// Try mariadb-admin, fallback to mysqladmin
	cmd := exec.CommandContext(ctx, "mariadb-admin",
		"-h", opts.Host,
		"-P", fmt.Sprintf("%d", port),
		"-u", opts.Username,
		"ping",
	)
	if opts.Password != "" {
		cmd.Env = append(cmd.Environ(), fmt.Sprintf("MYSQL_PWD=%s", opts.Password))
	}
	if err := cmd.Run(); err != nil {
		fallbackCmd := exec.CommandContext(ctx, "mysqladmin",
			"-h", opts.Host,
			"-P", fmt.Sprintf("%d", port),
			"-u", opts.Username,
			"ping",
		)
		if opts.Password != "" {
			fallbackCmd.Env = append(fallbackCmd.Environ(), fmt.Sprintf("MYSQL_PWD=%s", opts.Password))
		}
		return fallbackCmd.Run()
	}
	return nil
}

func (d *MariaDBDriver) Dump(ctx context.Context, opts backup.DumpOptions, writer io.Writer) (*backup.DumpResult, error) {
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

	binary := "mariadb-dump"
	if _, err := exec.LookPath(binary); err != nil {
		binary = "mysqldump"
		if _, err := exec.LookPath(binary); err != nil {
			return nil, fmt.Errorf("%w: %s", errors.ErrBinaryNotFound, binary)
		}
	}

	cmd := exec.CommandContext(ctx, binary, args...)
	if opts.Password != "" {
		cmd.Env = append(cmd.Environ(), fmt.Sprintf("MYSQL_PWD=%s", opts.Password))
	}

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, errors.Wrap(err, "creating stdout pipe for mariadb-dump")
	}

	if err := cmd.Start(); err != nil {
		return nil, errors.Wrap(err, "starting "+binary)
	}

	n, err := io.Copy(writer, stdoutPipe)
	if err != nil {
		return nil, errors.Wrap(err, "streaming mariadb-dump output")
	}

	if err := cmd.Wait(); err != nil {
		return nil, errors.Wrap(err, "mariadb-dump execution failed")
	}

	return &backup.DumpResult{
		UncompressedBytes: n,
		Duration:          time.Since(start),
	}, nil
}

func (d *MariaDBDriver) Restore(ctx context.Context, opts backup.RestoreOptions, reader io.Reader) error {
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

	binary := "mariadb"
	if _, err := exec.LookPath(binary); err != nil {
		binary = "mysql"
		if _, err := exec.LookPath(binary); err != nil {
			return fmt.Errorf("%w: %s", errors.ErrBinaryNotFound, binary)
		}
	}

	cmd := exec.CommandContext(ctx, binary, args...)
	if opts.Password != "" {
		cmd.Env = append(cmd.Environ(), fmt.Sprintf("MYSQL_PWD=%s", opts.Password))
	}

	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return errors.Wrap(err, "creating stdin pipe for mariadb restore")
	}

	if err := cmd.Start(); err != nil {
		return errors.Wrap(err, "starting mariadb restore")
	}

	if _, err := io.Copy(stdinPipe, reader); err != nil {
		return errors.Wrap(err, "streaming restore data to mariadb")
	}
	_ = stdinPipe.Close()

	if err := cmd.Wait(); err != nil {
		return errors.Wrap(err, "mariadb restore execution failed")
	}

	return nil
}
