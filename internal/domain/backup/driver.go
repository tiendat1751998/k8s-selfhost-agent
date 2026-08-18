package backup

import (
	"context"
	"io"
	"time"
)

// DumpOptions contains database dump parameters
type DumpOptions struct {
	DBType      string
	Host        string
	Port        int
	Database    string
	Username    string
	Password    string
	BackupType  string // full, schema_only, data_only
	Tables      []string
	ExcludeTab  []string
	ExtraParams map[string]string
}

// RestoreOptions contains database restore parameters
type RestoreOptions struct {
	DBType        string
	Host          string
	Port          int
	Database      string
	Username      string
	Password      string
	PITRTimestamp *time.Time
	DryRun        bool
	CleanTarget   bool
	ExtraParams   map[string]string
}

// DumpResult contains metadata produced after a database dump
type DumpResult struct {
	UncompressedBytes int64
	CompressedBytes   int64
	ChecksumSHA256    string
	Duration          time.Duration
	WALStartLSN       string
	WALEndLSN         string
	TablesCount       int
}

// DatabaseDriver defines the contract for database-specific dump and restore engines
type DatabaseDriver interface {
	Type() string
	ValidateConnection(ctx context.Context, opts DumpOptions) error
	Dump(ctx context.Context, opts DumpOptions, writer io.Writer) (*DumpResult, error)
	Restore(ctx context.Context, opts RestoreOptions, reader io.Reader) error
}

// StorageTarget defines the contract for persistent backup storage (Local, S3, MinIO, GCS)
type StorageTarget interface {
	Type() string
	UploadStream(ctx context.Context, path string, reader io.Reader, size int64, metadata map[string]string) (string, error)
	DownloadStream(ctx context.Context, path string) (io.ReadCloser, error)
	Delete(ctx context.Context, path string) error
	Exists(ctx context.Context, path string) (bool, error)
}

// VerificationResult contains post-restore or snapshot verification checks
type VerificationResult struct {
	Passed       bool
	ChecksumOK   bool
	HeaderValid  bool
	RecordsCount int64
	Log          string
	CheckedAt    time.Time
}
