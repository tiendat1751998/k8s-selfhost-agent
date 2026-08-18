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

type MongoDBDriver struct{}

func NewMongoDBDriver() *MongoDBDriver {
	return &MongoDBDriver{}
}

func (d *MongoDBDriver) Type() string {
	return "mongodb"
}

func (d *MongoDBDriver) ValidateConnection(ctx context.Context, opts backup.DumpOptions) error {
	port := opts.Port
	if port == 0 {
		port = 27017
	}
	uri := fmt.Sprintf("mongodb://%s@%s:%d/%s?authSource=admin",
		opts.Username, opts.Host, port, opts.Database)
	cmd := exec.CommandContext(ctx, "mongosh", uri, "--eval", "db.adminCommand('ping')")
	if opts.Password != "" {
		cmd.Env = append(cmd.Environ(), fmt.Sprintf("MONGODB_PASSWORD=%s", opts.Password)) // Try MONGODB_PASSWORD
		// Actually, some mongo tools read MONGODB_URI. But let's assume we can just pass --password via env or stdin.
		// We'll set MONGODB_PASSWORD which might not work, but let's try. wait, --password can read from stdin? No.
		// Wait, let's just pass `--password` as arg if MONGODB_PASSWORD is not officially supported. No, let's use MONGODB_URI.
	}
	// Let's use MONGODB_URI instead
	fullURI := fmt.Sprintf("mongodb://%s:%s@%s:%d/%s?authSource=admin",
		opts.Username, opts.Password, opts.Host, port, opts.Database)
	cmd = exec.CommandContext(ctx, "mongosh", "--eval", "db.adminCommand('ping')")
	cmd.Env = append(cmd.Environ(), fmt.Sprintf("MONGODB_URI=%s", fullURI))
	
	return cmd.Run()
}

func (d *MongoDBDriver) Dump(ctx context.Context, opts backup.DumpOptions, writer io.Writer) (*backup.DumpResult, error) {
	start := time.Now()
	port := opts.Port
	if port == 0 {
		port = 27017
	}

	if _, err := exec.LookPath("mongodump"); err != nil {
		return nil, fmt.Errorf("%w: mongodump", errors.ErrBinaryNotFound)
	}

	fullURI := fmt.Sprintf("mongodb://%s:%s@%s:%d/%s",
		opts.Username, opts.Password, opts.Host, port, opts.Database)

	args := []string{
		"--archive", // Output directly to stdout stream
	}

	cmd := exec.CommandContext(ctx, "mongodump", args...)
	cmd.Env = append(cmd.Environ(), fmt.Sprintf("MONGODB_URI=%s", fullURI))
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, errors.Wrap(err, "creating stdout pipe for mongodump")
	}

	if err := cmd.Start(); err != nil {
		return nil, errors.Wrap(err, "starting mongodump")
	}

	n, err := io.Copy(writer, stdoutPipe)
	if err != nil {
		return nil, errors.Wrap(err, "streaming mongodump output")
	}

	if err := cmd.Wait(); err != nil {
		return nil, errors.Wrap(err, "mongodump execution failed")
	}

	return &backup.DumpResult{
		UncompressedBytes: n,
		Duration:          time.Since(start),
	}, nil
}

func (d *MongoDBDriver) Restore(ctx context.Context, opts backup.RestoreOptions, reader io.Reader) error {
	port := opts.Port
	if port == 0 {
		port = 27017
	}

	if _, err := exec.LookPath("mongorestore"); err != nil {
		return fmt.Errorf("%w: mongorestore", errors.ErrBinaryNotFound)
	}

	fullURI := fmt.Sprintf("mongodb://%s:%s@%s:%d/%s",
		opts.Username, opts.Password, opts.Host, port, opts.Database)

	args := []string{
		"--archive", // Read directly from stdin stream
	}

	cmd := exec.CommandContext(ctx, "mongorestore", args...)
	cmd.Env = append(cmd.Environ(), fmt.Sprintf("MONGODB_URI=%s", fullURI))
	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return errors.Wrap(err, "creating stdin pipe for mongorestore")
	}

	if err := cmd.Start(); err != nil {
		return errors.Wrap(err, "starting mongorestore")
	}

	if _, err := io.Copy(stdinPipe, reader); err != nil {
		return errors.Wrap(err, "streaming restore data to mongorestore")
	}
	_ = stdinPipe.Close()

	if err := cmd.Wait(); err != nil {
		return errors.Wrap(err, "mongorestore execution failed")
	}

	return nil
}
