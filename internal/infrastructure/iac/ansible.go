package iac

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"time"

	"github.com/datdt/k8sselfhost/internal/pkg/errors"
)

type AnsiblePlaybookOptions struct {
	PlaybookFile string
	Inventory    string
	Limit        string
	ExtraVars    map[string]interface{}
	PrivateKey   string
	User         string
	Become       bool
}

type AnsibleResult struct {
	Playbook string
	Success  bool
	Output   string
	Duration time.Duration
}

type AnsibleRunner struct {
	binaryPath string
}

func NewAnsibleRunner(binaryPath string) *AnsibleRunner {
	if binaryPath == "" {
		binaryPath = "ansible-playbook"
	}
	return &AnsibleRunner{binaryPath: binaryPath}
}

func (r *AnsibleRunner) RunPlaybook(ctx context.Context, opts AnsiblePlaybookOptions, outStream io.Writer) (*AnsibleResult, error) {
	start := time.Now()

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

	err := cmd.Wait()
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
