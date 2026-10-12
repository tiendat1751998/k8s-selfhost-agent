package gitlab

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestEncodeProjectID(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"https://gitlab.com/owner/project", "owner%2Fproject"},
		{"https://gitlab.com/owner/subgroup/project.git", "owner%2Fsubgroup%2Fproject"},
		{"owner/project", "owner%2Fproject"},
		{"12345", "12345"},
	}

	for _, tt := range tests {
		got := encodeProjectID(tt.input)
		if got != tt.expected {
			t.Errorf("encodeProjectID(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestMergePullRequest_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("expected PUT method, got %s", r.Method)
		}
		if r.URL.EscapedPath() != "/projects/owner%2Fproject/merge_requests/12/merge" {
			t.Errorf("unexpected path: %s", r.URL.EscapedPath())
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":12,"state":"merged"}`))
	}))
	defer server.Close()

	p := NewProvider("mock-token", server.URL)
	err := p.MergePullRequest(context.Background(), "owner/project", 12)
	if err != nil {
		t.Fatalf("unexpected MergePullRequest error: %v", err)
	}
}

func TestMergePullRequest_Failure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusMethodNotAllowed)
		_, _ = w.Write([]byte(`{"message":"405 Method Not Allowed - Branch cannot be merged"}`))
	}))
	defer server.Close()

	p := NewProvider("mock-token", server.URL)
	err := p.MergePullRequest(context.Background(), "owner/project", 12)
	if err == nil {
		t.Fatal("expected error on 405 response, got nil")
	}
}
