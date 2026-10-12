package github

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParseRepo_FullURL(t *testing.T) {
	owner, repo := parseRepo("https://github.com/datdt/k8sselfhost")
	if owner != "datdt" {
		t.Errorf("expected owner 'datdt', got '%s'", owner)
	}
	if repo != "k8sselfhost" {
		t.Errorf("expected repo 'k8sselfhost', got '%s'", repo)
	}
}

func TestParseRepo_WithGitSuffix(t *testing.T) {
	owner, repo := parseRepo("https://github.com/datdt/k8sselfhost.git")
	if owner != "datdt" || repo != "k8sselfhost" {
		t.Errorf("unexpected: %s/%s", owner, repo)
	}
}

func TestParseRepo_ShortForm(t *testing.T) {
	owner, repo := parseRepo("datdt/k8sselfhost")
	if owner != "datdt" || repo != "k8sselfhost" {
		t.Errorf("unexpected: %s/%s", owner, repo)
	}
}

func TestParseRepo_TrailingSlash(t *testing.T) {
	owner, repo := parseRepo("https://github.com/datdt/k8sselfhost/")
	if owner != "datdt" || repo != "k8sselfhost" {
		t.Errorf("unexpected: %s/%s", owner, repo)
	}
}

func TestEncodeBase64(t *testing.T) {
	result := encodeBase64("hello world")
	expected := "aGVsbG8gd29ybGQ="
	if result != expected {
		t.Errorf("expected '%s', got '%s'", expected, result)
	}
}

func TestEncodeBase64_Empty(t *testing.T) {
	result := encodeBase64("")
	if result != "" {
		t.Errorf("expected empty, got '%s'", result)
	}
}

func TestNewProvider(t *testing.T) {
	p := NewProvider("test-token")
	if p.Name() != "github" {
		t.Errorf("expected name 'github', got '%s'", p.Name())
	}
	if p.apiURL != "https://api.github.com" {
		t.Errorf("unexpected API URL: %s", p.apiURL)
	}
}

func TestNewProviderWithURL(t *testing.T) {
	p := NewProviderWithURL("token", "https://github.company.com/api/v3/")
	if p.apiURL != "https://github.company.com/api/v3" {
		t.Errorf("unexpected API URL: %s", p.apiURL)
	}
}

func TestMergePullRequest_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("expected PUT method, got %s", r.Method)
		}
		if r.URL.Path != "/repos/owner/myrepo/pulls/42/merge" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		var req map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req["merge_method"] != "merge" {
			t.Errorf("expected merge_method 'merge', got %v", req["merge_method"])
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"sha":"6dcb09b5b57875f334f61aebed695e2e4193db5e","merged":true,"message":"Pull Request successfully merged"}`))
	}))
	defer server.Close()

	p := NewProviderWithURL("mock-token", server.URL)
	err := p.MergePullRequest(context.Background(), "owner/myrepo", 42)
	if err != nil {
		t.Fatalf("unexpected MergePullRequest error: %v", err)
	}
}

func TestMergePullRequest_Failure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusMethodNotAllowed)
		_, _ = w.Write([]byte(`{"message":"Pull Request is not mergeable"}`))
	}))
	defer server.Close()

	p := NewProviderWithURL("mock-token", server.URL)
	err := p.MergePullRequest(context.Background(), "owner/myrepo", 42)
	if err == nil {
		t.Fatal("expected error when GitHub rejects merge, got nil")
	}
}

