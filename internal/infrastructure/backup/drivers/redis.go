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

type RedisDriver struct{}

func NewRedisDriver() *RedisDriver {
	return &RedisDriver{}
}

func (d *RedisDriver) Type() string {
	return "redis"
}

func (d *RedisDriver) ValidateConnection(ctx context.Context, opts backup.DumpOptions) error {
	port := opts.Port
	if port == 0 {
		port = 6379
	}
	args := []string{"-h", opts.Host, "-p", fmt.Sprintf("%d", port)}
	if opts.Password != "" {
		args = append(args, "-a", opts.Password)
	}
	args = append(args, "PING")

	cmd := exec.CommandContext(ctx, "redis-cli", args...)
	return cmd.Run()
}

func (d *RedisDriver) Dump(ctx context.Context, opts backup.DumpOptions, writer io.Writer) (*backup.DumpResult, error) {
	start := time.Now()
	port := opts.Port
	if port == 0 {
		port = 6379
	}
	if _, err := exec.LookPath("redis-cli"); err != nil {
		return nil, fmt.Errorf("%w: redis-cli", errors.ErrInternal) // Fallback to ErrInternal since ErrBinaryNotFound is in errors package
	}

	args := []string{"-h", opts.Host, "-p", fmt.Sprintf("%d", port)}
	if opts.Password != "" {
		args = append(args, "-a", opts.Password)
	}
	args = append(args, "--rdb", "/dev/stdout")

	cmd := exec.CommandContext(ctx, "redis-cli", args...)

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, errors.Wrap(err, "creating stdout pipe")
	}

	if err := cmd.Start(); err != nil {
		return nil, errors.Wrap(err, "starting redis-cli")
	}

	n, err := io.Copy(writer, stdoutPipe)
	if err != nil {
		return nil, errors.Wrap(err, "streaming redis-cli output")
	}

	if err := cmd.Wait(); err != nil {
		return nil, errors.Wrap(err, "redis-cli execution failed")
	}

	return &backup.DumpResult{
		UncompressedBytes: n,
		Duration:          time.Since(start),
	}, nil
}

func (d *RedisDriver) Restore(ctx context.Context, opts backup.RestoreOptions, reader io.Reader) error {
	port := opts.Port
	if port == 0 {
		port = 6379
	}
	if _, err := exec.LookPath("redis-cli"); err != nil {
		return fmt.Errorf("%w: redis-cli", errors.ErrInternal)
	}

	args := []string{"-h", opts.Host, "-p", fmt.Sprintf("%d", port)}
	if opts.Password != "" {
		args = append(args, "-a", opts.Password)
	}
	// For restore, we can just pipe to redis-cli in mass-insertion mode or just execute script
	args = append(args, "--pipe")

	cmd := exec.CommandContext(ctx, "redis-cli", args...)

	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return errors.Wrap(err, "creating stdin pipe")
	}

	if err := cmd.Start(); err != nil {
		return errors.Wrap(err, "starting redis-cli restore")
	}

	if _, err := io.Copy(stdinPipe, reader); err != nil {
		return errors.Wrap(err, "streaming restore data to redis-cli")
	}
	_ = stdinPipe.Close()

	if err := cmd.Wait(); err != nil {
		return errors.Wrap(err, "redis-cli restore failed")
	}

	return nil
}
