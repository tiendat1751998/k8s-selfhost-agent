// Package gitea provides the Gitea implementation of the GitProvider interface.
package gitea

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	gitopsUC "github.com/datdt/k8sselfhost/internal/usecase/gitops"
)

// Provider implements gitops.GitProvider for Gitea.
type Provider struct {
	token      string
	apiURL     string
	httpClient *http.Client
}

// NewProvider creates a new Gitea provider.
func NewProvider(token, apiURL string) *Provider {
	return &Provider{
		token:  token,
		apiURL: strings.TrimRight(apiURL, "/") + "/api/v1",
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Name returns the provider name.
func (p *Provider) Name() string {
	return "gitea"
}

// CreateBranch creates a new branch from the base branch.
func (p *Provider) CreateBranch(ctx context.Context, repo, branch, baseBranch string) error {
	owner, repoName := parseGiteaRepo(repo)
	apiURL := fmt.Sprintf("%s/repos/%s/%s/branches", p.apiURL, owner, repoName)

	body := map[string]string{
		"new_branch_name": branch,
		"old_branch_name": baseBranch,
	}

	resp, err := p.doRequest(ctx, http.MethodPost, apiURL, body)
	if err != nil {
		return fmt.Errorf("creating Gitea branch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create branch (status %d): %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// CommitFiles commits files to the given branch using the Contents API.
func (p *Provider) CommitFiles(ctx context.Context, repo, branch, message string, files []gitopsUC.FileCommit) (string, error) {
	owner, repoName := parseGiteaRepo(repo)
	var lastSHA string

	for _, file := range files {
		apiURL := fmt.Sprintf("%s/repos/%s/%s/contents/%s", p.apiURL, owner, repoName, file.Path)

		content := base64.StdEncoding.EncodeToString([]byte(file.Content))

		// Check if file exists
		var existingSHA string
		getResp, err := p.doRequest(ctx, http.MethodGet, apiURL+"?ref="+branch, nil)
		if err == nil {
			defer getResp.Body.Close()
			if getResp.StatusCode == http.StatusOK {
				var existing struct {
					SHA string `json:"sha"`
				}
				if decErr := json.NewDecoder(getResp.Body).Decode(&existing); decErr == nil {
					existingSHA = existing.SHA
				}
			}
		}

		reqBody := map[string]string{
			"message": message,
			"content": content,
			"branch":  branch,
		}
		if existingSHA != "" {
			reqBody["sha"] = existingSHA
		}

		method := http.MethodPost
		if existingSHA != "" {
			method = http.MethodPut
		}

		putResp, putErr := p.doRequest(ctx, method, apiURL, reqBody)
		if putErr != nil {
			return "", fmt.Errorf("committing file %s: %w", file.Path, putErr)
		}
		defer putResp.Body.Close()

		if putResp.StatusCode != http.StatusOK && putResp.StatusCode != http.StatusCreated {
			respBody, _ := io.ReadAll(putResp.Body)
			return "", fmt.Errorf("failed to commit %s (status %d): %s", file.Path, putResp.StatusCode, string(respBody))
		}

		var commitResp struct {
			Content struct {
				SHA string `json:"sha"`
			} `json:"content"`
		}
		if decErr := json.NewDecoder(putResp.Body).Decode(&commitResp); decErr == nil {
			lastSHA = commitResp.Content.SHA
		}
	}

	return lastSHA, nil
}

// CreatePullRequest creates a pull request and returns the URL and PR number.
func (p *Provider) CreatePullRequest(ctx context.Context, repo, branch, baseBranch, title, body string) (string, int, error) {
	owner, repoName := parseGiteaRepo(repo)
	apiURL := fmt.Sprintf("%s/repos/%s/%s/pulls", p.apiURL, owner, repoName)

	reqBody := map[string]string{
		"title": title,
		"body":  body,
		"head":  branch,
		"base":  baseBranch,
	}

	resp, err := p.doRequest(ctx, http.MethodPost, apiURL, reqBody)
	if err != nil {
		return "", 0, fmt.Errorf("creating Gitea pull request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return "", 0, fmt.Errorf("failed to create PR (status %d): %s", resp.StatusCode, string(respBody))
	}

	var prResp struct {
		HTMLURL string `json:"html_url"`
		Number  int    `json:"number"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&prResp); err != nil {
		return "", 0, fmt.Errorf("decoding PR response: %w", err)
	}

	return prResp.HTMLURL, prResp.Number, nil
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

	req.Header.Set("Authorization", "token "+p.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	return p.httpClient.Do(req)
}

func parseGiteaRepo(repo string) (string, string) {
	// Handle various URL formats
	for _, prefix := range []string{"https://", "http://"} {
		if strings.HasPrefix(repo, prefix) {
			repo = repo[len(prefix):]
			// Remove hostname
			if idx := strings.Index(repo, "/"); idx >= 0 {
				repo = repo[idx+1:]
			}
		}
	}
	repo = strings.TrimSuffix(repo, ".git")
	repo = strings.TrimRight(repo, "/")

	parts := strings.SplitN(repo, "/", 2)
	if len(parts) != 2 {
		return repo, ""
	}
	return parts[0], parts[1]
}
