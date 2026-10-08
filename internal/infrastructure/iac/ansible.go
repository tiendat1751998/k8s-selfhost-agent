package iac

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
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

func (r *AnsibleRunner) RunPlaybook(ctx context.Context, opts AnsiblePlaybookOptions, outStream io.Writer) (*AnsibleResult, error) {
	start := time.Now()

	dir := opts.WorkingDir
	if dir == "" {
		if opts.PlaybookFile != "" {
			dir = filepath.Dir(opts.PlaybookFile)
		} else {
			dir = "."
		}
	}
	workDir, err := normalizeWorkDir(dir)
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

	args := []string{opts.PlaybookFile}
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

