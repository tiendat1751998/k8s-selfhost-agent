// Package gitlab provides the GitLab implementation of the GitProvider interface.
package gitlab

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	gitopsUC "github.com/datdt/k8sselfhost/internal/usecase/gitops"
)

// Provider implements gitops.GitProvider for GitLab.
type Provider struct {
	token      string
	apiURL     string
	httpClient *http.Client
}

// NewProvider creates a new GitLab provider.
func NewProvider(token, apiURL string) *Provider {
	if apiURL == "" {
		apiURL = "https://gitlab.com/api/v4"
	}
	return &Provider{
		token:  token,
		apiURL: strings.TrimRight(apiURL, "/"),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Name returns the provider name.
func (p *Provider) Name() string {
	return "gitlab"
}

// CreateBranch creates a new branch from the base branch.
func (p *Provider) CreateBranch(ctx context.Context, repo, branch, baseBranch string) error {
	projectID := encodeProjectID(repo)
	apiURL := fmt.Sprintf("%s/projects/%s/repository/branches", p.apiURL, projectID)

	body := map[string]string{
		"branch": branch,
		"ref":    baseBranch,
	}

	resp, err := p.doRequest(ctx, http.MethodPost, apiURL, body)
	if err != nil {
		return fmt.Errorf("creating GitLab branch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create branch (status %d): %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// CommitFiles commits files to the given branch using the Commits API.
func (p *Provider) CommitFiles(ctx context.Context, repo, branch, message string, files []gitopsUC.FileCommit) (string, error) {
	projectID := encodeProjectID(repo)
	apiURL := fmt.Sprintf("%s/projects/%s/repository/commits", p.apiURL, projectID)

	actions := make([]map[string]string, 0, len(files))
	for _, f := range files {
		actions = append(actions, map[string]string{
			"action":    "update",
			"file_path": f.Path,
			"content":   f.Content,
		})
	}

	body := map[string]interface{}{
		"branch":         branch,
		"commit_message": message,
		"actions":        actions,
	}

	resp, err := p.doRequest(ctx, http.MethodPost, apiURL, body)
	if err != nil {
		return "", fmt.Errorf("committing to GitLab: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("failed to commit (status %d): %s", resp.StatusCode, string(respBody))
	}

	var commitResp struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&commitResp); err != nil {
		return "", fmt.Errorf("decoding commit response: %w", err)
	}

	return commitResp.ID, nil
}

// CreatePullRequest creates a merge request (GitLab's equivalent of PR).
func (p *Provider) CreatePullRequest(ctx context.Context, repo, branch, baseBranch, title, body string) (string, int, error) {
	projectID := encodeProjectID(repo)
	apiURL := fmt.Sprintf("%s/projects/%s/merge_requests", p.apiURL, projectID)

	reqBody := map[string]string{
		"source_branch": branch,
		"target_branch": baseBranch,
		"title":         title,
		"description":   body,
	}

	resp, err := p.doRequest(ctx, http.MethodPost, apiURL, reqBody)
	if err != nil {
		return "", 0, fmt.Errorf("creating GitLab merge request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return "", 0, fmt.Errorf("failed to create MR (status %d): %s", resp.StatusCode, string(respBody))
	}

	var mrResp struct {
		WebURL string `json:"web_url"`
		IID    int    `json:"iid"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&mrResp); err != nil {
		return "", 0, fmt.Errorf("decoding MR response: %w", err)
	}

	return mrResp.WebURL, mrResp.IID, nil
}

func (p *Provider) doRequest(ctx context.Context, method, apiURL string, body interface{}) (*http.Response, error) {
	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshaling request body: %w", err)
		}
		reqBody = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, apiURL, reqBody)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("PRIVATE-TOKEN", p.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	return p.httpClient.Do(req)
}

func encodeProjectID(repo string) string {
	// GitLab expects URL-encoded project path (e.g., "group%2Fproject")
	repo = strings.TrimPrefix(repo, "https://gitlab.com/")
	repo = strings.TrimPrefix(repo, "http://gitlab.com/")
	repo = strings.TrimSuffix(repo, ".git")
	repo = strings.TrimRight(repo, "/")
	return url.PathEscape(repo)
}
