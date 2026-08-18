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

type SQLServerDriver struct {
	driverType string
}

func NewSQLServerDriver(driverType string) *SQLServerDriver {
	if driverType == "" {
		driverType = "sqlserver"
	}
	return &SQLServerDriver{driverType: driverType}
}

func (d *SQLServerDriver) Type() string {
	return d.driverType
}

func (d *SQLServerDriver) ValidateConnection(ctx context.Context, opts backup.DumpOptions) error {
	port := opts.Port
	if port == 0 {
		port = 1433
	}
	server := fmt.Sprintf("%s,%d", opts.Host, port)
	cmd := exec.CommandContext(ctx, "sqlcmd",
		"-S", server,
		"-U", opts.Username,
		"-Q", "SELECT 1;",
		"-b",
	)
	if opts.Password != "" {
		cmd.Env = append(cmd.Environ(), fmt.Sprintf("SQLCMDPASSWORD=%s", opts.Password))
	}
	return cmd.Run()
}

func (d *SQLServerDriver) Dump(ctx context.Context, opts backup.DumpOptions, writer io.Writer) (*backup.DumpResult, error) {
	start := time.Now()
	port := opts.Port
	if port == 0 {
		port = 1433
	}
	server := fmt.Sprintf("%s,%d", opts.Host, port)

	// Stream logical dump script via sqlcmd query or BCP
	query := fmt.Sprintf(`
		SET NOCOUNT ON;
		SELECT '-- MSSQL LOGICAL DUMP FOR [%s]' UNION ALL
		SELECT 'PRINT ''Starting restore for %s'';' UNION ALL
		SELECT TABLE_NAME FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_TYPE = 'BASE TABLE';
	`, opts.Database, opts.Database)

	if _, err := exec.LookPath("sqlcmd"); err != nil {
		return nil, fmt.Errorf("%w: sqlcmd", errors.ErrBinaryNotFound)
	}

	cmd := exec.CommandContext(ctx, "sqlcmd",
		"-S", server,
		"-U", opts.Username,
		"-d", opts.Database,
		"-Q", query,
		"-h", "-1",
		"-s", ",",
		"-W",
	)
	if opts.Password != "" {
		cmd.Env = append(cmd.Environ(), fmt.Sprintf("SQLCMDPASSWORD=%s", opts.Password))
	}

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, errors.Wrap(err, "creating stdout pipe for sqlcmd")
	}

	if err := cmd.Start(); err != nil {
		return nil, errors.Wrap(err, "starting sqlcmd dump")
	}

	n, err := io.Copy(writer, stdoutPipe)
	if err != nil {
		return nil, errors.Wrap(err, "streaming sqlcmd backup output")
	}

	if err := cmd.Wait(); err != nil {
		return nil, errors.Wrap(err, "sqlcmd dump execution failed")
	}

	return &backup.DumpResult{
		UncompressedBytes: n,
		Duration:          time.Since(start),
	}, nil
}

func (d *SQLServerDriver) Restore(ctx context.Context, opts backup.RestoreOptions, reader io.Reader) error {
	port := opts.Port
	if port == 0 {
		port = 1433
	}
	server := fmt.Sprintf("%s,%d", opts.Host, port)

	if _, err := exec.LookPath("sqlcmd"); err != nil {
		return fmt.Errorf("%w: sqlcmd", errors.ErrBinaryNotFound)
	}

	cmd := exec.CommandContext(ctx, "sqlcmd",
		"-S", server,
		"-U", opts.Username,
		"-d", opts.Database,
		"-b",
	)
	if opts.Password != "" {
		cmd.Env = append(cmd.Environ(), fmt.Sprintf("SQLCMDPASSWORD=%s", opts.Password))
	}

	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return errors.Wrap(err, "creating stdin pipe for sqlcmd restore")
	}

	if err := cmd.Start(); err != nil {
		return errors.Wrap(err, "starting sqlcmd restore")
	}

	if _, err := io.Copy(stdinPipe, reader); err != nil {
		return errors.Wrap(err, "streaming restore data to sqlcmd")
	}
	_ = stdinPipe.Close()

	if err := cmd.Wait(); err != nil {
		return errors.Wrap(err, "sqlcmd restore execution failed")
	}

	return nil
}
