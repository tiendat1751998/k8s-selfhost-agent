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

type AnsiblePlaybookOptions struct {
	PlaybookFile string
	Inventory    string
	Limit        string
	ExtraVars    map[string]interface{}
	PrivateKey   string
	User         string
	Become       bool
	WorkingDir   string
}

type AnsibleResult struct {
	Playbook string
	Success  bool
	Output   string
	Duration time.Duration
}

type AnsibleRunner struct {
	binaryPath     string
	defaultTimeout time.Duration
	baseDir        string
	locks          *syncutil.KeyedMutex
}

func NewAnsibleRunner(binaryPath string, defaultTimeout ...time.Duration) *AnsibleRunner {
	if binaryPath == "" {
		binaryPath = "ansible-playbook"
	}
	timeout := DefaultIaCTimeout
	if len(defaultTimeout) > 0 && defaultTimeout[0] > 0 {
		timeout = defaultTimeout[0]
	}
	return &AnsibleRunner{
		binaryPath:     binaryPath,
		defaultTimeout: timeout,
		locks:          defaultIaCLocks,
	}
}

func (r *AnsibleRunner) SetDefaultTimeout(d time.Duration) {
	r.defaultTimeout = d
}

func (r *AnsibleRunner) DefaultTimeout() time.Duration {
	return r.defaultTimeout
}

func (r *AnsibleRunner) SetBaseDir(baseDir string) {
	r.baseDir = baseDir
}

func (r *AnsibleRunner) BaseDir() string {
	return r.baseDir
}

func (r *AnsibleRunner) RunPlaybook(ctx context.Context, opts AnsiblePlaybookOptions, outStream io.Writer) (*AnsibleResult, error) {
	// SEC-12: Input validation against path traversal and flag injection
	trimmedPlaybook := strings.TrimSpace(opts.PlaybookFile)
	if trimmedPlaybook == "" {
		return nil, errors.NewValidation("playbook_file", "playbook file cannot be empty")
	}
	if strings.Contains(opts.PlaybookFile, "..") || strings.HasPrefix(trimmedPlaybook, "-") {
		return nil, errors.NewValidation("playbook_file", "invalid playbook file path")
	}
	if opts.Limit != "" && strings.HasPrefix(strings.TrimSpace(opts.Limit), "-") {
		return nil, errors.NewValidation("limit", "invalid limit parameter: flag injection detected")
	}
	if opts.User != "" && strings.HasPrefix(strings.TrimSpace(opts.User), "-") {
		return nil, errors.NewValidation("user", "invalid user parameter: flag injection detected")
	}

	start := time.Now()

	dir := opts.WorkingDir
	if dir == "" {
		if opts.PlaybookFile != "" {
			dir = filepath.Dir(opts.PlaybookFile)
		} else {
			dir = "."
		}
	}
	workDir, err := normalizeWorkDir(dir, r.baseDir)
	if err != nil {
		return nil, err
	}

	locks := r.locks
	if locks == nil {
		locks = defaultIaCLocks
	}
	if !locks.TryLock(workDir) {
		return nil, fmt.Errorf("%w: ansible operation in progress for %s", syncutil.ErrOperationInProgress, workDir)
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

	var args []string
	if opts.Inventory != "" {
		args = append(args, "-i", opts.Inventory)
	}
	if opts.Limit != "" {
		args = append(args, "--limit", opts.Limit)
	}
	if opts.User != "" {
		args = append(args, "-u", opts.User)
	}
	if opts.Become {
		args = append(args, "--become")
	}
	if opts.PrivateKey != "" {
		args = append(args, "--private-key", opts.PrivateKey)
	}
	args = append(args, "--", opts.PlaybookFile)

	cmd := exec.CommandContext(ctx, r.binaryPath, args...)
	if opts.WorkingDir != "" {
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
		return &AnsibleResult{
			Playbook: opts.PlaybookFile,
			Success:  false,
			Output:   buf.String(),
			Duration: time.Since(start),
		}, errors.Wrap(err, fmt.Sprintf("failed to start ansible-playbook binary '%s'", r.binaryPath))
	}

	err = cmd.Wait()
	res := &AnsibleResult{
		Playbook: opts.PlaybookFile,
		Success:  err == nil,
		Output:   buf.String(),
		Duration: time.Since(start),
	}

	if err != nil {
		return res, errors.Wrap(err, fmt.Sprintf("ansible-playbook '%s' failed", opts.PlaybookFile))
	}
	return res, nil
}

