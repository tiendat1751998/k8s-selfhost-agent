// Package gitops defines the GitOps Pull Request aggregate root.
package gitops

import (
	"fmt"
	"time"

	"github.com/datdt/k8sselfhost/internal/pkg/errors"
)

// PullRequest represents a GitOps pull request created for an incident remediation.
type PullRequest struct {
	ID          string
	IncidentID  string
	Provider    Provider
	RepoURL     string
	Branch      string
	BaseBranch  string
	Title       string
	Description string
	PRURL       string
	PRNumber    int
	Status      PRStatus
	FilesChanged []FileChange
	CreatedAt   time.Time
	UpdatedAt   time.Time
	MergedAt    *time.Time
}

// FileChange represents a file modification in the pull request.
type FileChange struct {
	Path    string
	Content string
	Action  FileAction
}

// FileAction represents the type of file modification.
type FileAction string

const (
	FileActionCreate FileAction = "create"
	FileActionModify FileAction = "modify"
	FileActionDelete FileAction = "delete"
)

// Provider represents the Git hosting provider.
type Provider string

const (
	ProviderGitHub Provider = "github"
	ProviderGitLab Provider = "gitlab"
	ProviderGitea  Provider = "gitea"
)

// PRStatus represents the lifecycle state of a pull request.
type PRStatus string

const (
	PRStatusPending  PRStatus = "pending"
	PRStatusOpen     PRStatus = "open"
	PRStatusMerged   PRStatus = "merged"
	PRStatusClosed   PRStatus = "closed"
	PRStatusFailed   PRStatus = "failed"
)

// New creates a new PullRequest with validated fields.
func New(incidentID string, provider Provider, repoURL, branch, baseBranch, title, description string) (*PullRequest, error) {
	if incidentID == "" {
		return nil, errors.NewValidation("incident_id", "must not be empty")
	}
	if repoURL == "" {
		return nil, errors.NewValidation("repo_url", "must not be empty")
	}
	if branch == "" {
		return nil, errors.NewValidation("branch", "must not be empty")
	}
	if title == "" {
		return nil, errors.NewValidation("title", "must not be empty")
	}

	now := time.Now().UTC()
	return &PullRequest{
		IncidentID:  incidentID,
		Provider:    provider,
		RepoURL:     repoURL,
		Branch:      branch,
		BaseBranch:  baseBranch,
		Title:       title,
		Description: description,
		Status:      PRStatusPending,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

// MarkOpen transitions the PR to open status with the given URL and number.
func (pr *PullRequest) MarkOpen(prURL string, prNumber int) error {
	if pr.Status != PRStatusPending {
		return errors.NewConflict("pull_request", fmt.Sprintf("cannot open PR in status %s", pr.Status))
	}
	pr.PRURL = prURL
	pr.PRNumber = prNumber
	pr.Status = PRStatusOpen
	pr.UpdatedAt = time.Now().UTC()
	return nil
}

// MarkMerged transitions the PR to merged status.
func (pr *PullRequest) MarkMerged() error {
	if pr.Status != PRStatusOpen {
		return errors.NewConflict("pull_request", fmt.Sprintf("cannot merge PR in status %s", pr.Status))
	}
	now := time.Now().UTC()
	pr.Status = PRStatusMerged
	pr.UpdatedAt = now
	pr.MergedAt = &now
	return nil
}

// MarkClosed transitions the PR to closed status.
func (pr *PullRequest) MarkClosed() error {
	if pr.Status != PRStatusOpen && pr.Status != PRStatusPending {
		return errors.NewConflict("pull_request", fmt.Sprintf("cannot close PR in status %s", pr.Status))
	}
	pr.Status = PRStatusClosed
	pr.UpdatedAt = time.Now().UTC()
	return nil
}

// MarkFailed transitions the PR to failed status.
func (pr *PullRequest) MarkFailed() {
	pr.Status = PRStatusFailed
	pr.UpdatedAt = time.Now().UTC()
}

// AddFileChange adds a file modification to the pull request.
func (pr *PullRequest) AddFileChange(path, content string, action FileAction) {
	pr.FilesChanged = append(pr.FilesChanged, FileChange{
		Path:    path,
		Content: content,
		Action:  action,
	})
	pr.UpdatedAt = time.Now().UTC()
}
