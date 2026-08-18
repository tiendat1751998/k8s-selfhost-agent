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

type OracleDriver struct{}

func NewOracleDriver() *OracleDriver {
	return &OracleDriver{}
}

func (d *OracleDriver) Type() string {
	return "oracle"
}

func (d *OracleDriver) buildConnectIdentifier(opts backup.DumpOptions) string {
	port := opts.Port
	if port == 0 {
		port = 1521
	}
	return fmt.Sprintf("//%s:%d/%s", opts.Host, port, opts.Database)
}

func (d *OracleDriver) ValidateConnection(ctx context.Context, opts backup.DumpOptions) error {
	connIdent := d.buildConnectIdentifier(opts)
	auth := fmt.Sprintf("%s@%s", opts.Username, connIdent)
	cmd := exec.CommandContext(ctx, "sqlplus", "-L", "-S", auth, "exit;")
	if opts.Password != "" {
		cmd.Env = append(cmd.Environ(), fmt.Sprintf("ORACLE_PWD=%s", opts.Password))
	}
	return cmd.Run()
}

func (d *OracleDriver) Dump(ctx context.Context, opts backup.DumpOptions, writer io.Writer) (*backup.DumpResult, error) {
	start := time.Now()
	connIdent := d.buildConnectIdentifier(opts)
	auth := fmt.Sprintf("%s@%s", opts.Username, connIdent)

	if _, err := exec.LookPath("sqlplus"); err != nil {
		return nil, fmt.Errorf("%w: sqlplus", errors.ErrBinaryNotFound)
	}

	// Stream SQL export via sqlplus or expdp Data Pump
	sqlScript := `
SET PAGESIZE 0 LINESIZE 32767 FEEDBACK OFF ECHO OFF TERMOUT OFF TRIMSPOOL ON
SELECT '/* ORACLE DATABASE DUMP */' FROM DUAL;
SELECT DBMS_METADATA.GET_DDL('TABLE', table_name) || ';' FROM user_tables;
EXIT;
`

	cmd := exec.CommandContext(ctx, "sqlplus", "-S", auth)
	if opts.Password != "" {
		cmd.Env = append(cmd.Environ(), fmt.Sprintf("ORACLE_PWD=%s", opts.Password))
	}
	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return nil, errors.Wrap(err, "creating stdin pipe for sqlplus")
	}

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, errors.Wrap(err, "creating stdout pipe for sqlplus")
	}

	if err := cmd.Start(); err != nil {
		return nil, errors.Wrap(err, "starting sqlplus")
	}

	go func() {
		defer stdinPipe.Close()
		_, _ = io.WriteString(stdinPipe, sqlScript)
	}()

	n, err := io.Copy(writer, stdoutPipe)
	if err != nil {
		return nil, errors.Wrap(err, "streaming sqlplus dump output")
	}

	if err := cmd.Wait(); err != nil {
		return nil, errors.Wrap(err, "sqlplus dump execution failed")
	}

	return &backup.DumpResult{
		UncompressedBytes: n,
		Duration:          time.Since(start),
	}, nil
}

func (d *OracleDriver) Restore(ctx context.Context, opts backup.RestoreOptions, reader io.Reader) error {
	port := opts.Port
	if port == 0 {
		port = 1521
	}
	connIdent := fmt.Sprintf("//%s:%d/%s", opts.Host, port, opts.Database)
	auth := fmt.Sprintf("%s@%s", opts.Username, connIdent)

	if _, err := exec.LookPath("sqlplus"); err != nil {
		return fmt.Errorf("%w: sqlplus", errors.ErrBinaryNotFound)
	}

	cmd := exec.CommandContext(ctx, "sqlplus", "-S", auth)
	if opts.Password != "" {
		cmd.Env = append(cmd.Environ(), fmt.Sprintf("ORACLE_PWD=%s", opts.Password))
	}
	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return errors.Wrap(err, "creating stdin pipe for sqlplus restore")
	}

	if err := cmd.Start(); err != nil {
		return errors.Wrap(err, "starting sqlplus restore")
	}

	if _, err := io.Copy(stdinPipe, reader); err != nil {
		return errors.Wrap(err, "streaming restore data to sqlplus")
	}
	_ = stdinPipe.Close()

	if err := cmd.Wait(); err != nil {
		return errors.Wrap(err, "sqlplus restore execution failed")
	}

	return nil
}
