package drivers

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
	args := []string{"-h", opts.Host, "-p", fmt.Sprintf("%d", port), "PING"}

	cmd := exec.CommandContext(ctx, "redis-cli", args...)
	if opts.Password != "" {
		cmd.Env = append(os.Environ(), "REDISCLI_AUTH="+opts.Password)
	}
	return cmd.Run()
}

func (d *RedisDriver) Dump(ctx context.Context, opts backup.DumpOptions, writer io.Writer) (*backup.DumpResult, error) {
	start := time.Now()
	port := opts.Port
	if port == 0 {
		port = 6379
	}
	if _, err := exec.LookPath("redis-cli"); err != nil {
		return nil, fmt.Errorf("%w: redis-cli", errors.ErrInternal)
	}

	tempFile, err := os.CreateTemp("", "redis-backup-*.rdb")
	if err != nil {
		return nil, errors.Wrap(err, "creating temp file for redis backup")
	}
	tempPath := tempFile.Name()
	_ = tempFile.Close()
	defer os.Remove(tempPath)

	args := []string{"-h", opts.Host, "-p", fmt.Sprintf("%d", port), "--rdb", tempPath}

	cmd := exec.CommandContext(ctx, "redis-cli", args...)
	if opts.Password != "" {
		cmd.Env = append(os.Environ(), "REDISCLI_AUTH="+opts.Password)
	}
	if err := cmd.Run(); err != nil {
		return nil, errors.Wrap(err, "redis-cli dump execution failed")
	}

	f, err := os.Open(tempPath)
	if err != nil {
		return nil, errors.Wrap(err, "reading dumped redis rdb file")
	}
	defer f.Close()

	n, err := io.Copy(writer, f)
	if err != nil {
		return nil, errors.Wrap(err, "streaming redis rdb backup")
	}

	return &backup.DumpResult{
		UncompressedBytes: n,
		Duration:          time.Since(start),
	}, nil
}

func isForbiddenPath(cleaned string) bool {
	lower := strings.ToLower(filepath.ToSlash(cleaned))
	forbiddenPrefixes := []string{
		"/etc",
		"/root",
		"/bin",
		"/sbin",
		"/usr",
		"/proc",
		"/sys",
		"/dev",
		"c:/windows",
	}
	for _, f := range forbiddenPrefixes {
		if lower == f || strings.HasPrefix(lower, f+"/") {
			return true
		}
	}
	return false
}

func sanitizeRDBPath(raw string) string {
	if strings.Contains(raw, "\x00") || strings.Contains(raw, "..") {
		return ""
	}
	cleaned := filepath.Clean(raw)
	if isForbiddenPath(cleaned) {
		return ""
	}
	return cleaned
}

func (d *RedisDriver) resolveRDBPath(opts backup.RestoreOptions) string {
	if opts.ExtraParams != nil {
		if path := opts.ExtraParams["rdb_path"]; path != "" {
			return sanitizeRDBPath(path)
		}
		if dir := opts.ExtraParams["data_dir"]; dir != "" {
			cleanedDir := sanitizeRDBPath(dir)
			if cleanedDir != "" {
				return filepath.Join(cleanedDir, "dump.rdb")
			}
			return ""
		}
	}
	for _, dir := range []string{"/data", "/var/lib/redis"} {
		if fi, err := os.Stat(dir); err == nil && fi.IsDir() {
			return filepath.Join(dir, "dump.rdb")
		}
	}
	return ""
}

func (d *RedisDriver) Restore(ctx context.Context, opts backup.RestoreOptions, reader io.Reader) error {
	// 1. Dry run validation: inspect RDB magic header
	if opts.DryRun {
		header := make([]byte, 5)
		if _, err := io.ReadFull(reader, header); err != nil {
			return errors.NewValidation("rdb_stream", fmt.Sprintf("invalid RDB stream: %v", err))
		}
		if !bytes.Equal(header, []byte("REDIS")) {
			return errors.NewValidation("rdb_stream", fmt.Sprintf("invalid RDB magic header: got %q, expected 'REDIS'", string(header)))
		}
		return nil
	}

	// 2. Direct RDB file replacement into Redis data directory
	targetPath := d.resolveRDBPath(opts)
	if targetPath == "" {
		// Save to temporary file and provide clear guidance error
		tempFile, err := os.CreateTemp("", "redis-restore-*.rdb")
		if err != nil {
			return errors.Wrap(err, "creating temp file for redis restore")
		}
		tempPath := tempFile.Name()
		defer tempFile.Close()

		if _, err := io.Copy(tempFile, reader); err != nil {
			return errors.Wrap(err, "buffering restore rdb stream")
		}

		return fmt.Errorf("%w: binary RDB restore requires direct file replacement into Redis data directory; restored RDB saved to %s, please specify 'rdb_path' or 'data_dir' in ExtraParams", errors.ErrValidation, tempPath)
	}

	// Ensure destination directory exists
	targetDir := filepath.Dir(targetPath)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return errors.Wrap(err, "creating redis target directory")
	}

	// Write to temporary file first for atomic replacement
	tmpFile, err := os.CreateTemp(targetDir, "dump-*.rdb.tmp")
	if err != nil {
		return errors.Wrap(err, "creating temporary rdb destination file")
	}
	tmpName := tmpFile.Name()
	defer func() {
		_ = tmpFile.Close()
		_ = os.Remove(tmpName)
	}()

	if _, err := io.Copy(tmpFile, reader); err != nil {
		return errors.Wrap(err, "writing rdb stream to temporary file")
	}
	if err := tmpFile.Sync(); err != nil {
		return errors.Wrap(err, "syncing rdb file to disk")
	}
	if err := tmpFile.Close(); err != nil {
		return errors.Wrap(err, "closing temporary rdb file")
	}

	// Atomically replace target dump.rdb
	if err := os.Rename(tmpName, targetPath); err != nil {
		// On Windows, rename over existing file may fail; try removal then rename
		_ = os.Remove(targetPath)
		if err := os.Rename(tmpName, targetPath); err != nil {
			return errors.Wrap(err, "atomically replacing target rdb file")
		}
	}

	return nil
}

