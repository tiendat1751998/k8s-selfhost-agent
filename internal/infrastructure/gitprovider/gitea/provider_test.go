package gitea

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParseGiteaRepo(t *testing.T) {
	tests := []struct {
		input     string
		wantOwner string
		wantRepo  string
	}{
		{"https://gitea.example.com/org/repo", "org", "repo"},
		{"https://gitea.example.com/org/repo.git", "org", "repo"},
		{"org/repo", "org", "repo"},
	}

	for _, tt := range tests {
		owner, repo := parseGiteaRepo(tt.input)
		if owner != tt.wantOwner || repo != tt.wantRepo {
			t.Errorf("parseGiteaRepo(%q) = (%q, %q), want (%q, %q)", tt.input, owner, repo, tt.wantOwner, tt.wantRepo)
		}
	}
}

func TestMergePullRequest_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST method, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/repos/org/repo/pulls/5/merge" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		var req map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req["Do"] != "merge" {
			t.Errorf("expected Do 'merge', got %v", req["Do"])
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"message":"merged"}`))
	}))
	defer server.Close()

	p := NewProvider("mock-token", server.URL)
	err := p.MergePullRequest(context.Background(), "org/repo", 5)
	if err != nil {
		t.Fatalf("unexpected MergePullRequest error: %v", err)
	}
}

func TestMergePullRequest_Failure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"message":"Merge conflict detected"}`))
	}))
	defer server.Close()

	p := NewProvider("mock-token", server.URL)
	err := p.MergePullRequest(context.Background(), "org/repo", 5)
	if err == nil {
		t.Fatal("expected error on 409 response, got nil")
	}
}
