package iac

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/datdt/k8sselfhost/internal/pkg/errors"
	"github.com/datdt/k8sselfhost/internal/pkg/syncutil"
)

const DefaultIaCTimeout = 15 * time.Minute

var defaultIaCLocks = syncutil.NewKeyedMutex()

// IaCLocks returns the KeyedMutex used for synchronization across IaC runners.
func IaCLocks() *syncutil.KeyedMutex {
	return defaultIaCLocks
}

func normalizeWorkDir(dir string) (string, error) {
	if strings.TrimSpace(dir) == "" {
		dir = "."
	}
	cleaned := filepath.Clean(dir)
	if strings.Contains(dir, "..") || strings.HasPrefix(cleaned, "..") {
		return "", fmt.Errorf("directory traversal rejected: %s", dir)
	}
	abs, err := filepath.Abs(cleaned)
	if err != nil {
		return "", fmt.Errorf("resolving absolute path: %w", err)
	}
	return abs, nil
}

type TerraformRunOptions struct {
	WorkingDir    string
	Vars          map[string]string
	AutoApprove   bool
	BackendConfig map[string]string
}

type TerraformResult struct {
	Command  string
	Success  bool
	Output   string
	Duration time.Duration
}

type TerraformRunner struct {
	binaryPath     string
	defaultTimeout time.Duration
	locks          *syncutil.KeyedMutex
}

func NewTerraformRunner(binaryPath string, defaultTimeout ...time.Duration) *TerraformRunner {
	if binaryPath == "" {
		// Prefer opentofu or terraform
		if _, err := exec.LookPath("tofu"); err == nil {
			binaryPath = "tofu"
		} else {
			binaryPath = "terraform"
		}
	}
	timeout := DefaultIaCTimeout
	if len(defaultTimeout) > 0 && defaultTimeout[0] > 0 {
		timeout = defaultTimeout[0]
	}
	return &TerraformRunner{
		binaryPath:     binaryPath,
		defaultTimeout: timeout,
		locks:          defaultIaCLocks,
	}
}

func (r *TerraformRunner) SetDefaultTimeout(d time.Duration) {
	r.defaultTimeout = d
}

func (r *TerraformRunner) DefaultTimeout() time.Duration {
	return r.defaultTimeout
}

func (r *TerraformRunner) Init(ctx context.Context, opts TerraformRunOptions, outStream io.Writer) (*TerraformResult, error) {
	start := time.Now()
	args := []string{"init", "-no-color"}
	for k, v := range opts.BackendConfig {
		args = append(args, fmt.Sprintf("-backend-config=%s=%s", k, v))
	}

	return r.execute(ctx, opts.WorkingDir, args, outStream, start)
}

func (r *TerraformRunner) Plan(ctx context.Context, opts TerraformRunOptions, outStream io.Writer) (*TerraformResult, error) {
	start := time.Now()
	args := []string{"plan", "-no-color"}
	for k, v := range opts.Vars {
		args = append(args, "-var", fmt.Sprintf("%s=%s", k, v))
	}

	return r.execute(ctx, opts.WorkingDir, args, outStream, start)
}

func (r *TerraformRunner) Apply(ctx context.Context, opts TerraformRunOptions, outStream io.Writer) (*TerraformResult, error) {
	start := time.Now()
	args := []string{"apply", "-no-color"}
	if opts.AutoApprove {
		args = append(args, "-auto-approve")
	}
	for k, v := range opts.Vars {
		args = append(args, "-var", fmt.Sprintf("%s=%s", k, v))
	}

	return r.execute(ctx, opts.WorkingDir, args, outStream, start)
}

func (r *TerraformRunner) Destroy(ctx context.Context, opts TerraformRunOptions, outStream io.Writer) (*TerraformResult, error) {
	start := time.Now()
	args := []string{"destroy", "-no-color"}
	if opts.AutoApprove {
		args = append(args, "-auto-approve")
	}
	for k, v := range opts.Vars {
		args = append(args, "-var", fmt.Sprintf("%s=%s", k, v))
	}

	return r.execute(ctx, opts.WorkingDir, args, outStream, start)
}

func (r *TerraformRunner) execute(ctx context.Context, dir string, args []string, outStream io.Writer, start time.Time) (*TerraformResult, error) {
	workDir, err := normalizeWorkDir(dir)
	if err != nil {
		return nil, err
	}

	locks := r.locks
	if locks == nil {
		locks = defaultIaCLocks
	}
	if !locks.TryLock(workDir) {
		return nil, fmt.Errorf("%w: terraform operation in progress for %s", syncutil.ErrOperationInProgress, workDir)
	}
	defer locks.Unlock(workDir)

	timeout := r.defaultTimeout
	if timeout <= 0 {
		timeout = DefaultIaCTimeout
	}
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, r.binaryPath, args...)
	if dir != "" {
		cmd.Dir = workDir
	}

	var buf bytes.Buffer
	var writer io.Writer = &buf
	if outStream != nil {
		writer = io.MultiWriter(&buf, outStream)
	}

	cmd.Stdout = writer
	cmd.Stderr = writer

	if err := cmd.Start(); err != nil {
		return &TerraformResult{
			Command:  fmt.Sprintf("%s %v", r.binaryPath, args),
			Success:  false,
			Output:   buf.String(),
			Duration: time.Since(start),
		}, errors.Wrap(err, fmt.Sprintf("failed to start terraform binary '%s'", r.binaryPath))
	}

	err = cmd.Wait()
	res := &TerraformResult{
		Command:  fmt.Sprintf("%s %v", r.binaryPath, args),
		Success:  err == nil,
		Output:   buf.String(),
		Duration: time.Since(start),
	}

	if err != nil {
		return res, errors.Wrap(err, fmt.Sprintf("terraform command '%v' failed", args))
	}
	return res, nil
}

