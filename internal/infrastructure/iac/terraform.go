package iac

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

	"github.com/datdt/k8sselfhost/internal/pkg/errors"
	"github.com/datdt/k8sselfhost/internal/pkg/syncutil"
)

const DefaultIaCTimeout = 15 * time.Minute

var defaultIaCLocks = syncutil.NewKeyedMutex()

// DefaultSandboxDir returns the dedicated fallback sandbox directory for IaC workspaces.
func DefaultSandboxDir() string {
	if env := os.Getenv("IAC_WORKSPACE_DIR"); env != "" {
		return env
	}
	return filepath.Join(os.TempDir(), "k8s-iac-workspaces")
}

// IaCLocks returns the KeyedMutex used for synchronization across IaC runners.
func IaCLocks() *syncutil.KeyedMutex {
	return defaultIaCLocks
}

var forbiddenSystemPrefixes = []string{
	"/etc", "/root", "/proc", "/sys", "/boot", "/dev", "/var/run", "/usr", "/bin", "/sbin",
	`c:/windows`, `c:/system32`,
}

// NormalizeWorkDir validates, cleans, and sandboxes an IaC runner's working directory.
// It rejects traversal (".."), forbidden system directories, and enforces containment
// within baseDir (which defaults to DefaultSandboxDir to prevent containment bypass).
func NormalizeWorkDir(dir string, baseDir ...string) (string, error) {
	return normalizeWorkDir(dir, baseDir...)
}

func normalizeWorkDir(dir string, baseDir ...string) (string, error) {
	if strings.TrimSpace(dir) == "" {
		dir = "."
	}
	cleaned := filepath.Clean(dir)
	if strings.Contains(dir, "..") || strings.HasPrefix(cleaned, "..") {
		return "", fmt.Errorf("directory traversal rejected: %s", dir)
	}

	cleanSlash := strings.ToLower(filepath.ToSlash(cleaned))
	for _, prefix := range forbiddenSystemPrefixes {
		pLower := strings.ToLower(filepath.ToSlash(prefix))
		if cleanSlash == pLower || strings.HasPrefix(cleanSlash, pLower+"/") {
			return "", fmt.Errorf("access to forbidden system directory rejected: %s", dir)
		}
	}

	// SEC-CRIT-02: NEVER allow empty baseDir to bypass path containment! Default to dedicated sandbox.
	effectiveBase := DefaultSandboxDir()
	if len(baseDir) > 0 && strings.TrimSpace(baseDir[0]) != "" {
		effectiveBase = strings.TrimSpace(baseDir[0])
	}

	baseClean := filepath.Clean(effectiveBase)
	absBase, err := filepath.Abs(baseClean)
	if err != nil {
		return "", fmt.Errorf("resolving base directory: %w", err)
	}

	var abs string
	if !filepath.IsAbs(cleaned) {
		abs = filepath.Join(absBase, cleaned)
	} else {
		abs, err = filepath.Abs(cleaned)
		if err != nil {
			return "", fmt.Errorf("resolving absolute path: %w", err)
		}
	}

	absSlash := strings.ToLower(filepath.ToSlash(abs))
	for _, prefix := range forbiddenSystemPrefixes {
		pLower := strings.ToLower(filepath.ToSlash(prefix))
		if absSlash == pLower || strings.HasPrefix(absSlash, pLower+"/") {
			return "", fmt.Errorf("access to forbidden system directory rejected: %s", dir)
		}
		if len(absSlash) >= 2 && absSlash[1] == ':' {
			driveStripped := absSlash[2:]
			if driveStripped == pLower || strings.HasPrefix(driveStripped, pLower+"/") {
				return "", fmt.Errorf("access to forbidden system directory rejected: %s", dir)
			}
		}
	}

	rel, err := filepath.Rel(absBase, abs)
	if err != nil || strings.HasPrefix(rel, "..") || rel == ".." || filepath.IsAbs(rel) {
		return "", fmt.Errorf("directory %s is outside allowed base directory %s", dir, effectiveBase)
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
	baseDir        string
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
		baseDir:        DefaultSandboxDir(),
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

func (r *TerraformRunner) SetBaseDir(baseDir string) {
	if strings.TrimSpace(baseDir) == "" {
		r.baseDir = DefaultSandboxDir()
	} else {
		r.baseDir = baseDir
	}
}

func (r *TerraformRunner) BaseDir() string {
	if strings.TrimSpace(r.baseDir) == "" {
		return DefaultSandboxDir()
	}
	return r.baseDir
}

func (r *TerraformRunner) execute(ctx context.Context, dir string, args []string, outStream io.Writer, start time.Time) (*TerraformResult, error) {
	workDir, err := normalizeWorkDir(dir, r.baseDir)
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

