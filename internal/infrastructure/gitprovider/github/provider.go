// Package github provides the GitHub implementation of the GitProvider interface.
package github

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

// Provider implements gitops.GitProvider for GitHub.
type Provider struct {
	token      string
	apiURL     string
	httpClient *http.Client
}

// NewProvider creates a new GitHub provider.
func NewProvider(token string) *Provider {
	return &Provider{
		token:  token,
		apiURL: "https://api.github.com",
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// NewProviderWithURL creates a new GitHub provider with a custom API URL (for GitHub Enterprise).
func NewProviderWithURL(token, apiURL string) *Provider {
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
	return "github"
}

// CreateBranch creates a new branch from the base branch.
func (p *Provider) CreateBranch(ctx context.Context, repo, branch, baseBranch string) error {
	owner, repoName := parseRepo(repo)

	// Get base branch SHA
	refURL := fmt.Sprintf("%s/repos/%s/%s/git/ref/heads/%s", p.apiURL, owner, repoName, baseBranch)
	resp, err := p.doRequest(ctx, http.MethodGet, refURL, nil)
	if err != nil {
		return fmt.Errorf("getting base branch ref: %w", err)
	}
	defer resp.Body.Close()

	var refResp struct {
		Object struct {
			SHA string `json:"sha"`
		} `json:"object"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&refResp); err != nil {
		return fmt.Errorf("decoding base ref response: %w", err)
	}

	// Create new branch
	createRefURL := fmt.Sprintf("%s/repos/%s/%s/git/refs", p.apiURL, owner, repoName)
	body := map[string]string{
		"ref": fmt.Sprintf("refs/heads/%s", branch),
		"sha": refResp.Object.SHA,
	}

	createResp, err := p.doRequest(ctx, http.MethodPost, createRefURL, body)
	if err != nil {
		return fmt.Errorf("creating branch ref: %w", err)
	}
	defer createResp.Body.Close()

	if createResp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(createResp.Body)
		return fmt.Errorf("failed to create branch (status %d): %s", createResp.StatusCode, string(respBody))
	}

	return nil
}

// CommitFiles commits files to the given branch using the Contents API.
func (p *Provider) CommitFiles(ctx context.Context, repo, branch, message string, files []gitopsUC.FileCommit) (string, error) {
	owner, repoName := parseRepo(repo)
	var lastSHA string

	for _, file := range files {
		url := fmt.Sprintf("%s/repos/%s/%s/contents/%s", p.apiURL, owner, repoName, file.Path)

		// Check if file exists to get its SHA
		var existingSHA string
		getResp, err := p.doRequest(ctx, http.MethodGet, url+"?ref="+branch, nil)
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

		body := map[string]string{
			"message": message,
			"content": encodeBase64(file.Content),
			"branch":  branch,
		}
		if existingSHA != "" {
			body["sha"] = existingSHA
		}

		putResp, putErr := p.doRequest(ctx, http.MethodPut, url, body)
		if putErr != nil {
			return "", fmt.Errorf("committing file %s: %w", file.Path, putErr)
		}
		defer putResp.Body.Close()

		if putResp.StatusCode != http.StatusOK && putResp.StatusCode != http.StatusCreated {
			respBody, _ := io.ReadAll(putResp.Body)
			return "", fmt.Errorf("failed to commit %s (status %d): %s", file.Path, putResp.StatusCode, string(respBody))
		}

		var commitResp struct {
			Commit struct {
				SHA string `json:"sha"`
			} `json:"commit"`
		}
		if decErr := json.NewDecoder(putResp.Body).Decode(&commitResp); decErr == nil {
			lastSHA = commitResp.Commit.SHA
		}
	}

	return lastSHA, nil
}

// CreatePullRequest creates a pull request and returns the URL and PR number.
func (p *Provider) CreatePullRequest(ctx context.Context, repo, branch, baseBranch, title, body string) (string, int, error) {
	owner, repoName := parseRepo(repo)

	url := fmt.Sprintf("%s/repos/%s/%s/pulls", p.apiURL, owner, repoName)
	reqBody := map[string]string{
		"title": title,
		"body":  body,
		"head":  branch,
		"base":  baseBranch,
	}

	resp, err := p.doRequest(ctx, http.MethodPost, url, reqBody)
	if err != nil {
		return "", 0, fmt.Errorf("creating pull request: %w", err)
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

func (p *Provider) doRequest(ctx context.Context, method, url string, body interface{}) (*http.Response, error) {
	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshaling request body: %w", err)
		}
		reqBody = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+p.token)
	req.Header.Set("Accept", "application/vnd.github+json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	return p.httpClient.Do(req)
}

// parseRepo extracts owner and repo name from a GitHub URL or "owner/repo" string.
func parseRepo(repo string) (string, string) {
	// Handle full URL
	repo = strings.TrimPrefix(repo, "https://github.com/")
	repo = strings.TrimPrefix(repo, "http://github.com/")
	repo = strings.TrimSuffix(repo, ".git")
	repo = strings.TrimRight(repo, "/")

	parts := strings.SplitN(repo, "/", 2)
	if len(parts) != 2 {
		return repo, ""
	}
	return parts[0], parts[1]
}

func encodeBase64(content string) string {
	return base64.StdEncoding.EncodeToString([]byte(content))
}
