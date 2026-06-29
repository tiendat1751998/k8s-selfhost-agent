// Package gitops provides the GitOps controller for automated PR creation.
package gitops

import (
	"context"
	"fmt"

	"go.uber.org/zap"

	domainGitops "github.com/datdt/k8sselfhost/internal/domain/gitops"
	"github.com/datdt/k8sselfhost/internal/domain/incident"
	"github.com/datdt/k8sselfhost/internal/domain/report"
	"github.com/datdt/k8sselfhost/pkg/logger"
)

// GitProvider defines the interface for interacting with Git hosting providers.
type GitProvider interface {
	// CreateBranch creates a new branch from the base branch.
	CreateBranch(ctx context.Context, repo, branch, baseBranch string) error
	// CommitFiles commits files to the given branch.
	CommitFiles(ctx context.Context, repo, branch, message string, files []FileCommit) (string, error)
	// CreatePullRequest creates a pull request and returns the URL and PR number.
	CreatePullRequest(ctx context.Context, repo, branch, baseBranch, title, body string) (string, int, error)
	// Name returns the provider name.
	Name() string
}

// FileCommit represents a file to be committed.
type FileCommit struct {
	Path    string
	Content string
}

// Controller orchestrates the GitOps flow: create branch → commit → open PR.
type Controller struct {
	providers  map[domainGitops.Provider]GitProvider
	prRepo     domainGitops.Repository
	incRepo    incident.Repository
}

// NewController creates a new GitOps controller with the given providers.
func NewController(prRepo domainGitops.Repository, incRepo incident.Repository) *Controller {
	return &Controller{
		providers: make(map[domainGitops.Provider]GitProvider),
		prRepo:    prRepo,
		incRepo:   incRepo,
	}
}

// RegisterProvider registers a Git provider implementation.
func (c *Controller) RegisterProvider(providerType domainGitops.Provider, provider GitProvider) {
	c.providers[providerType] = provider
}

// CreateRemediationPR creates a GitOps PR with the remediation changes from an RCA report.
func (c *Controller) CreateRemediationPR(ctx context.Context, inc *incident.Incident, rpt *report.Report, provider domainGitops.Provider, repoURL, baseBranch string, files []FileCommit) (*domainGitops.PullRequest, error) {
	log := logger.WithContext(ctx)

	gitProvider, ok := c.providers[provider]
	if !ok {
		return nil, fmt.Errorf("unsupported git provider: %s", provider)
	}

	// Create domain PR
	branch := fmt.Sprintf("fix/incident-%s", inc.ID[:8])
	title := fmt.Sprintf("fix(%s): %s - %s", inc.Namespace, inc.Type, truncateStr(inc.Message, 60))
	body := buildPRBody(inc, rpt)

	pr, err := domainGitops.New(inc.ID, provider, repoURL, branch, baseBranch, title, body)
	if err != nil {
		return nil, fmt.Errorf("creating PR entity: %w", err)
	}

	for _, f := range files {
		pr.AddFileChange(f.Path, f.Content, domainGitops.FileActionModify)
	}

	// Persist PR record
	if err := c.prRepo.Create(ctx, pr); err != nil {
		return nil, fmt.Errorf("persisting PR record: %w", err)
	}

	log.Info("creating remediation PR",
		zap.String("incident_id", inc.ID),
		zap.String("provider", gitProvider.Name()),
		zap.String("branch", branch),
	)

	// Step 1: Create branch
	if err := gitProvider.CreateBranch(ctx, repoURL, branch, baseBranch); err != nil {
		pr.MarkFailed()
		_ = c.prRepo.Update(ctx, pr)
		return nil, fmt.Errorf("creating branch: %w", err)
	}

	// Step 2: Commit files
	commitMsg := fmt.Sprintf("fix(%s): auto-remediation for %s incident\n\nIncident: %s\nConfidence: %.0f%%\nRisk: %s",
		inc.Namespace, inc.Type, inc.ID, rpt.Confidence*100, rpt.RiskLevel)

	commitSHA, err := gitProvider.CommitFiles(ctx, repoURL, branch, commitMsg, files)
	if err != nil {
		pr.MarkFailed()
		_ = c.prRepo.Update(ctx, pr)
		return nil, fmt.Errorf("committing files: %w", err)
	}

	log.Info("committed remediation files",
		zap.String("commit", commitSHA),
		zap.Int("files", len(files)),
	)

	// Step 3: Create PR
	prURL, prNumber, err := gitProvider.CreatePullRequest(ctx, repoURL, branch, baseBranch, title, body)
	if err != nil {
		pr.MarkFailed()
		_ = c.prRepo.Update(ctx, pr)
		return nil, fmt.Errorf("creating pull request: %w", err)
	}

	// Update PR to open
	if err := pr.MarkOpen(prURL, prNumber); err != nil {
		return nil, fmt.Errorf("marking PR as open: %w", err)
	}
	if err := c.prRepo.Update(ctx, pr); err != nil {
		return nil, fmt.Errorf("updating PR record: %w", err)
	}

	// Update incident to remediating
	if err := inc.MarkRemediating(); err == nil {
		_ = c.incRepo.Update(ctx, inc)
	}

	log.Info("remediation PR created",
		zap.String("pr_url", prURL),
		zap.Int("pr_number", prNumber),
	)

	return pr, nil
}

func buildPRBody(inc *incident.Incident, rpt *report.Report) string {
	return fmt.Sprintf(`## 🔧 Auto-Remediation PR

### Incident Details
| Field | Value |
|-------|-------|
| **ID** | %s |
| **Type** | %s |
| **Namespace** | %s |
| **Pod** | %s |
| **Severity** | %s |

### Root Cause Analysis
**Root Cause**: %s

**Confidence**: %.0f%%

**Risk Level**: %s

### Evidence
%s

### Remediation
%s

### Rollback Plan
%s

---
*This PR was automatically generated by the K8S Self-Healing Agent.*`,
		inc.ID, inc.Type, inc.Namespace, inc.PodName, inc.Severity,
		rpt.RootCause, rpt.Confidence*100, rpt.RiskLevel,
		formatEvidence(rpt.Evidence), rpt.Remediation, rpt.RollbackPlan,
	)
}

func formatEvidence(evidence []string) string {
	result := ""
	for _, e := range evidence {
		result += fmt.Sprintf("- %s\n", e)
	}
	return result
}

func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
